package sources_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/events/inbox/sources"
	"github.com/safedep/dry/events/internal/natstest"
	"github.com/safedep/dry/events/natsjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const fetchWait = 100 * time.Millisecond

type (
	observationBatch = []inbox.BatchEvent[*pkgregv1.PackageVersionObservationEvent]
	maintainerBatch  = []inbox.BatchEvent[*pkgregv1.MaintainerObservationEvent]
)

type feedEnv struct {
	js       jetstream.JetStream
	consumer jetstream.Consumer
	pub      *natsjs.Publisher
}

func newFeedEnv(t *testing.T, cfg natsjs.ConsumerConfig) feedEnv {
	t.Helper()
	js := natstest.JetStream(t)

	stream, err := natsjs.EnsureStream(t.Context(), js, natstest.StreamConfig())
	require.NoError(t, err)

	cfg.Durable = "writer"
	consumer, err := natsjs.EnsureConsumer(t.Context(), stream, cfg)
	require.NoError(t, err)

	pub, err := natsjs.NewPublisher(js)
	require.NoError(t, err)

	return feedEnv{js: js, consumer: consumer, pub: pub}
}

func (e feedEnv) publish(t *testing.T, events ...proto.Message) {
	t.Helper()
	for _, ev := range events {
		_, err := e.pub.Publish(t.Context(), ev)
		require.NoError(t, err)
	}
}

func (e feedEnv) source(t *testing.T) inbox.BatchSource {
	t.Helper()
	src, err := sources.NewJetStream(e.consumer, sources.WithFetchMaxWait(fetchWait))
	require.NoError(t, err)
	return src
}

// settled reads the consumer through its own handle, because Consumer.Info
// rewrites the cached info that the source reads.
func (e feedEnv) settled(t *testing.T) bool {
	t.Helper()
	consumer, err := e.js.Consumer(t.Context(), natstest.StreamName, "writer")
	require.NoError(t, err)
	info, err := consumer.Info(t.Context())
	require.NoError(t, err)
	return info.NumPending == 0 && info.NumAckPending == 0
}

// run runs the router until the test ends. It fails the test when Run returns an
// error other than the cancel.
func run(t *testing.T, r *inbox.BatchRouter) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		assert.ErrorIs(t, <-done, context.Canceled)
	})
}

// seen counts handler calls per event id.
type seen struct {
	mu    sync.Mutex
	calls map[string]int
}

func newSeen() *seen { return &seen{calls: map[string]int{}} }

func (s *seen) add(ids ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		s.calls[id]++
	}
	if len(ids) == 0 {
		return 0
	}
	return s.calls[ids[0]]
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

func ids[T proto.Message](batch []inbox.BatchEvent[T]) []string {
	out := make([]string, len(batch))
	for i, ev := range batch {
		out[i] = ev.Meta.GetEventId()
	}
	return out
}

// memDLQ fails the first failures stores, then keeps the records.
type memDLQ struct {
	mu       sync.Mutex
	failures int
	stored   []inbox.DeadLetterRecord
}

func (m *memDLQ) Store(_ context.Context, rec inbox.DeadLetterRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failures > 0 {
		m.failures--
		return errors.New("db down")
	}
	m.stored = append(m.stored, rec)
	return nil
}

func (m *memDLQ) records() []inbox.DeadLetterRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]inbox.DeadLetterRecord(nil), m.stored...)
}

func TestNewJetStream_Validation(t *testing.T) {
	_, err := sources.NewJetStream(nil)
	assert.Error(t, err)

	env := newFeedEnv(t, natsjs.ConsumerConfig{AckWait: time.Second})
	_, err = sources.NewJetStream(env.consumer, sources.WithFetchMaxWait(time.Second))
	assert.Error(t, err, "a fetch wait at AckWait")
	_, err = sources.NewJetStream(env.consumer, sources.WithFetchMaxWait(500*time.Millisecond))
	assert.NoError(t, err)
}

