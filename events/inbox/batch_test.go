package inbox_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/events/internal/natstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type (
	observationBatch = []inbox.BatchEvent[*pkgregv1.PackageVersionObservationEvent]
	maintainerBatch  = []inbox.BatchEvent[*pkgregv1.MaintainerObservationEvent]
)

// handle records what the router did with one fake record.
type handle struct {
	mu         sync.Mutex
	acks       int
	terms      int
	inProgress int
	nacks      []time.Duration
	touched    chan struct{}
}

func (h *handle) record(feed string, payload []byte, attempt int) inbox.BatchRecord {
	return inbox.BatchRecord{
		Subject: feed + "." + natstest.Tenant,
		Feed:    feed,
		Tenant:  natstest.Tenant,
		Payload: payload,
		Attempt: attempt,
		Ack:     func() error { h.mu.Lock(); defer h.mu.Unlock(); h.acks++; return nil },
		Nack: func(d time.Duration) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.nacks = append(h.nacks, d)
			return nil
		},
		Term: func() error { h.mu.Lock(); defer h.mu.Unlock(); h.terms++; return nil },
		InProgress: func() error {
			h.mu.Lock()
			h.inProgress++
			h.mu.Unlock()
			if h.touched != nil {
				select {
				case h.touched <- struct{}{}:
				default:
				}
			}
			return nil
		},
	}
}

// state returns the acks, terms and nack delays of the record.
func (h *handle) state() (int, int, []time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.acks, h.terms, append([]time.Duration(nil), h.nacks...)
}

type fetchStep struct {
	recs []inbox.BatchRecord
	err  error
}

// fakeBatchSource gives one step per Fetch and cancels the router when it has
// no steps left.
type fakeBatchSource struct {
	steps  []fetchStep
	cancel context.CancelFunc
	calls  int
}

func (f *fakeBatchSource) Fetch(ctx context.Context, _ int) ([]inbox.BatchRecord, error) {
	if f.calls >= len(f.steps) {
		f.cancel()
		return nil, ctx.Err()
	}
	s := f.steps[f.calls]
	f.calls++
	return s.recs, s.err
}

type limitedSource struct {
	fakeBatchSource
	limit int
	err   error
}

func (l *limitedSource) MaxDeliveries(context.Context) (int, error) { return l.limit, l.err }

func observationBytes(t *testing.T, id string) []byte {
	return natstest.Bytes(t, natstest.Observation(id, natstest.Tenant))
}

func maintainerBytes(t *testing.T, id string) []byte {
	return natstest.Bytes(t, natstest.Maintainer(id, natstest.Tenant))
}

func eventIDs[T proto.Message](batch []inbox.BatchEvent[T]) []string {
	ids := make([]string, len(batch))
	for i, ev := range batch {
		ids[i] = ev.Meta.GetEventId()
	}
	return ids
}

func failOn(bad string, err error) func(context.Context, observationBatch) error {
	return func(_ context.Context, b observationBatch) error {
		for _, id := range eventIDs(b) {
			if id == bad {
				return err
			}
		}
		return nil
	}
}

func TestBatchRouter_AcksOnlyTheGroupThatSucceeds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	obs1, obs2, mnt := &handle{}, &handle{}, &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		obs1.record(natstest.ObservationFeed, observationBytes(t, "obs-1"), 1),
		mnt.record(natstest.MaintainerFeed, maintainerBytes(t, "mnt-1"), 1),
		obs2.record(natstest.ObservationFeed, observationBytes(t, "obs-2"), 1),
	}}}}
	r, err := inbox.NewBatchRouter(src, &fakeDLQ{}, inbox.WithBatchRedeliveryBackoff(time.Second, time.Minute))
	require.NoError(t, err)

	var gotObs, gotMnt [][]string
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
		gotObs = append(gotObs, eventIDs(b))
		return nil
	}))
	require.NoError(t, inbox.Route(r, natstest.NewMaintainer, func(_ context.Context, b maintainerBatch) error {
		gotMnt = append(gotMnt, eventIDs(b))
		return errors.New("insert failed")
	}))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	assert.Equal(t, [][]string{{"obs-1", "obs-2"}}, gotObs, "one call with both records of the feed")
	assert.Equal(t, [][]string{{"mnt-1"}}, gotMnt)
	for _, h := range []*handle{obs1, obs2} {
		acks, terms, nacks := h.state()
		assert.Equal(t, 1, acks)
		assert.Zero(t, terms)
		assert.Empty(t, nacks, "the feed that succeeded is not redelivered")
	}
	acks, terms, nacks := mnt.state()
	assert.Zero(t, acks)
	assert.Zero(t, terms)
	assert.Equal(t, []time.Duration{time.Second}, nacks)
}

