package inbox_test

import (
	"testing"
	"unicode/utf8"

	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGormDeadLetter_Validation(t *testing.T) {
	_, err := inbox.NewGormDeadLetter(nil, "c")
	require.Error(t, err, "nil adapter is rejected")

	_, err = inbox.NewGormDeadLetter(newTestAdapter(t), "")
	require.Error(t, err, "empty consumer name is rejected")
}

func TestGormDeadLetter_StorePersistsRecord(t *testing.T) {
	adapter := newTestAdapter(t)
	q, err := inbox.NewGormDeadLetter(adapter, "consumer-a")
	require.NoError(t, err)

	require.NoError(t, q.Store(t.Context(), inbox.DeadLetterRecord{
		Payload:  []byte("poison-bytes"),
		Error:    "boom",
		Attempts: 4,
		EventID:  "evt-1",
		Feed:     "feed.v1.X",
	}))

	gdb, err := adapter.GetDB()
	require.NoError(t, err)

	var rows []inbox.DeadLetter
	require.NoError(t, gdb.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, "consumer-a", rows[0].ConsumerName)
	assert.Equal(t, []byte("poison-bytes"), rows[0].Payload)
	assert.Equal(t, "boom", rows[0].Error)
	assert.Equal(t, 4, rows[0].Attempts)
	assert.Equal(t, "evt-1", rows[0].EventID)
	assert.Equal(t, "feed.v1.X", rows[0].Feed)
	assert.NotEmpty(t, rows[0].PayloadHash)
	assert.False(t, rows[0].FailedAt.IsZero())
}

func TestGormDeadLetter_StoreIsIdempotentOnPayload(t *testing.T) {
	adapter := newTestAdapter(t)
	q, err := inbox.NewGormDeadLetter(adapter, "consumer-a")
	require.NoError(t, err)

	// Same bytes redelivered and dead-lettered twice → one row, no error.
	rec := inbox.DeadLetterRecord{Payload: []byte("same"), Error: "e", Attempts: 4}
	require.NoError(t, q.Store(t.Context(), rec))
	require.NoError(t, q.Store(t.Context(), rec))

	gdb, err := adapter.GetDB()
	require.NoError(t, err)
	var count int64
	require.NoError(t, gdb.Model(&inbox.DeadLetter{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGormDeadLetter_DistinctPayloadsAndConsumersCoexist(t *testing.T) {
	adapter := newTestAdapter(t)
	qa, err := inbox.NewGormDeadLetter(adapter, "consumer-a")
	require.NoError(t, err)
	qb, err := inbox.NewGormDeadLetter(adapter, "consumer-b")
	require.NoError(t, err)

	require.NoError(t, qa.Store(t.Context(), inbox.DeadLetterRecord{Payload: []byte("p1")}))
	require.NoError(t, qa.Store(t.Context(), inbox.DeadLetterRecord{Payload: []byte("p2")}))
	// Same payload as consumer-a's p1, but a different consumer → separate row.
	require.NoError(t, qb.Store(t.Context(), inbox.DeadLetterRecord{Payload: []byte("p1")}))

	gdb, err := adapter.GetDB()
	require.NoError(t, err)
	var count int64
	require.NoError(t, gdb.Model(&inbox.DeadLetter{}).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}

func TestGormDeadLetter_StoresEmptyPayload(t *testing.T) {
	adapter := newTestAdapter(t)
	q, err := inbox.NewGormDeadLetter(adapter, "consumer-a")
	require.NoError(t, err)

	// A zero-length poison record must be dead-letterable: rejecting it would make
	// the retry policy loop forever (Store error -> Retry), the exact stall this
	// package prevents.
	require.NoError(t, q.Store(t.Context(), inbox.DeadLetterRecord{Payload: nil, Error: "no envelope"}))

	gdb, err := adapter.GetDB()
	require.NoError(t, err)
	var rows []inbox.DeadLetter
	require.NoError(t, gdb.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.NotEmpty(t, rows[0].PayloadHash, "empty payload still gets a stable dedup key")
	assert.Equal(t, "no envelope", rows[0].Error)
}

func TestGormDeadLetter_SanitizesTextColumns(t *testing.T) {
	adapter := newTestAdapter(t)
	q, err := inbox.NewGormDeadLetter(adapter, "consumer-a")
	require.NoError(t, err)

	// The error string carries the very poison that failed the handler: a NUL and
	// an invalid UTF-8 byte. Postgres would reject those in a text column, sinking
	// the dead-letter insert — so they must be stripped before persistence.
	require.NoError(t, q.Store(t.Context(), inbox.DeadLetterRecord{
		Payload: []byte("raw\x00bytes"),
		Error:   "bad\x00utf\xff8",
	}))

	gdb, err := adapter.GetDB()
	require.NoError(t, err)
	var row inbox.DeadLetter
	require.NoError(t, gdb.First(&row).Error)

	assert.NotContains(t, row.Error, "\x00")
	assert.True(t, utf8.ValidString(row.Error), "error column must be valid UTF-8")
	assert.Equal(t, "badutf8", row.Error)
	// Payload is bytea: stored verbatim, NUL and all — the lossless replay source.
	assert.Equal(t, []byte("raw\x00bytes"), row.Payload)
}

// Migrate is idempotent — a second call over the same DB must not error.
func TestMigrate_Idempotent(t *testing.T) {
	adapter := newTestAdapter(t) // already migrates once
	require.NoError(t, inbox.Migrate(adapter))

	gdb, err := adapter.GetDB()
	require.NoError(t, err)
	assert.True(t, gdb.Migrator().HasTable(&inbox.DeadLetter{}))
}
