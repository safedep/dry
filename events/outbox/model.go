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
	Tenant  string `gorm:"column:tenant"`         // envelope tenant; empty for global feeds
	Payload []byte `gorm:"column:payload"`        // binary-proto <Feed>Event (envelope + payload)

	// DeliveredAt is set once every Delivery for this record is resolved
	// (published or poisoned). Nil while any delivery is still pending.
	DeliveredAt *time.Time `gorm:"column:delivered_at"`
	CreatedAt   time.Time
}

func (Record) TableName() string { return "event_outbox" }

// Delivery is the per-destination delivery state for a Record (§8). The drain
// retries only un-acked, non-poisoned deliveries, so a healthy destination is
// never re-sent because a sibling failed, and a persistently-failing destination
// is isolated rather than blocking the others.
type Delivery struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	OutboxID    uint64 `gorm:"column:outbox_id;index:idx_delivery_pending,priority:2"`
	Destination string `gorm:"column:destination;index:idx_delivery_pending,priority:1"`

	PublishedAt *time.Time `gorm:"column:published_at"` // this destination acked
	Attempts    int        `gorm:"column:attempts"`
	FailedAt    *time.Time `gorm:"column:failed_at"` // poisoned after maxAttempts (isolated)
	LastError   string     `gorm:"column:last_error"`
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
