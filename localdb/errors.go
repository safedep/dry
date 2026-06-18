package localdb

import "github.com/safedep/dry/usefulerror"

// Error codes carried by errors returned from this package. Consumers can
// classify failures without string matching via usefulerror.AsUsefulError and
// inspecting Code().
const (
	// ErrCodeInvalidDescriptor indicates a Descriptor (or the configured
	// FileName) failed validation: a malformed module Name, a FileName
	// containing a path separator, or a Name reused with a different
	// Migrations slice.
	ErrCodeInvalidDescriptor = "localdb_invalid_descriptor"

	// ErrCodeOpenFailure indicates the database directory or file could not be
	// created or opened.
	ErrCodeOpenFailure = "localdb_open_failure"

	// ErrCodeMigrationFailure indicates a framework or module migration failed
	// to apply.
	ErrCodeMigrationFailure = "localdb_migration_failure"

	// ErrCodeManagerClosed indicates Store was called after Close.
	ErrCodeManagerClosed = "localdb_manager_closed"

	// ErrCodeCheckpointFailure indicates the durability-barrier WAL checkpoint
	// in Close failed (an actual SQL/exec error, not a non-truncating
	// busy result).
	ErrCodeCheckpointFailure = "localdb_checkpoint_failure"

	// ErrCodeCloseFailure indicates the connection pool failed to close in
	// Close (after the checkpoint succeeded).
	ErrCodeCloseFailure = "localdb_close_failure"
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
