package inbox_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	observationFeed = "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent"
	maintainerFeed  = "safedep.events.private.packageregistry.v1.MaintainerObservationEvent"
)

type observationBatch = []inbox.BatchEvent[*pkgregv1.PackageVersionObservationEvent]
type maintainerBatch = []inbox.BatchEvent[*pkgregv1.MaintainerObservationEvent]

func newMaintainer() *pkgregv1.MaintainerObservationEvent {
	return &pkgregv1.MaintainerObservationEvent{}
}

func maintainerBytes(t *testing.T, eventID string) []byte {
	t.Helper()
	b, err := proto.Marshal(pkgregv1.MaintainerObservationEvent_builder{
		Meta: events.NewMeta(events.WithEventID(eventID)),
	}.Build())
	require.NoError(t, err)
	return b
}

// handle records what the router did with one fake record.
type handle struct {
	mu         sync.Mutex
	acks       int
	terms      int
	inProgress int
	naks       []time.Duration
}

func (h *handle) record(subject string, payload []byte, attempt int) inbox.BatchRecord {
	return inbox.BatchRecord{
		Subject: subject,
		Payload: payload,
		Attempt: attempt,
		Ack:     func() error { h.mu.Lock(); defer h.mu.Unlock(); h.acks++; return nil },
		Nak: func(d time.Duration) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.naks = append(h.naks, d)
			return nil
		},
		Term:       func() error { h.mu.Lock(); defer h.mu.Unlock(); h.terms++; return nil },
		InProgress: func() error { h.mu.Lock(); defer h.mu.Unlock(); h.inProgress++; return nil },
	}
}

func (h *handle) state() (acks, terms, naks int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.acks, h.terms, len(h.naks)
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

func runRouter(t *testing.T, steps []fetchStep, register func(r *inbox.BatchRouter), opts ...inbox.BatchOption) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	r := inbox.NewBatchRouter(&fakeBatchSource{steps: steps, cancel: cancel}, opts...)
	register(r)
	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
}

func eventIDs[T proto.Message](batch []inbox.BatchEvent[T]) []string {
	ids := make([]string, len(batch))
	for i, ev := range batch {
		ids[i] = ev.Meta.GetEventId()
	}
	return ids
}

func TestBatchRouter_AcksOnlyTheGroupThatSucceeds(t *testing.T) {
	obs1, obs2, mnt := &handle{}, &handle{}, &handle{}
	steps := []fetchStep{{recs: []inbox.BatchRecord{
		obs1.record(observationFeed+".t1", eventBytes(t, "obs-1"), 1),
		mnt.record(maintainerFeed+".t1", maintainerBytes(t, "mnt-1"), 1),
		obs2.record(observationFeed+".t2", eventBytes(t, "obs-2"), 1),
	}}}

	var gotObs, gotMnt []string
	runRouter(t, steps, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
			gotObs = append(gotObs, eventIDs(b)...)
			return nil
		}))
		require.NoError(t, inbox.Route(r, newMaintainer, func(_ context.Context, b maintainerBatch) error {
			gotMnt = append(gotMnt, eventIDs(b)...)
			return errors.New("insert failed")
		}))
	}, inbox.WithBatchBackoff(time.Second, time.Minute))

	assert.Equal(t, []string{"obs-1", "obs-2"}, gotObs, "one handler call with both records of the feed")
	assert.Equal(t, []string{"mnt-1"}, gotMnt)

	for _, h := range []*handle{obs1, obs2} {
		acks, terms, naks := h.state()
		assert.Equal(t, []int{1, 0, 0}, []int{acks, terms, naks}, "the feed that succeeded is acked")
	}
	acks, terms, _ := mnt.state()
	assert.Equal(t, []int{0, 0}, []int{acks, terms})
	assert.Equal(t, []time.Duration{time.Second}, mnt.naks, "the failed feed is redelivered with the first backoff")
}

