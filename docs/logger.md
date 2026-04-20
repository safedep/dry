# Logger

Structured logging for SafeDep services. Two modes coexist:

- **Classic per-event logging** via `log.Infof`, `log.Errorf`, etc. (zap-backed by default).
- **Canonical events** — one structured line per request/message/command, accumulating attributes across the handler chain (slog-backed, opt-in).

Canonical events are cheap to query, keep request state in one place, and cost O(1) log lines per unit of work.

## Getting Started

Pick one initializer at service startup:

```go
import drylog "github.com/safedep/dry/log"

// Classic (zap, per-event lines). Default behavior, unchanged.
drylog.Init("my-service", "prod")

// Canonical events (slog, one line per request). Opt-in.
drylog.InitSlogLogger("my-service", "prod")
```

Both satisfy the same `drylog.Logger` interface, so callers don't change.

## Emitting a Canonical Event

Anywhere you have a `context.Context`:

```go
ctx, end := drylog.BeginEvent(ctx, "http.request")
defer end() // flushes exactly one JSON line on return

drylog.Set(ctx, "user.id", userID)
drylog.Set(ctx, "db.queries", 4)
drylog.Counter(ctx, "cache.hits", 1) // atomic; safe across goroutines
drylog.Err(ctx, err)                 // sets error attr, promotes level
```

Result (production, JSON):

```json
{"time":"2026-04-19T12:34:56Z","level":"INFO","msg":"http.request","service":"my-service","env":"prod","event":"http.request","duration_ms":3.4,"user.id":"u1","db.queries":4,"cache.hits":1}
```

Dev mode (pretty-printed):

```
12:34:56.789 INFO  http.request service=my-service env=dev event=http.request duration_ms=3.4 user.id=u1 db.queries=4
```

## HTTP (Echo)

`NewEchoRouter` wires the middleware automatically. Any handler sees the event on its request context:

```go
router, _ := adaptershttp.NewEchoRouter(adaptershttp.EchoRouterConfig{
    ServiceName: "api",
})

router.AddRoute(adaptershttp.GET, "/users/:id", handler)

// In your handler:
func getUser(c echo.Context) error {
    drylog.Set(c.Request().Context(), "user.id", c.Param("id"))
    // ... your logic ...
    return c.JSON(200, user)
}
```

The middleware pre-populates `request_id`, `http.method`, `http.path`, `http.route`, `http.status`, `http.bytes_in`, `http.bytes_out`, `peer.ip`, `duration_ms`. Handler panics are captured onto the canonical line (`panic`, `stack`) and re-panicked for Echo's `Recover()` to convert to 500.

## Async (NATS)

Wrap any `MessageHandler` to get one canonical line per message:

```go
import "github.com/safedep/dry/async"

handler := async.WithEventLogging("msg.order.create",
    func(ctx context.Context, data []byte, extra async.MessageExtra) error {
        drylog.Set(ctx, "order.id", parseID(data))
        return process(ctx, data)
    })

msgSvc.QueueSubscribe(ctx, "orders.create", "workers", handler)
```

Pre-populated: `msg.subject`, `msg.reply_to`, `msg.bytes`, `duration_ms`, plus any `error` from the handler.

## CLI Commands

For cobra or standalone scripts:

```go
import drylog "github.com/safedep/dry/log"

err := drylog.RunCommand(ctx, "backfill", func(ctx context.Context) error {
    drylog.Set(ctx, "rows.processed", n)
    return doWork(ctx)
})
```

Emits one line with `msg=cmd.backfill`, `duration_ms`, and any `error`.

## Legacy `log.Infof` Inside an Event

`drylog.Infof`, `Errorf`, etc. always emit standalone lines — they're NOT captured into the canonical event's attributes. To attach state to the canonical line, use `drylog.Set(ctx, k, v)` explicitly. If you're migrating a high-volume code path to canonical events, replace `log.Infof("user %s seen", id)` with `log.Set(ctx, "user.id", id)`.

Outside an event scope (startup code, background workers), `Infof` continues to emit standalone JSON lines as before.

## Environment Variables

| Variable | Values | Default | Purpose |
|----------|--------|---------|---------|
| `APP_LOG_LEVEL` | `debug` \| `info` \| `warn` \| `error` | `info` | Level filter. |
| `APP_LOG_FORMAT` | `text` \| `json` | `text` for `dev`/`local`/empty env, else `json` | Output format for the stdout sink. |
| `APP_LOG_FILE` | path | unset | Enable rotating JSON file sink (100MB / 3 backups / 7 days). |
| `APP_LOG_SKIP_STDOUT_LOGGER` | `true` \| `false` | `false` | Disable stdout sink. |

