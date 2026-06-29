package inbox_test

import (
	"testing"

	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedup_SeenAfterMark(t *testing.T) {
	dedup, err := inbox.NewGormDedup(newTestAdapter(t), "consumer-a")
	require.NoError(t, err)
	ctx := t.Context()

	seen, err := dedup.Seen(ctx, "evt-1")
	require.NoError(t, err)
	assert.False(t, seen)

	require.NoError(t, dedup.Mark(ctx, "evt-1"))

	seen, err = dedup.Seen(ctx, "evt-1")
	require.NoError(t, err)
	assert.True(t, seen)
}

func TestDedup_MarkIsIdempotent(t *testing.T) {
	dedup, err := inbox.NewGormDedup(newTestAdapter(t), "consumer-a")
	require.NoError(t, err)
	ctx := t.Context()

	require.NoError(t, dedup.Mark(ctx, "evt-1"))
	require.NoError(t, dedup.Mark(ctx, "evt-1"), "re-marking the same event is a no-op, not an error")
}

func TestDedup_IndependentPerConsumer(t *testing.T) {
	adapter := newTestAdapter(t)
	a, err := inbox.NewGormDedup(adapter, "consumer-a")
	require.NoError(t, err)
	b, err := inbox.NewGormDedup(adapter, "consumer-b")
	require.NoError(t, err)
	ctx := t.Context()

	require.NoError(t, a.Mark(ctx, "evt-1"))

	seen, err := b.Seen(ctx, "evt-1")
	require.NoError(t, err)
	assert.False(t, seen, "consumer-b has its own processed set")
}
