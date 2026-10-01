package localdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var itemsDescriptor = Descriptor{
	Name:       "items",
	Migrations: []string{`CREATE TABLE items_rows (id INTEGER PRIMARY KEY, v TEXT)`},
}

func TestNewReturnsFileManager(t *testing.T) {
	mgr := New(Config{Dir: t.TempDir()})
	_, ok := mgr.(FileManager)
	assert.True(t, ok)
	require.NoError(t, mgr.Close())
}

func TestReadDB(t *testing.T) {
	cases := []struct {
		name      string
		readConns int
		wantMax   int
		shared    bool
	}{
		{name: "default shares the single connection pool", readConns: 0, wantMax: 1, shared: true},
		{name: "read pool has its own connections", readConns: 4, wantMax: 4, shared: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mgr := New(Config{Dir: t.TempDir(), ReadConns: tc.readConns})
			t.Cleanup(func() { assert.NoError(t, mgr.Close()) })

			store, err := mgr.Store(ctx, itemsDescriptor)
			require.NoError(t, err)

			assert.Equal(t, tc.shared, store.ReadDB() == store.DB())
			assert.Equal(t, tc.wantMax, store.ReadDB().Stats().MaxOpenConnections)
			assert.Equal(t, 1, store.DB().Stats().MaxOpenConnections)

			_, err = store.DB().ExecContext(ctx, `INSERT INTO items_rows (v) VALUES ('a')`)
			require.NoError(t, err)

			var n int
			require.NoError(t, store.ReadDB().QueryRowContext(ctx, `SELECT COUNT(*) FROM items_rows`).Scan(&n))
			assert.Equal(t, 1, n)
		})
	}
}

func TestReadDBRejectsWrites(t *testing.T) {
	ctx := context.Background()
	mgr := New(Config{Dir: t.TempDir(), ReadConns: 2})
	t.Cleanup(func() { assert.NoError(t, mgr.Close()) })

	store, err := mgr.Store(ctx, itemsDescriptor)
	require.NoError(t, err)

	_, err = store.ReadDB().ExecContext(ctx, `INSERT INTO items_rows (v) VALUES ('a')`)
	assert.Error(t, err)
}

func TestSchemaVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	mgr := New(Config{Dir: dir})
	store, err := mgr.Store(ctx, Descriptor{Name: "empty"})
	require.NoError(t, err)

	v, err := store.SchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, v)

	store, err = mgr.Store(ctx, Descriptor{
		Name: "two",
		Migrations: []string{
			`CREATE TABLE two_rows (id INTEGER PRIMARY KEY)`,
			`ALTER TABLE two_rows ADD COLUMN v TEXT`,
		},
	})
	require.NoError(t, err)

	v, err = store.SchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, v)

	require.NoError(t, mgr.Close())

	_, err = store.SchemaVersion(ctx)
	assertErrCode(t, err, ErrCodeMigrationFailure)
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "default file name", cfg: Config{Dir: dir}, want: filepath.Join(dir, "local.db")},
		{name: "custom file name", cfg: Config{Dir: dir, FileName: "pmg.db"}, want: filepath.Join(dir, "pmg.db")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, NewFileManager(tc.cfg).Path())
		})
	}
}

func TestSize(t *testing.T) {
	ctx := context.Background()
	mgr := NewFileManager(Config{Dir: t.TempDir()})
	t.Cleanup(func() { assert.NoError(t, mgr.Close()) })

	size, err := mgr.Size()
	require.NoError(t, err)
	assert.Zero(t, size)

	_, err = mgr.Store(ctx, itemsDescriptor)
	require.NoError(t, err)

	size, err = mgr.Size()
	require.NoError(t, err)
	assert.Positive(t, size)
}

