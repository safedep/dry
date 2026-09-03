package inbox_test

import (
	"testing"
	"time"

	"github.com/safedep/dry/db"
	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGormDeadLetterReader_Validation(t *testing.T) {
	_, err := inbox.NewGormDeadLetterReader(nil, "c")
	require.Error(t, err, "nil adapter is rejected")

	_, err = inbox.NewGormDeadLetterReader(newTestAdapter(t), "")
	require.Error(t, err, "empty consumer name is rejected")
}

// seedDeadLetter stores one record for consumerName and returns the whole store,
// so a test can seed then read through the reader.
func seedDeadLetter(t *testing.T, adapter db.SqlDataAdapter, consumerName string, recs ...inbox.DeadLetterRecord) {
	t.Helper()
	w, err := inbox.NewGormDeadLetter(adapter, consumerName)
	require.NoError(t, err)
	for _, r := range recs {
		require.NoError(t, w.Store(t.Context(), r))
	}
}

func TestDeadLetterReader_ListIsScopedAndOrdered(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: []byte("a1"), Feed: "feed.X", EventID: "e1"},
		inbox.DeadLetterRecord{Payload: []byte("a2"), Feed: "feed.Y", EventID: "e2"},
	)
	seedDeadLetter(t, adapter, "consumer-b",
		inbox.DeadLetterRecord{Payload: []byte("b1"), Feed: "feed.X"},
	)

	reader, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)

	rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 2, "only consumer-a's records, not consumer-b's")
	assert.Less(t, rows[0].ID, rows[1].ID, "ascending id order")
	assert.Equal(t, []byte("a1"), rows[0].Payload)
}

func TestDeadLetterReader_FiltersFeedAndEventID(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: []byte("a1"), Feed: "feed.X", EventID: "e1"},
		inbox.DeadLetterRecord{Payload: []byte("a2"), Feed: "feed.Y", EventID: "e2"},
	)
	reader, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)

	byFeed, err := reader.List(t.Context(), inbox.DeadLetterFilter{Feed: "feed.Y"})
	require.NoError(t, err)
	require.Len(t, byFeed, 1)
	assert.Equal(t, "e2", byFeed[0].EventID)

	byEvent, err := reader.List(t.Context(), inbox.DeadLetterFilter{EventID: "e1"})
	require.NoError(t, err)
	require.Len(t, byEvent, 1)
	assert.Equal(t, []byte("a1"), byEvent[0].Payload)
}

func TestDeadLetterReader_FiltersBefore(t *testing.T) {
	adapter := newTestAdapter(t)
	gdb, err := adapter.GetDB()
	require.NoError(t, err)

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()
	// Insert directly to control failed_at.
	require.NoError(t, gdb.Create(&inbox.DeadLetter{
		ConsumerName: "consumer-a", PayloadHash: "h-old", Payload: []byte("old"), FailedAt: old,
	}).Error)
	require.NoError(t, gdb.Create(&inbox.DeadLetter{
		ConsumerName: "consumer-a", PayloadHash: "h-new", Payload: []byte("new"), FailedAt: recent,
	}).Error)

	reader, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)

	rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{Before: time.Now().Add(-1 * time.Hour)})
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the record older than the cutoff")
	assert.Equal(t, []byte("old"), rows[0].Payload)
}

func TestDeadLetterReader_PaginatesByAfterID(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: []byte("a1")},
		inbox.DeadLetterRecord{Payload: []byte("a2")},
		inbox.DeadLetterRecord{Payload: []byte("a3")},
	)
	reader, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)

	page1, err := reader.List(t.Context(), inbox.DeadLetterFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	page2, err := reader.List(t.Context(), inbox.DeadLetterFilter{Limit: 2, AfterID: page1[1].ID})
	require.NoError(t, err)
	require.Len(t, page2, 1, "one record left after the first page")
	assert.Greater(t, page2[0].ID, page1[1].ID)
}

func TestDeadLetterReader_CountIgnoresPagination(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a",
		inbox.DeadLetterRecord{Payload: []byte("a1"), Feed: "feed.X"},
		inbox.DeadLetterRecord{Payload: []byte("a2"), Feed: "feed.X"},
		inbox.DeadLetterRecord{Payload: []byte("a3"), Feed: "feed.Y"},
	)
	reader, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)

	all, err := reader.Count(t.Context(), inbox.DeadLetterFilter{Limit: 1, AfterID: 999})
	require.NoError(t, err)
	assert.Equal(t, int64(3), all, "Count ignores AfterID/Limit")

	byFeed, err := reader.Count(t.Context(), inbox.DeadLetterFilter{Feed: "feed.X"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), byFeed)
}

func TestDeadLetterReader_GetAndScopedNotFound(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{Payload: []byte("a1")})
	seedDeadLetter(t, adapter, "consumer-b", inbox.DeadLetterRecord{Payload: []byte("b1")})

	readerA, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)
	rows, err := readerA.List(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	aID := rows[0].ID

	got, err := readerA.Get(t.Context(), aID)
	require.NoError(t, err)
	assert.Equal(t, []byte("a1"), got.Payload)

	// consumer-b's reader must not see consumer-a's record by id.
	readerB, err := inbox.NewGormDeadLetterReader(adapter, "consumer-b")
	require.NoError(t, err)
	_, err = readerB.Get(t.Context(), aID)
	require.ErrorIs(t, err, inbox.ErrDeadLetterNotFound)
}

func TestDeadLetterReader_DeleteIsScopedAndIdempotent(t *testing.T) {
	adapter := newTestAdapter(t)
	seedDeadLetter(t, adapter, "consumer-a", inbox.DeadLetterRecord{Payload: []byte("a1")})
	reader, err := inbox.NewGormDeadLetterReader(adapter, "consumer-a")
	require.NoError(t, err)

	rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	id := rows[0].ID

	require.NoError(t, reader.Delete(t.Context(), id))
	// Idempotent: deleting the now-gone record is not an error.
	require.NoError(t, reader.Delete(t.Context(), id))

	n, err := reader.Count(t.Context(), inbox.DeadLetterFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}
