package localdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/safedep/dry/log"

	// modernc.org/sqlite is a pure-Go SQLite driver (no CGO). It is registered
	// under the "sqlite" driver name (via the package init). Already a dry
	// dependency. Imported non-blank so SQLITE_BUSY can be detected from its
	// typed *sqlite.Error without string matching.
	sqlite "modernc.org/sqlite"
)

const (
	// driverName is the database/sql driver name registered by
	// modernc.org/sqlite.
	driverName = "sqlite"

	// defaultFileName is the DB file name used when Config.FileName is empty.
	defaultFileName = "local.db"

	// dirPerm is the permission used when creating Config.Dir.
	dirPerm = 0o700

	// pragmaQuery holds the fixed framework pragma invariants, applied per
	// connection via the modernc.org/sqlite _pragma DSN parameters. These are
	// not consumer-tunable (see the package design). modernc applies them on
	// every connection it opens (busy_timeout is pushed first by the driver).
	pragmaQuery = "_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(ON)"

	// trackerTable is the framework-owned migration tracker table. It is the
	// sole exception to "modules own their tables".
	trackerTable = "_localdb_schema_migrations"

	// sqliteBusyPrimaryCode is the SQLite primary result code SQLITE_BUSY (5).
	// Extended busy codes (e.g. SQLITE_BUSY_SNAPSHOT) share this low byte, so
	// matching against (code & 0xff) catches every busy variant.
	sqliteBusyPrimaryCode = 5

	// openMaxRetries bounds the cold-start open retry on SQLITE_BUSY. The
	// contention window (switching a fresh file into WAL mode) is sub-
	// millisecond, so a handful of short retries resolves it; the bound stops a
	// pathological peer from looping forever (busy_timeout still caps the wait
	// for the transactional lock contention separately).
	openMaxRetries = 10

	// openBackoffBase and openBackoffMax bound the exponential backoff between
	// cold-start open retries.
	openBackoffBase = time.Millisecond
	openBackoffMax  = 50 * time.Millisecond
)

// moduleNameRe validates Descriptor.Name. It doubles as the migration-tracking
// key and the recommended table prefix.
var moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// frameworkMigrations is localdb's own append-only schema, versioned by
// PRAGMA user_version (reserved entirely for localdb; modules never touch it).
// Entry 0 creates the tracker table. Evolving the tracker later is "append an
// ALTER TABLE" — never edit or reorder an existing entry.
var frameworkMigrations = []string{
	`CREATE TABLE ` + trackerTable + ` (
		module     TEXT    PRIMARY KEY,
		version    INTEGER NOT NULL,
		applied_at INTEGER NOT NULL
	)`,
}

// Descriptor is how a module declares its persistence needs.
type Descriptor struct {
	// Name is the module key, validated at Store against ^[a-z][a-z0-9_]*$.
	// Used as the migration-tracking key and the recommended table prefix.
	Name string

	// Migrations is an append-only list of SQL statements applied in order, each
	// exactly once. The tracker stores the count already applied; the next run
	// applies Migrations[count:]. May be empty (a module that only wants the
	// shared DB). Never edit or reorder an existing entry — only append.
	Migrations []string
}

// Manager owns the shared DB file and connection pool. Created once at startup.
type Manager interface {
	// Store lazily opens/creates the DB on the first call across all modules,
	// then applies this module's not-yet-applied migrations once, in order.
	// The whole operation is serialized under the Manager mutex. The resulting
	// *Store is cached by Name; repeated calls for the same Name return it. A
	// call reusing a Name with a different Migrations slice is a programming
	// error and returns an invalid-descriptor error.
	Store(ctx context.Context, d Descriptor) (*Store, error)

	// Close is idempotent and is a durability barrier. It waits for any
	// in-progress Store to finish (shared mutex), so quiesce module DB activity
	// before calling it.
	Close() error
}

// Config configures a Manager.
type Config struct {
	// Dir is the directory holding the DB file; chosen by the consumer. It must
	// be on a local filesystem (WAL is unsafe over network filesystems).
	Dir string

	// FileName overrides the DB file name. Defaults to "local.db" when empty.
	// Must be a bare file name — a path separator is rejected at first Store.
	FileName string
}