func TestJetStreamSource_MapsRecordAndAttempt(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	env.publish(t, natstest.Observation("evt-1", natstest.Tenant))
	src := env.source(t)

	recs, err := src.Fetch(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, natstest.ObservationFeed+"."+natstest.Tenant, recs[0].Subject)
	assert.Equal(t, natstest.ObservationFeed, recs[0].Feed)
	assert.Equal(t, natstest.Tenant, recs[0].Tenant)
	assert.Equal(t, 1, recs[0].Attempt)
	assert.NotEmpty(t, recs[0].Payload)
	require.NoError(t, recs[0].Nack(0))

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
	env := newFeedEnv(t, natsjs.ConsumerConfig{AckWait: time.Hour})
	src, err := sources.NewJetStream(env.consumer, sources.WithFetchMaxWait(time.Minute))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	_, err = src.Fetch(ctx, 10)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestJetStreamRouter_DeliveryLimit(t *testing.T) {
	cases := []struct {
		name       string
		maxDeliver int
		wantErr    bool
	}{
		{name: "no limit", maxDeliver: 0},
		{name: "limit above the budget", maxDeliver: 5},
		{name: "limit at the budget", maxDeliver: 4, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newFeedEnv(t, natsjs.ConsumerConfig{MaxDeliver: tc.maxDeliver})
			r, err := inbox.NewBatchRouter(env.source(t), &memDLQ{}, inbox.WithBatchRetry(inbox.WithMaxAttempts(4)))
			require.NoError(t, err)
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

			ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
			defer cancel()
			err = r.Run(ctx)
			if tc.wantErr {
				assert.ErrorContains(t, err, "remove the limit")
				return
			}
			assert.ErrorIs(t, err, context.DeadlineExceeded)
		})
	}
}

func TestJetStreamRouter_AcksABatch(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	for i := range 3 {
		env.publish(t, natstest.Observation(fmt.Sprintf("evt-%d", i), natstest.Tenant))
	}

	r, err := inbox.NewBatchRouter(env.source(t), &memDLQ{})
	require.NoError(t, err)
	got := newSeen()
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
		got.add(ids(b)...)
		return nil
	}))
	run(t, r)

	require.Eventually(t, func() bool { return env.settled(t) && len(got.snapshot()) == 3 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"evt-0": 1, "evt-1": 1, "evt-2": 1}, got.snapshot())
}

func TestJetStreamRouter_RedeliversOnlyTheFailedGroup(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	env.publish(t,
		natstest.Observation("obs-1", natstest.Tenant),
		natstest.Maintainer("mnt-1", natstest.Tenant),
		natstest.Observation("obs-2", natstest.Tenant))

	r, err := inbox.NewBatchRouter(env.source(t), &memDLQ{},
		inbox.WithBatchRedeliveryBackoff(20*time.Millisecond, 20*time.Millisecond))
	require.NoError(t, err)
	obs, mnt := newSeen(), newSeen()
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
		obs.add(ids(b)...)
		return nil
	}))
	require.NoError(t, inbox.Route(r, natstest.NewMaintainer, func(_ context.Context, b maintainerBatch) error {
		if mnt.add(ids(b)...) == 1 {
			return errors.New("insert failed")
		}
		return nil
	}))
	run(t, r)

	require.Eventually(t, func() bool { return env.settled(t) && mnt.snapshot()["mnt-1"] == 2 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"obs-1": 1, "obs-2": 1}, obs.snapshot(), "the feed that succeeded is not redelivered")
}

func TestJetStreamRouter_SplitDeadLettersOnlyTheBadRecord(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	env.publish(t,
		natstest.Observation("good-1", natstest.Tenant),
		natstest.Observation("bad", natstest.Tenant),
		natstest.Observation("good-2", natstest.Tenant))

	dlq := &memDLQ{}
	r, err := inbox.NewBatchRouter(env.source(t), dlq,
		inbox.WithBatchRetry(inbox.WithMaxAttempts(3)),
		inbox.WithBatchRedeliveryBackoff(20*time.Millisecond, 20*time.Millisecond))
	require.NoError(t, err)
	got := newSeen()
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
		batchIDs := ids(b)
		for _, id := range batchIDs {
			if id == "bad" {
				return errors.New("bad row")
			}
		}
		got.add(batchIDs...)
		return nil
	}))
	run(t, r)

	require.Eventually(t, func() bool { return env.settled(t) && len(dlq.records()) == 1 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"good-1": 1, "good-2": 1}, got.snapshot())
	rec := dlq.records()[0]
	assert.Equal(t, "bad", rec.EventID)
	assert.Equal(t, inbox.DeadLetterReasonHandler, rec.Reason)
	assert.Equal(t, 3, rec.Attempts)
}