func TestBatchRouter_NackDelayGrowsWithAttempt(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 3, want: 4 * time.Second},
		{attempt: 9, want: 10 * time.Second},
	}
	for _, tc := range cases {
		ctx, cancel := context.WithCancel(t.Context())
		h := &handle{}
		src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
			h.record(natstest.ObservationFeed, observationBytes(t, "e"), tc.attempt),
		}}}}
		r, err := inbox.NewBatchRouter(src, &fakeDLQ{},
			inbox.WithBatchRedeliveryBackoff(time.Second, 10*time.Second),
			inbox.WithBatchRetry(inbox.WithMaxAttempts(100)))
		require.NoError(t, err)
		require.NoError(t, inbox.Route(r, natstest.NewObservation, failOn("e", errors.New("down"))))

		assert.ErrorIs(t, r.Run(ctx), context.Canceled)
		_, _, nacks := h.state()
		assert.Equal(t, []time.Duration{tc.want}, nacks, "attempt %d", tc.attempt)
		cancel()
	}
}

func TestBatchRouter_SplitDeadLettersOnlyTheBadRecord(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		opts    []inbox.BatchOption
	}{
		{name: "final attempt", attempt: 3, opts: []inbox.BatchOption{inbox.WithBatchRetry(inbox.WithMaxAttempts(3))}},
		{name: "permanent error", attempt: 1, opts: []inbox.BatchOption{inbox.WithBatchRetry(
			inbox.WithPermanentClassifier(func(err error) bool { return err.Error() == "bad row" }))}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			good1, bad, good2 := &handle{}, &handle{}, &handle{}
			src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
				good1.record(natstest.ObservationFeed, observationBytes(t, "good-1"), tc.attempt),
				bad.record(natstest.ObservationFeed, observationBytes(t, "bad"), tc.attempt),
				good2.record(natstest.ObservationFeed, observationBytes(t, "good-2"), tc.attempt),
			}}}}
			dlq := &fakeDLQ{}
			r, err := inbox.NewBatchRouter(src, dlq, tc.opts...)
			require.NoError(t, err)

			var calls [][]string
			fail := failOn("bad", errors.New("bad row"))
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(ctx context.Context, b observationBatch) error {
				calls = append(calls, eventIDs(b))
				return fail(ctx, b)
			}))

			assert.ErrorIs(t, r.Run(ctx), context.Canceled)
			assert.Equal(t, [][]string{{"good-1", "bad", "good-2"}, {"good-1"}, {"bad"}, {"good-2"}}, calls)
			for _, h := range []*handle{good1, good2} {
				acks, terms, nacks := h.state()
				assert.Equal(t, 1, acks)
				assert.Zero(t, terms)
				assert.Empty(t, nacks)
			}
			acks, terms, nacks := bad.state()
			assert.Zero(t, acks)
			assert.Equal(t, 1, terms)
			assert.Empty(t, nacks)

			require.Len(t, dlq.stored, 1)
			assert.Equal(t, inbox.DeadLetterReasonHandler, dlq.stored[0].Reason)
			assert.Equal(t, natstest.ObservationFeed, dlq.stored[0].Feed)
			assert.Equal(t, "bad", dlq.stored[0].EventID)
			assert.Equal(t, tc.attempt, dlq.stored[0].Attempts)
			assert.Contains(t, dlq.stored[0].Error, "bad row")
		})
	}
}

