package localdb

import "github.com/safedep/dry/usefulerror"

// Error codes carried by errors returned from this package. Consumers can
// classify failures without string matching via usefulerror.AsUsefulError and
// inspecting Code().
const (
	// Validation failure: a bad module Name, a FileName with a path separator,
	// or a Name reused with a different Migrations slice.
	ErrCodeInvalidDescriptor = "localdb_invalid_descriptor"

	// The database directory or file could not be created or opened.
	ErrCodeOpenFailure = "localdb_open_failure"

	// A framework or module migration failed to apply.
	ErrCodeMigrationFailure = "localdb_migration_failure"

	// Store was called after Close.
	ErrCodeManagerClosed = "localdb_manager_closed"

	// The Close WAL checkpoint failed with an actual SQL/exec error.
	ErrCodeCheckpointFailure = "localdb_checkpoint_failure"

	// The connection pool failed to close (after a successful checkpoint).
	ErrCodeCloseFailure = "localdb_close_failure"

	// The stored schema version is ahead of what this binary/descriptor knows
	// (a downgrade or truncated migration list). Not retryable.
	ErrCodeIncompatibleSchema = "localdb_incompatible_schema"
)

// newError builds a usefulerror with the given code and message, optionally
// wrapping an underlying error so errors.Is/As keep working.
func newError(code, msg string, cause error) error {
	b := usefulerror.NewUsefulError().WithCode(code).WithMsg(msg)
	if cause != nil {
		b = b.Wrap(cause)
	}

	return b
}
