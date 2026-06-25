package outbox

import (
	"context"
	"fmt"

	"github.com/safedep/dry/events"
	"gorm.io/gorm"
)

// drainOnce publishes outstanding deliveries. It processes each destination's
// pending deliveries in outbox-id (causal) order, preserving per-subject order: a
// delivery that fails blocks only its own subject (other subjects keep flowing)
// and is retried, never skipped. Returns the number of deliveries (not records —
// a record fans out to one delivery per destination) published.
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
		Where("destination = ? AND published_at IS NULL", dest.Name()).
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

	// Subjects whose head delivery is unresolved this pass: later deliveries for
	// them are held back to preserve per-subject order.
	blocked := make(map[string]struct{})
	published := 0
	for i := range pending {
		del := &pending[i]

		rec, ok := records[del.OutboxID]
		if !ok {
			return published, fmt.Errorf("outbox: record %d missing for delivery %d", del.OutboxID, del.ID)
		}

		if rec.Subject != "" {
			if _, held := blocked[rec.Subject]; held {
				continue
			}
		}

		routing, err := events.RoutingForFullName(rec.FQN)
		if err != nil {
			// Unroutable row (should never happen — FQN was validated on write):
			// mark it stuck for alerting and hold its subject.
			if ferr := o.recordFailure(gdb, del, fmt.Sprintf("unroutable fqn: %v", err), true); ferr != nil {
				return published, ferr
			}
			block(blocked, rec.Subject)
			continue
		}

		req := PublishRequest{Routing: routing, Tenant: rec.Tenant, EventID: rec.EventID, Subject: rec.Subject, Record: rec.Payload}
		if perr := dest.Publish(ctx, req); perr != nil {
			if ferr := o.recordFailure(gdb, del, perr.Error(), false); ferr != nil {
				return published, ferr
			}
			block(blocked, rec.Subject)
			continue
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

// block holds a non-empty subject so its later deliveries are not published
// ahead of an unresolved earlier one. Empty subjects have no ordering domain.
func block(blocked map[string]struct{}, subject string) {
	if subject != "" {
		blocked[subject] = struct{}{}
	}
}

// recordFailure increments the attempt count, records the error, and flags the
// delivery stuck (for alerting) once it has exceeded maxAttempts — or immediately
// for an unrecoverable failure. The delivery stays pending and is retried.
func (o *Outbox) recordFailure(gdb *gorm.DB, del *Delivery, reason string, immediate bool) error {
	del.Attempts++
	del.LastError = reason
	if del.StuckSince == nil && (immediate || del.Attempts >= o.maxAttempts) {
		now := o.now()
		del.StuckSince = &now
	}
	if err := gdb.Save(del).Error; err != nil {
		return fmt.Errorf("outbox: save delivery %d: %w", del.ID, err)
	}

	return nil
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

// maybeMarkDelivered sets Record.delivered_at once every delivery for it has been
// published (a stuck delivery keeps the record outstanding).
func (o *Outbox) maybeMarkDelivered(gdb *gorm.DB, outboxID uint64) error {
	var outstanding int64
	if err := gdb.Model(&Delivery{}).
		Where("outbox_id = ? AND published_at IS NULL", outboxID).
		Count(&outstanding).Error; err != nil {
		return fmt.Errorf("outbox: count outstanding for %d: %w", outboxID, err)
	}
	if outstanding > 0 {
		return nil
	}

	now := o.now()
	if err := gdb.Model(&Record{}).Where("id = ?", outboxID).Update("delivered_at", now).Error; err != nil {
		return fmt.Errorf("outbox: mark record %d delivered: %w", outboxID, err)
	}

	return nil
}
