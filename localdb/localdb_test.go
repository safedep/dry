package localdb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/safedep/dry/usefulerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

// assertErrCode asserts that err is a usefulerror carrying the given code.
func assertErrCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	ue, ok := usefulerror.AsUsefulError(err)
	require.True(t, ok, "expected a usefulerror, got %v", err)
	assert.Equal(t, code, ue.Code())
}

// tableExists reports whether a table with the given name exists.
func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?`, name,
	).Scan(&n)
	require.NoError(t, err)
	return n > 0
}

// columnExists reports whether a column exists in a table.
func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		if name == column {
			return true
		}
	}
	require.NoError(t, rows.Err())
	return false
}

// moduleVersion reads the tracker version for the given module (-1 if absent).
func moduleVersion(t *testing.T, db *sql.DB, module string) int {
	t.Helper()
	var v int
	err := db.QueryRow(
		`SELECT version FROM `+trackerTable+` WHERE module = ?`, module,
	).Scan(&v)
	if err == sql.ErrNoRows {
		return -1
	}
	require.NoError(t, err)
	return v
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	require.NoError(t, db.QueryRow(`PRAGMA user_version`).Scan(&v))
	return v
}

func TestLazyOpen_NoFileUntilStore(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})

	dbPath := filepath.Join(dir, defaultFileName)

	// New must touch no disk.
	_, err := os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err), "DB file must not exist before Store")

	store, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)
	require.NotNil(t, store)

	// File appears only after first Store.
	_, err = os.Stat(dbPath)
	require.NoError(t, err, "DB file must exist after Store")

	require.NoError(t, mgr.Close())
}

func TestLazyOpen_NeverStored_CloseCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})

	require.NoError(t, mgr.Close())

	// Neither the dir contents (db file) should be created.
	_, err := os.Stat(filepath.Join(dir, defaultFileName))
	assert.True(t, os.IsNotExist(err), "DB file must not exist when Store never called")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no files should be created when Store never called")
}

func TestFrameworkBootstrap(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	store, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)

	db := store.DB()
	assert.True(t, tableExists(t, db, trackerTable), "tracker table must exist")
	assert.Equal(t, len(frameworkMigrations), userVersion(t, db),
		"user_version must equal number of framework migrations")
}

func TestFrameworkBootstrap_SecondOpenIsNoOp(t *testing.T) {
	dir := t.TempDir()

	mgrA := New(Config{Dir: dir})
	_, err := mgrA.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)
	require.NoError(t, mgrA.Close())

	mgrB := New(Config{Dir: dir})
	defer func() { _ = mgrB.Close() }()
	store, err := mgrB.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err, "second open on existing file must be a no-op, not an error")

	assert.Equal(t, len(frameworkMigrations), userVersion(t, store.DB()),
		"user_version unchanged on second open")
}

func TestMigrationAppliedOnce(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	ctx := context.Background()
	d := Descriptor{
		Name: "alpha",
		// CREATE TABLE is non-idempotent: a second apply would error.
		Migrations: []string{
			`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY, name TEXT)`,
		},
	}

	store1, err := mgr.Store(ctx, d)
	require.NoError(t, err)
	assert.Equal(t, 1, moduleVersion(t, store1.DB(), "alpha"))

	// Repeated Store with the same descriptor must not re-run the migration.
	store2, err := mgr.Store(ctx, d)
	require.NoError(t, err, "second Store must not re-apply non-idempotent migration")
	assert.Equal(t, 1, moduleVersion(t, store2.DB(), "alpha"))

	assert.True(t, tableExists(t, store2.DB(), "alpha_items"))
}

func TestUpgradePath(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	m1 := `CREATE TABLE beta_items (id INTEGER PRIMARY KEY)`
	m2 := `ALTER TABLE beta_items ADD COLUMN label TEXT`

	mgrA := New(Config{Dir: dir})
	storeA, err := mgrA.Store(ctx, Descriptor{Name: "beta", Migrations: []string{m1}})
	require.NoError(t, err)
	assert.Equal(t, 1, moduleVersion(t, storeA.DB(), "beta"))
	assert.False(t, columnExists(t, storeA.DB(), "beta_items", "label"))
	require.NoError(t, mgrA.Close())

	// Reopen with an appended migration. Only m2 should apply.
	mgrB := New(Config{Dir: dir})
	defer func() { _ = mgrB.Close() }()
	storeB, err := mgrB.Store(ctx, Descriptor{Name: "beta", Migrations: []string{m1, m2}})
	require.NoError(t, err)
	assert.Equal(t, 2, moduleVersion(t, storeB.DB(), "beta"))
	assert.True(t, columnExists(t, storeB.DB(), "beta_items", "label"),
		"appended migration must add the new column")
}

// TestDowngradeRejected: a stored module version ahead of the descriptor's
// migrations (e.g. an older binary or a truncated list) must fail fast with the
// incompatible-schema code, not silently succeed against an unknown schema.
func TestDowngradeRejected(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	m1 := `CREATE TABLE beta_items (id INTEGER PRIMARY KEY)`
	m2 := `ALTER TABLE beta_items ADD COLUMN label TEXT`

	mgrA := New(Config{Dir: dir})
	_, err := mgrA.Store(ctx, Descriptor{Name: "beta", Migrations: []string{m1, m2}})
	require.NoError(t, err)
	require.NoError(t, mgrA.Close())

	// Reopen with a truncated migration list (version 2 on disk > len 1).
	mgrB := New(Config{Dir: dir})
	defer func() { _ = mgrB.Close() }()
	_, err = mgrB.Store(ctx, Descriptor{Name: "beta", Migrations: []string{m1}})
	assertErrCode(t, err, ErrCodeIncompatibleSchema)
}

// TestEmptyDir: an empty Config.Dir means the current working directory and
// must not fail at os.MkdirAll.
func TestEmptyDir(t *testing.T) {
	t.Chdir(t.TempDir()) // isolate the CWD-relative DB file to a temp dir

	mgr := New(Config{Dir: "", FileName: "empty_dir_test.db"})
	defer func() { _ = mgr.Close() }()

	store, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)
	require.NotNil(t, store)

	_, err = os.Stat("empty_dir_test.db")
	require.NoError(t, err, "DB file should be created in the working directory")
}

func TestCrossProcessSimulation(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	d := Descriptor{
		Name: "alpha",
		Migrations: []string{
			`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY, name TEXT)`,
		},
	}

	mgrA := New(Config{Dir: dir})
	mgrB := New(Config{Dir: dir})
	defer func() { _ = mgrA.Close() }()
	defer func() { _ = mgrB.Close() }()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	stores := make([]*Store, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		stores[0], errs[0] = mgrA.Store(ctx, d)
	}()
	go func() {
		defer wg.Done()
		stores[1], errs[1] = mgrB.Store(ctx, d)
	}()
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NotNil(t, stores[0])
	require.NotNil(t, stores[1])

	// Migration applied exactly once (tracker version == 1).
	assert.Equal(t, 1, moduleVersion(t, stores[0].DB(), "alpha"))

	// Data readable / writable: no corruption.
	_, err := stores[0].DB().Exec(`INSERT INTO alpha_items (name) VALUES ('x')`)
	require.NoError(t, err)

	var n int
	require.NoError(t, stores[1].DB().QueryRow(`SELECT COUNT(*) FROM alpha_items`).Scan(&n))
	assert.Equal(t, 1, n)
}

// TestCrossProcessColdStartRace is a regression guard for the cold-start WAL
// race: two processes opening the same brand-new file concurrently both switch
// it into WAL mode, a transition SQLite does not cover with busy_timeout. A
// single race iteration only catches the bug ~50% of the time, so repeat it
// many times — every iteration uses a fresh directory so both Managers race to
// create and WAL-initialize the file from cold.
func TestCrossProcessColdStartRace(t *testing.T) {
	ctx := context.Background()
	d := Descriptor{
		Name:       "alpha",
		Migrations: []string{`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY)`},
	}

	for i := 0; i < 50; i++ {
		dir := t.TempDir()
		mgrA := New(Config{Dir: dir})
		mgrB := New(Config{Dir: dir})

		var wg sync.WaitGroup
		errs := make([]error, 2)

		wg.Add(2)
		go func() {
			defer wg.Done()
			_, errs[0] = mgrA.Store(ctx, d)
		}()
		go func() {
			defer wg.Done()
			_, errs[1] = mgrB.Store(ctx, d)
		}()
		wg.Wait()

		require.NoErrorf(t, errs[0], "cold-start race iteration %d (A)", i)
		require.NoErrorf(t, errs[1], "cold-start race iteration %d (B)", i)

		require.NoError(t, mgrA.Close())
		require.NoError(t, mgrB.Close())
	}
}

func TestNamingConventionCoexistence(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	alpha, err := mgr.Store(ctx, Descriptor{
		Name:       "alpha",
		Migrations: []string{`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY, v TEXT)`},
	})
	require.NoError(t, err)

	beta, err := mgr.Store(ctx, Descriptor{
		Name:       "beta",
		Migrations: []string{`CREATE TABLE beta_items (id INTEGER PRIMARY KEY, v TEXT)`},
	})
	require.NoError(t, err)

	assert.True(t, tableExists(t, alpha.DB(), "alpha_items"))
	assert.True(t, tableExists(t, beta.DB(), "beta_items"))

	// Independent read/write.
	_, err = alpha.DB().Exec(`INSERT INTO alpha_items (v) VALUES ('a')`)
	require.NoError(t, err)
	_, err = beta.DB().Exec(`INSERT INTO beta_items (v) VALUES ('b')`)
	require.NoError(t, err)

	var av, bv string
	require.NoError(t, alpha.DB().QueryRow(`SELECT v FROM alpha_items`).Scan(&av))
	require.NoError(t, beta.DB().QueryRow(`SELECT v FROM beta_items`).Scan(&bv))
	assert.Equal(t, "a", av)
	assert.Equal(t, "b", bv)

	// Both tracker rows present.
	assert.Equal(t, 1, moduleVersion(t, alpha.DB(), "alpha"))
	assert.Equal(t, 1, moduleVersion(t, beta.DB(), "beta"))
}

func TestEmptyMigrations(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	cases := []struct {
		name string
		desc Descriptor
	}{
		{"nil", Descriptor{Name: "alpha", Migrations: nil}},
		{"empty", Descriptor{Name: "beta", Migrations: []string{}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, err := mgr.Store(context.Background(), tc.desc)
			require.NoError(t, err)
			require.NotNil(t, store)
			require.NotNil(t, store.DB())

			var one int
			require.NoError(t, store.DB().QueryRow(`SELECT 1`).Scan(&one))
			assert.Equal(t, 1, one)
		})
	}
}

func TestDescriptorValidation_Name(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	cases := []struct {
		name    string
		modName string
	}{
		{"empty", ""},
		{"leading digit", "1alpha"},
		{"uppercase", "Alpha"},
		{"hyphen", "al-pha"},
		{"leading underscore", "_alpha"},
		{"space", "al pha"},
		{"dot", "al.pha"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mgr.Store(context.Background(), Descriptor{Name: tc.modName})
			assertErrCode(t, err, ErrCodeInvalidDescriptor)
		})
	}

	// File must never be created for invalid descriptors.
	_, err := os.Stat(filepath.Join(dir, defaultFileName))
	assert.True(t, os.IsNotExist(err), "no DB file for rejected descriptors")
}

func TestDescriptorValidation_FileNameSeparator(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir, FileName: "sub/local.db"})

	_, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	assertErrCode(t, err, ErrCodeInvalidDescriptor)
}

func TestDescriptorValidation_ValidCustomFileName(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir, FileName: "cache.db"})
	defer func() { _ = mgr.Close() }()

	store, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)
	require.NotNil(t, store)

	_, err = os.Stat(filepath.Join(dir, "cache.db"))
	require.NoError(t, err, "custom FileName must create that file")
}

func TestPerConnectionPragmas(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	store, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)
	db := store.DB()

	// Pin the (only) pooled connection with a transaction, then read pragmas
	// back from the same pool. With SetMaxOpenConns(1) the reused pooled
	// connection must still carry the framework pragmas.
	tx, err := db.Begin()
	require.NoError(t, err)

	var busyTimeout, synchronous, foreignKeys int
	var journalMode string

	require.NoError(t, tx.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout))
	require.NoError(t, tx.QueryRow(`PRAGMA synchronous`).Scan(&synchronous))
	require.NoError(t, tx.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys))
	require.NoError(t, tx.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode))

	require.NoError(t, tx.Rollback())

	assert.Equal(t, 5000, busyTimeout, "busy_timeout")
	assert.Equal(t, 1, synchronous, "synchronous=NORMAL")
	assert.Equal(t, 1, foreignKeys, "foreign_keys=ON")
	assert.Equal(t, "wal", journalMode, "journal_mode=WAL")
}

func TestDurabilityBarrier(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	d := Descriptor{
		Name:       "alpha",
		Migrations: []string{`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY, v TEXT)`},
	}

	mgr := New(Config{Dir: dir})
	store, err := mgr.Store(ctx, d)
	require.NoError(t, err)

	_, err = store.DB().Exec(`INSERT INTO alpha_items (v) VALUES ('durable')`)
	require.NoError(t, err)

	require.NoError(t, mgr.Close())

	// After Close the -wal sibling is truncated (size 0) or absent.
	walPath := filepath.Join(dir, defaultFileName+"-wal")
	info, statErr := os.Stat(walPath)
	if statErr == nil {
		assert.Equal(t, int64(0), info.Size(), "-wal file must be truncated after Close")
	} else {
		assert.True(t, os.IsNotExist(statErr), "unexpected stat error: %v", statErr)
	}

	// Reopen: data written before Close must be present.
	mgr2 := New(Config{Dir: dir})
	defer func() { _ = mgr2.Close() }()
	store2, err := mgr2.Store(ctx, d)
	require.NoError(t, err)

	var v string
	require.NoError(t, store2.DB().QueryRow(`SELECT v FROM alpha_items`).Scan(&v))
	assert.Equal(t, "durable", v)
}

// --- 11. Non-truncating checkpoint ---------------------------------------

func TestNonTruncatingCheckpoint(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	d := Descriptor{
		Name:       "alpha",
		Migrations: []string{`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY, v TEXT)`},
	}

	mgrA := New(Config{Dir: dir})
	storeA, err := mgrA.Store(ctx, d)
	require.NoError(t, err)

	_, err = storeA.DB().Exec(`INSERT INTO alpha_items (v) VALUES ('committed')`)
	require.NoError(t, err)

	// Second handle holds the file open so the WAL cannot be truncated.
	dbPath := filepath.Join(dir, defaultFileName)
	rawDB, err := sql.Open(driverName, "file:"+dbPath)
	require.NoError(t, err)
	rawConn, err := rawDB.Conn(ctx)
	require.NoError(t, err) // pins an open connection
	var probe int
	require.NoError(t, rawConn.QueryRowContext(ctx, `SELECT COUNT(*) FROM alpha_items`).Scan(&probe))

	// Close A: must still succeed (busy=1 is not an error) and lose no data.
	require.NoError(t, mgrA.Close(), "Close must not error on busy checkpoint")

	require.NoError(t, rawConn.Close())
	require.NoError(t, rawDB.Close())

	// Reopen and confirm the committed row survived.
	mgrC := New(Config{Dir: dir})
	defer func() { _ = mgrC.Close() }()
	storeC, err := mgrC.Store(ctx, d)
	require.NoError(t, err)

	var v string
	require.NoError(t, storeC.DB().QueryRow(`SELECT v FROM alpha_items`).Scan(&v))
	assert.Equal(t, "committed", v)
}

func TestCloseIdempotentAndStoreAfterClose(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	mgr := New(Config{Dir: dir})

	_, err := mgr.Store(ctx, Descriptor{Name: "alpha"})
	require.NoError(t, err)

	require.NoError(t, mgr.Close())
	require.NoError(t, mgr.Close(), "second Close must return nil")

	_, err = mgr.Store(ctx, Descriptor{Name: "alpha"})
	require.Error(t, err, "Store after Close must error")
	assertErrCode(t, err, ErrCodeManagerClosed)
}

func TestCloseRacing(t *testing.T) {
	dir := t.TempDir()
	mgr := New(Config{Dir: dir})
	_, err := mgr.Store(context.Background(), Descriptor{Name: "alpha"})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = mgr.Close()
		}(i)
	}
	wg.Wait()

	for _, e := range errs {
		assert.NoError(t, e, "racing Close must all return nil")
	}
}

func TestSameNameReuse(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	mgr := New(Config{Dir: dir})
	defer func() { _ = mgr.Close() }()

	migs := []string{`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY)`}

	store1, err := mgr.Store(ctx, Descriptor{Name: "alpha", Migrations: migs})
	require.NoError(t, err)

	// Same Name + identical Migrations => same cached pointer.
	store2, err := mgr.Store(ctx, Descriptor{Name: "alpha", Migrations: migs})
	require.NoError(t, err)
	assert.Same(t, store1, store2, "same Name + same migrations must return cached *Store")

	// Same Name + different Migrations => error.
	_, err = mgr.Store(ctx, Descriptor{
		Name:       "alpha",
		Migrations: []string{`CREATE TABLE alpha_items (id INTEGER PRIMARY KEY)`, `CREATE TABLE alpha_more (id INTEGER PRIMARY KEY)`},
	})
	assertErrCode(t, err, ErrCodeInvalidDescriptor)
}
