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

// ProcessedEvent records that a consumer has handled an event_id. It backs the
// optional WithDedup helper; see Dedup for the (best-effort, not exactly-once)
// semantics.
type ProcessedEvent struct {
	ConsumerName string    `gorm:"column:consumer_name;primaryKey"`
	EventID      string    `gorm:"column:event_id;primaryKey"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
}

func (ProcessedEvent) TableName() string { return "event_inbox_processed" }

type gormDedup struct {
	db           *gorm.DB
	consumerName string
}

var _ Dedup = &gormDedup{}

// NewGormDedup builds a consumer-scoped Dedup over the consumer's SQL adapter.
// The consumer name is bound here so Consume's Dedup calls stay keyless.
func NewGormDedup(adapter db.SqlDataAdapter, consumerName string) (Dedup, error) {
	gdb, err := adapter.GetDB()
	if err != nil {
		return nil, fmt.Errorf("inbox: dedup store: %w", err)
	}
	return &gormDedup{db: gdb, consumerName: consumerName}, nil
}

func (d *gormDedup) Seen(ctx context.Context, eventID string) (bool, error) {
	var row ProcessedEvent
	err := d.db.WithContext(ctx).
		Where("consumer_name = ? AND event_id = ?", d.consumerName, eventID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inbox: dedup seen: %w", err)
	}
	return true, nil
}

func (d *gormDedup) Mark(ctx context.Context, eventID string) error {
	row := ProcessedEvent{
		ConsumerName: d.consumerName,
		EventID:      eventID,
		ProcessedAt:  time.Now(),
	}
	// Idempotent insert: a concurrent/duplicate mark is a no-op, not an error.
	err := d.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row).Error
	if err != nil {
		return fmt.Errorf("inbox: dedup mark: %w", err)
	}
	return nil
}
