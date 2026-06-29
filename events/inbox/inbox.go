// Package inbox is the read-side dual of dry/events/outbox: a reusable way to
// consume a typed SafeDep event feed at-least-once. Subscribe to a feed via a
// Source, decode its <Feed>Event + envelope, and process it; the position
// advances only after the handler durably succeeds (handle-then-ack).
//
// One Consume call drives one feed (one stream = one message type). A consumer
// of several feeds runs several Consume goroutines, each with its own Source and
// handler — there is no in-feed demux because the type is known from the feed.
package inbox

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/log"
	"google.golang.org/protobuf/proto"
)

// defaultRestartDelay is the backoff between read-session restarts after a
// transient transport error from Source.Receive.
const defaultRestartDelay = 5 * time.Second

// Handler processes one decoded event plus its envelope. It is called in stream
// order with no concurrency. A non-nil error is routed through the error policy
// (see Disposition / WithErrorHandler); it does not advance the cursor.
type Handler[T proto.Message] func(ctx context.Context, event T, meta *commonv1.EventMeta) error

// Source is the inbound transport adapter — the dual of outbox.Destination. It is
// feed-scoped (constructed for one events.Routing) and yields one raw record at a
// time. Decode happens above the Source so a malformed record's bytes survive a
// decode failure (the error policy / a future DLQ can persist d.Payload).
type Source interface {
	// Receive yields the next record for the subscribed feed, or blocks until ctx
	// is done. The returned Delivery must be Ack'd after durable processing or
	// Nack'd to redeliver. A returned error is a transport failure: Consume retries
	// transient errors (the Source reopens) and returns only on ctx cancellation.
	Receive(ctx context.Context) (*Delivery, error)
}

// Delivery is one raw record from a Source plus its commit handles. Payload is
// the binary-proto <Feed>Event (envelope + payload).
type Delivery struct {
	Payload []byte
	Ack     func() error
	Nack    func() error
}

// Disposition is the error policy's verdict for a failed record.
type Disposition int

const (
	// Retry redelivers the record (Nack). The default; nothing is dropped on S2.
	Retry Disposition = iota
	// Skip advances past the record (Ack).
	Skip
)

// ErrorHandler is consulted on a decode or handler error. The raw Delivery is
// passed (a decode error has no typed event); err is the failure. Returning Retry
// redelivers; Skip acks past the record. A future DLQ is an ErrorHandler that
// persists d.Payload + err and returns Skip — no change to the Consume core.
type ErrorHandler func(ctx context.Context, d *Delivery, err error) Disposition

// Dedup is an optional, consumer-scoped processed-event store. It is best-effort
// duplicate-invocation suppression, NOT exactly-once: the handler runs outside
// the dedup write, so a crash between the two re-runs the handler on redelivery.
// Handlers must still be idempotent. Use it only to avoid expensive or noisy
// re-invocation in the happy path.
type Dedup interface {
	// Seen reports whether eventID was already processed by this consumer.
	Seen(ctx context.Context, eventID string) (bool, error)
	// Mark records eventID as processed.
	Mark(ctx context.Context, eventID string) error
}

type config struct {
	errorHandler ErrorHandler
	dedup        Dedup
	restartDelay time.Duration
}

// Option configures Consume.
type Option func(*config)

// WithErrorHandler overrides the default error policy (Retry for every decode and
// handler error). It is the seam a DLQ plugs into.
func WithErrorHandler(h ErrorHandler) Option {
	return func(c *config) { c.errorHandler = h }
}

// WithDedup enables best-effort duplicate suppression on event_id. See Dedup for
// the (not-exactly-once) semantics — handlers must remain idempotent regardless.
func WithDedup(d Dedup) Option {
	return func(c *config) { c.dedup = d }
}

// WithRestartDelay overrides the backoff between read-session restarts after a
// transient transport error (default 5s).
func WithRestartDelay(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.restartDelay = d
		}
	}
}

