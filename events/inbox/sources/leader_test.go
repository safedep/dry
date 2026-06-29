package sources

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/safedep/dry/db"
	"github.com/safedep/dry/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sqliteAdapter is a minimal SqlDataAdapter over sqlite — enough to exercise the
// "leader election requires PostgreSQL" guard. The live advisory-lock path needs
// a real Postgres and is covered by integration tests.
type sqliteAdapter struct{ gdb *gorm.DB }

func (a sqliteAdapter) GetDB() (*gorm.DB, error)            { return a.gdb, nil }
func (a sqliteAdapter) GetConn() (*sql.DB, error)           { return a.gdb.DB() }
func (a sqliteAdapter) Migrate(models ...interface{}) error { return a.gdb.AutoMigrate(models...) }
func (a sqliteAdapter) Ping() error {
	conn, err := a.gdb.DB()
	if err != nil {
		return err
	}
	return conn.Ping()
}

func newSqliteAdapter(t *testing.T) db.SqlDataAdapter {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "leader.db")), &gorm.Config{})
	require.NoError(t, err)
	return sqliteAdapter{gdb: gdb}
}

func TestLeaderKey(t *testing.T) {
	// Deterministic, and distinct per (consumer, feed) so feeds don't share a lock.
	assert.Equal(t, leaderKey("c", "f"), leaderKey("c", "f"))
	assert.NotEqual(t, leaderKey("c", "f1"), leaderKey("c", "f2"))
	assert.NotEqual(t, leaderKey("c1", "f"), leaderKey("c2", "f"))
	// The separator prevents (c+f) collisions across the boundary.
	assert.NotEqual(t, leaderKey("ab", "c"), leaderKey("a", "bc"))
}

func TestNewAdvisoryLeader_RequiresPostgres(t *testing.T) {
	_, err := newAdvisoryLeader(newSqliteAdapter(t), leaderKey("c", "f"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PostgreSQL")
}

func TestNewS2_WithLeaderRequiresPostgres(t *testing.T) {
	_, err := NewS2(stream.StreamFor(routingX()), stream.S2StreamProviderConfig{ApiKey: "k"},
		nil, newMemCursors(), "consumer-a", WithLeader(newSqliteAdapter(t)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PostgreSQL")
}

func TestNewS2_NoLeaderByDefault(t *testing.T) {
	src, err := NewS2(stream.StreamFor(routingX()), stream.S2StreamProviderConfig{ApiKey: "k"},
		nil, newMemCursors(), "consumer-a")
	require.NoError(t, err)
	assert.Nil(t, src.(*s2Source).leader)
}
