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

## Delivery guarantees

- **At-least-once per destination** for the durable paths (`Emit`, buffered
  `Send`). Consumers dedupe on the envelope `event_id`.
- **Per-subject ordering**: a failing delivery blocks only its own subject and is
  retried, never skipped — downstream state never advances past a gap. A delivery
  that exceeds `WithMaxAttempts` is flagged (`stuck_since`) for alerting but keeps
  retrying.
- `Send` without a store is best-effort (no persistence; lost on crash).

## Consuming events

The symmetric consumer toolkit (`events/inbox`) is forthcoming. Until then,
subscribe with the transport clients directly and resolve addresses with
`events.RoutingFor` / `stream.StreamFor` / `async.EventSubject`.
