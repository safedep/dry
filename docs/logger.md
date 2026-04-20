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

## Per-Event Log Calls Inside an Event

Legacy call sites like `drylog.Infof("step 1")` inside a request are captured into the canonical line's `messages[]` array (capped at 50; overflow counted as `messages_dropped`). `Errorf` additionally promotes the canonical event to error level.

Outside an event scope (startup code, background workers), these calls emit standalone JSON lines as before.

## Environment Variables

| Variable | Values | Default | Purpose |
|----------|--------|---------|---------|
| `APP_LOG_LEVEL` | `debug` \| `info` \| `warn` \| `error` | `info` | Level filter. |
| `APP_LOG_FORMAT` | `text` \| `json` | `text` for `dev`/`local`/empty env, else `json` | Output format for the stdout sink. |
| `APP_LOG_FILE` | path | unset | Enable rotating JSON file sink (100MB / 3 backups / 7 days). |
| `APP_LOG_SKIP_STDOUT_LOGGER` | `true` \| `false` | `false` | Disable stdout sink. |
| `APP_LOG_CAPTURE_MESSAGES` | `true` \| `false` | `true` | Capture in-event `Infof`/`Errorf` into `messages[]`. When `false`: dev still prints them inline, prod drops them silently. |

## Lifecycle Rules

- `BeginEvent` and its `EndFunc` must be called on the same goroutine. Dispatching work to another goroutine inside a handler is fine; just make sure the event is ended on the original goroutine.
- Nested `BeginEvent` is forbidden. A nested call returns the existing context unchanged and marks `nested_begin: true` on the outer event.
- `EndFunc` is idempotent — calling it a second time is a no-op.
- `Fatalf` inside an event flushes the canonical line before `os.Exit(1)`.

## Choosing Between Classic and Canonical

Use **canonical** for:
- HTTP APIs, async consumers, CLI commands — anything request-shaped.
- Services with high QPS where per-event log volume is a cost concern.
- Observability queries that want one row per request.

Stick with **classic** for:
- Startup logs, background daemons, code paths that aren't request-shaped.
- Services that haven't migrated yet. Both modes coexist; migrate on your own schedule.