## Lifecycle Rules

- Nested `BeginEvent` is forbidden. A nested call returns the existing context unchanged and marks `nested_begin: true` on the outer event.
- `EndFunc` is idempotent — calling it a second time is a no-op.
- `os.Exit` / `log.Fatalf` inside an event **skips** the deferred flush — the canonical line is lost. Prefer returning an error and exiting from `main`, or call `end()` explicitly before exiting.

## Guidance for Canonical Logging

Canonical events work best when they capture *everything you'd want to know about a unit of work* in a single structured row. The patterns below come from running this style in production at scale.

### Scope

- **One event per logical unit of work.** Open `BeginEvent` at a request, message, or command boundary — never inside a loop, never per database call. The whole point is O(1) lines per request.
- **Don't nest events.** Helper functions called from a handler must accept `ctx` and call `log.Set(ctx, ...)`, not start their own event. Nested `BeginEvent` is rejected by the API and surfaces `nested_begin: true` so misuse is loud.
- **Own the event at the entry point.** HTTP middleware, async wrapper, and `RunCommand` already do this — handler code should never call `BeginEvent` directly inside a request.

### Attribute design

- **Stable, dotted, lowercase keys.** `user.id`, `db.queries`, `http.status`, `cache.hits`. Pick a key once, keep it forever. A typo makes the field unqueryable.
- **Prefer semantic conventions.** Match OpenTelemetry attribute names where they exist (`http.method`, `http.status`, `peer.ip`, `db.system`). Easier joins across logs, traces, and metrics.
- **Primitive values.** Numbers, booleans, short strings. Avoid embedding JSON blobs or stringified arrays — they defeat indexing and inflate row size.
- **High-cardinality is fine on the event, dangerous as a bucket.** A `user.id` per row is normal; computing per-user counters from logs is not. Use proper metrics for cardinality-sensitive aggregations.

### What to record

- **Always:** `request_id` (or equivalent correlation key), inputs that shape the response (path, method, key params), outcome (status, error class), and a duration.
- **Often useful:** counts of expensive work (`db.queries`, `external_calls`, `cache.hits`/`cache.misses`), feature flags consumed, auth/tenant identity, downstream service latencies.
- **Errors:** call `log.Err(ctx, err)` for known failure modes — it sets the canonical line's level to `error` and adds the `error` attribute. Don't `log.Errorf` separately; that emits a second line.
- **Counters over repeated calls.** Use `log.Counter(ctx, "db.queries", 1)` in a loop, not `Set` (which overwrites and isn't atomic).

### What NOT to record

- **No secrets.** Credentials, tokens, full request/response bodies, raw PII. Redact at the call site; the canonical line is structured enough that nothing should sneak in unaudited.
- **No intermediate progress.** Don't sprinkle `log.Set(ctx, "step", "did X")` and overwrite it through the request — the canonical line shows only the final value. If you need a trail, use spans/traces.
- **No noisy debug crumbs.** `log.Debugf` calls inside an event still emit standalone lines and defeat the "one row per request" model. Convert them to attributes or delete them.

### Migration

- **Migrate hot paths first.** A request that today produces 30 `Infof` lines is the highest-value candidate. Replace each `Infof` with a `Set` (or just delete it).
- **Keep classic for non-request-shaped code.** Startup, background daemons, and one-off scripts have no canonical event scope; `Infof` is correct there.
- **Don't mix.** Inside an event scope, prefer `log.Set` exclusively. If you have to log a one-off line during a request (e.g. a noisy library you don't control), accept the standalone line and don't double-log the same fact as both an attr and a message.

### Operational

- **Query first, design second.** Before adding an attribute, ask "what dashboard or alert needs this?" If the answer is "none yet," skip it — adding fields later is cheap.
- **Treat the canonical line as a public schema.** Renaming `http.status` to `status_code` breaks every dashboard and alert downstream. Pick names you can live with.
- **Sample at the source if you must.** v1 has no built-in sampling. If volume becomes a problem, drop entire events at the entry-point middleware (e.g. health checks), not individual fields.

## Choosing Between Classic and Canonical

Use **canonical** for:

- HTTP APIs, async consumers, CLI commands — anything request-shaped.
- Services with high QPS where per-event log volume is a cost concern.
- Observability queries that want one row per request.

Stick with **classic** for:

- Startup logs, background daemons, code paths that aren't request-shaped.
- Services that haven't migrated yet. Both modes coexist; migrate on your own schedule.

## References

- <https://stripe.com/blog/canonical-log-lines>