// Consume reads the feed via source and processes each record with handler until
// ctx is done. newEvent constructs a fresh T to decode into (T is a proto pointer
// type, so it cannot be allocated generically without a constructor).
//
// The loop is: Receive → decode T + MetaOf → (optional dedup) → handler → Ack on
// success, error policy on failure. A transient Receive error backs off and
// retries (the Source reopens at the persisted cursor); only ctx cancellation
// returns. This folds every consumer's hand-rolled restart loop into one place.
func Consume[T proto.Message](ctx context.Context, source Source, newEvent func() T, handler Handler[T], opts ...Option) error {
	if source == nil {
		return errors.New("inbox: source is required")
	}
	if newEvent == nil {
		return errors.New("inbox: newEvent constructor is required")
	}
	if handler == nil {
		return errors.New("inbox: handler is required")
	}
	// Probe the constructor once: a nil message would panic proto.Unmarshal. The
	// factory is deterministic, so a nil here is a permanent misconfiguration —
	// fail fast rather than block the feed by routing every record through the
	// error policy.
	if nilMessage(newEvent()) {
		return errors.New("inbox: newEvent must return a non-nil message")
	}

	cfg := config{restartDelay: defaultRestartDelay}
	for _, o := range opts {
		o(&cfg)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		d, err := source.Receive(ctx)
		if err != nil {
			if isCtxErr(err) {
				return err
			}
			// Transient transport failure (session drop, EOF, broker blip): back
			// off and retry. The Source reopens at the persisted cursor on the next
			// Receive — no record is skipped.
			log.Warnf("inbox: receive failed: %v; restarting read in %s", err, cfg.restartDelay)
			if !sleepCtx(ctx, cfg.restartDelay) {
				return ctx.Err()
			}
			continue
		}

		handleOne(ctx, d, newEvent, handler, cfg)
	}
}

// handleOne decodes, optionally dedups, runs the handler, and commits. Every
// record-level error is absorbed via the error policy (Retry/Skip), so it returns
// nothing: the loop always proceeds to the next Receive.
func handleOne[T proto.Message](ctx context.Context, d *Delivery, newEvent func() T, handler Handler[T], cfg config) {
	event := newEvent()
	if err := proto.Unmarshal(d.Payload, event); err != nil {
		dispose(ctx, d, fmt.Errorf("decode: %w", err), cfg)
		return
	}

	meta, err := events.MetaOf(event)
	if err != nil {
		dispose(ctx, d, fmt.Errorf("envelope: %w", err), cfg)
		return
	}

	eventID := meta.GetEventId()
	if cfg.dedup != nil && eventID != "" {
		seen, err := cfg.dedup.Seen(ctx, eventID)
		if err != nil {
			dispose(ctx, d, fmt.Errorf("dedup check: %w", err), cfg)
			return
		}
		if seen {
			ackQuietly(d) // already processed: advance without re-running the handler
			return
		}
	}

	if err := handler(ctx, event, meta); err != nil {
		dispose(ctx, d, fmt.Errorf("handler: %w", err), cfg)
		return
	}

	if cfg.dedup != nil && eventID != "" {
		// Best-effort: the handler already succeeded, so a dedup-mark failure must
		// not block the cursor. A crash here re-runs the handler on redelivery —
		// absorbed by handler idempotency.
		if err := cfg.dedup.Mark(ctx, eventID); err != nil {
			log.Warnf("inbox: dedup mark event_id=%s: %v", eventID, err)
		}
	}

	if err := d.Ack(); err != nil {
		// Ack failure = non-advance: the cursor never leads the handler, so the
		// record redelivers on the next read and idempotency absorbs it. The Source
		// owns the reopen; here we just log and move on.
		log.Warnf("inbox: ack failed (record will be redelivered): %v", err)
	}
}

// dispose applies the error policy to a failed record: Retry (Nack/redeliver) by
// default, or whatever a configured ErrorHandler decides.
func dispose(ctx context.Context, d *Delivery, cause error, cfg config) {
	disposition := Retry
	if cfg.errorHandler != nil {
		disposition = cfg.errorHandler(ctx, d, cause)
	}

	switch disposition {
	case Skip:
		log.Warnf("inbox: skipping record after error: %v", cause)
		ackQuietly(d)
	default:
		// Retry: redeliver. On S2 this reopens at the cursor and re-reads the same
		// record; a permanently-failing record blocks the feed until an operator
		// wires WithErrorHandler->Skip (the Source emits a stalled-cursor warning).
		log.Warnf("inbox: redelivering record after error: %v", cause)
		if err := d.Nack(); err != nil {
			log.Warnf("inbox: nack failed: %v", err)
		}
	}
}

func ackQuietly(d *Delivery) {
	if err := d.Ack(); err != nil {
		log.Warnf("inbox: ack failed (record will be redelivered): %v", err)
	}
}

// nilMessage reports whether a constructed event is a nil pointer (or boxed nil),
// which would panic proto.Unmarshal. proto messages are pointer types.
func nilMessage[T proto.Message](m T) bool {
	v := reflect.ValueOf(m)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// sleepCtx waits for d or ctx cancellation. Returns true if the full delay
// elapsed, false if ctx was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
