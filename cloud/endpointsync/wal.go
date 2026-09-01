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

	// defaultMaxDedupKeys bounds the dedup_state table. At the cap a new
	// key skips dedup and its events deliver directly, so a
	// high-cardinality key source degrades to no dedup, never to
	// unbounded growth. No count is ever lost.
	defaultMaxDedupKeys = 10000

	// dedupSweepBatch bounds the sweep's memory to one page of state
	// rows.
	dedupSweepBatch = 1000

	statusPending   = "pending"
	statusDelivered = "delivered"
)

type walEvent struct {
	eventID string
	payload []byte
}

type wal struct {
	mu           sync.Mutex
	db           *sql.DB
	maxPending   int
	maxDedupKeys int
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
		db:           db,
		maxPending:   defaultMaxPending,
		maxDedupKeys: defaultMaxDedupKeys,
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
	// Dedup state, one row per open dedup window. The row holds the count
	// of held-back events and the last held-back event as the carrier.
	// The row count is bounded by distinct active keys, not event volume.
	"20260901000000": `
		CREATE TABLE IF NOT EXISTS dedup_state (
			key_hash BLOB NOT NULL PRIMARY KEY,
			rule TEXT NOT NULL,
			window_start INTEGER NOT NULL,
			suppressed INTEGER NOT NULL DEFAULT 0,
			carrier BLOB
		) WITHOUT ROWID;
	`,
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

	if err := w.txInsertEvents(tx, walEvent{eventID: eventID, payload: payload}); err != nil {
		return err
	}

	return tx.Commit()
}

// carrierFlushFunc rewrites a held-back event with its repeat count before
// the WAL persists it. It runs inside the WAL transaction, so a flush and
// its state reset commit together.
type carrierFlushFunc func(carrier []byte, suppressed int64) (eventID string, payload []byte, err error)

// dedupEmit runs the dedup decision for one claimed event in one
// transaction. The pending queue and the dedup state never disagree:
// a crash rolls back the whole branch.
func (w *wal) dedupEmit(eventID string, payload []byte, keyHash []byte, rule string, nowMs int64, windowMs int64, flush carrierFlushFunc) error {
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

	var windowStart, suppressed int64
	var carrier []byte
	err = tx.QueryRow(
		"SELECT window_start, suppressed, carrier FROM dedup_state WHERE key_hash = ?",
		keyHash,
	).Scan(&windowStart, &suppressed, &carrier)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := w.txInsertEvents(tx, walEvent{eventID: eventID, payload: payload}); err != nil {
			return err
		}

		// At the key cap the event delivers with no window, so the state
		// table never grows without bound.
		var keys int
		if err := tx.QueryRow("SELECT COUNT(*) FROM dedup_state").Scan(&keys); err != nil {
			return fmt.Errorf("failed to count dedup keys: %w", err)
		}
		if keys < w.maxDedupKeys {
			if _, err := tx.Exec(
				"INSERT INTO dedup_state (key_hash, rule, window_start, suppressed, carrier) VALUES (?, ?, ?, 0, NULL)",
				keyHash, rule, nowMs,
			); err != nil {
				return fmt.Errorf("failed to open dedup window: %w", err)
			}
		}
		return tx.Commit()

	case err != nil:
		return fmt.Errorf("failed to read dedup state: %w", err)
	}

	// A clock that steps backward counts as in-window: nowMs below
	// windowStart also satisfies this check. Expiring on a backward step
	// would turn a clock jump into an emit storm.
	if nowMs < windowStart+windowMs {
		if _, err := tx.Exec(
			"UPDATE dedup_state SET suppressed = suppressed + 1, carrier = ? WHERE key_hash = ?",
			payload, keyHash,
		); err != nil {
			return fmt.Errorf("failed to count held-back event: %w", err)
		}
		return tx.Commit()
	}

	inserts := []walEvent{{eventID: eventID, payload: payload}}
	if suppressed > 0 && carrier != nil {
		carrierID, carrierPayload, err := flush(carrier, suppressed)
		if err != nil {
			return err
		}
		inserts = append([]walEvent{{eventID: carrierID, payload: carrierPayload}}, inserts...)
	}
	if err := w.txInsertEvents(tx, inserts...); err != nil {
		return err
	}
	if _, err := tx.Exec(
		"UPDATE dedup_state SET window_start = ?, suppressed = 0, carrier = NULL WHERE key_hash = ?",
		nowMs, keyHash,
	); err != nil {
		return fmt.Errorf("failed to reset dedup window: %w", err)
	}
	return tx.Commit()
}

