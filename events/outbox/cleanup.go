package outbox

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Cleanup deletes fully-delivered records (and their deliveries) older than the
// configured retention, up to a bounded batch. Stuck and pending records are
// never removed — only records whose delivered_at is set and past the retention
// window. The transport (e.g. S2) remains the durable source for replay beyond
// that window. Returns the number of records deleted.
func (o *Outbox) Cleanup(ctx context.Context) (int, error) {
	if o.store == nil {
		return 0, nil
	}

	gdb, err := o.store.GetDB()
	if err != nil {
		return 0, fmt.Errorf("outbox: get db: %w", err)
	}
	gdb = gdb.WithContext(ctx)

	cutoff := o.now().Add(-o.retention)

	var ids []uint64
	if err := gdb.Model(&Record{}).
		Where("delivered_at IS NOT NULL AND delivered_at < ?", cutoff).
		Order("id ASC").
		Limit(o.cleanupBatchSize).
		Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("outbox: select expired records: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	err = gdb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("outbox_id IN ?", ids).Delete(&Delivery{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Delete(&Record{}).Error
	})
	if err != nil {
		return 0, fmt.Errorf("outbox: delete expired records: %w", err)
	}

	return len(ids), nil
}
