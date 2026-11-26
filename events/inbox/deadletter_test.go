package inbox_test

import (
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

// deadLetterBeforeReason is the dead-letter model on main before the reason
// and subject columns.
type deadLetterBeforeReason struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ConsumerName string    `gorm:"column:consumer_name;uniqueIndex:idx_event_inbox_dead_letter_unique,priority:1"`
	PayloadHash  string    `gorm:"column:payload_hash;uniqueIndex:idx_event_inbox_dead_letter_unique,priority:2"`
	Feed         string    `gorm:"column:feed"`
	EventID      string    `gorm:"column:event_id"`
	Payload      []byte    `gorm:"column:payload"`
	Error        string    `gorm:"column:error"`
	Attempts     int       `gorm:"column:attempts"`
	FailedAt     time.Time `gorm:"column:failed_at;index:idx_event_inbox_dead_letter_failed_at"`
}

func (deadLetterBeforeReason) TableName() string { return "event_inbox_dead_letter" }

func TestMigrate_GivesOldDeadLettersTheUnspecifiedReason(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "old.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&deadLetterBeforeReason{}))
	failedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	require.NoError(t, gdb.Create(&deadLetterBeforeReason{
		ConsumerName: "consumer-a", PayloadHash: "h", Payload: []byte("p"), FailedAt: failedAt,
	}).Error)

	adapter := testAdapter{gdb: gdb}
	require.NoError(t, inbox.Migrate(adapter))

	reader := newDeadLetterReader(t, adapter, "consumer-a")
	rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{Reason: inbox.DeadLetterReasonUnspecified})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, inbox.DeadLetterReasonUnspecified, rows[0].Reason)
	assert.Empty(t, rows[0].Subject)
	assert.True(t, failedAt.Equal(rows[0].FailedAt), "failed_at survives the migration: %s", rows[0].FailedAt)
	assert.True(t, gdb.Migrator().HasIndex(&inbox.DeadLetter{}, "idx_event_inbox_dead_letter_failed_at"))
}

func TestGormDeadLetter_StoreKeepsTheReason(t *testing.T) {
	cases := []struct {
		name   string
		reason inbox.DeadLetterReason
		want   inbox.DeadLetterReason
	}{
		{name: "empty reason", reason: "", want: inbox.DeadLetterReasonUnspecified},
		{name: "no route", reason: inbox.DeadLetterReasonNoRoute, want: inbox.DeadLetterReasonNoRoute},
		{name: "decode", reason: inbox.DeadLetterReasonDecode, want: inbox.DeadLetterReasonDecode},
		{name: "handler", reason: inbox.DeadLetterReasonHandler, want: inbox.DeadLetterReasonHandler},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := newTestAdapter(t)
			seedDeadLetter(t, adapter, "consumer-a",
				inbox.DeadLetterRecord{Payload: []byte("target"), Reason: tc.reason},
				inbox.DeadLetterRecord{Payload: []byte("other"), Reason: "other"},
			)
			reader := newDeadLetterReader(t, adapter, "consumer-a")

			rows, err := reader.List(t.Context(), inbox.DeadLetterFilter{Reason: tc.want})
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, []byte("target"), rows[0].Payload)
		})
	}
}