// dedupSweep flushes every dedup window that expired judges as closed, and
// deletes its row. A carrier leaves dedup_state only when its insert
// succeeds: on a full queue the row stays for the next sweep.
//
// The sweep pages by primary key and commits one transaction per page, so
// it buffers at most one page of rows and never holds the write lock for
// long. The sweep is idempotent per row, so a crash between pages leaves
// the remaining rows for the next sweep.
func (w *wal) dedupSweep(expired func(rule string, windowStart int64) bool, flush carrierFlushFunc) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	after := []byte{}
	for {
		count, last, err := w.sweepPage(after, expired, flush)
		if err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		after = last
	}
}

// sweepPage processes one page of dedup_state rows in its own transaction.
// The single connection cannot run statements while a cursor is open, so
// the page is read fully before it is processed.
func (w *wal) sweepPage(after []byte, expired func(rule string, windowStart int64) bool, flush carrierFlushFunc) (int, []byte, error) {
	tx, err := w.db.Begin()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Warnf("endpointsync: tx rollback: %v", err)
		}
	}()

	type stateRow struct {
		keyHash     []byte
		rule        string
		windowStart int64
		suppressed  int64
		carrier     []byte
	}

	rows, err := tx.Query(
		"SELECT key_hash, rule, window_start, suppressed, carrier FROM dedup_state WHERE key_hash > ? ORDER BY key_hash LIMIT ?",
		after, dedupSweepBatch,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read dedup state: %w", err)
	}

	states := make([]stateRow, 0, dedupSweepBatch)
	for rows.Next() {
		var s stateRow
		if err := rows.Scan(&s.keyHash, &s.rule, &s.windowStart, &s.suppressed, &s.carrier); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Warnf("endpointsync: rows close: %v", closeErr)
			}
			return 0, nil, fmt.Errorf("failed to scan dedup state: %w", err)
		}
		states = append(states, s)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("failed to read dedup state: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, nil, fmt.Errorf("failed to close dedup state rows: %w", err)
	}

	if len(states) == 0 {
		return 0, nil, nil
	}

	for _, s := range states {
		if !expired(s.rule, s.windowStart) {
			continue
		}
		if s.suppressed > 0 && s.carrier != nil {
			carrierID, carrierPayload, err := flush(s.carrier, s.suppressed)
			if err != nil {
				return 0, nil, err
			}
			if err := w.txInsertEvents(tx, walEvent{eventID: carrierID, payload: carrierPayload}); err != nil {
				if errors.Is(err, ErrWALFull) {
					continue
				}
				return 0, nil, err
			}
		}
		if _, err := tx.Exec("DELETE FROM dedup_state WHERE key_hash = ?", s.keyHash); err != nil {
			return 0, nil, fmt.Errorf("failed to delete dedup state: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, nil, fmt.Errorf("failed to commit sweep page: %w", err)
	}
	return len(states), states[len(states)-1].keyHash, nil
}

// txInsertEvents inserts events and maintains pending_count inside the
// caller's transaction. It enforces the maxPending bound for the whole
// group, so a partial group never commits.
func (w *wal) txInsertEvents(tx *sql.Tx, events ...walEvent) error {
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO wal_meta (id, pending_count) VALUES (1, 0)",
	); err != nil {
		return fmt.Errorf("failed to ensure wal_meta row: %w", err)
	}

	var count int
	if err := tx.QueryRow("SELECT pending_count FROM wal_meta WHERE id = 1").Scan(&count); err != nil {
		return fmt.Errorf("failed to read pending count: %w", err)
	}
	if count+len(events) > w.maxPending {
		return ErrWALFull
	}

	for _, e := range events {
		if _, err := tx.Exec(
			"INSERT INTO events (event_id, payload, status) VALUES (?, ?, ?)",
			e.eventID, e.payload, statusPending,
		); err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	if _, err := tx.Exec(
		"UPDATE wal_meta SET pending_count = pending_count + ? WHERE id = 1",
		len(events),
	); err != nil {
		return fmt.Errorf("failed to update pending count: %w", err)
	}
	return nil
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
