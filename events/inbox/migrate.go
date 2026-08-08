package inbox

import "github.com/safedep/dry/db"

// Migrate creates/updates the inbox tables via the consumer's adapter. The tables
// live in the consumer's database; dry owns only the models. Run it from the
// consumer's migration pipeline. The processed-event and dead-letter tables are
// only used when WithDedup / a dead-letter error policy are enabled, but are
// migrated unconditionally so enabling either later needs no schema change.
func Migrate(adapter db.SqlDataAdapter) error {
	gdb, err := adapter.GetDB()
	if err != nil {
		return err
	}
	return gdb.AutoMigrate(&Cursor{}, &ProcessedEvent{}, &DeadLetter{})
}
