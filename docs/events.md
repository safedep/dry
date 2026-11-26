# Events

Produce SafeDep platform events to one or more transports (S2, NATS) with a
durable, at-least-once outbox. Event schemas are protobuf messages defined in the
[`safedep/api`](https://github.com/safedep/api) repository; this package
(`events`, `events/outbox`) is the Go producer toolkit.

## Event convention

Events are first-class, versioned contracts declared in `safedep/api` under
`proto/safedep/events/<exposure>/<domain>/v<major>/`. Read
[**EVENTS.md** in `safedep/api`](https://github.com/safedep/api/blob/main/EVENTS.md)
before defining or changing an event. In short:

- One feed = one top-level `<Feed>Event` message, with the shared
  `safedep.events.common.v1.EventMeta` envelope at **field 1**.
- `exposure` is `private` (SafeDep-internal) or `public` (customer-facing),
  encoded in the package path.
- Routing is derived from the message name — never hardcode stream/subject names;
  resolve them via `events.RoutingFor` / the SDK helpers.
- Wire format is binary protobuf.

You consume the generated message types from the hosted SDK
(`buf.build/gen/go/safedep/api/...`).

## Construct an event

Stamp the shared envelope onto a feed message with `events.New`. It sets a ULID
`event_id` and `occurred_at` by default, then validates the message:

```go
import (
    pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
    "github.com/safedep/dry/events"
)

obs := pkgregv1.PackageVersionObservationEvent_builder{
    PackageVersion: pv,
    Kind:           pkgregv1.PackageVersionObservationEvent_KIND_PUBLISHED,
}.Build()

evt, err := events.New(obs,
    events.WithSubject(packageVersionURN), // per-subject ordering key
    events.WithProducer("malysis"),
)
if err != nil {
    return err
}
```

## Producer (outbox)

Construct an `Outbox` over the destinations you publish to. Inject a
`db.SqlDataAdapter` with `WithStore` for the durable paths; without it, `Send`
publishes directly.

```go
import (
    "github.com/safedep/dry/events/outbox"
    "github.com/safedep/dry/events/outbox/destinations"
)

ob, err := outbox.New(
    []outbox.Destination{
        destinations.NewNATS(natsClient),      // intra-platform; private feeds
        destinations.NewS2(s2Config, nil),     // crosses to customers; required for public
    },
    outbox.WithStore(adapter), // optional: enables Emit + buffered Send + Run
)
```

The outbox tables live in your database; create them from your migration
pipeline:

```go
if err := outbox.Migrate(adapter); err != nil {
    return err
}
```

`Migrate` runs GORM `AutoMigrate`. It creates a missing index with a plain
`CREATE INDEX`, which blocks writes to the table until the build ends. On a
large `event_outbox_delivery` table, create a new index with
`CREATE INDEX CONCURRENTLY` before you deploy the version that adds it.
`AutoMigrate` then finds the index and skips it. For example:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_outbox_delivery_outbox_id
    ON event_outbox_delivery (outbox_id);
```

### Two write paths

**`Emit` — transactional.** The event is written inside your business
transaction, so the two commit atomically. The drain publishes it afterwards.

```go
err = db.Transaction(func(tx *gorm.DB) error {
    // ... your business write on tx ...
    return ob.Emit(ctx, tx, evt)
})
```

**`Send` — fire-and-forget.** With a store the event is buffered and the drain
publishes it; without a store it is published directly (lost on crash, accepted).

```go
err = ob.Send(ctx, evt)
```

### Run the drain

When using a store, run the drain (single instance) to publish buffered events:

```go
go func() {
    if err := ob.Run(ctx); err != nil {
        log.Errorf("outbox drain stopped: %v", err)
    }
}()
```

### Retention

`Run` periodically purges fully-delivered records past the retention window
(`WithRetention`, default 24h). The transport (e.g. S2) remains the durable replay
source beyond that window; the outbox table is only a delivery buffer.

## Concurrency (multiple replicas)

- **`Emit` and `Send` are safe on every replica** — produce events from all of them.
- **The drain (`Run`) is single-writer** — it preserves per-subject ordering, so it
  must not run concurrently.

For multi-replica deployments, enable `WithLeaderElection()` and call `Run` on every
replica: only the holder of a Postgres advisory lock drains, and a standby takes over
automatically if the leader dies (the lock is released when its connection drops — no
TTL or lease renewal). It requires a PostgreSQL store; `Run` errors otherwise.

```go
ob, err := outbox.New(dests,
    outbox.WithStore(adapter),
    outbox.WithLeaderElection(),
)
// ... on every replica:
go ob.Run(ctx)
```

Without `WithLeaderElection`, run `Run` on exactly one instance (a single-replica
worker). Cleanup rides inside `Run`, so it runs on the leader only.

### Connection handling

Leader election uses a Postgres **session advisory lock**, which the leader holds on
one dedicated connection for the duration of leadership (drain/cleanup queries use the
normal pool, not this connection). So:

- The pool needs **≥2 connections** (1 pinned for the lock, ≥1 for the drain).
- Works with a **direct** connection, **RDS Proxy** (it pins the connection — expect 1
  in `DatabaseConnectionsCurrentlySessionPinned`), or **PgBouncer in `session` mode**.
  It does **not** work under PgBouncer `transaction`/`statement` pooling, which rebinds
  server connections per transaction and would break the session lock.
- The lock connection is kept alive by a ping every `pollInterval`, so it is not reaped
  by idle timeouts (keep `pollInterval` well below any client idle timeout).

## Delivery guarantees

- **At-least-once per destination** for the durable paths (`Emit`, buffered
  `Send`). Consumers dedupe on the envelope `event_id`.
- **Per-subject ordering**: a failing delivery blocks only its own subject and is
  retried, never skipped — downstream state never advances past a gap. A delivery
  that exceeds `WithMaxAttempts` is flagged (`stuck_since`) for alerting but keeps
  retrying.
- `Send` without a store is best-effort (no persistence; lost on crash).

## Consuming events

`events/inbox` is the consumer toolkit. `Consume` reads one feed from a `Source`
one record at a time (S2). `BatchRouter` reads many feeds from one
`BatchSource` in batches (JetStream).

## JetStream

`events/jetstream` and `events/inbox/sources.NewJetStream` carry private feeds
over NATS JetStream. One stream holds one feed family. Writer replicas share one
durable pull consumer, and JetStream gives each record to one replica. The
transport does not keep the order of one tenant.

### Subject

A record subject is `<full message name>.<tenant>`, from
`async.TenantEventSubject`. The tenant is one subject token. A tenant with a
dot, a wildcard (`*`, `>`), whitespace or a control character is an error.
`async.TenantEventSubjects(routing)` gives `<full message name>.*` for stream
subjects and consumer filters. `async.EventSubject` does not change.

### Stream and consumer

```go
stream, err := jetstream.EnsureStream(ctx, js, jetstream.StreamConfig{
    Name:            "PACKAGEREGISTRY",
    Subjects:        []string{"safedep.events.private.packageregistry.v1.>"},
    MaxAge:          7 * 24 * time.Hour,
    MaxBytes:        10 << 30,
    Replicas:        3,
    DuplicateWindow: 10 * time.Minute,
})

consumer, err := jetstream.EnsureConsumer(ctx, stream, jetstream.ConsumerConfig{
    Durable:    "clickhouse-writer",
    AckWait:    time.Minute,
    MaxDeliver: inbox.DefaultMaxAttempts,
})
```

`EnsureStream` creates or updates a file stream with limits retention that
discards the oldest records. `MaxAge` and `MaxBytes` are required.
`EnsureConsumer` creates or updates a durable pull consumer with explicit acks.
No `FilterSubjects` reads the whole stream. More than one filter needs NATS
server 2.10 or later. Set `MaxDeliver` to at least the delivery budget of the
router. Otherwise the server stops a record before the router can dead-letter
it.

### Publish

```go
pub, err := jetstream.NewPublisher(js)
res, err := pub.Publish(ctx, evt, tenant)
```

`Publish` sends binary proto and waits for the acknowledgement of the stream.
It sets the `Nats-Msg-Id` header to the envelope `event_id`, or to the id of
`jetstream.WithMsgID`. The stream drops a second record with the same id in its
duplicate window. A duplicate is a success, and `res.Duplicate` is true.
`Publish` refuses a public feed.

The outbox NATS destination also sets `Nats-Msg-Id` to the event id when its
publisher has `PublishWithHeaders`, as the `dry/async` client has.

### Batch consume

```go
src, err := sources.NewJetStream(consumer)
router := inbox.NewBatchRouter(src,
    inbox.WithBatchSize(500),
    inbox.WithBatchDeadLetter(dlq), // inbox.NewGormDeadLetter
    inbox.WithBatchHeartbeat(15*time.Second),
)
err = inbox.Route(router, newObservation, insertObservations)
err = inbox.Route(router, newMaintainer, insertMaintainers)
err = router.Run(ctx)
```

`Route` registers one `BatchHandler` for each event type. The route key is the
full message name, which is the subject without its tenant token. The router
also checks that the decoded event has the routing of its feed.

`Run` fetches a batch and runs the handler of each feed of the batch:

- A record that does not decode is dead-lettered and terminated. The other
  records go on.
- A record whose subject has no route is dead-lettered and terminated.
- If a handler succeeds, the router acks the records of its feed.
- If a handler fails, the router redelivers the records of its feed with a
  backoff by attempt. The records of the other feeds are not affected.
- If the error is permanent (`inbox.WithPermanentClassifier`), or a record is on
  its last attempt (`inbox.WithMaxAttempts`), the router runs the handler with
  each record alone. It acks each record that passes. It dead-letters and
  terminates each record that fails for good.
- While the handlers run, a heartbeat marks every open record of the batch as in
  progress, so a slow insert does not pass `AckWait`.
- A fetch error makes the router wait and fetch again. `Run` returns only when
  ctx is done.

Without a dead-letter store, the router logs a record that it gives up on and
terminates it. If the store fails, the router redelivers the record.
`ConsumeBatch` is a router with one route.

### Dead letters

The router writes dead letters to the `event_inbox_dead_letter` table, with the
feed and a reason: `no_route`, `decode` or `handler`. Rows from before the
reason column, and rows of `NewRetryPolicy`, have the reason `unspecified`.
`DeadLetterReader` filters on `Feed` and `Reason`.

`router.Replay(ctx, reader, filter)` runs each dead letter through the handler
of its feed, as a batch of one event. It deletes a dead letter that passes. A
dead letter that fails again stays, with the new reason and error and one more
attempt. A dead letter whose feed has no route stays as it is. So a record that
was dead-lettered for `no_route` clears after a release adds its route.

### Purge a tenant

```go
err := jetstream.PurgeTenant(ctx, stream, routing, tenant)
```

`PurgeTenant` removes the records of one tenant of one feed from the stream.
