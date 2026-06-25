package outbox

import (
	"time"

	"github.com/safedep/dry/db"
)

// Record is one event awaiting delivery. It lives in the consumer's database
// (the event row is written in the producer's transaction by Emit), and dry owns
// the schema. One row per event; fan-out to destinations is tracked in Delivery.
type Record struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement"`
	EventID string `gorm:"column:event_id;index"` // ULID from the envelope; dedup / header
	FQN     string `gorm:"column:fqn"`            // events.Routing.FQN; address derived from it
	Subject string `gorm:"column:subject"`        // envelope subject; the per-subject ordering domain
	Tenant  string `gorm:"column:tenant"`         // envelope tenant; empty for global feeds
	Payload []byte `gorm:"column:payload"`        // binary-proto <Feed>Event (envelope + payload)

	// DeliveredAt is set once every Delivery for this record has been published.
	// Nil while any delivery is still outstanding (including stuck ones).
	DeliveredAt *time.Time `gorm:"column:delivered_at"`
	CreatedAt   time.Time
}

func (Record) TableName() string { return "event_outbox" }

// Delivery is the per-destination delivery state for a Record. The drain
// preserves per-subject order: a delivery that keeps failing blocks only its own
// subject (other subjects and destinations keep flowing) and is retried — never
// skipped — so downstream state never advances past a gap.
type Delivery struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	OutboxID    uint64 `gorm:"column:outbox_id;index:idx_delivery_pending,priority:2"`
	Destination string `gorm:"column:destination;index:idx_delivery_pending,priority:1"`

	PublishedAt *time.Time `gorm:"column:published_at"` // this destination acked
	Attempts    int        `gorm:"column:attempts"`

	// StuckSince flags a delivery that has exceeded maxAttempts, for alerting. It
	// is NOT terminal — the delivery is still retried (and blocks its subject)
	// until it succeeds or an operator intervenes.
	StuckSince *time.Time `gorm:"column:stuck_since"`
	LastError  string     `gorm:"column:last_error"`
}

func (Delivery) TableName() string { return "event_outbox_delivery" }

// Migrate creates/updates the outbox tables via the consumer's adapter. The
// tables live in the consumer's database; dry owns only the model. Run it from
// the consumer's migration pipeline.
func Migrate(adapter db.SqlDataAdapter) error {
	gdb, err := adapter.GetDB()
	if err != nil {
		return err
	}

	return gdb.AutoMigrate(&Record{}, &Delivery{})
}