func TestBatchRouter_SplitWithNoPassingRecordIsAnOutage(t *testing.T) {
	permanent := errors.New("permanent")
	cases := []struct {
		name          string
		errs          map[string]error
		wantDeadIDs   []string
		wantRetriedID []string
	}{
		{name: "every record fails with a transient error",
			errs:          map[string]error{"a": errors.New("down"), "b": errors.New("down")},
			wantRetriedID: []string{"a", "b"}},
		{name: "a permanent error still dead-letters its record",
			errs:          map[string]error{"a": permanent, "b": errors.New("down")},
			wantDeadIDs:   []string{"a"},
			wantRetriedID: []string{"b"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			handles := map[string]*handle{"a": {}, "b": {}}
			src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
				handles["a"].record(natstest.ObservationFeed, observationBytes(t, "a"), 4),
				handles["b"].record(natstest.ObservationFeed, observationBytes(t, "b"), 4),
			}}}}
			dlq := &fakeDLQ{}
			r, err := inbox.NewBatchRouter(src, dlq,
				inbox.WithBatchRedeliveryBackoff(time.Second, time.Minute),
				inbox.WithBatchRetry(inbox.WithPermanentClassifier(func(err error) bool { return errors.Is(err, permanent) })))
			require.NoError(t, err)
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
				if len(b) > 1 {
					return errors.New("down")
				}
				return tc.errs[b[0].Meta.GetEventId()]
			}))

			assert.ErrorIs(t, r.Run(ctx), context.Canceled)

			var dead []string
			for _, rec := range dlq.stored {
				dead = append(dead, rec.EventID)
			}
			assert.Equal(t, tc.wantDeadIDs, dead)
			for _, id := range tc.wantRetriedID {
				_, terms, nacks := handles[id].state()
				assert.Zero(t, terms, id)
				assert.Equal(t, []time.Duration{time.Minute}, nacks, "%s waits the maximum backoff", id)
			}
		})
	}
}

func TestBatchRouter_DeadLettersARecordThatDoesNotDecode(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
	}{
		{name: "bad bytes", payload: []byte{0xff, 0xff, 0xff}},
		{name: "envelope tenant is not the subject tenant", payload: natstest.Bytes(t, natstest.Observation("other", "tenant-b"))},
		{name: "envelope without a tenant", payload: natstest.Bytes(t, natstest.Observation("none", ""))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			poison, good := &handle{}, &handle{}
			src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
				poison.record(natstest.ObservationFeed, tc.payload, 1),
				good.record(natstest.ObservationFeed, observationBytes(t, "good"), 1),
			}}}}
			dlq := &fakeDLQ{}
			r, err := inbox.NewBatchRouter(src, dlq)
			require.NoError(t, err)

			var got []string
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
				got = append(got, eventIDs(b)...)
				return nil
			}))

			assert.ErrorIs(t, r.Run(ctx), context.Canceled)
			assert.Equal(t, []string{"good"}, got)
			acks, _, _ := good.state()
			assert.Equal(t, 1, acks)
			_, terms, nacks := poison.state()
			assert.Equal(t, 1, terms, "a record that does not decode is dead-lettered on its first attempt")
			assert.Empty(t, nacks)
			require.Len(t, dlq.stored, 1)
			assert.Equal(t, inbox.DeadLetterReasonDecode, dlq.stored[0].Reason)
			assert.Equal(t, natstest.ObservationFeed, dlq.stored[0].Feed)
			assert.Equal(t, tc.payload, dlq.stored[0].Payload)
		})
	}
}

