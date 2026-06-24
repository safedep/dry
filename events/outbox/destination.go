package outbox

import (
	"context"

	"github.com/safedep/dry/events"
)

// Destination is a transport sink for events. Implementations render their
// transport-native address from the routing (S2 via stream.StreamFor, NATS via
// async.EventSubject) and publish the already-serialized record bytes.
//
// Name must be stable and unique within an Outbox: it is persisted in the
// Delivery rows to track per-destination delivery, so changing it strands
// in-flight deliveries.
type Destination interface {
	// Name identifies the destination in delivery state (e.g. "s2", "nats").
	Name() string

	// Publish delivers one record to this transport. tenant is the envelope's
	// tenant (empty for global feeds); a per-tenant transport renders a
	// per-tenant address from it. record is the binary-proto <Feed>Event.
	Publish(ctx context.Context, routing events.Routing, tenant string, record []byte) error
}
