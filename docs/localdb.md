# localdb

Shared local SQLite database for a tool's modules. One file, one connection
pool. Each module owns its own tables and migrations. The file is created lazily
on first use and lives at `<Config.Dir>/<FileName>` (`FileName` defaults to
`local.db`).

Backend is SQLite via `modernc.org/sqlite` (pure Go, no CGO). The API exposes a
raw `*sql.DB` — this is not a generic, swappable storage abstraction, and it is
distinct from `dry/db` (GORM, MySQL/Postgres).

## Import

```go
import "github.com/safedep/dry/localdb"
```

## Concepts

- **Manager** — owns the DB file and the shared connection pool. Construct once
  at startup. Opens no file until the first `Store` call.
- **Descriptor** — a module's declaration: its name and its migrations.
- **Store** — a module's handle to the shared DB. Returns the raw `*sql.DB`.

## API

### type Config

```go
type Config struct {
    Dir      string // directory holding the DB file
    FileName string // optional; defaults to "local.db"
}
```

The consumer chooses `Dir`; `localdb` has no knowledge of any config system. For
reconstructible (cache-like) data, point `Dir` at a cache directory.

### func New

```go
func New(cfg Config) Manager
```

Returns a Manager bound to `<cfg.Dir>/<cfg.FileName>`. Touches no disk until the
first `Store`.

### type Manager

```go
type Manager interface {
    Store(ctx context.Context, d Descriptor) (*Store, error)
    Close() error
}
```

- `Store` lazily opens (creating if absent) the DB on the first call across any
  module, applies `d`'s not-yet-applied migrations, and returns the module
  handle. Safe for concurrent use. Repeated calls with the same `d.Name` return
  the cached `*Store`; reusing a `Name` with a different `Migrations` slice
  returns an invalid-descriptor error. `Name` (`^[a-z][a-z0-9_]*$`) and
  `FileName` (no path separator) are validated here. `Migrations` may be empty.
  Returns an error if called after `Close`; returns the context error (safe to
  retry) if `ctx` is cancelled mid-call.
- `Close` flushes committed writes to disk and closes the pool. Call it once at
  process shutdown (e.g. `defer mgr.Close()`). Idempotent, and safe to call if
  the DB was never opened. Quiesce module DB activity before calling it.

### type Descriptor

```go
type Descriptor struct {
    Name       string   // ^[a-z][a-z0-9_]*$
    Migrations []string
}
```

- `Name` keys migration tracking and is the required prefix for the module's
  tables. Invalid names are rejected by `Store`.
- `Migrations` is append-only. See [Migrations](#migrations).

### type Store

```go
type Store struct { /* ... */ }

func (s *Store) DB() *sql.DB
```

`DB` returns the shared connection pool. Run your own SQL against your own
tables.

## Usage

```go
// startup
mgr := localdb.New(localdb.Config{Dir: cacheDir})
defer mgr.Close()

// in a module, gated by its own enablement
store, err := mgr.Store(ctx, localdb.Descriptor{
    Name: "malysis_cache",
    Migrations: []string{
        `CREATE TABLE malysis_cache_verdicts (
            purl       TEXT PRIMARY KEY,
            result     BLOB NOT NULL,
            created_at INTEGER NOT NULL
        )`,
    },
})
if err != nil {
    return err
}

db := store.DB()

// read
var blob []byte
err = db.QueryRowContext(ctx,
    `SELECT result FROM malysis_cache_verdicts WHERE purl = ?`, purl,
).Scan(&blob)

// write — one short autocommit statement
_, err = db.ExecContext(ctx,
    `INSERT INTO malysis_cache_verdicts (purl, result, created_at)
     VALUES (?, ?, ?)
     ON CONFLICT(purl) DO UPDATE SET result = excluded.result,
                                     created_at = excluded.created_at`,
    purl, blob, createdAt,
)
```

## Migrations

`Migrations` is an ordered, append-only list of SQL statements. On `Store`, only
the not-yet-applied entries run, each exactly once.

- Append only. Never edit or reorder an existing entry — it has already run on
  other machines.
- Each entry runs exactly once, so non-idempotent statements (`ALTER TABLE`,
  backfills) are valid.
- Prefix every table and index with the module `Name`.

To evolve schema, append:

```go
Migrations: []string{
    `CREATE TABLE malysis_cache_verdicts (...)`,                    // v1
    `ALTER TABLE malysis_cache_verdicts ADD COLUMN ecosystem TEXT`, // v2
},
```

## Transaction discipline (required)

SQLite holds the single-writer lock only while a write transaction is open.

- Keep write transactions tiny. Open, write, commit.
- Never hold a write transaction across network calls, long computation, or
  other long-running work.
- Chunk large writes into multiple small transactions.

A long-running operation must do its work holding no lock and write in short
bursts. Violating this blocks other processes.

## Errors

`Store` and migration errors are returned, never swallowed, and carry a
`usefulerror` code so callers can classify them without string matching. The
consumer decides the failure policy: a cache treats a backend error as a miss
and proceeds; a fail-closed consumer may abort.

## Constraints

- `Config.Dir` must be on a **local filesystem**. WAL mode is unsafe over network
  filesystems (NFS/SMB/overlay) and can corrupt the DB there.
- Treat stored data as reconstructible for cache-like use — a cache directory
  may be wiped at any time.
- Tables are isolated by naming convention (`<name>_*`), not enforced. Do not
  read or write another module's tables.