func TestJetStreamRouter_DeadLettersDecodeFailuresAndUnroutedFeeds(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	_, err := env.js.Publish(t.Context(), natstest.ObservationFeed+"."+natstest.Tenant, []byte{0xff, 0xff, 0xff})
	require.NoError(t, err)
	_, err = env.js.Publish(t.Context(), natstest.ObservationFeed+".tenant-b",
		natstest.Bytes(t, natstest.Observation("wrong-tenant", natstest.Tenant)))
	require.NoError(t, err)
	env.publish(t,
		natstest.Maintainer("mnt-1", natstest.Tenant),
		natstest.Observation("obs-1", natstest.Tenant))

	dlq := &memDLQ{}
	r, err := inbox.NewBatchRouter(env.source(t), dlq,
		inbox.WithBatchRetry(inbox.WithMaxAttempts(2)),
		inbox.WithBatchRedeliveryBackoff(20*time.Millisecond, 20*time.Millisecond))
	require.NoError(t, err)
	got := newSeen()
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
		got.add(ids(b)...)
		return nil
	}))
	run(t, r)

	require.Eventually(t, func() bool { return env.settled(t) && len(dlq.records()) == 3 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]int{"obs-1": 1}, got.snapshot())

	byReason := map[inbox.DeadLetterReason][]int{}
	for _, rec := range dlq.records() {
		byReason[rec.Reason] = append(byReason[rec.Reason], rec.Attempts)
	}
	assert.ElementsMatch(t, []int{1, 1}, byReason[inbox.DeadLetterReasonDecode], "decode failures go at once")
	assert.Equal(t, []int{2}, byReason[inbox.DeadLetterReasonNoRoute], "an unrouted record waits for its last attempt")
}

func TestJetStreamRouter_StoreFailureKeepsTheRecordPastTheBudget(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	env.publish(t, natstest.Observation("bad", natstest.Tenant))

	dlq := &memDLQ{failures: 2}
	permanent := errors.New("bad row")
	r, err := inbox.NewBatchRouter(env.source(t), dlq,
		inbox.WithBatchRetry(inbox.WithMaxAttempts(2), inbox.WithPermanentClassifier(func(err error) bool {
			return errors.Is(err, permanent)
		})),
		inbox.WithBatchRedeliveryBackoff(20*time.Millisecond, 20*time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error {
		return permanent
	}))
	run(t, r)

	require.Eventually(t, func() bool { return env.settled(t) && len(dlq.records()) == 1 }, 10*time.Second, 20*time.Millisecond)
	assert.Equal(t, 3, dlq.records()[0].Attempts, "the record outlives two failed stores")
}

func TestJetStreamRouter_HeartbeatKeepsASlowBatchFromRedelivery(t *testing.T) {
	const ackWait = 500 * time.Millisecond
	env := newFeedEnv(t, natsjs.ConsumerConfig{AckWait: ackWait})
	env.publish(t, natstest.Observation("slow", natstest.Tenant))

	started := make(chan struct{})
	release := make(chan struct{})
	calls := make(chan struct{}, 2)
	handler := func(context.Context, observationBatch) error {
		calls <- struct{}{}
		select {
		case started <- struct{}{}:
			<-release
		default:
		}
		return nil
	}

	// The second replica takes the record if the server redelivers it.
	for range 2 {
		r, err := inbox.NewBatchRouter(env.source(t), &memDLQ{}, inbox.WithBatchHeartbeat(ackWait/5))
		require.NoError(t, err)
		require.NoError(t, inbox.Route(r, natstest.NewObservation, handler))
		run(t, r)
	}

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		require.Fail(t, "the handler did not start")
	}
	<-calls

	select {
	case <-calls:
		assert.Fail(t, "the server redelivered a record while its handler ran")
	case <-time.After(3 * ackWait):
	}
	close(release)

	require.Eventually(t, func() bool { return env.settled(t) }, 10*time.Second, 20*time.Millisecond)
	assert.Empty(t, calls)
}

func TestJetStreamRouter_ReplicasShareOneDurableConsumer(t *testing.T) {
	env := newFeedEnv(t, natsjs.ConsumerConfig{})
	const n = 60
	for i := range n {
		env.publish(t, natstest.Observation(fmt.Sprintf("evt-%02d", i), natstest.Tenant))
	}

	all := newSeen()
	perReplica := []*seen{newSeen(), newSeen()}
	for _, mine := range perReplica {
		r, err := inbox.NewBatchRouter(env.source(t), &memDLQ{}, inbox.WithBatchSize(5))
		require.NoError(t, err)
		require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
			all.add(ids(b)...)
			mine.add(ids(b)...)
			time.Sleep(20 * time.Millisecond)
			return nil
		}))
		run(t, r)
	}

	require.Eventually(t, func() bool { return env.settled(t) && len(all.snapshot()) == n }, 20*time.Second, 20*time.Millisecond)
	for id, calls := range all.snapshot() {
		assert.Equal(t, 1, calls, "event %s", id)
	}
	for i, mine := range perReplica {
		assert.NotEmpty(t, mine.snapshot(), "replica %d got no records", i)
	}
}