// Store is a module's handle to the shared database.
type Store struct {
	db *sql.DB
}

// DB returns the shared *sql.DB connection pool. The module runs its own SQL
// against its own tables.
func (s *Store) DB() *sql.DB {
	return s.db
}

// cachedStore records a returned *Store together with the migrations slice it
// was created with, so reusing a Name with a different slice can be rejected.
type cachedStore struct {
	store      *Store
	migrations []string
}

// manager is the concrete Manager. All state is guarded by mu; the whole Store
// operation runs under it, so the file opens exactly once and a module's
// migrations apply exactly once.
type manager struct {
	cfg Config

	mu     sync.Mutex
	db     *sql.DB // nil until the first Store opens it
	stores map[string]*cachedStore
	closed bool
}

// New returns a Manager bound to <Config.Dir>/<FileName> (FileName defaults to
// "local.db"). It touches no disk until the first Store call.
func New(cfg Config) Manager {
	return &manager{
		cfg:    cfg,
		stores: make(map[string]*cachedStore),
	}
}

// Store implements Manager.
func (m *manager) Store(ctx context.Context, d Descriptor) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := validateDescriptor(d); err != nil {
		return nil, err
	}

	if err := m.validateFileName(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, newError(ErrCodeManagerClosed, "manager is closed", nil)
	}

	if cached, ok := m.stores[d.Name]; ok {
		if !slices.Equal(cached.migrations, d.Migrations) {
			return nil, newError(ErrCodeInvalidDescriptor,
				fmt.Sprintf("module %q reused with a different migrations slice", d.Name), nil)
		}

		return cached.store, nil
	}

	// ctx may have been cancelled while waiting for the mutex.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := m.ensureOpen(ctx); err != nil {
		return nil, err
	}

	conn, err := m.db.Conn(ctx)
	if err != nil {
		return nil, newError(ErrCodeMigrationFailure, "acquire connection", err)
	}

	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Warnf("localdb: failed to release connection: %v", cerr)
		}
	}()

	if err := runModuleMigrations(ctx, conn, d); err != nil {
		return nil, err
	}

	store := &Store{db: m.db}
	m.stores[d.Name] = &cachedStore{
		store:      store,
		migrations: slices.Clone(d.Migrations),
	}

	return store, nil
}

// Close implements Manager. It is idempotent and a durability barrier: it runs
// PRAGMA wal_checkpoint(TRUNCATE) to flush committed WAL frames into the main
// database file and fsync them before closing the pool. Under
// synchronous=NORMAL commits are not fsync'd per commit, so this checkpoint is
// what makes committed writes durable once Close returns.
func (m *manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	if m.db == nil {
		// Never opened: nothing to checkpoint or close.
		return nil
	}

	db := m.db

	// wal_checkpoint reports its outcome in a result row (busy, log,
	// checkpointed) rather than as a SQL error. A busy=1 result means another
	// connection holds the WAL so it could not be truncated — expected, not a
	// failure.
	var busy, logFrames, checkpointed int
	row := db.QueryRowContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")
	if err := row.Scan(&busy, &logFrames, &checkpointed); err != nil {
		if cerr := db.Close(); cerr != nil {
			log.Warnf("localdb: failed to close db after checkpoint error: %v", cerr)
		}

		return newError(ErrCodeCheckpointFailure, "wal_checkpoint(TRUNCATE)", err)
	}

	if busy == 1 {
		log.Warnf("localdb: wal_checkpoint(TRUNCATE) did not truncate the WAL; " +
			"another connection holds the database open")
	}

	if err := db.Close(); err != nil {
		return newError(ErrCodeCloseFailure, "close db", err)
	}

	return nil
}

