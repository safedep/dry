package inbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/safedep/dry/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Cursor is the durable read position for one (consumer_name, feed). It is the
// read-side dual of the outbox's per-destination delivery row: dry owns the
// model, the table lives in the consumer's database. One row per consumer per
// feed, so two consumers of the same feed track independent positions.
//
// Following the outbox models, the identity is a surrogate auto-increment PK with
// a unique index on the natural key — rather than a natural composite PK or an
// embedded gorm.Model (whose soft-delete deleted_at has no meaning for a cursor).
// The unique index both enforces one-cursor-per-feed and backs the Load lookup
// and the Advance upsert's conflict target.
type Cursor struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	ConsumerName string    `gorm:"column:consumer_name;uniqueIndex:idx_event_inbox_cursor_unique,priority:1"`
	Feed         string    `gorm:"column:feed;uniqueIndex:idx_event_inbox_cursor_unique,priority:2"`
	Position     string    `gorm:"column:position"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (Cursor) TableName() string { return "event_inbox_cursor" }

// CursorStore loads and advances the durable read position. It is keyed
// explicitly on (consumer_name, feed) — the S2 source supplies both — rather than
// being consumer-scoped, so one store instance serves every feed in a process.
type CursorStore interface {
	// Load returns the persisted position for (consumerName, feed), or "" when no
	// cursor exists yet (first run — read from the start).
	Load(ctx context.Context, consumerName, feed string) (string, error)
	// Advance upserts the position for (consumerName, feed).
	Advance(ctx context.Context, consumerName, feed, position string) error
}

type gormCursorStore struct {
	db *gorm.DB
}

var _ CursorStore = &gormCursorStore{}

// NewGormCursorStore builds a CursorStore over the consumer's SQL adapter.
func NewGormCursorStore(adapter db.SqlDataAdapter) (CursorStore, error) {
	if adapter == nil {
		return nil, fmt.Errorf("inbox: cursor store: adapter is required")
	}
	gdb, err := adapter.GetDB()
	if err != nil {
		return nil, fmt.Errorf("inbox: cursor store: %w", err)
	}
	return &gormCursorStore{db: gdb}, nil
}

func (s *gormCursorStore) Load(ctx context.Context, consumerName, feed string) (string, error) {
	var cursor Cursor
	err := s.db.WithContext(ctx).
		Where("consumer_name = ? AND feed = ?", consumerName, feed).
		First(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inbox: load cursor: %w", err)
	}
	return cursor.Position, nil
}

func (s *gormCursorStore) Advance(ctx context.Context, consumerName, feed, position string) error {
	row := Cursor{
		ConsumerName: consumerName,
		Feed:         feed,
		Position:     position,
		UpdatedAt:    time.Now(),
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "consumer_name"}, {Name: "feed"}},
		DoUpdates: clause.AssignmentColumns([]string{"position", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("inbox: advance cursor: %w", err)
	}
	return nil
}
