package inbox

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
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

// DeliveryLimiter is the optional capability of a BatchSource whose transport
// stops the redelivery of a record after a number of deliveries.
type DeliveryLimiter interface {
	// MaxDeliveries returns the limit, or a value below 1 for no limit.
	MaxDeliveries(ctx context.Context) (int, error)
}

// BatchRecord is one raw record from a BatchSource and its commit handles.
type BatchRecord struct {
	// Subject is the transport address, for diagnostics.
	Subject string
	// Feed is the full message name of the record. It is empty when the source
	// cannot read the feed from the subject.
	Feed string
	// Tenant is the tenant of the subject.
	Tenant string
	// Payload is the binary-proto <Feed>Event.
	Payload []byte
	// Attempt is the delivery count of the record. The first delivery is 1.
	Attempt int

	Ack func() error
	// Nack asks for a redelivery after delay.
	Nack func(delay time.Duration) error
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

// BatchHandler processes the events of one feed from one fetch. A nil error acks
// every event. The router can run it again with each event alone, and delivery
// is at least once, so it must be idempotent.
type BatchHandler[T proto.Message] func(ctx context.Context, batch []BatchEvent[T]) error

type batchConfig struct {
	size              int
	retry             retryConfig
	backoffBase       time.Duration
	backoffMax        time.Duration
	restartDelay      time.Duration
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

// WithBatchRetry sets the delivery budget (WithMaxAttempts, default
// DefaultMaxAttempts) and the permanent error classifier
// (WithPermanentClassifier).
func WithBatchRetry(opts ...RetryOption) BatchOption {
	return func(c *batchConfig) {
		for _, o := range opts {
			o(&c.retry)
		}
	}
}

// WithBatchRedeliveryBackoff sets the redelivery delay of a failed record: base
// on the first attempt, doubled on each next attempt, up to max (default 1s to
// 60s). A non-positive base is ignored. A max below base is raised to base.
func WithBatchRedeliveryBackoff(base, max time.Duration) BatchOption {
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

// WithBatchRestartDelay sets the wait after a fetch error (default 5s).
func WithBatchRestartDelay(d time.Duration) BatchOption {
	return func(c *batchConfig) {
		if d > 0 {
			c.restartDelay = d
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
// feed to the handler of that feed. docs/events.md describes what it does with a
// record that fails.
//
// Register every feed with Route before Run or Replay. Several routers, each in
// its own replica, can share one durable consumer.
type BatchRouter struct {
	source      BatchSource
	deadLetters DeadLetterQueue
	cfg         batchConfig
	routes      map[string]batchRoute
}

type batchRoute struct {
	handle func(ctx context.Context, recs []*inflight)
	replay func(ctx context.Context, payload []byte) (DeadLetterReason, error)
}

// NewBatchRouter builds a router over source that stores the records it gives up
// on in deadLetters. A router for Replay alone can have a nil source.
func NewBatchRouter(source BatchSource, deadLetters DeadLetterQueue, opts ...BatchOption) (*BatchRouter, error) {
	if deadLetters == nil {
		return nil, errors.New("inbox: batch router: dead-letter store is required")
	}

	cfg := batchConfig{
		size:              defaultBatchSize,
		retry:             retryConfig{maxAttempts: DefaultMaxAttempts},
		backoffBase:       defaultRedeliveryBase,
		backoffMax:        defaultRedeliveryMax,
		restartDelay:      defaultRestartDelay,
		heartbeatInterval: defaultHeartbeatInterval,
	}
	for _, o := range opts {
		o(&cfg)
	}

	return &BatchRouter{source: source, deadLetters: deadLetters, cfg: cfg, routes: map[string]batchRoute{}}, nil
}

// Route registers handler for the feed of T. The route key is the full message
// name of T. Call Route before Run, not at the same time.
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
		handle: func(ctx context.Context, recs []*inflight) {
			handleGroup(ctx, r, feed, newEvent, handler, recs)
		},
		replay: func(ctx context.Context, payload []byte) (DeadLetterReason, error) {
			ev, err := decodeBatchEvent(payload, newEvent)
			if err != nil {
				return DeadLetterReasonDecode, err
			}
			if err := callHandler(ctx, handler, []BatchEvent[T]{ev}); err != nil {
				return DeadLetterReasonHandler, fmt.Errorf("handler: %w", err)
			}
			return "", nil
		},
	}

	return nil
}

// Run fetches and routes batches until ctx is done, and then returns the context
// error. It refuses to start when the transport stops a record at or before the
// last attempt of the router, because such a record is lost and not
// dead-lettered.
func (r *BatchRouter) Run(ctx context.Context) error {
	if r.source == nil {
		return errors.New("inbox: batch router: source is required")
	}
	if len(r.routes) == 0 {
		return errors.New("inbox: batch router: no routes")
	}
	if err := r.checkDeliveryLimit(ctx); err != nil {
		return err
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
			log.Warnf("inbox: batch fetch failed: %v; fetching again in %s", err, r.cfg.restartDelay)
			if !sleepCtx(ctx, r.cfg.restartDelay) {
				return ctx.Err()
			}
			continue
		}

		if len(recs) > 0 {
			r.process(ctx, recs)
		}
	}
}

func (r *BatchRouter) checkDeliveryLimit(ctx context.Context) error {
	limiter, ok := r.source.(DeliveryLimiter)
	if !ok {
		return nil
	}

	limit, err := limiter.MaxDeliveries(ctx)
	if err != nil {
		return fmt.Errorf("inbox: batch router: delivery limit: %w", err)
	}
	if limit > 0 && limit <= r.cfg.retry.maxAttempts {
		return fmt.Errorf("inbox: batch router: the transport stops a record after %d deliveries, "+
			"which is not above the delivery budget %d: remove the limit of the consumer", limit, r.cfg.retry.maxAttempts)
	}

	return nil
}

// Replay runs each dead letter that matches f through the handler of its feed,
// as a batch of one event. It deletes a dead letter that its handler accepts. A
// dead letter that fails again stays, and the store of NewGormDeadLetterReader
// keeps the new reason and error and counts the attempt. A dead letter whose
// feed has no route stays as it is. Replay returns a List error. The failure of
// one record goes to its outcome.
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

		reason, err := route.replay(ctx, row.Payload)
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
		if _, ok := r.routes[it.Feed]; !ok {
			r.unrouted(ctx, it)
			continue
		}
		if _, ok := groups[it.Feed]; !ok {
			order = append(order, it.Feed)
		}
		groups[it.Feed] = append(groups[it.Feed], it)
	}

	for _, feed := range order {
		if ctx.Err() != nil {
			return
		}
		r.routes[feed].handle(ctx, groups[feed])
	}
}

// unrouted redelivers a record that has no route until its last attempt. In a
// rolling deploy an old replica can fetch a record of a feed that only the new
// version routes.
func (r *BatchRouter) unrouted(ctx context.Context, it *inflight) {
	if !r.final(it) {
		r.retry(ctx, it, r.nackDelay(it.Attempt))
		return
	}
	r.deadLetter(ctx, it, it.Feed, "", DeadLetterReasonNoRoute, fmt.Errorf("no route for subject %q", it.Subject))
}

func handleGroup[T proto.Message](ctx context.Context, r *BatchRouter, feed string,
	newEvent func() T, handler BatchHandler[T], recs []*inflight) {
	batch, live := decodeGroup(ctx, r, feed, newEvent, recs)
	if len(batch) == 0 {
		return
	}

	err := callHandler(ctx, handler, batch)
	switch {
	case err == nil:
		for _, it := range live {
			it.ack()
		}
	case ctx.Err() != nil:
		// Shutdown. A nack spends a delivery, so the records wait for AckWait.
	case !r.permanent(err) && !r.anyFinal(live):
		log.Warnf("inbox: batch of %d %s record(s) failed: %v; redelivering", len(live), feed, err)
		for _, it := range live {
			r.retry(ctx, it, r.nackDelay(it.Attempt))
		}
	default:
		log.Warnf("inbox: batch of %d %s record(s) failed: %v; running each record alone", len(live), feed, err)
		runAlone(ctx, r, feed, handler, batch, live)
	}
}

// decodeGroup decodes the records of one feed. It dead-letters a record that
// does not decode, or whose envelope tenant is not the tenant of its subject.
func decodeGroup[T proto.Message](ctx context.Context, r *BatchRouter, feed string,
	newEvent func() T, recs []*inflight) ([]BatchEvent[T], []*inflight) {
	batch := make([]BatchEvent[T], 0, len(recs))
	live := make([]*inflight, 0, len(recs))
	for _, it := range recs {
		ev, err := decodeBatchEvent(it.Payload, newEvent)
		if err == nil {
			if tenant := ev.Meta.GetTenant().GetTenantId(); tenant != it.Tenant {
				err = fmt.Errorf("envelope tenant %q is not the subject tenant %q", tenant, it.Tenant)
			}
		}
		if err != nil {
			r.deadLetter(ctx, it, feed, ev.Meta.GetEventId(), DeadLetterReasonDecode, err)
			continue
		}
		batch = append(batch, ev)
		live = append(live, it)
	}
	return batch, live
}

// runAlone runs the handler with each record alone and acks each record that
// passes. When one record passes, the others are bad records: the router
// dead-letters them. When none passes, the cause is most likely an outage: the
// router dead-letters only the records with a permanent error, and redelivers
// the rest after the maximum backoff.
func runAlone[T proto.Message](ctx context.Context, r *BatchRouter, feed string,
	handler BatchHandler[T], batch []BatchEvent[T], live []*inflight) {
	type failure struct {
		index int
		err   error
	}

	var failed []failure
	passed := 0
	for i, it := range live {
		if ctx.Err() != nil {
			return
		}
		err := callHandler(ctx, handler, batch[i:i+1])
		if err == nil {
			it.ack()
			passed++
			continue
		}
		if ctx.Err() != nil {
			return
		}
		failed = append(failed, failure{index: i, err: err})
	}

	if passed == 0 && len(failed) > 0 {
		log.Warnf("inbox: every %s record failed alone; redelivering the records without a permanent error", feed)
	}
	for _, f := range failed {
		it := live[f.index]
		if passed > 0 || r.permanent(f.err) {
			r.deadLetter(ctx, it, feed, batch[f.index].Meta.GetEventId(), DeadLetterReasonHandler,
				fmt.Errorf("handler: %w", f.err))
			continue
		}
		r.retry(ctx, it, r.cfg.backoffMax)
	}
}

func decodeBatchEvent[T proto.Message](payload []byte, newEvent func() T) (BatchEvent[T], error) {
	event := newEvent()
	if err := proto.Unmarshal(payload, event); err != nil {
		return BatchEvent[T]{}, fmt.Errorf("decode: %w", err)
	}

	meta, err := events.MetaOf(event)
	if err != nil {
		return BatchEvent[T]{}, fmt.Errorf("envelope: %w", err)
	}

	return BatchEvent[T]{Event: event, Meta: meta}, nil
}

// callHandler turns a handler panic into an error, so that one bad record cannot
// stop the router of every feed.
func callHandler[T proto.Message](ctx context.Context, handler BatchHandler[T], batch []BatchEvent[T]) (err error) {
	defer func() {
		if p := recover(); p != nil {
			log.Errorf("inbox: batch handler panic: %v\n%s", p, debug.Stack())
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return handler(ctx, batch)
}

// deadLetter stores the record and terminates it. If the store fails, the record
// is redelivered, so the router never drops a record it could store later.
func (r *BatchRouter) deadLetter(ctx context.Context, it *inflight, feed, eventID string,
	reason DeadLetterReason, cause error) {
	err := r.deadLetters.Store(ctx, DeadLetterRecord{
		Payload:  it.Payload,
		Error:    errString(cause),
		Attempts: it.Attempt,
		EventID:  eventID,
		Feed:     feed,
		Reason:   reason,
	})
	if err != nil {
		log.Warnf("inbox: dead-letter store failed (record will be redelivered): %v", err)
		r.retry(ctx, it, r.nackDelay(it.Attempt))
		return
	}

	log.Warnf("inbox: dead-lettered record %s of subject %q after %d attempt(s): %s: %v",
		shortFingerprint(payloadFingerprint(it.Payload)), it.Subject, it.Attempt, reason, cause)
	it.term()
}

// retry asks for a redelivery after delay. At shutdown it leaves the record for
// AckWait, because a nack spends a delivery.
func (r *BatchRouter) retry(ctx context.Context, it *inflight, delay time.Duration) {
	if ctx.Err() != nil {
		return
	}
	it.nack(delay)
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

func (r *BatchRouter) nackDelay(attempt int) time.Duration {
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

func (it *inflight) nack(delay time.Duration) {
	var f func() error
	if it.Nack != nil {
		f = func() error { return it.Nack(delay) }
	}
	it.settle("nack", f)
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
