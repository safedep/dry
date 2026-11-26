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
`BatchSource` in batches (JetStream). The outbox NATS destination publishes on
the bare feed subject without a tenant token, so the JetStream feeds take their
events from `natsjs.Publisher`.

## JetStream

`events/natsjs` and `events/inbox/sources.NewJetStream` carry private feeds over
NATS JetStream. One stream holds one feed family. Writer replicas share one
durable pull consumer, and JetStream gives each record to one replica. The
transport does not keep the order of one tenant.

### Subject

A record subject is `<full message name>.<tenant>`, from
`async.TenantEventSubject`. The tenant is a canonical tenant id of 1 to 64 bytes
from `[A-Za-z0-9_-]`. A dotted tenant domain such as `ab.cd.safedep.io` is an
error, because each dot adds a subject token. `async.TenantEventSubjects`
gives `<full message name>.*` for stream subjects and consumer filters.
`async.ParseTenantEventSubject` splits a subject into the feed and the tenant.
`async.EventSubject` does not change.

### Publish

```go
pub, err := natsjs.NewPublisher(js)
res, err := pub.Publish(ctx, evt)
```

`Publish` takes the feed from the message type and the tenant from the envelope
`tenant.tenant_id`. It refuses an event without a tenant, and a public feed. It
sends binary proto and waits for the acknowledgement of the stream. It sets the
`Nats-Msg-Id` header to the envelope `event_id`, or to the id of
`natsjs.WithMsgID`. The stream drops a second record with the same id in its
duplicate window. A duplicate is a success, and `res.Duplicate` is true.

The router trusts the subject to route a record. Publish only through
`Publisher`. A raw publish to a feed subject is not supported.

### Stream and consumer

`natsjs.EnsureStream` creates a file stream with limits retention that discards
the oldest records. `MaxAge` and `MaxBytes` are required. `natsjs.EnsureConsumer`
creates a durable pull consumer with explicit acks that starts at the first
record. No `FilterSubjects` reads the whole stream. More than one filter needs
NATS server 2.10 or later.

Both functions create what is missing and never change what exists. An older
replica would otherwise revert a newer config, and a lower `MaxAge` or
`MaxBytes` discards records. When the server config differs from the code, they
return an error that wraps `natsjs.ErrDrift` and names each field. They also
compare the fields that the config does not expose: the stream message limits
and the consumer ack policy and headers-only mode. Change a
stream or a consumer on purpose with `natsjs.UpdateStream` or
`natsjs.UpdateConsumer`.

The consumer has no delivery limit, and `ConsumerConfig` has no field for one.
The router owns the delivery budget and dead-letters a record on its last
attempt. During an outage, or while the dead-letter store fails, it redelivers a
record without a limit. A server limit would drop that record with no dead
letter. So `EnsureConsumer` reports a limit on an existing consumer as drift,
and `BatchRouter.Run` refuses to start when the consumer has a limit.

### Writer

```go
package writer

import (
	"context"
	"os"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/async"
	"github.com/safedep/dry/db"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/inbox"
	"github.com/safedep/dry/events/inbox/sources"
	"github.com/safedep/dry/events/natsjs"
)

type (
	observation = pkgregv1.PackageVersionObservationEvent
	maintainer  = pkgregv1.MaintainerObservationEvent
)

// row is one ClickHouse row.
type row struct {
	EventID    string
	Tenant     string
	OccurredAt time.Time
}

func runWriter(ctx context.Context) error {
	nc, err := nats.Connect(os.Getenv("NATS_URL"))
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}

	observations, err := events.RoutingFor(&observation{})
	if err != nil {
		return err
	}
	maintainers, err := events.RoutingFor(&maintainer{})
	if err != nil {
		return err
	}

	stream, err := natsjs.EnsureStream(ctx, js, natsjs.StreamConfig{
		Name:            "PACKAGEREGISTRY",
		Subjects:        []string{async.TenantEventSubjects(observations), async.TenantEventSubjects(maintainers)},
		MaxAge:          7 * 24 * time.Hour,
		MaxBytes:        10 << 30,
		Replicas:        3,
		DuplicateWindow: 10 * time.Minute,
	})
	if err != nil {
		return err
	}

	consumer, err := natsjs.EnsureConsumer(ctx, stream, natsjs.ConsumerConfig{
		Durable: "clickhouse-writer",
		AckWait: time.Minute,
	})
	if err != nil {
		return err
	}

	src, err := sources.NewJetStream(consumer)
	if err != nil {
		return err
	}

	adapter, err := db.NewPostgreSqlAdapter(db.PostgreSqlAdapterConfig{DSN: os.Getenv("DATABASE_URL")})
	if err != nil {
		return err
	}
	deadLetters, err := inbox.NewGormDeadLetter(adapter, "clickhouse-writer")
	if err != nil {
		return err
	}

	router, err := inbox.NewBatchRouter(src, deadLetters,
		inbox.WithBatchSize(500),
		inbox.WithBatchHeartbeat(15*time.Second),
	)
	if err != nil {
		return err
	}

	err = inbox.Route(router, func() *observation { return &observation{} },
		func(ctx context.Context, batch []inbox.BatchEvent[*observation]) error {
			rows := make([]row, 0, len(batch))
			for _, ev := range batch {
				rows = append(rows, row{
					EventID:    ev.Meta.GetEventId(),
					Tenant:     ev.Meta.GetTenant().GetTenantId(),
					OccurredAt: ev.Meta.GetOccurredAt().AsTime(),
				})
			}
			return insert(ctx, "package_version_observations", rows)
		})
	if err != nil {
		return err
	}

	err = inbox.Route(router, func() *maintainer { return &maintainer{} },
		func(ctx context.Context, batch []inbox.BatchEvent[*maintainer]) error {
			rows := make([]row, 0, len(batch))
			for _, ev := range batch {
				rows = append(rows, row{
					EventID:    ev.Meta.GetEventId(),
					Tenant:     ev.Meta.GetTenant().GetTenantId(),
					OccurredAt: ev.Meta.GetOccurredAt().AsTime(),
				})
			}
			return insert(ctx, "maintainer_observations", rows)
		})
	if err != nil {
		return err
	}

	return router.Run(ctx)
}

// insert writes rows to a ClickHouse table in one statement.
func insert(ctx context.Context, table string, rows []row) error {
	return nil
}
```

