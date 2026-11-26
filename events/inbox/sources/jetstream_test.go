package sources_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/events/inbox/sources"
	"github.com/safedep/dry/events/internal/natstest"
	"github.com/safedep/dry/events/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	observationFeed = "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent"
	maintainerFeed  = "safedep.events.private.packageregistry.v1.MaintainerObservationEvent"
)

type (
	observationBatch = []inbox.BatchEvent[*pkgregv1.PackageVersionObservationEvent]
	maintainerBatch  = []inbox.BatchEvent[*pkgregv1.MaintainerObservationEvent]
)

func newObservation() *pkgregv1.PackageVersionObservationEvent {
	return &pkgregv1.PackageVersionObservationEvent{}
}

func newMaintainer() *pkgregv1.MaintainerObservationEvent {
	return &pkgregv1.MaintainerObservationEvent{}
}

type feedEnv struct {
	js       natsjs.JetStream
	stream   natsjs.Stream
	consumer natsjs.Consumer
	pub      *jetstream.Publisher
}

func newFeedEnv(t *testing.T, cfg jetstream.ConsumerConfig) feedEnv {
	t.Helper()
	js := natstest.JetStream(t)

	stream, err := jetstream.EnsureStream(t.Context(), js, jetstream.StreamConfig{
		Name:     "PACKAGEREGISTRY",
		Subjects: []string{"safedep.events.private.packageregistry.v1.>"},
		MaxAge:   time.Hour,
		MaxBytes: 1 << 24,
	})
	require.NoError(t, err)

	if cfg.Durable == "" {
		cfg.Durable = "writer"
	}
	consumer, err := jetstream.EnsureConsumer(t.Context(), stream, cfg)
	require.NoError(t, err)

	pub, err := jetstream.NewPublisher(js)
	require.NoError(t, err)

	return feedEnv{js: js, stream: stream, consumer: consumer, pub: pub}
}

func (e feedEnv) publishObservation(t *testing.T, id string) {
	t.Helper()
	ev := pkgregv1.PackageVersionObservationEvent_builder{Meta: events.NewMeta(events.WithEventID(id))}.Build()
	_, err := e.pub.Publish(t.Context(), ev, "tenant-a")
	require.NoError(t, err)
}

func (e feedEnv) publishMaintainer(t *testing.T, id string) {
	t.Helper()
	ev := pkgregv1.MaintainerObservationEvent_builder{Meta: events.NewMeta(events.WithEventID(id))}.Build()
	_, err := e.pub.Publish(t.Context(), ev, "tenant-a")
	require.NoError(t, err)
}

func (e feedEnv) settled(t *testing.T) bool {
	t.Helper()
	info, err := e.consumer.Info(t.Context())
	require.NoError(t, err)
	return info.NumPending == 0 && info.NumAckPending == 0
}

// startRouter runs a router over the consumer until the test ends or stop.
func (e feedEnv) startRouter(t *testing.T, register func(r *inbox.BatchRouter), opts ...inbox.BatchOption) (stop func()) {
	t.Helper()
	src, err := sources.NewJetStream(e.consumer, sources.WithFetchMaxWait(200*time.Millisecond))
	require.NoError(t, err)

	r := inbox.NewBatchRouter(src, opts...)
	register(r)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()

	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			assert.ErrorIs(t, <-done, context.Canceled)
		})
	}
	t.Cleanup(stop)
	return stop
}

// seen counts handler calls per event id.
type seen struct {
	mu    sync.Mutex
	calls map[string]int
}

func newSeen() *seen { return &seen{calls: map[string]int{}} }

func (s *seen) add(ids ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		s.calls[id]++
	}
}

func (s *seen) snapshot() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int, len(s.calls))
	for k, v := range s.calls {
		out[k] = v
	}
	return out
}

func observationIDs(b observationBatch) []string {
	out := make([]string, len(b))
	for i, ev := range b {
		out[i] = ev.Meta.GetEventId()
	}
	return out
}

func maintainerIDs(b maintainerBatch) []string {
	out := make([]string, len(b))
	for i, ev := range b {
		out[i] = ev.Meta.GetEventId()
	}
	return out
}

type memDLQ struct {
	mu     sync.Mutex
	stored []inbox.DeadLetterRecord
}

func (m *memDLQ) Store(_ context.Context, rec inbox.DeadLetterRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stored = append(m.stored, rec)
	return nil
}

func (m *memDLQ) records() []inbox.DeadLetterRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]inbox.DeadLetterRecord(nil), m.stored...)
}

func TestNewJetStream_RequiresConsumer(t *testing.T) {
	_, err := sources.NewJetStream(nil)
	assert.Error(t, err)
}

func TestJetStreamSource_MapsRecordAndAttempt(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{})
	env.publishObservation(t, "evt-1")

	src, err := sources.NewJetStream(env.consumer, sources.WithFetchMaxWait(time.Second))
	require.NoError(t, err)

	recs, err := src.Fetch(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, observationFeed+".tenant-a", recs[0].Subject)
	assert.Equal(t, 1, recs[0].Attempt)
	assert.NotEmpty(t, recs[0].Payload)
	require.NoError(t, recs[0].Nak(0))

	recs, err = src.Fetch(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, 2, recs[0].Attempt, "a redelivery counts as an attempt")
	require.NoError(t, recs[0].Ack())

	recs, err = src.Fetch(t.Context(), 10)
	require.NoError(t, err)
	assert.Empty(t, recs, "an empty wait is not an error")
}

