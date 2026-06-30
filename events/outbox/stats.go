package outbox

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/safedep/dry/db"
	"gorm.io/gorm"
)

// StatsReader exposes read-only aggregate views over the outbox tables for a
// monitoring UI. It is independent of Outbox (the writer/drain): a consumer that
// only displays pipeline health constructs a reader over its database without
// owning any destinations.
type StatsReader struct {
	store db.SqlDataAdapter
}

// NewStatsReader builds a reader over the consumer's outbox database.
func NewStatsReader(store db.SqlDataAdapter) (*StatsReader, error) {
	if store == nil {
		return nil, errors.New("outbox: NewStatsReader requires a store")
	}

	return &StatsReader{store: store}, nil
}

// StateTotals is the headline delivery state across all events. Delivered and
// Pending partition Emitted (every record is one or the other). Stuck is a
// subset of Pending: an undelivered record with at least one delivery past
// maxAttempts — surfaced separately for alerting, not a fourth bucket.
type StateTotals struct {
	Emitted   int64
	Delivered int64
	Pending   int64
	Stuck     int64
}

// FQNStat is the per-event-type breakdown row. Pending is Emitted-Delivered (all
// records of this FQN not yet fully delivered). LastEmitted is the most recent
// emission, for spotting feeds that have gone quiet.
type FQNStat struct {
	FQN         string
	Emitted     int64
	Delivered   int64
	Pending     int64
	LastEmitted time.Time
}

// StateTotals returns the headline counts. A zero since includes all history;
// otherwise only records emitted at or after since are counted (the UI maps its
// time-range dropdown onto since).
func (r *StatsReader) StateTotals(ctx context.Context, since time.Time) (StateTotals, error) {
	gdb, err := r.store.GetDB()
	if err != nil {
		return StateTotals{}, fmt.Errorf("outbox: get db: %w", err)
	}
	gdb = gdb.WithContext(ctx)

	scope := func(tx *gorm.DB) *gorm.DB {
		if since.IsZero() {
			return tx
		}
		return tx.Where("created_at >= ?", since)
	}

	var totals StateTotals
	if err := scope(gdb.Model(&Record{})).Count(&totals.Emitted).Error; err != nil {
		return StateTotals{}, fmt.Errorf("outbox: count emitted: %w", err)
	}
	if err := scope(gdb.Model(&Record{})).Where("delivered_at IS NOT NULL").Count(&totals.Delivered).Error; err != nil {
		return StateTotals{}, fmt.Errorf("outbox: count delivered: %w", err)
	}
	totals.Pending = totals.Emitted - totals.Delivered

	// Stuck records: undelivered and carrying at least one delivery flagged stuck.
	// EXISTS keeps it a single scan of records with a correlated probe.
	stuckProbe := "EXISTS (SELECT 1 FROM event_outbox_delivery d WHERE d.outbox_id = event_outbox.id AND d.stuck_since IS NOT NULL)"
	if err := scope(gdb.Model(&Record{})).
		Where("delivered_at IS NULL").
		Where(stuckProbe).
		Count(&totals.Stuck).Error; err != nil {
		return StateTotals{}, fmt.Errorf("outbox: count stuck: %w", err)
	}

	return totals, nil
}

// PerFQN returns the per-event-type breakdown, busiest feed first. A zero since
// includes all history; otherwise only records emitted at or after since count.
func (r *StatsReader) PerFQN(ctx context.Context, since time.Time) ([]FQNStat, error) {
	gdb, err := r.store.GetDB()
	if err != nil {
		return nil, fmt.Errorf("outbox: get db: %w", err)
	}
	gdb = gdb.WithContext(ctx)

	q := gdb.Model(&Record{}).
		Select("fqn AS fqn, COUNT(*) AS emitted, COUNT(delivered_at) AS delivered, MAX(created_at) AS last_emitted").
		Group("fqn").
		Order("emitted DESC")
	if !since.IsZero() {
		q = q.Where("created_at >= ?", since)
	}

	var rows []fqnRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("outbox: per-fqn breakdown: %w", err)
	}

	out := make([]FQNStat, len(rows))
	for i, r := range rows {
		out[i] = FQNStat{
			FQN:         r.FQN,
			Emitted:     r.Emitted,
			Delivered:   r.Delivered,
			Pending:     r.Emitted - r.Delivered,
			LastEmitted: r.LastEmitted.t,
		}
	}

	return out, nil
}

// fqnRow is the raw scan target for PerFQN. LastEmitted is a scannedTime because
// MAX(created_at) loses datetime affinity under SQLite (returned as text) while
// Postgres returns a time.Time — scannedTime accepts both.
type fqnRow struct {
	FQN         string
	Emitted     int64
	Delivered   int64
	LastEmitted scannedTime
}

// scannedTime reads a timestamp that a driver may hand back as time.Time
// (Postgres) or as a text/byte string (SQLite aggregate columns). It implements
// driver.Valuer too so GORM treats it as a scalar column, not a relation.
type scannedTime struct{ t time.Time }

func (s scannedTime) Value() (driver.Value, error) { return s.t, nil }

func (s *scannedTime) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		return nil
	case time.Time:
		s.t = x
		return nil
	case []byte:
		return s.parse(string(x))
	case string:
		return s.parse(x)
	default:
		return fmt.Errorf("outbox: cannot scan %T into time", v)
	}
}

func (s *scannedTime) parse(v string) error {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, v); err == nil {
			s.t = t
			return nil
		}
	}

	return fmt.Errorf("outbox: unrecognized time format %q", v)
}
