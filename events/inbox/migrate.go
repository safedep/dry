package inbox

import "github.com/safedep/dry/db"

// Migrate creates/updates the inbox tables via the consumer's adapter. The tables
// live in the consumer's database; dry owns only the models. Run it from the
// consumer's migration pipeline. The processed-event table is only used when
// WithDedup is enabled, but is migrated unconditionally so enabling dedup later
// needs no schema change.
func Migrate(adapter db.SqlDataAdapter) error {
	gdb, err := adapter.GetDB()
	if err != nil {
		return err
	}
	return gdb.AutoMigrate(&Cursor{}, &ProcessedEvent{})
}
