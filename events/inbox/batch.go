package inbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/log"
	"google.golang.org/protobuf/proto"
)

const (
	defaultBatchSize         = 100
	defaultHeartbeatInterval = 10 * time.Second
)

// BatchSource is a transport adapter that yields records in batches. One source
// can carry many feeds, for example one JetStream consumer over a stream family.
type BatchSource interface {
	// Fetch returns up to max records. It can return no records and no error when
	// its wait ends without a record. An error is a transport failure. The router
	// waits and fetches again.
	Fetch(ctx context.Context, max int) ([]BatchRecord, error)
}

// BatchRecord is one raw record from a BatchSource and its commit handles.
type BatchRecord struct {
	// Subject is the transport address: "<full message name>.<tenant>".
	Subject string
	// Payload is the binary-proto <Feed>Event.
	Payload []byte
	// Attempt is the delivery count of the record. The first delivery is 1.
	Attempt int

	Ack func() error
	// Nak asks for a redelivery after delay.
	Nak func(delay time.Duration) error
	// Term stops all redelivery of the record.
	Term func() error
	// InProgress resets the redelivery timer of the transport.
	InProgress func() error
}

// BatchEvent is one decoded event of a batch and its envelope.
type BatchEvent[T proto.Message] struct {
	Event T
	Meta  *commonv1.EventMeta
}

// BatchHandler processes the events of one feed from one fetch. It must be
// idempotent: delivery is at least once. A nil error acks every event.
type BatchHandler[T proto.Message] func(ctx context.Context, batch []BatchEvent[T]) error

type batchConfig struct {
	size              int
	deadLetter        DeadLetterQueue
	retry             retryConfig
	backoffBase       time.Duration
	backoffMax        time.Duration
	fetchRetryDelay   time.Duration
	heartbeatInterval time.Duration
}

// BatchOption configures a BatchRouter.
type BatchOption func(*batchConfig)

// WithBatchSize sets the maximum number of records of one fetch (default 100).
func WithBatchSize(n int) BatchOption {
	return func(c *batchConfig) {
		if n > 0 {
			c.size = n
		}
	}
}

// WithBatchDeadLetter sets the store for records that the router gives up on.
// Without a store the router logs and drops such a record, so that a poison
// record cannot block the consumer.
func WithBatchDeadLetter(dlq DeadLetterQueue) BatchOption {
	return func(c *batchConfig) { c.deadLetter = dlq }
}

// WithBatchRetry sets the delivery budget (WithMaxAttempts, default
// DefaultMaxAttempts) and the permanent error classifier
// (WithPermanentClassifier). Set the MaxDeliver of the transport consumer to at
// least the budget. Otherwise the transport stops a record before the router
// can dead-letter it.
func WithBatchRetry(opts ...RetryOption) BatchOption {
	return func(c *batchConfig) {
		for _, o := range opts {
			o(&c.retry)
		}
	}
}

// WithBatchBackoff sets the redelivery delay of a failed batch: base for the
// first attempt, doubled for each next attempt, up to max (default 1s to 60s).
func WithBatchBackoff(base, max time.Duration) BatchOption {
	return func(c *batchConfig) {
		if base <= 0 {
			return
		}
		if max < base {
			max = base
		}
		c.backoffBase, c.backoffMax = base, max
	}
}

// WithBatchFetchRetryDelay sets the wait after a fetch error (default 5s).
func WithBatchFetchRetryDelay(d time.Duration) BatchOption {
	return func(c *batchConfig) {
		if d > 0 {
			c.fetchRetryDelay = d
		}
	}
}

// WithBatchHeartbeat sets how often the router marks in-flight records as in
// progress while handlers run (default 10s). Keep it well below the AckWait of
// the transport consumer.
func WithBatchHeartbeat(d time.Duration) BatchOption {
	return func(c *batchConfig) {
		if d > 0 {
			c.heartbeatInterval = d
		}
	}
}

// BatchRouter reads batches from one BatchSource and gives the records of each
// feed to the handler of that feed. Each feed of the batch is acked, redelivered
// or dead-lettered on its own, so a failing feed never redelivers the records of
// a feed that succeeded.
//
// Register every feed with Route before Run. Several routers, each in its own
// replica, can share one durable consumer: the transport gives each record to
// one of them.
type BatchRouter struct {
	source BatchSource
	cfg    batchConfig
	routes map[string]batchRoute
}

