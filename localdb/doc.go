// Package localdb is a shared local SQLite database framework for a tool's
// modules. Multiple independent modules persist into one shared SQLite file
// with a single connection pool; each module owns its own tables and an
// append-only migration list but does not manage the database file, connection
// pool, or lifecycle.
//
// The file is created lazily — only when at least one module calls Store — and
// lives at <Config.Dir>/<FileName> (FileName defaults to "local.db"), with
// sibling -wal and -shm files from WAL mode.
//
// Typical usage:
//
//	mgr := localdb.New(localdb.Config{Dir: cacheDir})
//	defer mgr.Close()
//
//	store, err := mgr.Store(ctx, localdb.Descriptor{
//	    Name: "malysis_cache",
//	    Migrations: []string{
//	        `CREATE TABLE malysis_cache_verdicts (
//	            purl       TEXT PRIMARY KEY,
//	            result     BLOB NOT NULL,
//	            created_at INTEGER NOT NULL
//	        )`,
//	    },
//	})
//	if err != nil {
//	    return err
//	}
//	db := store.DB() // raw *sql.DB shared by all modules
//
// localdb exposes a raw *sql.DB, raw-SQL migrations, and SQLite-specific
// pragmas. It is deliberately distinct from dry/db (GORM, MySQL/Postgres): this
// is raw database/sql + SQLite for local, embedded use, not a generic,
// swappable storage abstraction.
//
// Isolation between modules is by naming convention: each module prefixes its
// tables and indexes with its Name (e.g. malysis_cache_*). The framework owns
// exactly one table, the migration tracker _localdb_schema_migrations.
//
// Concurrency and durability: the file uses WAL mode with busy_timeout,
// synchronous=NORMAL, and foreign_keys=ON, applied per connection, with the
// pool limited to a single connection. Multiple processes may safely share the
// file on a local filesystem; Config.Dir must not be a network filesystem
// (NFS/SMB/overlay), where WAL is unsafe. Consumers must keep write
// transactions tiny and short-lived. See the package design for the full
// contract.
package localdb