func TestBatchRouter_UnroutedRecordWaitsForItsLastAttempt(t *testing.T) {
	cases := []struct {
		name     string
		feed     string
		attempt  int
		wantDead bool
	}{
		{name: "first attempt is redelivered", feed: natstest.MaintainerFeed, attempt: 1},
		{name: "last attempt is dead-lettered", feed: natstest.MaintainerFeed, attempt: 4, wantDead: true},
		{name: "a subject without a feed is dead-lettered on its last attempt", feed: "", attempt: 4, wantDead: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			h := &handle{}
			src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
				h.record(tc.feed, maintainerBytes(t, "m"), tc.attempt),
			}}}}
			dlq := &fakeDLQ{}
			r, err := inbox.NewBatchRouter(src, dlq)
			require.NoError(t, err)
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

			assert.ErrorIs(t, r.Run(ctx), context.Canceled)
			_, terms, nacks := h.state()
			if !tc.wantDead {
				assert.Empty(t, dlq.stored)
				assert.Zero(t, terms)
				assert.Len(t, nacks, 1)
				return
			}
			assert.Equal(t, 1, terms)
			require.Len(t, dlq.stored, 1)
			assert.Equal(t, inbox.DeadLetterReasonNoRoute, dlq.stored[0].Reason)
			assert.Equal(t, tc.feed, dlq.stored[0].Feed)
		})
	}
}

func TestBatchRouter_StoreFailureRedelivers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		h.record(natstest.ObservationFeed, []byte{0xff}, 1),
	}}}}
	r, err := inbox.NewBatchRouter(src, &fakeDLQ{failErr: errors.New("db down")})
	require.NoError(t, err)
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	_, terms, nacks := h.state()
	assert.Zero(t, terms)
	assert.Len(t, nacks, 1)
}

func TestBatchRouter_HeartbeatTouchesEveryOpenRecordWhileAHandlerRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	running := &handle{touched: make(chan struct{}, 1)}
	waiting := &handle{touched: make(chan struct{}, 1)}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		running.record(natstest.ObservationFeed, observationBytes(t, "running"), 1),
		waiting.record(natstest.MaintainerFeed, maintainerBytes(t, "waiting"), 1),
	}}}}
	r, err := inbox.NewBatchRouter(src, &fakeDLQ{}, inbox.WithBatchHeartbeat(time.Millisecond))
	require.NoError(t, err)

	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error {
		for _, h := range []*handle{running, waiting} {
			select {
			case <-h.touched:
			case <-time.After(5 * time.Second):
				return errors.New("no heartbeat")
			}
		}
		return nil
	}))
	require.NoError(t, inbox.Route(r, natstest.NewMaintainer, func(context.Context, maintainerBatch) error { return nil }))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	for _, h := range []*handle{running, waiting} {
		acks, _, nacks := h.state()
		assert.Equal(t, 1, acks)
		assert.Empty(t, nacks)
	}
}

func TestBatchRouter_HeartbeatTouchesRecordsBeforeTheFirstHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		h.record(natstest.ObservationFeed, observationBytes(t, "e"), 1),
	}}}}
	r, err := inbox.NewBatchRouter(src, &fakeDLQ{}, inbox.WithBatchHeartbeat(time.Hour))
	require.NoError(t, err)

	touchedBefore := 0
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		touchedBefore = h.inProgress
		return nil
	}))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	assert.Equal(t, 1, touchedBefore, "the record waited in the fetch, so its AckWait restarts before the handler")
}

func TestBatchRouter_ShutdownLeavesRecordsForAckWait(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		h.record(natstest.ObservationFeed, observationBytes(t, "e"), 4),
	}}}}
	dlq := &fakeDLQ{}
	r, err := inbox.NewBatchRouter(src, dlq)
	require.NoError(t, err)
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(ctx context.Context, _ observationBatch) error {
		cancel()
		return ctx.Err()
	}))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	acks, terms, nacks := h.state()
	assert.Zero(t, acks)
	assert.Zero(t, terms)
	assert.Empty(t, nacks, "a nack at shutdown spends a delivery")
	assert.Empty(t, dlq.stored)
}

// cancelDLQ cancels the router and fails, as a store does when the process
// stops in the middle of a write.
type cancelDLQ struct{ cancel context.CancelFunc }