type batchRoute struct {
	handle func(ctx context.Context, r *BatchRouter, feed string, recs []*inflight)
	replay func(ctx context.Context, feed string, payload []byte) (DeadLetterReason, error)
}

// NewBatchRouter builds a router over source. Run errors when source is nil.
func NewBatchRouter(source BatchSource, opts ...BatchOption) *BatchRouter {
	cfg := batchConfig{
		size:              defaultBatchSize,
		retry:             retryConfig{maxAttempts: DefaultMaxAttempts},
		backoffBase:       defaultRedeliveryBase,
		backoffMax:        defaultRedeliveryMax,
		fetchRetryDelay:   defaultRestartDelay,
		heartbeatInterval: defaultHeartbeatInterval,
	}
	for _, o := range opts {
		o(&cfg)
	}

	return &BatchRouter{source: source, cfg: cfg, routes: map[string]batchRoute{}}
}

// Route registers handler for the feed of T. The route key is the full message
// name of T, which is the subject of a record without its tenant token. Call
// Route before Run, not at the same time.
func Route[T proto.Message](r *BatchRouter, newEvent func() T, handler BatchHandler[T]) error {
	if r == nil {
		return errors.New("inbox: route: router is required")
	}
	if newEvent == nil {
		return errors.New("inbox: route: newEvent constructor is required")
	}
	if handler == nil {
		return errors.New("inbox: route: handler is required")
	}
	probe := newEvent()
	if nilMessage(probe) {
		return errors.New("inbox: route: newEvent must return a non-nil message")
	}

	routing, err := events.RoutingFor(probe)
	if err != nil {
		return fmt.Errorf("inbox: route: %w", err)
	}

	feed := routing.FullName()
	if _, ok := r.routes[feed]; ok {
		return fmt.Errorf("inbox: route: %s already has a handler", feed)
	}

	r.routes[feed] = batchRoute{
		handle: func(ctx context.Context, r *BatchRouter, feed string, recs []*inflight) {
			handleGroup(ctx, r, feed, newEvent, handler, recs)
		},
		replay: func(ctx context.Context, feed string, payload []byte) (DeadLetterReason, error) {
			ev, err := decodeBatchEvent(payload, feed, newEvent)
			if err != nil {
				return DeadLetterReasonDecode, err
			}
			if err := handler(ctx, []BatchEvent[T]{ev}); err != nil {
				return DeadLetterReasonHandler, fmt.Errorf("handler: %w", err)
			}
			return "", nil
		},
	}

	return nil
}

// ConsumeBatch reads one feed in batches until ctx is done. It is a BatchRouter
// with one route.
func ConsumeBatch[T proto.Message](ctx context.Context, source BatchSource, newEvent func() T,
	handler BatchHandler[T], opts ...BatchOption) error {
	r := NewBatchRouter(source, opts...)
	if err := Route(r, newEvent, handler); err != nil {
		return err
	}

	return r.Run(ctx)
}

// Run fetches and routes batches until ctx is done. It returns the context error
// then. A fetch error does not stop it.
//
// For each feed of a batch: a record that does not decode is dead-lettered and
// terminated. If the handler succeeds, every record is acked. If the handler
// fails, every record is redelivered with a backoff by attempt. If the error is
// permanent, or a record is on its last attempt, the router runs the handler
// with each record alone. It acks each record that passes, and dead-letters and
// terminates each record that fails for good. A record whose subject has no
// route is dead-lettered and terminated.
func (r *BatchRouter) Run(ctx context.Context) error {
	if r.source == nil {
		return errors.New("inbox: batch router: source is required")
	}
	if len(r.routes) == 0 {
		return errors.New("inbox: batch router: no routes")
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		recs, err := r.source.Fetch(ctx, r.cfg.size)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			log.Warnf("inbox: batch fetch failed: %v; fetching again in %s", err, r.cfg.fetchRetryDelay)
			if !sleepCtx(ctx, r.cfg.fetchRetryDelay) {
				return ctx.Err()
			}
			continue
		}

		if len(recs) > 0 {
			r.process(ctx, recs)
		}
	}
}

