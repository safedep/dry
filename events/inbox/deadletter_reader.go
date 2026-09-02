package inbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/safedep/dry/db"
	"gorm.io/gorm"
)

// defaultDeadLetterPageSize bounds a List page when the caller does not set one.
// It also serves as Replay's internal page size while it drains a backlog.
const defaultDeadLetterPageSize = 100

// ErrDeadLetterNotFound is returned by DeadLetterReader.Get when no record with
// the id exists for this consumer. Mirrors ErrNoCursor: an exported sentinel so
// callers distinguish "not there" from a query failure.
var ErrDeadLetterNotFound = errors.New("inbox: dead-letter record not found")

// DeadLetterFilter narrows a List / Count. All fields are optional; a zero filter
// matches every record for the consumer. Feed / EventID / Before select records;
// AfterID / Limit page over them (id-ordered, ascending). Count ignores AfterID
// and Limit — it answers "how many match the selection", not "how many on a page".
type DeadLetterFilter struct {
	Feed    string
	EventID string
	Before  time.Time // failed_at strictly before this instant
	AfterID uint64    // cursor: records with id greater than this
	Limit   int       // page size; <= 0 uses defaultDeadLetterPageSize
}

// DeadLetterReader is the read/replay dual of DeadLetterQueue: it lists, fetches,
// counts, and deletes the records the write side has parked. It is consumer-scoped
// like the writer, so an operator can never list or replay another consumer's
// records through it (replaying bytes under the wrong typed handler is the footgun
// the scoping removes).
type DeadLetterReader interface {
	// List returns one id-ordered page of records matching f (ascending id). Use
	// f.AfterID = the last id seen to page forward.
	List(ctx context.Context, f DeadLetterFilter) ([]DeadLetter, error)
	// Get fetches one record by id, or ErrDeadLetterNotFound.
	Get(ctx context.Context, id uint64) (*DeadLetter, error)
	// Count returns how many records match f's selection (AfterID / Limit ignored).
	Count(ctx context.Context, f DeadLetterFilter) (int64, error)
	// Delete removes one record by id. It is idempotent: deleting a record that is
	// already gone is not an error, so a replay that deletes after a successful
	// handler never races a concurrent purge into a failure.
	Delete(ctx context.Context, id uint64) error
}

var _ DeadLetterReader = &gormDeadLetter{}

// NewGormDeadLetterReader builds a consumer-scoped DeadLetterReader over the
// consumer's SQL adapter. Same validation and scoping as NewGormDeadLetter.
func NewGormDeadLetterReader(adapter db.SqlDataAdapter, consumerName string) (DeadLetterReader, error) {
	return newGormDeadLetter(adapter, consumerName)
}

// selection applies the consumer scope and content filters (Feed / EventID /
// Before), the predicates shared by List and Count. Pagination (AfterID / Limit)
// is layered on by List only.
func (q *gormDeadLetter) selection(tx *gorm.DB, f DeadLetterFilter) *gorm.DB {
	tx = tx.Where("consumer_name = ?", q.consumerName)
	if f.Feed != "" {
		tx = tx.Where("feed = ?", f.Feed)
	}
	if f.EventID != "" {
		tx = tx.Where("event_id = ?", f.EventID)
	}
	if !f.Before.IsZero() {
		tx = tx.Where("failed_at < ?", f.Before)
	}
	return tx
}

func (q *gormDeadLetter) List(ctx context.Context, f DeadLetterFilter) ([]DeadLetter, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultDeadLetterPageSize
	}
	tx := q.selection(q.db.WithContext(ctx), f)
	if f.AfterID > 0 {
		tx = tx.Where("id > ?", f.AfterID)
	}
	var rows []DeadLetter
	if err := tx.Order("id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("inbox: dead-letter list: %w", err)
	}
	return rows, nil
}

func (q *gormDeadLetter) Get(ctx context.Context, id uint64) (*DeadLetter, error) {
	var row DeadLetter
	err := q.db.WithContext(ctx).
		Where("consumer_name = ? AND id = ?", q.consumerName, id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDeadLetterNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("inbox: dead-letter get: %w", err)
	}
	return &row, nil
}

func (q *gormDeadLetter) Count(ctx context.Context, f DeadLetterFilter) (int64, error) {
	var n int64
	if err := q.selection(q.db.WithContext(ctx).Model(&DeadLetter{}), f).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("inbox: dead-letter count: %w", err)
	}
	return n, nil
}

func (q *gormDeadLetter) Delete(ctx context.Context, id uint64) error {
	err := q.db.WithContext(ctx).
		Where("consumer_name = ? AND id = ?", q.consumerName, id).
		Delete(&DeadLetter{}).Error
	if err != nil {
		return fmt.Errorf("inbox: dead-letter delete: %w", err)
	}
	return nil
}