// ensureOpen creates the directory and opens the DB on first use, applies the
// fixed pragmas (via the DSN) and runs the framework migrations. Subsequent
// calls are a no-op. On failure m.db is left nil so the next Store retries.
func (m *manager) ensureOpen(ctx context.Context) error {
	if m.db != nil {
		return nil
	}

	if err := os.MkdirAll(m.cfg.Dir, dirPerm); err != nil {
		return newError(ErrCodeOpenFailure, "create directory", err)
	}

	db, err := sql.Open(driverName, m.dsn())
	if err != nil {
		return newError(ErrCodeOpenFailure, "open database", err)
	}

	// Serialize access within the process so goroutines never trip
	// SQLITE_BUSY against each other; cross-process contention is handled by
	// busy_timeout.
	db.SetMaxOpenConns(1)

	// Establishing the first physical connection switches a fresh file into WAL
	// mode, which takes a brief exclusive lock. SQLite does NOT invoke the
	// busy_timeout handler for this journal-mode transition, so two processes
	// opening the same new file can collide with SQLITE_BUSY. Retry the
	// connect + framework-migration a bounded number of times with backoff so
	// the cross-process cold-start race is absorbed centrally (re-running the
	// framework migrations is a no-op via user_version, so retries are safe).
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			closeQuietly(db)
			return err
		}

		err := bootstrapOnce(ctx, db)
		if err == nil {
			m.db = db
			return nil
		}

		if !isSQLiteBusy(err) || attempt >= openMaxRetries {
			closeQuietly(db)
			return err
		}

		if berr := sleepBackoff(ctx, attempt); berr != nil {
			closeQuietly(db)
			return berr
		}
	}
}

// bootstrapOnce acquires a single connection (which surfaces a malformed DSN /
// unreadable path or a cold-start SQLITE_BUSY) and runs the framework
// migrations on it.
func bootstrapOnce(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return newError(ErrCodeOpenFailure, "open database", err)
	}

	defer func() {
		if cerr := conn.Close(); cerr != nil {
			log.Warnf("localdb: failed to release connection: %v", cerr)
		}
	}()

	return runFrameworkMigrations(ctx, conn)
}

// isSQLiteBusy reports whether err (or any error it wraps) is a SQLite
// SQLITE_BUSY result, including its extended variants.
func isSQLiteBusy(err error) bool {
	var serr *sqlite.Error
	if errors.As(err, &serr) {
		return serr.Code()&0xff == sqliteBusyPrimaryCode
	}

	return false
}

