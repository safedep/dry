// Package outbox delivers typed event messages to one or more transport
// destinations. It has two write paths:
//
//   - Emit(ctx, tx, msg) writes the event into the caller's transaction, so the
//     business write and the event commit atomically. Requires a store.
//   - Send(ctx, msg) is fire-and-forget: with a store the event is buffered and
//     the drain (Run) publishes it; without a store it is published directly.
//
// Durability is a db.SqlDataAdapter injected via WithStore. Delivery is
// at-least-once per destination; consumers dedupe on event_id.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/safedep/dry/db"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/log"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ErrNoStore is returned by Emit when no store was injected: a transactional
// write is meaningless without a durable table.
var ErrNoStore = errors.New("outbox: Emit requires a store (WithStore)")

const (
	defaultMaxAttempts  = 5
	defaultPollInterval = time.Second
	defaultBatchSize    = 100
)

// Outbox delivers events to its destinations. Construct with New.
type Outbox struct {
	dests      []Destination
	destByName map[string]Destination

	store db.SqlDataAdapter // optional; nil ⇒ Send publishes direct, Emit errors

	maxAttempts  int
	pollInterval time.Duration
	batchSize    int
	now          func() time.Time
}

// Option configures an Outbox.
type Option func(*Outbox)

// WithStore enables the durable paths (Emit, and Send buffering + Run) backed by
// the adapter's database. Without it, Send publishes directly and Emit errors.
func WithStore(adapter db.SqlDataAdapter) Option {
	return func(o *Outbox) { o.store = adapter }
}

// WithMaxAttempts sets how many times the drain retries a destination before
// poisoning that delivery (default 5).
func WithMaxAttempts(n int) Option {
	return func(o *Outbox) { o.maxAttempts = n }
}

// WithPollInterval sets the drain poll interval (default 1s).
func WithPollInterval(d time.Duration) Option {
	return func(o *Outbox) { o.pollInterval = d }
}

// New constructs an Outbox over one or more destinations.
func New(dests []Destination, opts ...Option) (*Outbox, error) {
	if len(dests) == 0 {
		return nil, errors.New("outbox: at least one destination is required")
	}

	o := &Outbox{
		dests:        dests,
		destByName:   make(map[string]Destination, len(dests)),
		maxAttempts:  defaultMaxAttempts,
		pollInterval: defaultPollInterval,
		batchSize:    defaultBatchSize,
		now:          time.Now,
	}

	for _, opt := range opts {
		opt(o)
	}

	// Guard against options set to non-positive values (Run's ticker panics on a
	// non-positive interval; a non-positive maxAttempts would poison immediately).
	if o.maxAttempts < 1 {
		o.maxAttempts = defaultMaxAttempts
	}
	if o.pollInterval <= 0 {
		o.pollInterval = defaultPollInterval
	}
	if o.batchSize < 1 {
		o.batchSize = defaultBatchSize
	}

	for _, d := range dests {
		name := d.Name()
		if name == "" {
			return nil, errors.New("outbox: destination has an empty name")
		}
		if _, dup := o.destByName[name]; dup {
			return nil, fmt.Errorf("outbox: duplicate destination name %q", name)
		}
		o.destByName[name] = d
	}

	return o, nil
}

// Emit writes the event into the caller's transaction (transactional outbox).
// The business write and the event commit or roll back together; the drain (Run)
// publishes it afterwards. Requires a store.
func (o *Outbox) Emit(ctx context.Context, tx *gorm.DB, msg proto.Message) error {
	if o.store == nil {
		return ErrNoStore
	}
	if tx == nil {
		return errors.New("outbox: Emit requires a non-nil transaction")
	}

	rec, err := o.buildRecord(msg)
	if err != nil {
		return err
	}

	return o.insert(tx.WithContext(ctx), rec)
}

// Send delivers the event fire-and-forget. With a store it is buffered durably
// (the drain publishes it); without a store it is published directly to every
// destination — an in-flight event is lost on crash (accepted trade-off).
func (o *Outbox) Send(ctx context.Context, msg proto.Message) error {
	if o.store == nil {
		return o.publishDirect(ctx, msg)
	}

	rec, err := o.buildRecord(msg)
	if err != nil {
		return err
	}

	gdb, err := o.store.GetDB()
	if err != nil {
		return fmt.Errorf("outbox: get db: %w", err)
	}

	return gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return o.insert(tx, rec)
	})
}

// prepare resolves the routing, envelope ids, and serialized bytes of a typed
// event message, and enforces the outbox's contract: RoutingFor rejects a
// non-event message, and a missing event_id is rejected before anything is
// written or published — an empty event_id would break the consumer dedupe path.
func (o *Outbox) prepare(msg proto.Message) (routing events.Routing, eventID, tenant string, payload []byte, err error) {
	routing, err = events.RoutingFor(msg)
	if err != nil {
		return events.Routing{}, "", "", nil, err
	}

	meta, err := events.MetaOf(msg)
	if err != nil {
		return events.Routing{}, "", "", nil, err
	}

	eventID = meta.GetEventId()
	if eventID == "" {
		return events.Routing{}, "", "", nil,
			errors.New("outbox: event has no event_id envelope (stamp it with events.New before sending)")
	}

	payload, err = proto.Marshal(msg)
	if err != nil {
		return events.Routing{}, "", "", nil, fmt.Errorf("outbox: marshal event: %w", err)
	}

	return routing, eventID, meta.GetTenant().GetTenantId(), payload, nil
}

// buildRecord derives the durable row from a typed event message.
func (o *Outbox) buildRecord(msg proto.Message) (*Record, error) {
	routing, eventID, tenant, payload, err := o.prepare(msg)
	if err != nil {
		return nil, err
	}

	return &Record{
		EventID: eventID,
		FQN:     routing.FQN,
		Tenant:  tenant,
		Payload: payload,
	}, nil
}

// insert writes the record and one pending Delivery per destination. Used by
// Emit (caller's tx) and Send (own tx); the caller supplies the transaction.
func (o *Outbox) insert(tx *gorm.DB, rec *Record) error {
	if err := tx.Create(rec).Error; err != nil {
		return fmt.Errorf("outbox: insert record: %w", err)
	}

	for _, d := range o.dests {
		del := &Delivery{OutboxID: rec.ID, Destination: d.Name()}
		if err := tx.Create(del).Error; err != nil {
			return fmt.Errorf("outbox: insert delivery for %s: %w", d.Name(), err)
		}
	}

	return nil
}

// publishDirect is the no-store path: publish to every destination synchronously.
// Best-effort — partial delivery is possible and an in-flight event is lost on
// crash. Errors from all destinations are joined.
func (o *Outbox) publishDirect(ctx context.Context, msg proto.Message) error {
	routing, eventID, tenant, payload, err := o.prepare(msg)
	if err != nil {
		return err
	}

	req := PublishRequest{Routing: routing, Tenant: tenant, EventID: eventID, Record: payload}

	var errs []error
	for _, d := range o.dests {
		if err := d.Publish(ctx, req); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Name(), err))
		}
	}

	return errors.Join(errs...)
}

// Run is the drain loop: poll the store, publish un-acked deliveries per
// destination in order, mark them. It is a no-op without a store. Run a single
// instance (single-writer) so per-subject order is preserved on S2. Returns nil
// on context cancellation.
func (o *Outbox) Run(ctx context.Context) error {
	if o.store == nil {
		return nil
	}

	ticker := time.NewTicker(o.pollInterval)
	defer ticker.Stop()

	for {
		if _, err := o.drainOnce(ctx); err != nil {
			log.Warnf("outbox: drain error: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
