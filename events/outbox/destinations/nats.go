// Package destinations provides the built-in outbox.Destination adapters for S2
// and NATS. They are a separate package so the outbox engine stays transport-free
// — a consumer that only needs one transport (or a custom destination) does not
// pull both transport SDKs.
package destinations

import (
	"context"
	"fmt"

	"github.com/safedep/dry/async"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/outbox"
)

// NatsPublisher is the minimal NATS capability the destination needs. The
// dry/async messaging client satisfies it.
type NatsPublisher interface {
	Publish(ctx context.Context, topic string, data []byte) error
}

// natsHeaderPublisher is the optional capability to publish with headers. The
// dry/async messaging client has it.
type natsHeaderPublisher interface {
	PublishWithHeaders(ctx context.Context, topic string, data []byte, headers map[string][]string) error
}

// natsMsgIDHeader makes a JetStream stream drop a second copy of an event in its
// duplicate window, for example after the outbox retries a delivery.
const natsMsgIDHeader = "Nats-Msg-Id"

// NatsDestination publishes events to NATS on the subject derived from the
// routing. NATS is intra-platform only, so public feeds are rejected.
type NatsDestination struct {
	pub NatsPublisher
}

var _ outbox.Destination = (*NatsDestination)(nil)

// NewNATS builds a NATS destination over a publisher (e.g. dry/async messaging).
// When the publisher can also publish with headers, the destination sets the
// Nats-Msg-Id header to the event id.
func NewNATS(pub NatsPublisher) *NatsDestination {
	return &NatsDestination{pub: pub}
}

func (d *NatsDestination) Name() string { return "nats" }

// Accepts rejects public feeds: NATS is intra-platform only.
func (d *NatsDestination) Accepts(routing events.Routing) bool {
	return !routing.IsPublic()
}

func (d *NatsDestination) Publish(ctx context.Context, req outbox.PublishRequest) error {
	if req.Routing.IsPublic() {
		return fmt.Errorf("nats destination: public feed %s is not eligible for NATS (S2 only)", req.Routing.FQN)
	}
	if d.pub == nil {
		return fmt.Errorf("nats destination: nil publisher")
	}

	subject := async.EventSubject(req.Routing)
	if hp, ok := d.pub.(natsHeaderPublisher); ok && req.EventID != "" {
		return hp.PublishWithHeaders(ctx, subject, req.Record, map[string][]string{natsMsgIDHeader: {req.EventID}})
	}

	return d.pub.Publish(ctx, subject, req.Record)
}