func TestVacuum(t *testing.T) {
	ctx := context.Background()

	t.Run("missing file is a no-op and creates nothing", func(t *testing.T) {
		mgr := NewFileManager(Config{Dir: t.TempDir()})
		require.NoError(t, mgr.Vacuum(ctx))
		assert.NoFileExists(t, mgr.Path())
		require.NoError(t, mgr.Close())
	})

	t.Run("releases free pages", func(t *testing.T) {
		mgr := NewFileManager(Config{Dir: t.TempDir()})
		t.Cleanup(func() { assert.NoError(t, mgr.Close()) })

		store, err := mgr.Store(ctx, itemsDescriptor)
		require.NoError(t, err)

		db := store.DB()
		_, err = db.ExecContext(ctx,
			`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 2000)
			 INSERT INTO items_rows (v) SELECT hex(randomblob(512)) FROM n`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM items_rows`)
		require.NoError(t, err)

		require.NoError(t, mgr.Vacuum(ctx))

		var free int
		require.NoError(t, db.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&free))
		assert.Zero(t, free)

		wal, err := os.Stat(mgr.Path() + "-wal")
		require.NoError(t, err)
		assert.Zero(t, wal.Size())
	})

	t.Run("fails after close", func(t *testing.T) {
		mgr := NewFileManager(Config{Dir: t.TempDir()})
		require.NoError(t, mgr.Close())
		assertErrCode(t, mgr.Vacuum(ctx), ErrCodeManagerClosed)
	})
}

func TestRemove(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the file and its siblings and closes the manager", func(t *testing.T) {
		dir := t.TempDir()
		mgr := NewFileManager(Config{Dir: dir})

		_, err := mgr.Store(ctx, itemsDescriptor)
		require.NoError(t, err)
		require.FileExists(t, mgr.Path())

		require.NoError(t, mgr.Remove())

		for _, s := range fileSuffixes {
			assert.NoFileExists(t, mgr.Path()+s)
		}

		_, err = mgr.Store(ctx, itemsDescriptor)
		assertErrCode(t, err, ErrCodeManagerClosed)
	})

	t.Run("missing file is not an error", func(t *testing.T) {
		mgr := NewFileManager(Config{Dir: t.TempDir()})
		require.NoError(t, mgr.Remove())
	})

	t.Run("rejects a file name with a path separator", func(t *testing.T) {
		mgr := NewFileManager(Config{Dir: t.TempDir(), FileName: "../escape.db"})
		assertErrCode(t, mgr.Remove(), ErrCodeInvalidDescriptor)
	})
}

func TestCheckLocalFilesystem(t *testing.T) {
	cases := []struct {
		name     string
		detect   func(string) (string, bool, error)
		wantCode string
	}{
		{
			name:   "local file system passes",
			detect: func(string) (string, bool, error) { return "ext4", false, nil },
		},
		{
			name:     "network file system fails",
			detect:   func(string) (string, bool, error) { return "nfs", true, nil },
			wantCode: ErrCodeUnsafeFilesystem,
		},
		{
			name:     "inspect failure fails",
			detect:   func(string) (string, bool, error) { return "", false, errors.New("statfs failed") },
			wantCode: ErrCodeOpenFailure,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubNetworkFilesystem(t, tc.detect)

			err := CheckLocalFilesystem(t.TempDir())
			if tc.wantCode == "" {
				assert.NoError(t, err)
				return
			}

			assertErrCode(t, err, tc.wantCode)
		})
	}
}

func TestRejectNetworkFS(t *testing.T) {
	ctx := context.Background()
	network := func(string) (string, bool, error) { return "nfs", true, nil }

	cases := []struct {
		name     string
		reject   bool
		wantCode string
	}{
		{name: "default ignores the file system", reject: false},
		{name: "opt-in rejects a network file system", reject: true, wantCode: ErrCodeUnsafeFilesystem},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubNetworkFilesystem(t, network)

			mgr := NewFileManager(Config{Dir: t.TempDir(), RejectNetworkFS: tc.reject})
			t.Cleanup(func() { assert.NoError(t, mgr.Close()) })

			_, err := mgr.Store(ctx, itemsDescriptor)
			if tc.wantCode == "" {
				assert.NoError(t, err)
				return
			}

			assertErrCode(t, err, tc.wantCode)
			assert.NoFileExists(t, mgr.Path())
		})
	}
}

func TestDetectNetworkFilesystemOnTempDir(t *testing.T) {
	_, _, err := detectNetworkFilesystem(t.TempDir())
	assert.NoError(t, err)
}

func stubNetworkFilesystem(t *testing.T, fn func(string) (string, bool, error)) {
	t.Helper()
	orig := networkFilesystem
	networkFilesystem = fn
	t.Cleanup(func() { networkFilesystem = orig })
}