func (c cancelDLQ) Store(ctx context.Context, _ inbox.DeadLetterRecord) error {
	c.cancel()
	return ctx.Err()
}

func TestBatchRouter_ShutdownDuringADeadLetterStoreLeavesTheRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		h.record(natstest.ObservationFeed, []byte{0xff}, 1),
	}}}}
	r, err := inbox.NewBatchRouter(src, cancelDLQ{cancel: cancel})
	require.NoError(t, err)
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	acks, terms, nacks := h.state()
	assert.Zero(t, acks)
	assert.Zero(t, terms)
	assert.Empty(t, nacks)
}

func TestBatchRouter_HandlerPanicIsAHandlerError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		h.record(natstest.ObservationFeed, observationBytes(t, "e"), 1),
	}}}}
	r, err := inbox.NewBatchRouter(src, &fakeDLQ{}, inbox.WithBatchRedeliveryBackoff(time.Second, time.Minute))
	require.NoError(t, err)
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error {
		panic("nil map")
	}))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	_, _, nacks := h.state()
	assert.Equal(t, []time.Duration{time.Second}, nacks)
}

func TestBatchRouter_FetchErrorRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{
		{err: errors.New("broker blip")},
		{recs: []inbox.BatchRecord{h.record(natstest.ObservationFeed, observationBytes(t, "e"), 1)}},
	}}
	r, err := inbox.NewBatchRouter(src, &fakeDLQ{}, inbox.WithBatchRestartDelay(time.Millisecond))
	require.NoError(t, err)
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	acks, _, _ := h.state()
	assert.Equal(t, 1, acks)
}