func (r *BatchRouter) process(ctx context.Context, recs []BatchRecord) {
	items := make([]*inflight, len(recs))
	for i := range recs {
		items[i] = &inflight{BatchRecord: recs[i]}
	}

	stop := r.startHeartbeat(items)
	defer stop()

	var order []string
	groups := map[string][]*inflight{}
	for _, it := range items {
		feed := feedOfSubject(it.Subject)
		if _, ok := r.routes[feed]; !ok {
			r.deadLetter(ctx, it, feed, "", DeadLetterReasonNoRoute, fmt.Errorf("no route for subject %q", it.Subject))
			continue
		}
		if _, ok := groups[feed]; !ok {
			order = append(order, feed)
		}
		groups[feed] = append(groups[feed], it)
	}

	for _, feed := range order {
		r.routes[feed].handle(ctx, r, feed, groups[feed])
	}
}

func handleGroup[T proto.Message](ctx context.Context, r *BatchRouter, feed string,
	newEvent func() T, handler BatchHandler[T], recs []*inflight) {
	batch := make([]BatchEvent[T], 0, len(recs))
	live := make([]*inflight, 0, len(recs))
	for _, it := range recs {
		ev, err := decodeBatchEvent(it.Payload, feed, newEvent)
		if err != nil {
			r.deadLetter(ctx, it, feed, "", DeadLetterReasonDecode, err)
			continue
		}
		batch = append(batch, ev)
		live = append(live, it)
	}
	if len(batch) == 0 {
		return
	}

	err := handler(ctx, batch)
	switch {
	case err == nil:
		ackAll(live)
		return
	case ctx.Err() != nil:
		nakAll(live, 0)
		return
	case !r.permanent(err) && !r.anyFinal(live):
		log.Warnf("inbox: batch of %d %s record(s) failed: %v; redelivering", len(live), feed, err)
		for _, it := range live {
			it.nak(r.nakDelay(it.Attempt))
		}
		return
	}

	log.Warnf("inbox: batch of %d %s record(s) failed: %v; running each record alone", len(live), feed, err)
	for i, it := range live {
		if ctx.Err() != nil {
			nakAll(live[i:], 0)
			return
		}

		err := handler(ctx, batch[i:i+1])
		switch {
		case err == nil:
			it.ack()
		case ctx.Err() != nil:
			it.nak(0)
		case r.permanent(err) || r.final(it):
			r.deadLetter(ctx, it, feed, batch[i].Meta.GetEventId(), DeadLetterReasonHandler, fmt.Errorf("handler: %w", err))
		default:
			it.nak(r.nakDelay(it.Attempt))
		}
	}
}

// decodeBatchEvent decodes a record of the feed. The route comes from the
// subject, which the publisher sets. The check of the decoded routing makes
// sure that the router gives a handler only events of its own feed.
func decodeBatchEvent[T proto.Message](payload []byte, feed string, newEvent func() T) (BatchEvent[T], error) {
	event := newEvent()
	if err := proto.Unmarshal(payload, event); err != nil {
		return BatchEvent[T]{}, fmt.Errorf("decode: %w", err)
	}

	routing, err := events.RoutingFor(event)
	if err != nil {
		return BatchEvent[T]{}, fmt.Errorf("routing: %w", err)
	}
	if routing.FullName() != feed {
		return BatchEvent[T]{}, fmt.Errorf("routing: decoded %s on feed %s", routing.FullName(), feed)
	}

	meta, err := events.MetaOf(event)
	if err != nil {
		return BatchEvent[T]{}, fmt.Errorf("envelope: %w", err)
	}

	return BatchEvent[T]{Event: event, Meta: meta}, nil
}

// deadLetter stores the record and terminates it. If the store fails, the record
// is redelivered, so the router never drops a record it could store later.
func (r *BatchRouter) deadLetter(ctx context.Context, it *inflight, feed, eventID string,
	reason DeadLetterReason, cause error) {
	if r.cfg.deadLetter == nil {
		log.Errorf("inbox: dropping record %s of subject %q after %d attempt(s), no dead-letter store: %v",
			shortFingerprint(payloadFingerprint(it.Payload)), it.Subject, it.Attempt, cause)
		it.term()
		return
	}

	err := r.cfg.deadLetter.Store(ctx, DeadLetterRecord{
		Payload:  it.Payload,
		Error:    errString(cause),
		Attempts: it.Attempt,
		EventID:  eventID,
		Feed:     feed,
		Reason:   reason,
	})
	if err != nil {
		log.Warnf("inbox: dead-letter store failed (record will be redelivered): %v", err)
		it.nak(r.nakDelay(it.Attempt))
		return
	}

	log.Warnf("inbox: dead-lettered record %s of subject %q after %d attempt(s): %v",
		shortFingerprint(payloadFingerprint(it.Payload)), it.Subject, it.Attempt, cause)
	it.term()
}