func TestBatchRouter_NakDelayGrowsWithAttempt(t *testing.T) {
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
		h := &handle{}
		runRouter(t, []fetchStep{{recs: []inbox.BatchRecord{h.record(observationFeed+".t1", eventBytes(t, "e"), tc.attempt)}}},
			func(r *inbox.BatchRouter) {
				require.NoError(t, inbox.Route(r, newObservation, func(context.Context, observationBatch) error {
					return errors.New("down")
				}))
			},
			inbox.WithBatchBackoff(time.Second, 10*time.Second),
			inbox.WithBatchRetry(inbox.WithMaxAttempts(100)))
		assert.Equal(t, []time.Duration{tc.want}, h.naks, "attempt %d", tc.attempt)
	}
}

func TestBatchRouter_SplitsAndDeadLettersOnlyTheBadRecord(t *testing.T) {
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
			good1, bad, good2 := &handle{}, &handle{}, &handle{}
			steps := []fetchStep{{recs: []inbox.BatchRecord{
				good1.record(observationFeed+".t1", eventBytes(t, "good-1"), tc.attempt),
				bad.record(observationFeed+".t1", eventBytes(t, "bad"), tc.attempt),
				good2.record(observationFeed+".t1", eventBytes(t, "good-2"), tc.attempt),
			}}}

			var calls [][]string
			dlq := &fakeDLQ{}
			runRouter(t, steps, func(r *inbox.BatchRouter) {
				require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
					ids := eventIDs(b)
					calls = append(calls, ids)
					for _, id := range ids {
						if id == "bad" {
							return errors.New("bad row")
						}
					}
					return nil
				}))
			}, append(tc.opts, inbox.WithBatchDeadLetter(dlq))...)

			assert.Equal(t, [][]string{{"good-1", "bad", "good-2"}, {"good-1"}, {"bad"}, {"good-2"}}, calls)
			for _, h := range []*handle{good1, good2} {
				acks, terms, naks := h.state()
				assert.Equal(t, []int{1, 0, 0}, []int{acks, terms, naks})
			}
			acks, terms, naks := bad.state()
			assert.Equal(t, []int{0, 1, 0}, []int{acks, terms, naks})

			require.Len(t, dlq.stored, 1)
			assert.Equal(t, inbox.DeadLetterReasonHandler, dlq.stored[0].Reason)
			assert.Equal(t, observationFeed, dlq.stored[0].Feed)
			assert.Equal(t, "bad", dlq.stored[0].EventID)
			assert.Equal(t, tc.attempt, dlq.stored[0].Attempts)
			assert.Contains(t, dlq.stored[0].Error, "bad row")
		})
	}
}

func TestBatchRouter_SplitNaksARecordThatFailsBeforeItsFinalAttempt(t *testing.T) {
	final, early := &handle{}, &handle{}
	steps := []fetchStep{{recs: []inbox.BatchRecord{
		final.record(observationFeed+".t1", eventBytes(t, "final"), 3),
		early.record(observationFeed+".t1", eventBytes(t, "early"), 1),
	}}}

	dlq := &fakeDLQ{}
	runRouter(t, steps, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(context.Context, observationBatch) error {
			return errors.New("down")
		}))
	}, inbox.WithBatchDeadLetter(dlq), inbox.WithBatchRetry(inbox.WithMaxAttempts(3)))

	_, terms, _ := final.state()
	assert.Equal(t, 1, terms)
	_, terms, naks := early.state()
	assert.Equal(t, []int{0, 1}, []int{terms, naks}, "a record with attempts left is redelivered, not dead-lettered")
	assert.Len(t, dlq.stored, 1)
}