func TestBatchRouter_RefusesADeliveryLimit(t *testing.T) {
	cases := []struct {
		name    string
		limit   int
		err     error
		wantErr bool
	}{
		{name: "no limit", limit: -1},
		{name: "zero is no limit", limit: 0},
		{name: "limit above the budget", limit: 50, wantErr: true},
		{name: "limit below the budget", limit: 1, wantErr: true},
		{name: "limit lookup fails", err: errors.New("no info"), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			src := &limitedSource{fakeBatchSource: fakeBatchSource{cancel: cancel}, limit: tc.limit, err: tc.err}
			r, err := inbox.NewBatchRouter(src, &fakeDLQ{}, inbox.WithBatchRetry(inbox.WithMaxAttempts(4)))
			require.NoError(t, err)
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

			err = r.Run(ctx)
			if tc.wantErr {
				assert.Error(t, err)
				assert.NotErrorIs(t, err, context.Canceled)
				return
			}
			assert.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestBatchRouter_Validation(t *testing.T) {
	ok := func(context.Context, observationBatch) error { return nil }

	_, err := inbox.NewBatchRouter(&fakeBatchSource{}, nil)
	assert.Error(t, err, "no dead-letter store")

	r, err := inbox.NewBatchRouter(&fakeBatchSource{}, &fakeDLQ{})
	require.NoError(t, err)
	assert.Error(t, r.Run(t.Context()), "no routes")

	require.NoError(t, inbox.Route(r, natstest.NewObservation, ok))
	assert.Error(t, inbox.Route(r, natstest.NewObservation, ok), "a second route for one feed")
	assert.Error(t, inbox.Route[*pkgregv1.PackageVersionObservationEvent](nil, natstest.NewObservation, ok))
	assert.Error(t, inbox.Route[*pkgregv1.PackageVersionObservationEvent](r, nil, ok))
	assert.Error(t, inbox.Route(r, natstest.NewObservation, nil))
	assert.Error(t, inbox.Route(r, func() *pkgregv1.PackageVersionObservationEvent { return nil }, ok))
	assert.Error(t, inbox.Route(r, func() *commonv1.EventMeta { return &commonv1.EventMeta{} },
		func(context.Context, []inbox.BatchEvent[*commonv1.EventMeta]) error { return nil }),
		"a message that is not an event")

	noSource, err := inbox.NewBatchRouter(nil, &fakeDLQ{})
	require.NoError(t, err)
	require.NoError(t, inbox.Route(noSource, natstest.NewObservation, ok))
	assert.Error(t, noSource.Run(t.Context()), "no source")
}

func TestBatchRouter_ReplaysBatchDeadLetters(t *testing.T) {
	adapter := newTestAdapter(t)
	dlq, err := inbox.NewGormDeadLetter(adapter, "writer")
	require.NoError(t, err)
	reader := newDeadLetterReader(t, adapter, "writer")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	noRoute, bad, good := &handle{}, &handle{}, &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		noRoute.record(natstest.MaintainerFeed, maintainerBytes(t, "mnt-1"), 4),
		bad.record(natstest.ObservationFeed, observationBytes(t, "obs-bad"), 4),
		good.record(natstest.ObservationFeed, observationBytes(t, "obs-good"), 4),
	}}}}
	writer, err := inbox.NewBatchRouter(src, dlq)
	require.NoError(t, err)
	require.NoError(t, inbox.Route(writer, natstest.NewObservation, failOn("obs-bad", errors.New("bad row"))))
	assert.ErrorIs(t, writer.Run(ctx), context.Canceled)

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{Reason: inbox.DeadLetterReasonNoRoute, Feed: natstest.MaintainerFeed})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = reader.Count(t.Context(), inbox.DeadLetterFilter{Reason: inbox.DeadLetterReasonHandler, Feed: natstest.ObservationFeed})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// A new release routes the maintainer feed. The observation record still fails.
	cli, err := inbox.NewBatchRouter(nil, dlq)
	require.NoError(t, err)
	require.NoError(t, inbox.Route(cli, natstest.NewObservation, func(context.Context, observationBatch) error {
		return errors.New("still bad")
	}))
	var replayed []string
	require.NoError(t, inbox.Route(cli, natstest.NewMaintainer, func(_ context.Context, b maintainerBatch) error {
		replayed = append(replayed, eventIDs(b)...)
		return nil
	}))

	res, err := cli.Replay(t.Context(), reader, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Succeeded)
	assert.Equal(t, 1, res.Failed)
	assert.Equal(t, []string{"mnt-1"}, replayed)

	rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1, "the replayed dead letter is deleted")
	assert.Equal(t, "obs-bad", rows[0].EventID)
	assert.Equal(t, inbox.DeadLetterReasonHandler, rows[0].Reason)
	assert.Equal(t, 5, rows[0].Attempts, "the failed replay counts as an attempt")
	assert.Contains(t, rows[0].Error, "still bad")
}

func TestBatchRouter_ReplayKeepsTheSubjectTenantCheck(t *testing.T) {
	adapter := newTestAdapter(t)
	dlq, err := inbox.NewGormDeadLetter(adapter, "writer")
	require.NoError(t, err)
	reader := newDeadLetterReader(t, adapter, "writer")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wrong := &handle{}
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		wrong.record(natstest.ObservationFeed, natstest.Bytes(t, natstest.Observation("other", "tenant-b")), 1),
	}}}}
	writer, err := inbox.NewBatchRouter(src, dlq)
	require.NoError(t, err)
	var handled []string
	handler := func(_ context.Context, b observationBatch) error {
		handled = append(handled, eventIDs(b)...)
		return nil
	}
	require.NoError(t, inbox.Route(writer, natstest.NewObservation, handler))
	assert.ErrorIs(t, writer.Run(ctx), context.Canceled)

	rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{Reason: inbox.DeadLetterReasonDecode})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, natstest.ObservationFeed+"."+natstest.Tenant, rows[0].Subject)
	assert.Equal(t, natstest.Tenant, rows[0].Tenant)

	res, err := writer.Replay(t.Context(), reader, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)
	assert.Empty(t, handled, "the handler never sees an event of another tenant")

	row, err := reader.Get(t.Context(), rows[0].ID)
	require.NoError(t, err)
	assert.Equal(t, 2, row.Attempts)
	assert.Contains(t, row.Error, "tenant")
}

