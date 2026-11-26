package sources

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/log"
)

const defaultFetchMaxWait = 5 * time.Second

type jetStreamSource struct {
	consumer jetstream.Consumer
	maxWait  time.Duration
}

var _ inbox.BatchSource = &jetStreamSource{}

// JetStreamOption configures a JetStream inbox BatchSource.
type JetStreamOption func(*jetStreamSource)

// WithFetchMaxWait sets how long one fetch waits for records (default 5s).
func WithFetchMaxWait(d time.Duration) JetStreamOption {
	return func(s *jetStreamSource) {
		if d > 0 {
			s.maxWait = d
		}
	}
}

// NewJetStream builds a BatchSource over a durable pull consumer. Replicas that
// share the consumer share its records: each record goes to one replica.
func NewJetStream(consumer jetstream.Consumer, opts ...JetStreamOption) (inbox.BatchSource, error) {
	if consumer == nil {
		return nil, errors.New("inbox/jetstream: consumer is required")
	}

	src := &jetStreamSource{consumer: consumer, maxWait: defaultFetchMaxWait}
	for _, opt := range opts {
		opt(src)
	}

	return src, nil
}

func (s *jetStreamSource) Fetch(ctx context.Context, max int) ([]inbox.BatchRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fetchCtx, cancel := context.WithTimeout(ctx, s.maxWait)
	defer cancel()

	batch, err := s.consumer.Fetch(max, jetstream.FetchContext(fetchCtx))
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
	case err == nil:
		return recs, nil
	case ctx.Err() != nil:
		if len(recs) == 0 {
			return nil, ctx.Err()
		}
		return recs, nil
	case errors.Is(err, context.DeadlineExceeded):
		return recs, nil
	case len(recs) == 0:
		return nil, err
	default:
		log.Warnf("inbox/jetstream: fetch ended with an error after %d record(s): %v", len(recs), err)
		return recs, nil
	}
}

func record(msg jetstream.Msg) inbox.BatchRecord {
	return inbox.BatchRecord{
		Subject:    msg.Subject(),
		Payload:    msg.Data(),
		Attempt:    attempt(msg),
		Ack:        msg.Ack,
		Nak:        msg.NakWithDelay,
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
