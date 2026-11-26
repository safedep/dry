package natsjs

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/async"
	"github.com/safedep/dry/events"
	"google.golang.org/protobuf/proto"
)

// Publisher publishes SafeDep events to JetStream and waits for the server to
// store each one.
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher builds a Publisher over a JetStream context.
func NewPublisher(js jetstream.JetStream) (*Publisher, error) {
	if js == nil {
		return nil, errors.New("natsjs: publisher: JetStream is required")
	}
	return &Publisher{js: js}, nil
}

// PublishResult is the acknowledgement of the server for one event.
type PublishResult struct {
	Stream   string
	Sequence uint64
	// Duplicate is true when the stream already had a record with the same
	// message id in its duplicate window, and did not store this one.
	Duplicate bool
}

type publishConfig struct {
	msgID string
}

// PublishOption configures one Publish call.
type PublishOption func(*publishConfig)

// WithMsgID sets the Nats-Msg-Id of the record. The default is the event_id of
// the envelope.
func WithMsgID(id string) PublishOption {
	return func(c *publishConfig) { c.msgID = id }
}

// Publish sends event as binary proto on "<full message name>.<tenant>". The
// message type gives the feed and the envelope tenant_id gives the tenant, so a
// consumer can route by subject. Publish refuses an event without a tenant, and
// a public feed.
//
// The Nats-Msg-Id header makes the stream drop a retry of the same event within
// its duplicate window. A duplicate is a success and PublishResult.Duplicate is
// true.
func (p *Publisher) Publish(ctx context.Context, event proto.Message, opts ...PublishOption) (PublishResult, error) {
	routing, err := events.RoutingFor(event)
	if err != nil {
		return PublishResult{}, fmt.Errorf("natsjs: publish: %w", err)
	}
	if routing.IsPublic() {
		return PublishResult{}, fmt.Errorf("natsjs: publish: public feed %s is not eligible for NATS", routing.FullName())
	}

	meta, err := events.MetaOf(event)
	if err != nil {
		return PublishResult{}, fmt.Errorf("natsjs: publish: %w", err)
	}

	subject, err := async.TenantEventSubject(routing, meta.GetTenant().GetTenantId())
	if err != nil {
		return PublishResult{}, fmt.Errorf("natsjs: publish: envelope tenant: %w", err)
	}

	cfg := publishConfig{msgID: meta.GetEventId()}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.msgID == "" {
		return PublishResult{}, errors.New("natsjs: publish: the event has no event_id and no message id is set")
	}

	data, err := proto.Marshal(event)
	if err != nil {
		return PublishResult{}, fmt.Errorf("natsjs: publish: marshal: %w", err)
	}

	ack, err := p.js.PublishMsg(ctx, &nats.Msg{Subject: subject, Data: data}, jetstream.WithMsgID(cfg.msgID))
	if err != nil {
		return PublishResult{}, fmt.Errorf("natsjs: publish %s: %w", subject, err)
	}

	return PublishResult{Stream: ack.Stream, Sequence: ack.Sequence, Duplicate: ack.Duplicate}, nil
}

// PurgeTenant removes every record of one tenant of a feed from stream.
func PurgeTenant(ctx context.Context, stream jetstream.Stream, routing events.Routing, tenant string) error {
	if stream == nil {
		return errors.New("natsjs: purge: stream is required")
	}

	subject, err := async.TenantEventSubject(routing, tenant)
	if err != nil {
		return fmt.Errorf("natsjs: purge: %w", err)
	}

	if err := stream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
		return fmt.Errorf("natsjs: purge %s: %w", subject, err)
	}

	return nil
}
