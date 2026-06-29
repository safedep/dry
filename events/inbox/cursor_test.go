package inbox_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/safedep/dry/db"
	"github.com/safedep/dry/events/inbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testAdapter is a minimal SqlDataAdapter over an on-disk sqlite database, the
// same shape the outbox tests use.
type testAdapter struct{ gdb *gorm.DB }

func (a testAdapter) GetDB() (*gorm.DB, error)  { return a.gdb, nil }
func (a testAdapter) GetConn() (*sql.DB, error) { return a.gdb.DB() }
func (a testAdapter) Migrate(models ...interface{}) error {
	return a.gdb.AutoMigrate(models...)
}
func (a testAdapter) Ping() error {
	conn, err := a.gdb.DB()
	if err != nil {
		return err
	}
	return conn.Ping()
}

func newTestAdapter(t *testing.T) db.SqlDataAdapter {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "inbox.db")), &gorm.Config{})
	require.NoError(t, err)
	a := testAdapter{gdb: gdb}
	require.NoError(t, inbox.Migrate(a))
	return a
}

func TestCursorStore_LoadMissingReturnsErrNoCursor(t *testing.T) {
	store, err := inbox.NewGormCursorStore(newTestAdapter(t))
	require.NoError(t, err)

	pos, err := store.Load(t.Context(), "consumer-a", "feed.v1.X")
	require.ErrorIs(t, err, inbox.ErrNoCursor, "no cursor yet is signalled explicitly")
	assert.Equal(t, "", pos)
}

func TestCursorStore_AdvanceAndLoad(t *testing.T) {
	store, err := inbox.NewGormCursorStore(newTestAdapter(t))
	require.NoError(t, err)

	require.NoError(t, store.Advance(t.Context(), "consumer-a", "feed.v1.X", "42"))
	pos, err := store.Load(t.Context(), "consumer-a", "feed.v1.X")
	require.NoError(t, err)
	assert.Equal(t, "42", pos)

	// Advancing again overwrites in place (upsert on the (consumer, feed) PK).
	require.NoError(t, store.Advance(t.Context(), "consumer-a", "feed.v1.X", "100"))
	pos, err = store.Load(t.Context(), "consumer-a", "feed.v1.X")
	require.NoError(t, err)
	assert.Equal(t, "100", pos)
}

func TestCursorStore_IndependentPerConsumerAndFeed(t *testing.T) {
	store, err := inbox.NewGormCursorStore(newTestAdapter(t))
	require.NoError(t, err)
	ctx := t.Context()

	require.NoError(t, store.Advance(ctx, "consumer-a", "feed.v1.X", "10"))
	require.NoError(t, store.Advance(ctx, "consumer-b", "feed.v1.X", "20"))
	require.NoError(t, store.Advance(ctx, "consumer-a", "feed.v1.Y", "30"))

	for _, tc := range []struct {
		consumer, feed, want string
	}{
		{"consumer-a", "feed.v1.X", "10"},
		{"consumer-b", "feed.v1.X", "20"},
		{"consumer-a", "feed.v1.Y", "30"},
	} {
		pos, err := store.Load(ctx, tc.consumer, tc.feed)
		require.NoError(t, err)
		assert.Equal(t, tc.want, pos, "consumer=%s feed=%s", tc.consumer, tc.feed)
	}
}
