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

// NatsDestination publishes events to NATS on the subject derived from the
// routing (the fully-qualified message name). NATS is intra-platform only:
// public feeds are rejected (events spec §7 eligibility).
type NatsDestination struct {
	pub NatsPublisher
}

var _ outbox.Destination = (*NatsDestination)(nil)

// NewNATS builds a NATS destination over a publisher (e.g. dry/async messaging).
func NewNATS(pub NatsPublisher) *NatsDestination {
	return &NatsDestination{pub: pub}
}

func (d *NatsDestination) Name() string { return "nats" }

func (d *NatsDestination) Publish(ctx context.Context, routing events.Routing, _ string, record []byte) error {
	if routing.IsPublic() {
		return fmt.Errorf("nats destination: public feed %s is not eligible for NATS (S2 only)", routing.FQN)
	}

	return d.pub.Publish(ctx, async.EventSubject(routing), record)
}
