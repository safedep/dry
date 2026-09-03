package endpointsync

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
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

	// dedupKeys tracks the dedup_state row count in this process, loaded
	// once at open, so the key cap costs no per-emit table scan. Another
	// process can drift it, so the cap is approximate across processes:
	// acceptable for a defense bound.
	dedupKeys int

	closed bool
}

func openWAL(path string) (*wal, error) {
	// _txlock=immediate makes every transaction take the write lock at
	// BEGIN, where busy_timeout applies. A deferred transaction that
	// reads first and writes later fails with SQLITE_BUSY_SNAPSHOT when
	// another process commits in between, and the busy handler never
	// runs mid-transaction.
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	db, err := sql.Open("sqlite", path+separator+"_txlock=immediate")
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

	var dedupKeys int
	if err := db.QueryRow("SELECT COUNT(*) FROM dedup_state").Scan(&dedupKeys); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Warnf("endpointsync: failed to close db after dedup count error: %v", closeErr)
		}
		return nil, fmt.Errorf("%w: failed to count dedup keys: %w", ErrWALOpen, err)
	}

	return &wal{
		db:           db,
		maxPending:   defaultMaxPending,
		maxDedupKeys: defaultMaxDedupKeys,
		dedupKeys:    dedupKeys,
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
	// of held-back events, the last held-back event as the carrier, and
	// its own expiry, so any client that opens the WAL sweeps it on time
	// whatever rules that client declares. The row count is bounded by
	// distinct active keys, not event volume.
	"20260901000000": `
		CREATE TABLE IF NOT EXISTS dedup_state (
			key_hash BLOB NOT NULL PRIMARY KEY,
			rule TEXT NOT NULL,
			window_start INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			suppressed INTEGER NOT NULL DEFAULT 0,
			carrier BLOB
		) WITHOUT ROWID;
	`,
}

