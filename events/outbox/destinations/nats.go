// Package destinations provides the built-in outbox.Destination adapters for S2
// and NATS. They are a separate package so the outbox engine stays transport-free
// — a consumer that only needs one transport (or a custom destination) does not
// pull both transport SDKs.
package destinations

import (
	"context"
	"fmt"

	"github.com/safedep/dry/async"
	"github.com/safedep/dry/events/outbox"
)

// NatsPublisher is the minimal NATS capability the destination needs. The
// dry/async messaging client satisfies it.
type NatsPublisher interface {
	Publish(ctx context.Context, topic string, data []byte) error
}

// NatsDestination publishes events to NATS on the subject derived from the
// routing. NATS is intra-platform only, so public feeds are rejected.
type NatsDestination struct {
	pub NatsPublisher
}

var _ outbox.Destination = (*NatsDestination)(nil)

// NewNATS builds a NATS destination over a publisher (e.g. dry/async messaging).
func NewNATS(pub NatsPublisher) *NatsDestination {
	return &NatsDestination{pub: pub}
}

func (d *NatsDestination) Name() string { return "nats" }

func (d *NatsDestination) Publish(ctx context.Context, req outbox.PublishRequest) error {
	if req.Routing.IsPublic() {
		return fmt.Errorf("nats destination: public feed %s is not eligible for NATS (S2 only)", req.Routing.FQN)
	}

	return d.pub.Publish(ctx, async.EventSubject(req.Routing), req.Record)
}
