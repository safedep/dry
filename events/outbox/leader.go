package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/safedep/dry/log"
)

const (
	// defaultLeaderKey is the advisory-lock key for the single drain leader.
	// Override with WithLeaderKey when several independent outboxes share a
	// database, so they do not contend for one lock.
	defaultLeaderKey int64 = 0x5AFEDE0E_E1EC

	leaderRetryInterval = 5 * time.Second
)

// WithLeaderElection makes Run safe to start on every replica: only the holder of
// a Postgres advisory lock drains; the rest stand by and take over if it dies.
// Requires a PostgreSQL store — Run errors otherwise.
func WithLeaderElection() Option {
	return func(o *Outbox) { o.leaderElection = true }
}

// WithLeaderKey overrides the advisory-lock key (default defaultLeaderKey).
func WithLeaderKey(key int64) Option {
	return func(o *Outbox) { o.leaderKey = key }
}

// runWithLeader drains only while this instance holds the advisory lock. It keeps
// trying to acquire leadership so a standby fails over when the current leader
// dies (its connection drops, which releases the lock).
func (o *Outbox) runWithLeader(ctx context.Context) error {
	if err := o.requirePostgres(); err != nil {
		return err
	}

	sqlDB, err := o.store.GetConn()
	if err != nil {
		return fmt.Errorf("outbox: get conn: %w", err)
	}

	ticker := time.NewTicker(leaderRetryInterval)
	defer ticker.Stop()

	for {
		if err := o.leadAndDrain(ctx, sqlDB); err != nil {
			log.Warnf("outbox: leader: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// leadAndDrain acquires the advisory lock on a dedicated connection and, if it
// wins, drains until the context is cancelled or the connection is lost. The lock
// is a session lock: holding the connection holds the lock, and closing it (or a
// crash) releases it.
func (o *Outbox) leadAndDrain(ctx context.Context, sqlDB *sql.DB) error {
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", o.leaderKey).Scan(&acquired); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	if !acquired {
		return nil // another instance is the leader
	}
	defer func() {
		// Release explicitly; closing the connection would release it anyway.
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", o.leaderKey)
	}()

	log.Infof("outbox: acquired drain leadership")
	return o.drainLoop(ctx, conn)
}

// drainLoop runs the drain + cleanup cadence. When lease is non-nil it is the
// connection holding the leader lock; losing it (ping fails) ends the loop so
// runWithLeader can re-acquire from scratch.
func (o *Outbox) drainLoop(ctx context.Context, lease *sql.Conn) error {
	drainTicker := time.NewTicker(o.pollInterval)
	defer drainTicker.Stop()
	cleanupTicker := time.NewTicker(o.cleanupInterval)
	defer cleanupTicker.Stop()

	for {
		if _, err := o.drainOnce(ctx); err != nil {
			log.Warnf("outbox: drain error: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-drainTicker.C:
		case <-cleanupTicker.C:
			if _, err := o.Cleanup(ctx); err != nil {
				log.Warnf("outbox: cleanup error: %v", err)
			}
		}

		if lease != nil {
			if err := lease.PingContext(ctx); err != nil {
				log.Warnf("outbox: lost drain leadership: %v", err)
				return nil
			}
		}
	}
}

func (o *Outbox) requirePostgres() error {
	gdb, err := o.store.GetDB()
	if err != nil {
		return err
	}

	if name := gdb.Dialector.Name(); name != "postgres" {
		return fmt.Errorf("outbox: leader election requires PostgreSQL, got %q", name)
	}

	return nil
}
