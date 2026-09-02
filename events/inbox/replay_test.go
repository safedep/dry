package inbox_test

import (
	"context"
	"errors"
	"testing"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/safedep/dry/db"
	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDeadLetterReader(t *testing.T, adapter db.SqlDataAdapter, consumerName string) inbox.DeadLetterReader {
	t.Helper()
	r, err := inbox.NewGormDeadLetterReader(adapter, consumerName)
	require.NoError(t, err)
	return r
}

func TestReplay_Validation(t *testing.T) {
	reader := newDeadLetterReader(t, newTestAdapter(t), "consumer-a")
	ok := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		return nil
	}

	_, err := inbox.Replay(t.Context(), nil, newObservation, ok, inbox.DeadLetterFilter{})
	require.Error(t, err, "nil reader")

	_, err = inbox.Replay[*pkgregv1.PackageVersionObservationEvent](t.Context(), reader, nil, ok, inbox.DeadLetterFilter{})
	require.Error(t, err, "nil newEvent")

	_, err = inbox.Replay(t.Context(), reader, newObservation, nil, inbox.DeadLetterFilter{})
	require.Error(t, err, "nil handler")

	boxedNil := func() *pkgregv1.PackageVersionObservationEvent { return nil }
	_, err = inbox.Replay(t.Context(), reader, boxedNil, ok, inbox.DeadLetterFilter{})
	require.Error(t, err, "newEvent returning a nil message")
}

func TestReplay_SucceedsAndDeletes(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{
		Payload: eventBytes(t, "evt-1"), Feed: "feed.X", EventID: "evt-1",
	})
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	var seen []string
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, meta *commonv1.EventMeta) error {
		seen = append(seen, meta.GetEventId())
		return nil
	}

	res, err := inbox.Replay(t.Context(), reader, newObservation, handler, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Succeeded)
	assert.Equal(t, 0, res.Failed)
	require.Len(t, res.Outcomes, 1)
	assert.NoError(t, res.Outcomes[0].Err)
	assert.Equal(t, []string{"evt-1"}, seen)

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "a replayed record is removed")
}

func TestReplay_HandlerFailureKeepsRow(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{
		Payload: eventBytes(t, "evt-1"), EventID: "evt-1",
	})
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		return errors.New("still failing")
	}

	res, err := inbox.Replay(t.Context(), reader, newObservation, handler, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Succeeded)
	assert.Equal(t, 1, res.Failed)
	require.Len(t, res.Outcomes, 1)
	require.Error(t, res.Outcomes[0].Err)
	assert.Contains(t, res.Outcomes[0].Err.Error(), "handler")

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "a record whose handler still fails stays parked")
}

func TestReplay_DecodeFailureKeepsRow(t *testing.T) {
	adapter := newTestAdapter(t)
	// A truncated varint tag: proto.Unmarshal cannot decode it. This is the
	// decode-poison class a generic dead-letter never gets an event_id for.
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{Payload: []byte{0xff}})
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	called := false
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		called = true
		return nil
	}

	res, err := inbox.Replay(t.Context(), reader, newObservation, handler, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)
	assert.False(t, called, "handler is not reached when the payload cannot decode")
	require.Len(t, res.Outcomes, 1)
	assert.Contains(t, res.Outcomes[0].Err.Error(), "decode")

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestReplay_ScopedToConsumer(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{Payload: eventBytes(t, "a"), EventID: "a"})
	seedDeadLetter(t, adapter, "consumer-b", inbox.DeadLetterRecord{Payload: eventBytes(t, "b"), EventID: "b"})

	var seen []string
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, meta *commonv1.EventMeta) error {
		seen = append(seen, meta.GetEventId())
		return nil
	}

	res, err := inbox.Replay(t.Context(), newDeadLetterReader(t, adapter, "consumer-a"),
		newObservation, handler, inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Succeeded)
	assert.Equal(t, []string{"a"}, seen, "only consumer-a's record replayed")

	// consumer-b's record is untouched.
	nB, err := newDeadLetterReader(t, adapter, "consumer-b").Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), nB)
}

func TestReplay_DrainsAllPages(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e1"), EventID: "e1"},
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e2"), EventID: "e2"},
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e3"), EventID: "e3"},
	)
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	count := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		count++
		return nil
	}

	// Limit 1 forces the internal drain loop to page across every record.
	res, err := inbox.Replay(t.Context(), reader, newObservation, handler, inbox.DeadLetterFilter{Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, 3, res.Succeeded)
	assert.Equal(t, 3, count)

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestReplay_AlreadyCancelledContextDoesNothing(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{Payload: eventBytes(t, "e1"), EventID: "e1"})
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	called := false
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		called = true
		return nil
	}

	res, err := inbox.Replay(ctx, reader, newObservation, handler, inbox.DeadLetterFilter{})
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called, "no handler runs under an already-cancelled context")
	assert.Equal(t, 0, res.Succeeded)

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "the record stays parked")
}

// Cancelling mid-drain stops before the next buffered row: a context-unaware
// handler must not keep running side effects after the caller cancels.
func TestReplay_StopsMidPageOnCancellation(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e1"), EventID: "e1"},
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e2"), EventID: "e2"},
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e3"), EventID: "e3"},
	)
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	// Default limit pulls all three into one page, so the per-row guard — not a
	// List error on the next page — is what stops the loop.
	ctx, cancel := context.WithCancel(t.Context())
	count := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		count++
		cancel()
		return nil
	}

	res, err := inbox.Replay(ctx, reader, newObservation, handler, inbox.DeadLetterFilter{})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, count, "handler runs for the cancelling row, then the loop stops")
	assert.Len(t, res.Outcomes, 1, "no outcome recorded for the skipped rows")
}

// A permanently-failing record between good ones must not stall the drain: Replay
// advances past it (rather than re-listing it forever) and still clears the rest.
func TestReplay_AdvancesPastPersistentFailure(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e1"), EventID: "e1"},
		inbox.DeadLetterRecord{Payload: []byte{0xff}}, // never decodes
		inbox.DeadLetterRecord{Payload: eventBytes(t, "e3"), EventID: "e3"},
	)
	reader := newDeadLetterReader(t, adapter, "consumer-a")

	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		return nil
	}

	// Limit 1 makes the poison record its own page; the drain must still terminate.
	res, err := inbox.Replay(t.Context(), reader, newObservation, handler, inbox.DeadLetterFilter{Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Succeeded)
	assert.Equal(t, 1, res.Failed)

	remaining, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), remaining, "only the undecodable record stays parked")
}
