package outbox

import (
	"context"
	"fmt"

	"github.com/safedep/dry/events"
	"gorm.io/gorm"
)

// drainOnce publishes outstanding deliveries. It processes each destination's
// pending deliveries in outbox-id (causal) order and stops that destination's
// queue at the first transient failure — preserving per-destination order — but
// poisons (and steps past) a delivery that fails maxAttempts times, so one bad
// destination never starves the others. Returns the number of records published.
func (o *Outbox) drainOnce(ctx context.Context) (int, error) {
	gdb, err := o.store.GetDB()
	if err != nil {
		return 0, fmt.Errorf("outbox: get db: %w", err)
	}
	gdb = gdb.WithContext(ctx)

	published := 0
	for _, dest := range o.dests {
		n, err := o.drainDestination(ctx, gdb, dest)
		published += n
		if err != nil {
			return published, err
		}
	}

	return published, nil
}

func (o *Outbox) drainDestination(ctx context.Context, gdb *gorm.DB, dest Destination) (int, error) {
	var pending []Delivery
	err := gdb.
		Where("destination = ? AND published_at IS NULL AND failed_at IS NULL", dest.Name()).
		Order("outbox_id ASC, id ASC").
		Limit(o.batchSize).
		Find(&pending).Error
	if err != nil {
		return 0, fmt.Errorf("outbox: load pending for %s: %w", dest.Name(), err)
	}

	published := 0
	for i := range pending {
		del := &pending[i]

		var rec Record
		if err := gdb.First(&rec, del.OutboxID).Error; err != nil {
			return published, fmt.Errorf("outbox: load record %d: %w", del.OutboxID, err)
		}

		routing, err := events.RoutingForFullName(rec.FQN)
		if err != nil {
			// Unroutable row (should never happen — FQN was validated on write).
			// Poison it so the queue is not stuck forever.
			o.poison(gdb, del, fmt.Sprintf("unroutable fqn: %v", err))
			continue
		}

		if perr := dest.Publish(ctx, routing, rec.Tenant, rec.Payload); perr != nil {
			del.Attempts++
			del.LastError = perr.Error()
			if del.Attempts >= o.maxAttempts {
				o.poison(gdb, del, del.LastError)
				continue // poisoned — isolate and keep delivering later events
			}
			if err := gdb.Save(del).Error; err != nil {
				return published, err
			}
			break // transient — stop this destination's queue to preserve order
		}

		now := o.now()
		del.PublishedAt = &now
		if err := gdb.Save(del).Error; err != nil {
			return published, err
		}
		o.maybeMarkDelivered(gdb, del.OutboxID)
		published++
	}

	return published, nil
}

func (o *Outbox) poison(gdb *gorm.DB, del *Delivery, reason string) {
	now := o.now()
	del.FailedAt = &now
	del.LastError = reason
	if err := gdb.Save(del).Error; err != nil {
		return
	}
	o.maybeMarkDelivered(gdb, del.OutboxID)
}

// maybeMarkDelivered sets Record.delivered_at once no delivery for it is still
// pending (every destination has either acked or been poisoned).
func (o *Outbox) maybeMarkDelivered(gdb *gorm.DB, outboxID uint64) {
	var pending int64
	if err := gdb.Model(&Delivery{}).
		Where("outbox_id = ? AND published_at IS NULL AND failed_at IS NULL", outboxID).
		Count(&pending).Error; err != nil {
		return
	}
	if pending > 0 {
		return
	}

	now := o.now()
	gdb.Model(&Record{}).Where("id = ?", outboxID).Update("delivered_at", now)
}
