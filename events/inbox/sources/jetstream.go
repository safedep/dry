package sources

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/async"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/log"
)

const defaultFetchMaxWait = 2 * time.Second

type jetStreamSource struct {
	consumer jetstream.Consumer
	maxWait  time.Duration
}

var (
	_ inbox.BatchSource     = &jetStreamSource{}
	_ inbox.DeliveryLimiter = &jetStreamSource{}
)

// JetStreamOption configures a JetStream inbox BatchSource.
type JetStreamOption func(*jetStreamSource)

// WithFetchMaxWait sets how long one fetch waits for a full batch (default 2s).
// It must be below the AckWait of the consumer. A cancelled context stops the
// router only after the current fetch ends.
func WithFetchMaxWait(d time.Duration) JetStreamOption {
	return func(s *jetStreamSource) {
		if d > 0 {
			s.maxWait = d
		}
	}
}

// NewJetStream builds a BatchSource over a durable pull consumer. Replicas that
// share the consumer share its records: each record goes to one replica. The
// source reads the feed and the tenant of a record from its subject (see
// async.ParseTenantEventSubject).
func NewJetStream(consumer jetstream.Consumer, opts ...JetStreamOption) (inbox.BatchSource, error) {
	if consumer == nil {
		return nil, errors.New("inbox/jetstream: consumer is required")
	}
	info := consumer.CachedInfo()
	if info == nil {
		return nil, errors.New("inbox/jetstream: consumer has no info")
	}

	src := &jetStreamSource{consumer: consumer, maxWait: defaultFetchMaxWait}
	for _, opt := range opts {
		opt(src)
	}

	// A record of a fetch waits in the client until the fetch ends. A wait at or
	// above AckWait makes the server redeliver it before the router sees it.
	if ackWait := info.Config.AckWait; ackWait > 0 && src.maxWait >= ackWait {
		return nil, fmt.Errorf("inbox/jetstream: fetch max wait %s must be below the consumer AckWait %s", src.maxWait, ackWait)
	}

	return src, nil
}

// MaxDeliveries reads the cached consumer info. Consumer.Info rewrites that
// cache without a lock, so the source does not call it.
func (s *jetStreamSource) MaxDeliveries(context.Context) (int, error) {
	info := s.consumer.CachedInfo()
	if info == nil {
		return 0, errors.New("inbox/jetstream: consumer has no info")
	}
	return info.Config.MaxDeliver, nil
}

func (s *jetStreamSource) Fetch(ctx context.Context, max int) ([]inbox.BatchRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Fetch cannot stop on ctx. A cancel takes up to maxWait to stop the router.
	batch, err := s.consumer.Fetch(max, jetstream.FetchMaxWait(s.maxWait))
	if err != nil {
		return nil, err
	}

	var recs []inbox.BatchRecord
	for msg := range batch.Messages() {
		recs = append(recs, record(msg))
	}

	// The records that came before an error go to the router, so that they do
	// not wait for their AckWait to end.
	err = batch.Error()
	switch {
	case len(recs) == 0 && ctx.Err() != nil:
		return nil, ctx.Err()
	case err == nil:
		return recs, nil
	case len(recs) == 0:
		return nil, err
	default:
		log.Warnf("inbox/jetstream: fetch ended with an error after %d record(s): %v", len(recs), err)
		return recs, nil
	}
}

func record(msg jetstream.Msg) inbox.BatchRecord {
	feed, tenant, err := async.ParseTenantEventSubject(msg.Subject())
	if err != nil {
		log.Warnf("inbox/jetstream: %v", err)
	}

	return inbox.BatchRecord{
		Subject:    msg.Subject(),
		Feed:       feed,
		Tenant:     tenant,
		Payload:    msg.Data(),
		Attempt:    attempt(msg),
		Ack:        msg.Ack,
		Nack:       msg.NakWithDelay,
		Term:       msg.Term,
		InProgress: msg.InProgress,
	}
}

func attempt(msg jetstream.Msg) int {
	meta, err := msg.Metadata()
	if err != nil {
		log.Warnf("inbox/jetstream: no metadata on a record of subject %q: %v", msg.Subject(), err)
		return 1
	}
	if meta.NumDelivered > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(meta.NumDelivered)
}
