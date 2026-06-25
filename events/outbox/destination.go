package outbox

import (
	"context"

	"github.com/safedep/dry/events"
)

// PublishRequest is one event to deliver to a Destination. The fields are a
// struct (rather than positional args) so adding transport metadata later is
// non-breaking and adjacent string params can't be transposed.
type PublishRequest struct {
	// Routing identifies the feed; the destination renders its transport-native
	// address from it (S2 via stream.StreamFor, NATS via async.EventSubject).
	Routing events.Routing

	// Tenant is the envelope's tenant (empty for global feeds); a per-tenant
	// transport renders a per-tenant address from it.
	Tenant string

	// EventID is the envelope's ULID, for transport headers (decode-free ops
	// dedup/correlation). Distinct from Routing.FQN, which is the feed name.
	EventID string

	// Subject is the envelope's per-subject ordering domain (empty if none), for
	// transport headers and consumer-side ordering.
	Subject string

	// Record is the binary-proto <Feed>Event (envelope + payload).
	Record []byte
}

// Destination is a transport sink for events. Implementations render their
// transport-native address from the routing and publish the record bytes.
//
// Name must be stable and unique within an Outbox: it is persisted in the
// Delivery rows to track per-destination delivery, so changing it strands
// in-flight deliveries.
type Destination interface {
	// Name identifies the destination in delivery state (e.g. "s2", "nats").
	Name() string

	// Publish delivers one record to this transport.
	Publish(ctx context.Context, req PublishRequest) error
}
