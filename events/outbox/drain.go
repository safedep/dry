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
// destination never starves the others. Returns the number of deliveries (not
// records — a record fans out to one delivery per destination) published.
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

	records, err := loadRecords(gdb, pending)
	if err != nil {
		return 0, err
	}

	published := 0
	for i := range pending {
		del := &pending[i]

		rec, ok := records[del.OutboxID]
		if !ok {
			return published, fmt.Errorf("outbox: record %d missing for delivery %d", del.OutboxID, del.ID)
		}

		routing, err := events.RoutingForFullName(rec.FQN)
		if err != nil {
			// Unroutable row (should never happen — FQN was validated on write).
			// Poison it so the queue is not stuck forever.
			if perr := o.poison(gdb, del, fmt.Sprintf("unroutable fqn: %v", err)); perr != nil {
				return published, perr
			}
			continue
		}

		req := PublishRequest{Routing: routing, Tenant: rec.Tenant, EventID: rec.EventID, Record: rec.Payload}
		if perr := dest.Publish(ctx, req); perr != nil {
			del.Attempts++
			del.LastError = perr.Error()
			if del.Attempts >= o.maxAttempts {
				if err := o.poison(gdb, del, del.LastError); err != nil {
					return published, err
				}
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
		if err := o.maybeMarkDelivered(gdb, del.OutboxID); err != nil {
			return published, err
		}
		published++
	}

	return published, nil
}

// loadRecords fetches all records referenced by a batch of deliveries in one
// query (avoids an N+1 lookup), keyed by record id.
func loadRecords(gdb *gorm.DB, deliveries []Delivery) (map[uint64]Record, error) {
	if len(deliveries) == 0 {
		return map[uint64]Record{}, nil
	}

	ids := make([]uint64, 0, len(deliveries))
	seen := make(map[uint64]struct{}, len(deliveries))
	for i := range deliveries {
		id := deliveries[i].OutboxID
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	var recs []Record
	if err := gdb.Where("id IN ?", ids).Find(&recs).Error; err != nil {
		return nil, fmt.Errorf("outbox: load records: %w", err)
	}

	byID := make(map[uint64]Record, len(recs))
	for i := range recs {
		byID[recs[i].ID] = recs[i]
	}

	return byID, nil
}

func (o *Outbox) poison(gdb *gorm.DB, del *Delivery, reason string) error {
	now := o.now()
	del.FailedAt = &now
	del.LastError = reason
	if err := gdb.Save(del).Error; err != nil {
		return fmt.Errorf("outbox: poison delivery %d: %w", del.ID, err)
	}

	return o.maybeMarkDelivered(gdb, del.OutboxID)
}

// maybeMarkDelivered sets Record.delivered_at once no delivery for it is still
// pending (every destination has either acked or been poisoned).
func (o *Outbox) maybeMarkDelivered(gdb *gorm.DB, outboxID uint64) error {
	var pending int64
	if err := gdb.Model(&Delivery{}).
		Where("outbox_id = ? AND published_at IS NULL AND failed_at IS NULL", outboxID).
		Count(&pending).Error; err != nil {
		return fmt.Errorf("outbox: count pending for %d: %w", outboxID, err)
	}
	if pending > 0 {
		return nil
	}

	now := o.now()
	if err := gdb.Model(&Record{}).Where("id = ?", outboxID).Update("delivered_at", now).Error; err != nil {
		return fmt.Errorf("outbox: mark record %d delivered: %w", outboxID, err)
	}

	return nil
}
