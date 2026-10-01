package localdb

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// FileManager is a Manager that also operates on the database file. New and
// NewFileManager return the same type, so callers that hold a Manager from New
// can use a type assertion.
type FileManager interface {
	Manager

	// Path returns <Config.Dir>/<FileName>.
	Path() string

	// Size returns the total size in bytes of the database file and its -wal,
	// -shm and -journal siblings. Missing files count as zero.
	Size() (int64, error)

	// Vacuum rebuilds the database file to release free pages, then truncates
	// the WAL. It is a no-op when the file does not exist. It takes the write
	// lock for the whole rebuild.
	Vacuum(ctx context.Context) error

	// Remove closes the manager, then deletes the database file and its -wal,
	// -shm and -journal siblings. The manager stays closed. Other processes
	// must not have the file open.
	Remove() error
}

var fileSuffixes = []string{"", "-wal", "-shm", "-journal"}

func (m *manager) Path() string {
	return m.dbPath()
}

func (m *manager) files() []string {
	p := m.dbPath()
	files := make([]string, 0, len(fileSuffixes))
	for _, s := range fileSuffixes {
		files = append(files, p+s)
	}

	return files
}

func (m *manager) Size() (int64, error) {
	if err := m.validateFileName(); err != nil {
		return 0, err
	}

	var total int64
	for _, p := range m.files() {
		fi, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return 0, newError(ErrCodeSizeFailure, fmt.Sprintf("stat %s", p), err)
		}

		total += fi.Size()
	}

	return total, nil
}

func (m *manager) Vacuum(ctx context.Context) error {
	if err := m.validateFileName(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return newError(ErrCodeManagerClosed, "manager is closed", nil)
	}

	if m.db == nil {
		_, err := os.Stat(m.dbPath())
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		if err != nil {
			return newError(ErrCodeVacuumFailure, "stat database file", err)
		}
	}

	if err := m.ensureOpen(ctx); err != nil {
		return err
	}

	if _, err := m.db.ExecContext(ctx, "VACUUM"); err != nil {
		return newError(ErrCodeVacuumFailure, "vacuum", err)
	}

	var busy, logFrames, checkpointed int
	row := m.db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	if err := row.Scan(&busy, &logFrames, &checkpointed); err != nil {
		return newError(ErrCodeVacuumFailure, "wal_checkpoint(TRUNCATE)", err)
	}

	return nil
}

func (m *manager) Remove() error {
	if err := m.validateFileName(); err != nil {
		return err
	}

	var errs []error
	if err := m.Close(); err != nil {
		errs = append(errs, err)
	}

	for _, p := range m.files() {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, newError(ErrCodeRemoveFailure, fmt.Sprintf("remove %s", p), err))
		}
	}

	return errors.Join(errs...)
}
