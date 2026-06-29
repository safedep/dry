package sources

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func routingX() events.Routing {
	return events.Routing{
		Exposure: events.ExposurePrivate,
		Domain:   "packageregistry",
		Major:    1,
		Message:  "PackageVersionObservationEvent",
	}
}

type fakeSession struct {
	records []*stream.StreamRecord
	i       int
	nextErr error // returned once records are exhausted (nil -> io.EOF)
	closed  bool
}

func (f *fakeSession) Next() (*stream.StreamRecord, error) {
	if f.i < len(f.records) {
		r := f.records[f.i]
		f.i++
		return r, nil
	}
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return nil, io.EOF
}

func (f *fakeSession) Close() error { f.closed = true; return nil }

type memCursors struct{ m map[string]string }

func newMemCursors() *memCursors { return &memCursors{m: map[string]string{}} }

func (c *memCursors) key(consumer, feed string) string { return consumer + "|" + feed }
func (c *memCursors) Load(_ context.Context, consumer, feed string) (string, error) {
	pos, ok := c.m[c.key(consumer, feed)]
	if !ok {
		return "", inbox.ErrNoCursor
	}
	return pos, nil
}
func (c *memCursors) Advance(_ context.Context, consumer, feed, position string) error {
	c.m[c.key(consumer, feed)] = position
	return nil
}

// sessionScript hands out pre-built sessions in order, recording the position
// each (re)open was requested at.
type sessionScript struct {
	sessions      []*fakeSession
	i             int
	openPositions []string
}

func (s *sessionScript) open(_ context.Context, startPosition string) (stream.StreamReadSession, error) {
	s.openPositions = append(s.openPositions, startPosition)
	sess := s.sessions[s.i]
	s.i++
	return sess, nil
}

func newSource(cursors *memCursors, script *sessionScript) *s2Source {
	return &s2Source{
		cursors:      cursors,
		consumerName: "consumer-a",
		feed:         "feed.v1.X",
		newSession:   script.open,
	}
}

func rec(body, position, next string) *stream.StreamRecord {
	return &stream.StreamRecord{Body: []byte(body), Position: position, Next: next}
}

func TestS2Source_AckAdvancesCursorAndReopensPastIt(t *testing.T) {
	cursors := newMemCursors()
	script := &sessionScript{sessions: []*fakeSession{
		{records: []*stream.StreamRecord{rec("a", "5", "6")}}, // first session: one record, then EOF
		{},                                                    // second session (reopen): empty
	}}
	src := newSource(cursors, script)
	ctx := context.Background()

	d, err := src.Receive(ctx)
	require.NoError(t, err)
	assert.Equal(t, []byte("a"), d.Payload)
	require.NoError(t, d.Ack())
	assert.Equal(t, "6", cursors.m["consumer-a|feed.v1.X"], "Ack persists the record's Next position")

	// Next read hits EOF on the first session; the source closes it.
	_, err = src.Receive(ctx)
	require.ErrorIs(t, err, io.EOF)
	assert.True(t, script.sessions[0].closed)

	// The reopen starts one past the acked record.
	_, err = src.Receive(ctx)
	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, []string{"", "6"}, script.openPositions)
}

func TestS2Source_NackReopensAtSamePositionAndCountsStall(t *testing.T) {
	cursors := newMemCursors()
	script := &sessionScript{sessions: []*fakeSession{
		{records: []*stream.StreamRecord{rec("poison", "5", "6")}},
		{records: []*stream.StreamRecord{rec("poison", "5", "6")}}, // redelivered after reopen
	}}
	src := newSource(cursors, script)
	ctx := context.Background()

	d, err := src.Receive(ctx)
	require.NoError(t, err)
	require.NoError(t, d.Nack())
	assert.True(t, script.sessions[0].closed, "Nack tears down the session")
	assert.Equal(t, "", cursors.m["consumer-a|feed.v1.X"], "Nack does not advance the cursor")

	d2, err := src.Receive(ctx)
	require.NoError(t, err)
	assert.Equal(t, []byte("poison"), d2.Payload, "the same record is redelivered")
	assert.Equal(t, []string{"", ""}, script.openPositions, "reopen lands on the same un-advanced position")
	assert.Equal(t, 2, src.redeliverAttempts, "consecutive same-position reopens are counted (stall signal)")
}

func TestS2Source_TransientNextErrorClosesSession(t *testing.T) {
	cursors := newMemCursors()
	boom := errors.New("session dropped")
	script := &sessionScript{sessions: []*fakeSession{
		{nextErr: boom},
		{},
	}}
	src := newSource(cursors, script)
	ctx := context.Background()

	_, err := src.Receive(ctx)
	require.ErrorIs(t, err, boom)
	assert.True(t, script.sessions[0].closed, "a transport error drops the session so the next Receive reopens")
}

func TestS2Source_ConstructorValidation(t *testing.T) {
	cfg := stream.S2StreamProviderConfig{ApiKey: "k"}
	st := stream.StreamFor(routingX())

	_, err := NewS2(st, stream.S2StreamProviderConfig{}, nil, newMemCursors(), "c")
	assert.Error(t, err, "missing API key")
	_, err = NewS2(st, cfg, nil, nil, "c")
	assert.Error(t, err, "missing cursor store")
	_, err = NewS2(st, cfg, nil, newMemCursors(), "")
	assert.Error(t, err, "missing consumer name")
	_, err = NewS2(stream.Stream{}, cfg, nil, newMemCursors(), "c")
	assert.Error(t, err, "invalid stream (no namespace/name)")
	_, err = NewS2(st, cfg, nil, newMemCursors(), "c")
	assert.NoError(t, err)
}

func TestS2Source_CursorKeyIncludesExposureAndTenant(t *testing.T) {
	// Two feeds that differ only by exposure/tenant must not collide on the cursor
	// key — the key is the full stream id, not just the routing name.
	global := stream.StreamFor(routingX())
	tenant := stream.StreamForWithTenant(routingX(), "tenant-1")

	gid, err := global.ID()
	require.NoError(t, err)
	tid, err := tenant.ID()
	require.NoError(t, err)
	assert.NotEqual(t, gid, tid)
}
