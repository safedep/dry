package endpointsync

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/safedep/dry/log"
	_ "modernc.org/sqlite"
)

const (
	defaultMaxPending = 100000

	statusPending   = "pending"
	statusDelivered = "delivered"
)

type walEvent struct {
	eventID string
	payload []byte
}

type wal struct {
	mu         sync.Mutex
	db         *sql.DB
	maxPending int
}

func openWAL(path string) (*wal, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWALOpen, err)
	}

	// Serialize all operations through a single connection to avoid
	// SQLITE_BUSY errors from concurrent connections in the pool.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Warnf("endpointsync: failed to close db after busy_timeout error: %v", closeErr)
		}
		return nil, fmt.Errorf("%w: failed to set busy_timeout: %w", ErrWALOpen, err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Warnf("endpointsync: failed to close db after WAL mode error: %v", closeErr)
		}
		return nil, fmt.Errorf("%w: failed to set WAL mode: %w", ErrWALOpen, err)
	}

	if err := initSchema(db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Warnf("endpointsync: failed to close db after schema init error: %v", closeErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrWALOpen, err)
	}

	if err := migrateSchema(db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Warnf("endpointsync: failed to close db after migration error: %v", closeErr)
		}
		return nil, fmt.Errorf("%w: migration failed: %w", ErrWALOpen, err)
	}

	return &wal{
		db:         db,
		maxPending: defaultMaxPending,
	}, nil
}

// initSchema creates the base WAL tables. This runs on every open via
// CREATE IF NOT EXISTS, so it is safe to call repeatedly.
func initSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id TEXT NOT NULL UNIQUE,
			payload BLOB NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
		CREATE TABLE IF NOT EXISTS wal_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			pending_count INTEGER NOT NULL DEFAULT 0
		);
		INSERT OR IGNORE INTO wal_meta (id, pending_count) VALUES (1, 0);
		CREATE TABLE IF NOT EXISTS wal_migrations (
			id TEXT PRIMARY KEY
		);
	`
	_, err := db.Exec(schema)
	return err
}

// migrations is a declarative map of migration ID to SQL statement.
// IDs are timestamps at second granularity (YYYYMMDDHHMMSS) to ensure
// natural ordering and avoid conflicts.
//
// To add a new migration:
//  1. Add an entry with a timestamp ID and the SQL to execute
//  2. Migrations must be idempotent (use IF NOT EXISTS, IF EXISTS, etc.)
//     since a crash mid-migration could leave partial state
//  3. Never remove or modify an existing migration
//
// Example:
//
//	var migrations = map[string]string{
//	    "20260407120000": "ALTER TABLE events ADD COLUMN source TEXT DEFAULT '';",
//	    "20260410090000": "CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);",
//	}
var migrations = map[string]string{
	// No migrations yet. Base schema is created by initSchema.
}

// migrateSchema applies any migrations from the migrations map that have
// not yet been recorded in the wal_migrations table. Each migration runs
// in its own transaction. On success, the migration ID is inserted into
// wal_migrations so it won't run again.
func migrateSchema(db *sql.DB) error {
	ids := make([]string, 0, len(migrations))
	for id := range migrations {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		query := migrations[id]
		var exists int
		err := db.QueryRow("SELECT COUNT(*) FROM wal_migrations WHERE id = ?", id).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration %s: %w", id, err)
		}
		if exists > 0 {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin migration %s: %w", id, err)
		}

		if _, err := tx.Exec(query); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Warnf("endpointsync: rollback failed for migration %s: %v", id, rbErr)
			}
			return fmt.Errorf("migration %s failed: %w", id, err)
		}

		if _, err := tx.Exec("INSERT INTO wal_migrations (id) VALUES (?)", id); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				log.Warnf("endpointsync: rollback failed for migration %s: %v", id, rbErr)
			}
			return fmt.Errorf("failed to record migration %s: %w", id, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", id, err)
		}

		log.Infof("endpointsync: applied migration %s", id)
	}

	return nil
}

func (w *wal) insert(eventID string, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	tx, err := w.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Warnf("endpointsync: tx rollback: %v", err)
		}
	}()

	// Ensure wal_meta row exists (defensive: initSchema should have created it)
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO wal_meta (id, pending_count) VALUES (1, 0)",
	); err != nil {
		return fmt.Errorf("failed to ensure wal_meta row: %w", err)
	}

	var count int
	if err := tx.QueryRow("SELECT pending_count FROM wal_meta WHERE id = 1").Scan(&count); err != nil {
		return fmt.Errorf("failed to read pending count: %w", err)
	}

	if count >= w.maxPending {
		return ErrWALFull
	}

	if _, err := tx.Exec(
		"INSERT INTO events (event_id, payload, status) VALUES (?, ?, ?)",
		eventID, payload, statusPending,
	); err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	if _, err := tx.Exec(
		"UPDATE wal_meta SET pending_count = pending_count + 1 WHERE id = 1",
	); err != nil {
		return fmt.Errorf("failed to update pending count: %w", err)
	}

	return tx.Commit()
}

func (w *wal) readPending(limit int) ([]walEvent, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	rows, err := w.db.Query(
		"SELECT event_id, payload FROM events WHERE status = ? ORDER BY id LIMIT ?",
		statusPending, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read pending events: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Warnf("endpointsync: rows close: %v", err)
		}
	}()

	var events []walEvent
	for rows.Next() {
		var e walEvent
		if err := rows.Scan(&e.eventID, &e.payload); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (w *wal) markDelivered(eventIDs []string) (int, error) {
	if len(eventIDs) == 0 {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	tx, err := w.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Warnf("endpointsync: tx rollback: %v", err)
		}
	}()

	var actualCount int64
	for _, id := range eventIDs {
		result, err := tx.Exec(
			"UPDATE events SET status = ? WHERE event_id = ? AND status = ?",
			statusDelivered, id, statusPending,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to mark event delivered: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("failed to get rows affected: %w", err)
		}
		actualCount += affected
	}

	if actualCount > 0 {
		if _, err := tx.Exec(
			"UPDATE wal_meta SET pending_count = pending_count - ? WHERE id = 1",
			actualCount,
		); err != nil {
			return 0, fmt.Errorf("failed to update pending count: %w", err)
		}
	}

	return int(actualCount), tx.Commit()
}

func (w *wal) purge() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, err := w.db.Exec("DELETE FROM events WHERE status = ?", statusDelivered)
	if err != nil {
		return fmt.Errorf("failed to purge delivered events: %w", err)
	}
	return nil
}

func (w *wal) close() error {
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}