func TestBatchRouter_DeadLettersOneRecordThatDoesNotDecodeOrRoute(t *testing.T) {
	cases := []struct {
		name       string
		subject    string
		payload    []byte
		wantReason inbox.DeadLetterReason
		wantFeed   string
	}{
		{name: "decode failure", subject: observationFeed + ".t1", payload: []byte{0xff, 0xff, 0xff},
			wantReason: inbox.DeadLetterReasonDecode, wantFeed: observationFeed},
		{name: "unrouted subject", subject: maintainerFeed + ".t1", payload: maintainerBytes(t, "m"),
			wantReason: inbox.DeadLetterReasonNoRoute, wantFeed: maintainerFeed},
		{name: "subject without a tenant token", subject: "plain", payload: eventBytes(t, "p"),
			wantReason: inbox.DeadLetterReasonNoRoute, wantFeed: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			poison, good := &handle{}, &handle{}
			steps := []fetchStep{{recs: []inbox.BatchRecord{
				poison.record(tc.subject, tc.payload, 1),
				good.record(observationFeed+".t1", eventBytes(t, "good"), 1),
			}}}

			var got []string
			dlq := &fakeDLQ{}
			runRouter(t, steps, func(r *inbox.BatchRouter) {
				require.NoError(t, inbox.Route(r, newObservation, func(_ context.Context, b observationBatch) error {
					got = append(got, eventIDs(b)...)
					return nil
				}))
			}, inbox.WithBatchDeadLetter(dlq))

			assert.Equal(t, []string{"good"}, got)
			acks, _, _ := good.state()
			assert.Equal(t, 1, acks)
			acks, terms, naks := poison.state()
			assert.Equal(t, []int{0, 1, 0}, []int{acks, terms, naks})
			require.Len(t, dlq.stored, 1)
			assert.Equal(t, tc.wantReason, dlq.stored[0].Reason)
			assert.Equal(t, tc.wantFeed, dlq.stored[0].Feed)
			assert.Equal(t, tc.payload, dlq.stored[0].Payload)
		})
	}
}

func TestBatchRouter_DeadLetterFallbacks(t *testing.T) {
	cases := []struct {
		name      string
		dlq       inbox.DeadLetterQueue
		wantTerms int
		wantNaks  int
	}{
		{name: "no store terminates", dlq: nil, wantTerms: 1},
		{name: "store failure redelivers", dlq: &fakeDLQ{failErr: errors.New("db down")}, wantNaks: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &handle{}
			steps := []fetchStep{{recs: []inbox.BatchRecord{h.record(observationFeed+".t1", []byte{0xff}, 1)}}}
			var opts []inbox.BatchOption
			if tc.dlq != nil {
				opts = append(opts, inbox.WithBatchDeadLetter(tc.dlq))
			}
			runRouter(t, steps, func(r *inbox.BatchRouter) {
				require.NoError(t, inbox.Route(r, newObservation, func(context.Context, observationBatch) error { return nil }))
			}, opts...)

			_, terms, naks := h.state()
			assert.Equal(t, tc.wantTerms, terms)
			assert.Equal(t, tc.wantNaks, naks)
		})
	}
}

func TestBatchRouter_HeartbeatMarksRecordsInProgressWhileTheHandlerRuns(t *testing.T) {
	slow, waiting := &handle{}, &handle{}
	steps := []fetchStep{{recs: []inbox.BatchRecord{
		slow.record(observationFeed+".t1", eventBytes(t, "slow"), 1),
		waiting.record(maintainerFeed+".t1", maintainerBytes(t, "waiting"), 1),
	}}}

	runRouter(t, steps, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(context.Context, observationBatch) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		}))
		require.NoError(t, inbox.Route(r, newMaintainer, func(context.Context, maintainerBatch) error { return nil }))
	}, inbox.WithBatchHeartbeat(20*time.Millisecond))

	slow.mu.Lock()
	defer slow.mu.Unlock()
	waiting.mu.Lock()
	defer waiting.mu.Unlock()
	assert.GreaterOrEqual(t, slow.inProgress, 3)
	assert.GreaterOrEqual(t, waiting.inProgress, 3, "a record that waits for another feed is in flight too")
	assert.Equal(t, 1, slow.acks)
	assert.Equal(t, 1, waiting.acks)
}

func TestBatchRouter_FetchErrorRetries(t *testing.T) {
	h := &handle{}
	steps := []fetchStep{
		{err: errors.New("broker blip")},
		{recs: []inbox.BatchRecord{h.record(observationFeed+".t1", eventBytes(t, "e"), 1)}},
	}

	runRouter(t, steps, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(context.Context, observationBatch) error { return nil }))
	}, inbox.WithBatchFetchRetryDelay(time.Millisecond))

	acks, _, _ := h.state()
	assert.Equal(t, 1, acks)
}

func TestBatchRouter_CancelDuringHandlerRedeliversAtOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	h := &handle{}
	src := &fakeBatchSource{steps: []fetchStep{{recs: []inbox.BatchRecord{h.record(observationFeed+".t1", eventBytes(t, "e"), 1)}}}, cancel: cancel}
	r := inbox.NewBatchRouter(src)
	require.NoError(t, inbox.Route(r, newObservation, func(ctx context.Context, _ observationBatch) error {
		cancel()
		return ctx.Err()
	}))

	assert.ErrorIs(t, r.Run(ctx), context.Canceled)
	assert.Equal(t, []time.Duration{0}, h.naks)
}