Run `inbox.Migrate` from your migration pipeline before a new version consumes.
It adds the `reason`, `subject` and `tenant` columns to the dead-letter table.

`sources.NewJetStream` waits up to 2s for a full batch (`WithFetchMaxWait`). The
wait must be below the consumer `AckWait`. A cancel stops the router after the
current fetch wait ends. `NewJetStream` refuses a consumer that does not ack
each record explicitly, or that delivers headers only.

### What the router does with a record

`Route` registers one `BatchHandler` for each event type. The route key is the
full message name. For each fetch, the router groups the records by feed and
runs the handler of each feed once. Each feed is acked or redelivered on its
own, so a failing feed never redelivers the records of a feed that succeeded.

- A record that does not decode, or whose envelope tenant is not the tenant of
  its subject, is dead-lettered (`decode`) and terminated at once.
- A record without a route is redelivered with a backoff. On its last attempt
  it is dead-lettered (`no_route`). In a rolling deploy an old replica can fetch
  a feed that only the new version routes.
- If a handler succeeds, the router acks the records of its feed.
- If a handler fails, the router redelivers the records of its feed with a
  backoff by attempt (`WithBatchRedeliveryBackoff`).
- If the error is permanent (`inbox.WithPermanentClassifier`), or a record is on
  its last attempt (`inbox.WithMaxAttempts`), the router runs the handler with
  each record alone. It acks each record that passes. If one record passed, it
  dead-letters (`handler`) each record that failed. If none passed, the cause is
  most likely an outage: it dead-letters only the records with a permanent error
  and redelivers the rest after the maximum backoff. The consumer lag alert
  covers the outage.
- A handler can thus run again with single records, and delivery is at least
  once. A handler must be idempotent.
- A handler panic is a handler error.
- Before the first handler runs, and then at each heartbeat interval
  (`WithBatchHeartbeat`), the router marks every open record of the batch as in
  progress. A record can wait in the fetch for most of `AckWait`, and a slow
  insert must not pass `AckWait`.
- If the dead-letter store fails, the router redelivers the record.
- At shutdown the router does not nack, because a nack spends a delivery. The
  open records come back after `AckWait`.
- A fetch error makes the router wait (`WithBatchRestartDelay`) and fetch again.
  `Run` returns only when ctx is done.

### Dead letters

The router writes dead letters to the `event_inbox_dead_letter` table, with the
feed, the subject, the source tenant and a reason: `no_route`, `decode` or
`handler`. Rows from before these columns, and rows of `Consume`, have an empty
subject and tenant. Rows from before the
reason column, and rows of `NewRetryPolicy`, have the reason `unspecified`.
`DeadLetterReader` filters on `Feed` and `Reason`.

To replay, build a router with the routes of the writer and call
`router.Replay(ctx, reader, filter)`. A router for Replay alone can have a nil
source. Replay runs each dead letter through the handler of its feed, as a batch
of one event. The envelope tenant must equal the source tenant that the router
stored on the row, so a row without a stored tenant stays dead-lettered. It
deletes a dead letter that passes. A dead letter that fails
again stays, with the new reason and error and one more attempt. A dead letter
whose feed has no route stays as it is. So a `no_route` record clears after a
release adds its route.
The generic `inbox.Replay[T]` refuses a row of the router, because it has no
tenant to check. It replays only the rows of `Consume`.

### Purge a tenant

```go
err := natsjs.PurgeTenant(ctx, stream, routing, tenant)
```

`PurgeTenant` removes the records of one tenant of one feed from the stream.

### Tests

The JetStream tests need a NATS server with JetStream. They skip when
`DRY_NATS_TEST_URL` is not set. See [nats_test.md](nats_test.md).