// migrateSchema applies any migrations from the migrations map that have
// not yet been recorded in the wal_migrations table. Each migration runs
// in its own immediate transaction, with the applied-check inside it, so
// two processes that open the same WAL concurrently cannot both apply
// one migration or fail on its record.
func migrateSchema(db *sql.DB) error {
	ids := make([]string, 0, len(migrations))
	for id := range migrations {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		if err := applyMigration(db, id, migrations[id]); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(db *sql.DB, id string, query string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin migration %s: %w", id, err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Warnf("endpointsync: rollback failed for migration %s: %v", id, err)
		}
	}()

	var applied int
	if err := tx.QueryRow("SELECT COUNT(*) FROM wal_migrations WHERE id = ?", id).Scan(&applied); err != nil {
		return fmt.Errorf("failed to check migration %s: %w", id, err)
	}
	if applied > 0 {
		return nil
	}

	if _, err := tx.Exec(query); err != nil {
		return fmt.Errorf("migration %s failed: %w", id, err)
	}

	if _, err := tx.Exec("INSERT OR IGNORE INTO wal_migrations (id) VALUES (?)", id); err != nil {
		return fmt.Errorf("failed to record migration %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %s: %w", id, err)
	}

	log.Infof("endpointsync: applied migration %s", id)
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

// dedupEmit runs the dedup decision for one claimed event in one
// transaction. The pending queue and the dedup state never disagree:
// a crash rolls back the whole branch.
func (w *wal) dedupEmit(eventID string, payload []byte, keyHash []byte, rule string, nowMs int64, windowMs int64) error {
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

	// The hot path: one conditional update covers the in-window
	// suppress, with no carrier read. A window whose start lies in the
	// future (the clock stepped back, or the window opened under a
	// forward-skewed clock) counts the event and rebases onto the
	// current clock, so a skew costs at most one window and never an
	// emit storm.
	res, err := tx.Exec(`
		UPDATE dedup_state SET
			suppressed = suppressed + 1,
			carrier = ?,
			expires_at = CASE WHEN ? < window_start THEN ? + ? ELSE expires_at END,
			window_start = CASE WHEN ? < window_start THEN ? ELSE window_start END
		WHERE key_hash = ? AND (? < expires_at OR ? < window_start)`,
		payload, nowMs, nowMs, windowMs, nowMs, nowMs, keyHash, nowMs, nowMs)
	if err != nil {
		return fmt.Errorf("failed to count held-back event: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read rows affected: %w", err)
	}
	if affected == 1 {
		// The suppress branch never touches the events table, so it must
		// reject a reused event id itself to keep the WAL's uniqueness
		// contract. The branches that insert rely on the UNIQUE
		// constraint. Without this, the duplicate surfaces later as an
		// unflushable carrier. The rollback discards the count above.
		var one int
		err = tx.QueryRow("SELECT 1 FROM events WHERE event_id = ?", eventID).Scan(&one)
		if err == nil {
			return fmt.Errorf("endpointsync: duplicate event id %s", eventID)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check event id: %w", err)
		}
		return tx.Commit()
	}

	var suppressed int64
	var carrier []byte
	err = tx.QueryRow(
		"SELECT suppressed, carrier FROM dedup_state WHERE key_hash = ?",
		keyHash,
	).Scan(&suppressed, &carrier)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := w.txInsertEvents(tx, walEvent{eventID: eventID, payload: payload}); err != nil {
			return err
		}

		// At the key cap the event delivers with no window, so the state
		// table never grows without bound.
		if w.dedupKeys >= w.maxDedupKeys {
			return tx.Commit()
		}
		if _, err := tx.Exec(
			"INSERT INTO dedup_state (key_hash, rule, window_start, expires_at, suppressed, carrier) VALUES (?, ?, ?, ?, 0, NULL)",
			keyHash, rule, nowMs, nowMs+windowMs,
		); err != nil {
			return fmt.Errorf("failed to open dedup window: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		w.dedupKeys++
		return nil

	case err != nil:
		return fmt.Errorf("failed to read dedup state: %w", err)
	}

	// The window expired. The live event outranks the carrier: it takes
	// the first free slot, and a carrier that does not fit stays in the
	// row for the next sweep, with the window rebased so counting
	// continues on top of the kept count.
	if err := w.txInsertEvents(tx, walEvent{eventID: eventID, payload: payload}); err != nil {
		return err
	}
	keepCount := false
	if suppressed > 0 && carrier != nil {
		if err := w.txFlushCarrier(tx, carrier, suppressed); err != nil {
			if errors.Is(err, ErrWALFull) {
				keepCount = true
			} else {
				// A carrier that cannot flush is poison: keeping it
				// blackholes this key forever. Drop the count, keep
				// the events flowing.
				log.Errorf("endpointsync: dropping unflushable dedup carrier, %d held-back events lost: %v", suppressed, err)
			}
		}
	}
	if keepCount {
		if _, err := tx.Exec(
			"UPDATE dedup_state SET window_start = ?, expires_at = ? WHERE key_hash = ?",
			nowMs, nowMs+windowMs, keyHash,
		); err != nil {
			return fmt.Errorf("failed to rebase dedup window: %w", err)
		}
	} else {
		if _, err := tx.Exec(
			"UPDATE dedup_state SET window_start = ?, expires_at = ?, suppressed = 0, carrier = NULL WHERE key_hash = ?",
			nowMs, nowMs+windowMs, keyHash,
		); err != nil {
			return fmt.Errorf("failed to reset dedup window: %w", err)
		}
	}
	return tx.Commit()
}

// txFlushCarrier rewrites a held-back event with its repeat count and
// inserts it inside a savepoint, so a failed insert never leaves the
// pending counter out of step with the events table.
func (w *wal) txFlushCarrier(tx *sql.Tx, carrier []byte, suppressed int64) error {
	carrierID, carrierPayload, err := carrierWithRepeatCount(carrier, suppressed)
	if err != nil {
		return err
	}

	if _, err := tx.Exec("SAVEPOINT carrier_flush"); err != nil {
		return fmt.Errorf("failed to create savepoint: %w", err)
	}
	if err := w.txInsertEvents(tx, walEvent{eventID: carrierID, payload: carrierPayload}); err != nil {
		if _, rbErr := tx.Exec("ROLLBACK TO carrier_flush"); rbErr != nil {
			return fmt.Errorf("failed to roll back savepoint after %w: %w", err, rbErr)
		}
		if _, relErr := tx.Exec("RELEASE carrier_flush"); relErr != nil {
			return fmt.Errorf("failed to release savepoint after %w: %w", err, relErr)
		}
		return err
	}
	if _, err := tx.Exec("RELEASE carrier_flush"); err != nil {
		return fmt.Errorf("failed to release savepoint: %w", err)
	}
	return nil
}

// dedupSweep flushes every dedup window whose recorded expiry has passed
// and deletes its row. The row carries its own expiry, so the sweep does
// not depend on which client's rules are declared. A carrier that does
// not fit the queue stays for the next sweep; a carrier that cannot
// flush is poison and its row is dropped with the loss logged.
//
// The sweep pages by primary key and commits one transaction per page, so
// it buffers at most one page of rows and never holds the write lock for
// long. The sweep is idempotent per row, so a crash between pages leaves
// the remaining rows for the next sweep.
func (w *wal) dedupSweep(nowMs int64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	after := []byte{}
	for {
		count, last, deleted, err := w.sweepPage(after, nowMs)
		w.dedupKeys -= deleted
		if w.dedupKeys < 0 {
			w.dedupKeys = 0
		}
		if err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		after = last
	}
}

// sweepPage processes one page of expired dedup_state rows in its own
// transaction. The single connection cannot run statements while a
// cursor is open, so the page is read fully before it is processed. A
// window whose start lies in the future counts as expired here: the
// sweep cannot rebase it the way an emit can, and flushing bounds a
// forward clock skew to one window.
func (w *wal) sweepPage(after []byte, nowMs int64) (int, []byte, int, error) {
	tx, err := w.db.Begin()
	if err != nil {
		return 0, nil, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Warnf("endpointsync: tx rollback: %v", err)
		}
	}()

	type stateRow struct {
		keyHash    []byte
		suppressed int64
		carrier    []byte
	}

	rows, err := tx.Query(
		"SELECT key_hash, suppressed, carrier FROM dedup_state WHERE key_hash > ? AND (expires_at <= ? OR window_start > ?) ORDER BY key_hash LIMIT ?",
		after, nowMs, nowMs, dedupSweepBatch,
	)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("failed to read dedup state: %w", err)
	}

	states := make([]stateRow, 0, dedupSweepBatch)
	for rows.Next() {
		var s stateRow
		if err := rows.Scan(&s.keyHash, &s.suppressed, &s.carrier); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Warnf("endpointsync: rows close: %v", closeErr)
			}
			return 0, nil, 0, fmt.Errorf("failed to scan dedup state: %w", err)
		}
		states = append(states, s)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, 0, fmt.Errorf("failed to read dedup state: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, nil, 0, fmt.Errorf("failed to close dedup state rows: %w", err)
	}

	if len(states) == 0 {
		return 0, nil, 0, nil
	}

	deleted := 0
	for _, s := range states {
		if s.suppressed > 0 && s.carrier != nil {
			if err := w.txFlushCarrier(tx, s.carrier, s.suppressed); err != nil {
				if errors.Is(err, ErrWALFull) {
					// The queue is full. Keep the row: the next sweep
					// retries and the count is not lost.
					continue
				}
				// Poison: keeping the row wedges every later sweep.
				log.Errorf("endpointsync: dropping unflushable dedup carrier, %d held-back events lost: %v", s.suppressed, err)
			}
		}
		if _, err := tx.Exec("DELETE FROM dedup_state WHERE key_hash = ?", s.keyHash); err != nil {
			return 0, nil, 0, fmt.Errorf("failed to delete dedup state: %w", err)
		}
		deleted++
	}

	if err := tx.Commit(); err != nil {
		return 0, nil, 0, fmt.Errorf("failed to commit sweep page: %w", err)
	}
	return len(states), states[len(states)-1].keyHash, deleted, nil
}

// txInsertEvents inserts events and maintains pending_count inside the
// caller's transaction. The guarded counter update enforces the
// maxPending bound for the whole group before any insert, so a partial
// group never commits.
func (w *wal) txInsertEvents(tx *sql.Tx, events ...walEvent) error {
	res, err := tx.Exec(
		"UPDATE wal_meta SET pending_count = pending_count + ? WHERE id = 1 AND pending_count + ? <= ?",
		len(events), len(events), w.maxPending,
	)
	if err != nil {
		return fmt.Errorf("failed to update pending count: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read rows affected: %w", err)
	}
	if affected == 0 {
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
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed || w.db == nil {
		return nil
	}
	w.closed = true
	return w.db.Close()
}