func TestRoute_Validation(t *testing.T) {
	ok := func(context.Context, observationBatch) error { return nil }

	r := inbox.NewBatchRouter(&fakeBatchSource{})
	require.NoError(t, inbox.Route(r, newObservation, ok))
	assert.Error(t, inbox.Route(r, newObservation, ok), "a second route for one feed")
	assert.Error(t, inbox.Route[*pkgregv1.PackageVersionObservationEvent](nil, newObservation, ok))
	assert.Error(t, inbox.Route[*pkgregv1.PackageVersionObservationEvent](r, nil, ok))
	assert.Error(t, inbox.Route(r, newObservation, nil))
	assert.Error(t, inbox.Route(r, func() *pkgregv1.PackageVersionObservationEvent { return nil }, ok))
	assert.Error(t, inbox.Route(r, func() *commonv1.EventMeta { return &commonv1.EventMeta{} },
		func(context.Context, []inbox.BatchEvent[*commonv1.EventMeta]) error { return nil }),
		"a message that is not an event")

	assert.Error(t, inbox.NewBatchRouter(nil).Run(t.Context()), "no source")
	assert.Error(t, inbox.NewBatchRouter(&fakeBatchSource{}).Run(t.Context()), "no routes")
}

func TestBatchRouter_ReplaysBatchDeadLetters(t *testing.T) {
	adapter := newTestAdapter(t)
	dlq, err := inbox.NewGormDeadLetter(adapter, "writer")
	require.NoError(t, err)
	reader := newDeadLetterReader(t, adapter, "writer")

	noRoute, bad := &handle{}, &handle{}
	steps := []fetchStep{{recs: []inbox.BatchRecord{
		noRoute.record(maintainerFeed+".t1", maintainerBytes(t, "mnt-1"), 1),
		bad.record(observationFeed+".t1", eventBytes(t, "obs-1"), 4),
	}}}
	runRouter(t, steps, func(r *inbox.BatchRouter) {
		require.NoError(t, inbox.Route(r, newObservation, func(context.Context, observationBatch) error {
			return errors.New("bad row")
		}))
	}, inbox.WithBatchDeadLetter(dlq))

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{Reason: inbox.DeadLetterReasonNoRoute})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = reader.Count(t.Context(), inbox.DeadLetterFilter{Reason: inbox.DeadLetterReasonHandler, Feed: observationFeed})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// The CLI registers the routes of the writer. The maintainer feed now has a
	// route and its dead letter clears. The observation handler still fails.
	var replayed []string
	cli := inbox.NewBatchRouter(nil)
	require.NoError(t, inbox.Route(cli, newObservation, func(context.Context, observationBatch) error {
		return errors.New("still bad")
	}))
	require.NoError(t, inbox.Route(cli, newMaintainer, func(_ context.Context, b maintainerBatch) error {
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
	assert.Equal(t, observationFeed, rows[0].Feed)
	assert.Equal(t, inbox.DeadLetterReasonHandler, rows[0].Reason)
	assert.Equal(t, 5, rows[0].Attempts, "the failed replay counts as an attempt")
	assert.Contains(t, rows[0].Error, "still bad")
}

func TestBatchRouter_ReplayKeepsADeadLetterWithoutARoute(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "writer", inbox.DeadLetterRecord{
		Payload: maintainerBytes(t, "mnt-1"), Feed: maintainerFeed, Reason: inbox.DeadLetterReasonNoRoute, Attempts: 1,
	})
	reader := newDeadLetterReader(t, adapter, "writer")

	cli := inbox.NewBatchRouter(nil)
	require.NoError(t, inbox.Route(cli, newObservation, func(context.Context, observationBatch) error { return nil }))

	res, err := cli.Replay(t.Context(), reader, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)

	row, err := reader.Get(t.Context(), res.Outcomes[0].ID)
	require.NoError(t, err)
	assert.Equal(t, inbox.DeadLetterReasonNoRoute, row.Reason)
	assert.Equal(t, 1, row.Attempts)
}
