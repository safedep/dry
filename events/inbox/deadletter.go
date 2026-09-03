package inbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/safedep/dry/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeadLetter is one record a consumer could not process and has given up
// retrying. It preserves the raw Payload so the event is never lost — an
// operator can inspect it and replay it out of band. Same identity shape as
// Cursor / ProcessedEvent: surrogate PK plus a unique index on the natural key.
//
// The natural key is (consumer_name, payload_hash), not (consumer_name,
// event_id): a decode failure has no event_id, and a redelivered poison record
// carries identical bytes, so hashing the payload dedups redeliveries whether or
// not the envelope could be read. EventID / Feed are best-effort context for
// triage queries — empty when the generic retry policy dead-letters, since it
// does not decode the payload.
type DeadLetter struct {
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

func (DeadLetter) TableName() string { return "event_inbox_dead_letter" }

// DeadLetterRecord is the input to DeadLetterQueue.Store. Payload is the raw
// record and the source of truth; EventID and Feed are optional context populated
// only when the caller has decoded the envelope. An empty payload is accepted on
// purpose: a zero-length poison record (e.g. one whose envelope won't decode)
// must still be dead-letterable, or the feed it blocks could never advance — the
// exact stall this package exists to prevent.
type DeadLetterRecord struct {
	Payload  []byte
	Error    string
	Attempts int
	EventID  string
	Feed     string
}

// DeadLetterQueue persists records a consumer has given up on. It is the sink an
// error policy skips a poison record into (see NewRetryPolicy) so the feed can
// advance without dropping data.
type DeadLetterQueue interface {
	Store(ctx context.Context, rec DeadLetterRecord) error
}

type gormDeadLetter struct {
	db           *gorm.DB
	consumerName string
}

var _ DeadLetterQueue = &gormDeadLetter{}

// newGormDeadLetter builds the consumer-scoped gorm store both the write
// (DeadLetterQueue) and read (DeadLetterReader) sides share, so their validation
// stays in one place.
func newGormDeadLetter(adapter db.SqlDataAdapter, consumerName string) (*gormDeadLetter, error) {
	if adapter == nil {
		return nil, fmt.Errorf("inbox: dead-letter queue: adapter is required")
	}
	if consumerName == "" {
		// An empty name would silently share one dead-letter set across consumers.
		return nil, fmt.Errorf("inbox: dead-letter queue: consumer name is required")
	}
	gdb, err := adapter.GetDB()
	if err != nil {
		return nil, fmt.Errorf("inbox: dead-letter queue: %w", err)
	}
	return &gormDeadLetter{db: gdb, consumerName: consumerName}, nil
}

// NewGormDeadLetter builds a consumer-scoped DeadLetterQueue over the consumer's
// SQL adapter. The consumer name is bound here so Store stays keyless, mirroring
// NewGormDedup.
func NewGormDeadLetter(adapter db.SqlDataAdapter, consumerName string) (DeadLetterQueue, error) {
	return newGormDeadLetter(adapter, consumerName)
}

func (q *gormDeadLetter) Store(ctx context.Context, rec DeadLetterRecord) error {
	// An empty payload is stored, not rejected: SHA-256 of empty is still a stable
	// dedup key, and refusing it would make the retry policy loop forever on a
	// zero-length poison record (Store error -> Retry) — reintroducing the stall.
	row := DeadLetter{
		ConsumerName: q.consumerName,
		PayloadHash:  payloadFingerprint(rec.Payload),
		Feed:         sanitizeForText(rec.Feed),
		EventID:      sanitizeForText(rec.EventID),
		Payload:      rec.Payload,
		// The error string can itself carry a NUL or invalid UTF-8 (the very poison
		// that triggered the failure), which Postgres would reject — sinking the
		// dead-letter insert and defeating the whole point. Sanitize the text
		// columns; the raw bytes stay untouched in Payload.
		Error:    sanitizeForText(rec.Error),
		Attempts: rec.Attempts,
		FailedAt: time.Now(),
	}
	// Idempotent: a redelivered poison record hashes to the same key, so a repeat
	// dead-letter is a no-op rather than a duplicate row or an error.
	err := q.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error
	if err != nil {
		return fmt.Errorf("inbox: dead-letter store: %w", err)
	}
	return nil
}

// payloadFingerprint is a stable content key over a raw record. Redeliveries of
// the same bytes fingerprint identically, which backs both the retry policy's
// consecutive-failure counter and the dead-letter table's dedup key. SHA-256 (not
// a 64-bit hash) so a collision can't make a distinct poison record dedup away as
// "already stored" and get skipped — that would be silent data loss.
func payloadFingerprint(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// shortFingerprint abbreviates a payload fingerprint for log lines. The prefix
// is enough to correlate with a dead-letter row's PayloadHash.
func shortFingerprint(fp string) string {
	const n = 12
	if len(fp) <= n {
		return fp
	}
	return fp[:n]
}

// sanitizeForText makes a diagnostic string safe for a Postgres text column,
// which rejects NUL (0x00) and invalid UTF-8. Only the human-readable columns are
// coerced; the raw record is preserved verbatim in the bytea payload column.
func sanitizeForText(s string) string {
	if s == "" {
		return s
	}
	return strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "")
}