// Replay runs each dead letter that matches f through the handler of its feed,
// as a batch of one event. Register the routes first. Replay deletes a dead
// letter that its handler accepts. A dead letter that fails again stays, and the
// stores of NewGormDeadLetterReader keep the new reason and error and count the
// attempt. A dead letter whose feed has no route stays as it is.
//
// The handler runs at least once per record, as in Run, so it must be
// idempotent. Replay returns a List error. A failure of one record goes to its
// outcome.
func (r *BatchRouter) Replay(ctx context.Context, reader DeadLetterReader, f DeadLetterFilter) (ReplayResult, error) {
	if reader == nil {
		return ReplayResult{}, errors.New("inbox: batch replay: reader is required")
	}
	recorder, _ := reader.(deadLetterFailureRecorder)

	return replayRows(ctx, reader, f, func(ctx context.Context, row DeadLetter) error {
		route, ok := r.routes[row.Feed]
		if !ok {
			return fmt.Errorf("no route for feed %q", row.Feed)
		}

		reason, err := route.replay(ctx, row.Feed, row.Payload)
		if err != nil {
			if recorder != nil {
				if rerr := recorder.recordFailure(ctx, row.ID, reason, err.Error()); rerr != nil {
					return errors.Join(err, rerr)
				}
			}
			return err
		}

		if err := reader.Delete(ctx, row.ID); err != nil {
			return fmt.Errorf("delete after replay: %w", err)
		}
		return nil
	})
}

func (r *BatchRouter) permanent(err error) bool {
	return r.cfg.retry.permanent != nil && r.cfg.retry.permanent(rootCause(err))
}

func (r *BatchRouter) final(it *inflight) bool {
	return it.Attempt >= r.cfg.retry.maxAttempts
}

func (r *BatchRouter) anyFinal(recs []*inflight) bool {
	for _, it := range recs {
		if r.final(it) {
			return true
		}
	}
	return false
}

func (r *BatchRouter) nakDelay(attempt int) time.Duration {
	return attemptBackoff(attempt, r.cfg.backoffBase, r.cfg.backoffMax)
}

// startHeartbeat marks every open record as in progress at each interval, until
// stop. Without it, a slow handler passes the AckWait of the transport, and the
// transport redelivers the batch to another replica while the handler runs.
func (r *BatchRouter) startHeartbeat(items []*inflight) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(r.cfg.heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				for _, it := range items {
					it.touch()
				}
			}
		}
	}()

	return func() {
		close(done)
		wg.Wait()
	}
}

// feedOfSubject returns the subject without its last token, the tenant.
func feedOfSubject(subject string) string {
	i := strings.LastIndexByte(subject, '.')
	if i < 0 {
		return ""
	}
	return subject[:i]
}

// inflight is a record that the router has not acked, redelivered or
// terminated yet. The lock keeps the heartbeat from touching a record after
// the router settles it.
type inflight struct {
	BatchRecord
	mu      sync.Mutex
	settled bool
}

func (it *inflight) settle(action string, f func() error) {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.settled {
		return
	}
	it.settled = true
	if f == nil {
		log.Warnf("inbox: record of subject %q has no %s handle", it.Subject, action)
		return
	}
	if err := f(); err != nil {
		log.Warnf("inbox: %s of record of subject %q failed: %v", action, it.Subject, err)
	}
}

func (it *inflight) ack() { it.settle("ack", it.Ack) }

func (it *inflight) term() { it.settle("term", it.Term) }

func (it *inflight) nak(delay time.Duration) {
	var f func() error
	if it.Nak != nil {
		f = func() error { return it.Nak(delay) }
	}
	it.settle("nak", f)
}

func (it *inflight) touch() {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.settled || it.InProgress == nil {
		return
	}
	if err := it.InProgress(); err != nil {
		log.Warnf("inbox: in-progress of record of subject %q failed: %v", it.Subject, err)
	}
}

func ackAll(recs []*inflight) {
	for _, it := range recs {
		it.ack()
	}
}

func nakAll(recs []*inflight, delay time.Duration) {
	for _, it := range recs {
		it.nak(delay)
	}
}
