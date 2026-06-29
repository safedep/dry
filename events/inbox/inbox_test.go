package inbox_test

import (
	"context"
	"errors"
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

// observation is a convenient concrete <Feed>Event for the loop tests: it carries
// EventMeta at field 1, so MetaOf works and we can stamp an event_id.
func newObservation() *pkgregv1.PackageVersionObservationEvent {
	return &pkgregv1.PackageVersionObservationEvent{}
}

func eventBytes(t *testing.T, eventID string) []byte {
	t.Helper()
	ev := pkgregv1.PackageVersionObservationEvent_builder{
		Meta: events.NewMeta(events.WithEventID(eventID)),
	}.Build()
	b, err := proto.Marshal(ev)
	require.NoError(t, err)
	return b
}

// step scripts one fakeSource.Receive call: either a transport error, or a
// delivery carrying payload (with optional Ack/Nack errors).
type step struct {
	payload []byte
	err     error
	ackErr  error
	nackErr error
}

type fakeSource struct {
	steps []step
	i     int
	acks  int
	nacks int
}

func (f *fakeSource) Receive(_ context.Context) (*inbox.Delivery, error) {
	if f.i >= len(f.steps) {
		// Drained: behave like a cancelled context so Consume returns cleanly.
		return nil, context.Canceled
	}
	s := f.steps[f.i]
	f.i++
	if s.err != nil {
		return nil, s.err
	}
	return &inbox.Delivery{
		Payload: s.payload,
		Ack:     func() error { f.acks++; return s.ackErr },
		Nack:    func() error { f.nacks++; return s.nackErr },
	}, nil
}

type fakeDedup struct{ marked map[string]bool }

func newFakeDedup() *fakeDedup { return &fakeDedup{marked: map[string]bool{}} }

func (f *fakeDedup) Seen(_ context.Context, id string) (bool, error) { return f.marked[id], nil }
func (f *fakeDedup) Mark(_ context.Context, id string) error        { f.marked[id] = true; return nil }

func TestConsume_HandleThenAck(t *testing.T) {
	src := &fakeSource{steps: []step{
		{payload: eventBytes(t, "evt-1")},
		{payload: eventBytes(t, "evt-2")},
	}}

	var got []string
	handler := func(_ context.Context, ev *pkgregv1.PackageVersionObservationEvent, meta *commonv1.EventMeta) error {
		got = append(got, meta.GetEventId())
		return nil
	}

	err := inbox.Consume(t.Context(), src, newObservation, handler)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"evt-1", "evt-2"}, got)
	assert.Equal(t, 2, src.acks)
	assert.Equal(t, 0, src.nacks)
}

func TestConsume_HandlerErrorNacks(t *testing.T) {
	src := &fakeSource{steps: []step{{payload: eventBytes(t, "evt-1")}}}

	calls := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		calls++
		return errors.New("boom")
	}

	err := inbox.Consume(t.Context(), src, newObservation, handler)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 0, src.acks)
	assert.Equal(t, 1, src.nacks)
}

func TestConsume_DecodeErrorNacksWithoutHandler(t *testing.T) {
	src := &fakeSource{steps: []step{{payload: []byte{0xff, 0xff, 0xff}}}}

	calls := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		calls++
		return nil
	}

	err := inbox.Consume(t.Context(), src, newObservation, handler)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, calls, "handler must not run on an undecodable record")
	assert.Equal(t, 1, src.nacks)
	assert.Equal(t, 0, src.acks)
}

func TestConsume_ErrorHandlerSkipAcks(t *testing.T) {
	src := &fakeSource{steps: []step{{payload: eventBytes(t, "evt-1")}}}

	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		return errors.New("permanent")
	}
	skip := func(_ context.Context, _ *inbox.Delivery, _ error) inbox.Disposition { return inbox.Skip }

	err := inbox.Consume(t.Context(), src, newObservation, handler, inbox.WithErrorHandler(skip))
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, src.acks, "Skip advances past the record")
	assert.Equal(t, 0, src.nacks)
}

func TestConsume_TransientReceiveErrorRetries(t *testing.T) {
	src := &fakeSource{steps: []step{
		{err: errors.New("session dropped")},
		{payload: eventBytes(t, "evt-1")},
	}}

	calls := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		calls++
		return nil
	}

	err := inbox.Consume(t.Context(), src, newObservation, handler, inbox.WithRestartDelay(time.Millisecond))
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls, "the record after a transient error is still delivered")
	assert.Equal(t, 1, src.acks)
}

func TestConsume_CtxErrorReturnsWithoutHandling(t *testing.T) {
	src := &fakeSource{steps: []step{{err: context.Canceled}}}

	calls := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		calls++
		return nil
	}

	err := inbox.Consume(t.Context(), src, newObservation, handler)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, calls)
}

func TestConsume_DedupSkipsDuplicateEventID(t *testing.T) {
	// Same event_id delivered twice: the handler runs once; the duplicate is
	// acked and skipped.
	payload := eventBytes(t, "evt-dup")
	src := &fakeSource{steps: []step{{payload: payload}, {payload: payload}}}

	calls := 0
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		calls++
		return nil
	}

	err := inbox.Consume(t.Context(), src, newObservation, handler, inbox.WithDedup(newFakeDedup()))
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls, "duplicate event_id must not re-run the handler")
	assert.Equal(t, 2, src.acks, "both the original and the skipped duplicate advance")
}

func TestConsume_Validation(t *testing.T) {
	handler := func(_ context.Context, _ *pkgregv1.PackageVersionObservationEvent, _ *commonv1.EventMeta) error {
		return nil
	}

	assert.Error(t, inbox.Consume(t.Context(), nil, newObservation, handler))
	assert.Error(t, inbox.Consume[*pkgregv1.PackageVersionObservationEvent](t.Context(), &fakeSource{}, nil, handler))
	assert.Error(t, inbox.Consume(t.Context(), &fakeSource{}, newObservation, nil))
}
