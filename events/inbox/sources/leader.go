package sources

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/safedep/dry/db"
	"github.com/safedep/dry/log"
)

const (
	leaderRetryInterval = 5 * time.Second
	// leaderPingInterval throttles the liveness check on the held lock connection,
	// bounding the split-brain window if the connection dies silently.
	leaderPingInterval = 5 * time.Second
)

// advisoryLeader gates an S2 source so only one replica reads a feed — the
// single-active-consumer invariant the client-side cursor requires. It holds a
// Postgres session advisory lock on a dedicated connection: holding the
// connection holds the lock, and losing it (crash / network) releases it so a
// standby takes over. Mirrors the outbox drain leader.
//
// A leadership term has its own context, cancelled the moment the term ends. The
// read session is opened with it, so a read blocked in Next() unwinds immediately
// when leadership is lost — without that, a former leader's session would keep
// returning records alongside the new leader's and both would advance the cursor.
type advisoryLeader struct {
	sqlDB *sql.DB
	key   int64

	mu      sync.Mutex
	conn    *sql.Conn // non-nil while this instance holds leadership
	leadCtx context.Context
	cancel  context.CancelFunc
}

func newAdvisoryLeader(adapter db.SqlDataAdapter, key int64) (*advisoryLeader, error) {
	gdb, err := adapter.GetDB()
	if err != nil {
		return nil, err
	}
	if name := gdb.Dialector.Name(); name != "postgres" {
		return nil, fmt.Errorf("inbox/s2: leader election requires PostgreSQL, got %q", name)
	}
	sqlDB, err := adapter.GetConn()
	if err != nil {
		return nil, err
	}
	return &advisoryLeader{sqlDB: sqlDB, key: key}, nil
}

// leaderKey derives the advisory-lock key from the cursor identity, so each
// (consumer_name, feed) elects independently and two feeds under one consumer
// name are not serialized behind a single lock.
func leaderKey(consumerName, feed string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(consumerName + "\x00" + feed))
	return int64(h.Sum64())
}

// closeLockConn returns a lock connection to the pool, logging an unexpected
// close failure rather than swallowing it.
func closeLockConn(conn *sql.Conn) {
	if err := conn.Close(); err != nil {
		log.Warnf("inbox/s2: close lock connection: %v", err)
	}
}

// ensureLeading blocks until this instance holds leadership, returning the term
// context — alive while it leads, cancelled the moment leadership is lost (the
// lock connection drops) or baseCtx is done. The read session MUST be opened with
// it (§ the type doc). reacquired is true on a fresh term so the caller reopens
// its session under the new context, resuming from the persisted cursor.
func (l *advisoryLeader) ensureLeading(baseCtx context.Context) (leadCtx context.Context, reacquired bool, err error) {
	l.mu.Lock()
	if l.leadCtx != nil && l.leadCtx.Err() == nil {
		current := l.leadCtx
		l.mu.Unlock()
		return current, false, nil
	}
	l.mu.Unlock()

	for {
		if err := baseCtx.Err(); err != nil {
			return nil, false, err
		}
		ctx, acquired, err := l.tryAcquire(baseCtx)
		if err != nil {
			return nil, false, err
		}
		if acquired {
			log.Infof("inbox/s2: acquired feed leadership")
			return ctx, true, nil
		}
		// Another replica leads; idle and retry so we fail over when it dies.
		select {
		case <-baseCtx.Done():
			return nil, false, baseCtx.Err()
		case <-time.After(leaderRetryInterval):
		}
	}
}

func (l *advisoryLeader) tryAcquire(baseCtx context.Context) (context.Context, bool, error) {
	conn, err := l.sqlDB.Conn(baseCtx)
	if err != nil {
		return nil, false, fmt.Errorf("acquire connection: %w", err)
	}

	var acquired bool
	if err := conn.QueryRowContext(baseCtx, "SELECT pg_try_advisory_lock($1)", l.key).Scan(&acquired); err != nil {
		closeLockConn(conn)
		return nil, false, fmt.Errorf("advisory lock: %w", err)
	}
	if !acquired {
		closeLockConn(conn) // returns the connection to the pool, holding no lock
		return nil, false, nil
	}

	leadCtx, cancel := context.WithCancel(baseCtx)
	l.mu.Lock()
	l.conn = conn
	l.leadCtx = leadCtx
	l.cancel = cancel
	l.mu.Unlock()

	go l.hold(conn, leadCtx, cancel)
	return leadCtx, true, nil
}

// hold owns a leadership term: it pings the lock connection until the ping fails
// (the lock is gone) or leadCtx is cancelled (baseCtx / graceful stop), then
// releases the lock exactly once. Running release here — not in the read path —
// frees the lock promptly on a graceful stop even though Source has no Close hook.
func (l *advisoryLeader) hold(conn *sql.Conn, leadCtx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(leaderPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-leadCtx.Done():
			l.release(conn, cancel)
			return
		case <-ticker.C:
			if err := conn.PingContext(leadCtx); err != nil {
				if leadCtx.Err() == nil {
					log.Warnf("inbox/s2: lost feed leadership: %v", err)
				}
				l.release(conn, cancel)
				return
			}
		}
	}
}

// release cancels the term (unblocking the read), unlocks, and returns the
// connection to the pool. Background context so the unlock runs even when the
// term ended via cancellation.
func (l *advisoryLeader) release(conn *sql.Conn, cancel context.CancelFunc) {
	cancel()
	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", l.key); err != nil {
		// Non-fatal: closing the connection below releases the session lock anyway.
		log.Warnf("inbox/s2: advisory unlock: %v", err)
	}
	closeLockConn(conn)

	l.mu.Lock()
	if l.conn == conn {
		l.conn = nil
		l.leadCtx = nil
		l.cancel = nil
	}
	l.mu.Unlock()
}