// sleepBackoff waits an exponentially increasing, capped duration between
// cold-start open retries, returning early if ctx is cancelled.
func sleepBackoff(ctx context.Context, attempt int) error {
	d := openBackoffBase << attempt
	if d > openBackoffMax || d <= 0 {
		d = openBackoffMax
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// dsn builds the modernc.org/sqlite DSN. The path is URL-escaped (keeping path
// separators) so characters like '?' cannot corrupt the query; SQLite opens it
// in URI mode and the driver applies the _pragma parameters per connection.
func (m *manager) dsn() string {
	escaped := (&url.URL{Path: m.dbPath()}).EscapedPath()
	return "file:" + escaped + "?" + pragmaQuery
}

// dbPath is the absolute-or-relative path to the DB file.
func (m *manager) dbPath() string {
	return filepath.Join(m.cfg.Dir, m.fileName())
}

// fileName returns the configured file name or the default.
func (m *manager) fileName() string {
	if m.cfg.FileName != "" {
		return m.cfg.FileName
	}

	return defaultFileName
}

// validateFileName rejects a FileName containing a path separator.
func (m *manager) validateFileName() error {
	fn := m.cfg.FileName
	if fn == "" {
		return nil
	}

	if strings.ContainsAny(fn, `/\`) || fn != filepath.Base(fn) {
		return newError(ErrCodeInvalidDescriptor,
			fmt.Sprintf("FileName %q must not contain a path separator", fn), nil)
	}

	return nil
}

// validateDescriptor checks the module Name.
func validateDescriptor(d Descriptor) error {
	if !moduleNameRe.MatchString(d.Name) {
		return newError(ErrCodeInvalidDescriptor,
			fmt.Sprintf("invalid module name %q (must match %s)", d.Name, moduleNameRe.String()), nil)
	}

	return nil
}

// runFrameworkMigrations applies frameworkMigrations[user_version:], each in
// its own BEGIN IMMEDIATE transaction together with the user_version bump.
// Cross-process safe: the loser of a race waits (busy_timeout), re-reads
// user_version inside the transaction, and no-ops.
func runFrameworkMigrations(ctx context.Context, conn *sql.Conn) error {
	// Fast path: avoid acquiring the write lock when already up to date.
	uv, err := readUserVersion(ctx, conn)
	if err != nil {
		return newError(ErrCodeMigrationFailure, "read user_version", err)
	}

	for i := uv; i < len(frameworkMigrations); i++ {
		err := withImmediateTx(ctx, conn, func() error {
			// Re-read inside the write lock: a peer process may have applied
			// this entry between our read and acquiring the lock, in which case
			// we no-op and move on.
			cur, err := readUserVersion(ctx, conn)
			if err != nil {
				return err
			}

			if cur != i {
				return nil
			}

			if _, err := conn.ExecContext(ctx, frameworkMigrations[i]); err != nil {
				return err
			}

			// PRAGMA cannot be parameterized; the value is a trusted integer.
			_, err = conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1))
			return err
		})
		if err != nil {
			return newError(ErrCodeMigrationFailure, "apply framework migration", err)
		}
	}

	return nil
}

// runModuleMigrations applies d.Migrations[version:], each in its own
// BEGIN IMMEDIATE transaction together with the tracker version/applied_at bump
// for the module, so a partially applied migration cannot leave the tracker
// inconsistent.
func runModuleMigrations(ctx context.Context, conn *sql.Conn, d Descriptor) error {
	version, err := readModuleVersion(ctx, conn, d.Name)
	if err != nil {
		return newError(ErrCodeMigrationFailure, "read module version", err)
	}

	for i := version; i < len(d.Migrations); i++ {
		err := withImmediateTx(ctx, conn, func() error {
			// Re-read inside the write lock: a peer process may have applied
			// this entry between our read and acquiring the lock, in which case
			// we no-op and move on.
			cur, err := readModuleVersion(ctx, conn, d.Name)
			if err != nil {
				return err
			}
			if cur != i {
				return nil
			}

			if _, err := conn.ExecContext(ctx, d.Migrations[i]); err != nil {
				return err
			}

			_, err = conn.ExecContext(ctx,
				`INSERT INTO `+trackerTable+` (module, version, applied_at)
				 VALUES (?, ?, ?)
				 ON CONFLICT(module) DO UPDATE SET
				   version = excluded.version,
				   applied_at = excluded.applied_at`,
				d.Name, i+1, time.Now().Unix())
			return err
		})
		if err != nil {
			return newError(ErrCodeMigrationFailure,
				fmt.Sprintf("apply migration for module %q", d.Name), err)
		}
	}

	return nil
}

// readUserVersion reads PRAGMA user_version (0 on a new file).
func readUserVersion(ctx context.Context, conn *sql.Conn) (int, error) {
	var v int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, err
	}

	return v, nil
}

// readModuleVersion reads a module's last-applied migration count (0 if absent).
func readModuleVersion(ctx context.Context, conn *sql.Conn, name string) (int, error) {
	var v int
	err := conn.QueryRowContext(ctx,
		`SELECT version FROM `+trackerTable+` WHERE module = ?`, name).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return v, nil
}

// withImmediateTx runs fn inside a BEGIN IMMEDIATE / COMMIT, rolling back on
// error. BEGIN IMMEDIATE acquires the write lock up front so two processes
// cannot migrate concurrently. Rollback uses a background context so a
// cancelled ctx does not prevent cleanup.
func withImmediateTx(ctx context.Context, conn *sql.Conn, fn func() error) (err error) {
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if _, rbErr := conn.ExecContext(context.Background(), "ROLLBACK"); rbErr != nil {
				// A failed ROLLBACK on a local SQLite connection is essentially
				// impossible (it only clears in-memory state), but if it does
				// happen the connection may still hold an open transaction.
				// Surface it joined with the original error rather than
				// swallowing it, so the caller does not treat a half-failed
				// transaction as a clean failure.
				log.Warnf("localdb: transaction rollback failed: %v", rbErr)
				err = errors.Join(err, fmt.Errorf("rollback after failure: %w", rbErr))
			}
		}
	}()

	if err = fn(); err != nil {
		return err
	}

	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}

	return nil
}

// closeQuietly closes db, logging any error.
func closeQuietly(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Warnf("localdb: failed to close db: %v", err)
	}
}