func TestJetStreamSource_FetchStopsOnCancel(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{})
	src, err := sources.NewJetStream(env.consumer, sources.WithFetchMaxWait(time.Minute))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	_, err = src.Fetch(ctx, 10)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestJetStreamRouter_AcksABatch(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{})
	for i := range 3 {
		env.publishObservation(t, fmt.Sprintf("evt-%d", i))
	}

	got := newSeen()
	env.startRouter(t, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
			got.add(observationIDs(b)...)
			return nil
		}))
	})

	require.Eventually(t, func() bool { return env.settled(t) && len(got.snapshot()) == 3 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"evt-0": 1, "evt-1": 1, "evt-2": 1}, got.snapshot())
}

func TestJetStreamRouter_RedeliversOnlyTheFailedGroup(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{})
	env.publishObservation(t, "obs-1")
	env.publishMaintainer(t, "mnt-1")
	env.publishObservation(t, "obs-2")

	obs, mnt := newSeen(), newSeen()
	env.startRouter(t, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
			obs.add(observationIDs(b)...)
			return nil
		}))
		require.NoError(t, inbox.Route(r, newMaintainer, func(_ context.Context, b maintainerBatch) error {
			mnt.add(maintainerIDs(b)...)
			if mnt.snapshot()["mnt-1"] == 1 {
				return errors.New("insert failed")
			}
			return nil
		}))
	}, inbox.WithBatchBackoff(50*time.Millisecond, 50*time.Millisecond))

	require.Eventually(t, func() bool { return env.settled(t) && mnt.snapshot()["mnt-1"] == 2 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"obs-1": 1, "obs-2": 1}, obs.snapshot(), "the feed that succeeded is not redelivered")
	assert.Equal(t, map[string]int{"mnt-1": 2}, mnt.snapshot())
}

func TestJetStreamRouter_SplitOnFinalAttemptDeadLettersOnlyTheBadRecord(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{MaxDeliver: 3})
	for _, id := range []string{"good-1", "bad", "good-2"} {
		env.publishObservation(t, id)
	}

	got := newSeen()
	dlq := &memDLQ{}
	env.startRouter(t, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
			batchIDs := observationIDs(b)
			for _, id := range batchIDs {
				if id == "bad" {
					return errors.New("bad row")
				}
			}
			got.add(batchIDs...)
			return nil
		}))
	},
		inbox.WithBatchDeadLetter(dlq),
		inbox.WithBatchRetry(inbox.WithMaxAttempts(3)),
		inbox.WithBatchBackoff(20*time.Millisecond, 20*time.Millisecond))

	require.Eventually(t, func() bool { return env.settled(t) && len(dlq.records()) == 1 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"good-1": 1, "good-2": 1}, got.snapshot())

	rec := dlq.records()[0]
	assert.Equal(t, "bad", rec.EventID)
	assert.Equal(t, inbox.DeadLetterReasonHandler, rec.Reason)
	assert.Equal(t, 3, rec.Attempts)
}

func TestJetStreamRouter_DeadLettersADecodeFailureAndAnUnroutedSubject(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{})
	_, err := env.js.Publish(t.Context(), observationFeed+".tenant-a", []byte{0xff, 0xff, 0xff})
	require.NoError(t, err)
	env.publishMaintainer(t, "mnt-1")
	env.publishObservation(t, "obs-1")

	got := newSeen()
	dlq := &memDLQ{}
	env.startRouter(t, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
			got.add(observationIDs(b)...)
			return nil
		}))
	}, inbox.WithBatchDeadLetter(dlq))

	require.Eventually(t, func() bool { return env.settled(t) && len(dlq.records()) == 2 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"obs-1": 1}, got.snapshot())

	reasons := map[inbox.DeadLetterReason]string{}
	for _, rec := range dlq.records() {
		reasons[rec.Reason] = rec.Feed
	}
	assert.Equal(t, map[inbox.DeadLetterReason]string{
		inbox.DeadLetterReasonDecode:  observationFeed,
		inbox.DeadLetterReasonNoRoute: maintainerFeed,
	}, reasons)
}

func TestJetStreamRouter_HeartbeatKeepsASlowBatchFromRedelivery(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{AckWait: time.Second})
	env.publishObservation(t, "slow")

	got := newSeen()
	register := func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
			got.add(observationIDs(b)...)
			time.Sleep(2500 * time.Millisecond)
			return nil
		}))
	}
	// The second replica takes the record if the server redelivers it.
	env.startRouter(t, register, inbox.WithBatchHeartbeat(200*time.Millisecond))
	env.startRouter(t, register, inbox.WithBatchHeartbeat(200*time.Millisecond))

	require.Eventually(t, func() bool { return env.settled(t) && len(got.snapshot()) == 1 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"slow": 1}, got.snapshot())
}

func TestJetStreamRouter_ReplicasShareOneDurableConsumer(t *testing.T) {
	env := newFeedEnv(t, jetstream.ConsumerConfig{})
	const n = 60
	for i := range n {
		env.publishObservation(t, fmt.Sprintf("evt-%02d", i))
	}

	all := newSeen()
	perReplica := []*seen{newSeen(), newSeen()}
	for _, mine := range perReplica {
		env.startRouter(t, func(r *inbox.BatchRouter) {
			require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
				all.add(observationIDs(b)...)
				mine.add(observationIDs(b)...)
				time.Sleep(20 * time.Millisecond)
				return nil
			}))
		}, inbox.WithBatchSize(5))
	}

	require.Eventually(t, func() bool { return env.settled(t) && len(all.snapshot()) == n }, 20*time.Second, 20*time.Millisecond)
	for id, calls := range all.snapshot() {
		assert.Equal(t, 1, calls, "event %s", id)
	}
	for i, mine := range perReplica {
		assert.NotEmpty(t, mine.snapshot(), "replica %d got no records", i)
	}
}