func TestBatchRouter_ReplayRequiresTheStoredTenant(t *testing.T) {
	cases := []struct {
		name   string
		tenant string
		want   string
	}{
		{name: "a tenant without a subject and another envelope tenant", tenant: "tenant-b", want: "tenant"},
		{name: "no stored tenant", tenant: "", want: "no source tenant"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := newTestAdapter(t)
			seedDeadLetter(t, adapter, "writer", inbox.DeadLetterRecord{
				Payload:  observationBytes(t, "e"),
				Feed:     natstest.ObservationFeed,
				Tenant:   tc.tenant,
				Reason:   inbox.DeadLetterReasonHandler,
				Attempts: 4,
			})
			reader := newDeadLetterReader(t, adapter, "writer")

			r, err := inbox.NewBatchRouter(nil, &fakeDLQ{})
			require.NoError(t, err)
			var handled []string
			require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
				handled = append(handled, eventIDs(b)...)
				return nil
			}))

			res, err := r.Replay(t.Context(), reader, inbox.DeadLetterFilter{})
			require.NoError(t, err)
			assert.Equal(t, 1, res.Failed)
			assert.Empty(t, handled)

			row, err := reader.Get(t.Context(), res.Outcomes[0].ID)
			require.NoError(t, err)
			assert.Equal(t, 5, row.Attempts)
			assert.Equal(t, inbox.DeadLetterReasonDecode, row.Reason)
			assert.Contains(t, row.Error, tc.want)
		})
	}
}

func TestBatchRouter_SplitPairsEachRecordWithItsOwnEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bad, good1, good2 := &handle{}, &handle{}, &handle{}
	badPayload := observationBytes(t, "bad")
	src := &fakeBatchSource{cancel: cancel, steps: []fetchStep{{recs: []inbox.BatchRecord{
		bad.record(natstest.ObservationFeed, badPayload, 4),
		good1.record(natstest.ObservationFeed, observationBytes(t, "good-1"), 4),
		good2.record(natstest.ObservationFeed, observationBytes(t, "good-2"), 4),
	}}}}
	dlq := &fakeDLQ{}
	r, err := inbox.NewBatchRouter(src, dlq)
	require.NoError(t, err)

	// The handler reverses the batch and changes an event before it fails.
	require.NoError(t, inbox.Route(r, natstest.NewObservation, func(_ context.Context, b observationBatch) error {
		slices.Reverse(b)
		ids := eventIDs(b)
		b[0].Meta.SetEventId("changed")
		if len(b) > 1 || ids[0] == "bad" {
			return errors.New("bad row")
		}
		return nil
	}))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	_, terms, _ := bad.state()
	assert.Equal(t, 1, terms)
	for _, h := range []*handle{good1, good2} {
		acks, terms, _ := h.state()
		assert.Equal(t, 1, acks)
		assert.Zero(t, terms)
	}
	require.Len(t, dlq.stored, 1)
	assert.Equal(t, "bad", dlq.stored[0].EventID)
	assert.Equal(t, badPayload, dlq.stored[0].Payload)
}

func TestBatchRouter_ReplayKeepsADeadLetterWithoutARoute(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "writer", inbox.DeadLetterRecord{
		Payload: maintainerBytes(t, "mnt-1"), Feed: natstest.MaintainerFeed, Reason: inbox.DeadLetterReasonNoRoute, Attempts: 4,
	})
	reader := newDeadLetterReader(t, adapter, "writer")

	cli, err := inbox.NewBatchRouter(nil, &fakeDLQ{})
	require.NoError(t, err)
	require.NoError(t, inbox.Route(cli, natstest.NewObservation, func(context.Context, observationBatch) error { return nil }))

	res, err := cli.Replay(t.Context(), reader, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)

	row, err := reader.Get(t.Context(), res.Outcomes[0].ID)
	require.NoError(t, err)
	assert.Equal(t, inbox.DeadLetterReasonNoRoute, row.Reason)
	assert.Equal(t, 4, row.Attempts)
}
