package sources

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
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
type advisoryLeader struct {
	sqlDB *sql.DB
	key   int64

	conn     *sql.Conn // non-nil while this instance holds leadership
	lastPing time.Time
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

// ensureLeading guarantees this instance holds leadership before returning, or
// returns ctx's error. reacquired is true when leadership was freshly taken
// (first acquisition, or after a lost lock) — the caller reopens its read session
// so it resumes from the persisted cursor under the new leadership rather than
// continuing a session opened by (or concurrent with) a former leader.
func (l *advisoryLeader) ensureLeading(ctx context.Context) (reacquired bool, err error) {
	if l.conn != nil {
		if time.Since(l.lastPing) < leaderPingInterval {
			return false, nil
		}
		if pingErr := l.conn.PingContext(ctx); pingErr == nil {
			l.lastPing = time.Now()
			return false, nil
		}
		log.Warnf("inbox/s2: lost feed leadership; re-acquiring")
		l.release()
	}

	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, ctxErr
		}
		acquired, acqErr := l.tryAcquire(ctx)
		if acqErr != nil {
			return false, acqErr
		}
		if acquired {
			log.Infof("inbox/s2: acquired feed leadership")
			return true, nil
		}
		// Another replica leads; idle and retry so we fail over when it dies.
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(leaderRetryInterval):
		}
	}
}

func (l *advisoryLeader) tryAcquire(ctx context.Context) (bool, error) {
	conn, err := l.sqlDB.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire connection: %w", err)
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", l.key).Scan(&acquired); err != nil {
		_ = conn.Close()
		return false, fmt.Errorf("advisory lock: %w", err)
	}
	if !acquired {
		_ = conn.Close() // returns the connection to the pool, holding no lock
		return false, nil
	}

	l.conn = conn
	l.lastPing = time.Now()
	return true, nil
}

// release unlocks and returns the held connection to the pool. Uses a background
// context so the unlock runs even when the caller's context is already cancelled.
func (l *advisoryLeader) release() {
	if l.conn == nil {
		return
	}
	_, _ = l.conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", l.key)
	_ = l.conn.Close()
	l.conn = nil
}
