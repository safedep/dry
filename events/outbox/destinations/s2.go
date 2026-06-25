package destinations

import (
	"context"
	"fmt"
	"sync"

	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/outbox"
	"github.com/safedep/dry/stream"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// S2Destination publishes events to S2 streams. The stream is derived from the
// routing (and the envelope tenant for per-tenant feeds) via stream.StreamFor;
// one StreamWriter is created and cached per stream id. The record bytes are
// appended verbatim (a passthrough serializer over BytesValue).
type S2Destination struct {
	config        stream.S2StreamProviderConfig
	basinResolver stream.S2BasinResolver

	mu      sync.Mutex
	writers map[string]stream.StreamWriter[*wrapperspb.BytesValue]
}

var _ outbox.Destination = (*S2Destination)(nil)

// NewS2 builds an S2 destination. A zero AppendBatchSize defaults to 100.
func NewS2(config stream.S2StreamProviderConfig, basinResolver stream.S2BasinResolver) *S2Destination {
	if config.AppendBatchSize == 0 {
		config.AppendBatchSize = 100
	}
	if basinResolver == nil {
		basinResolver = stream.NewDefaultS2BasinResolver()
	}

	return &S2Destination{
		config:        config,
		basinResolver: basinResolver,
		writers:       make(map[string]stream.StreamWriter[*wrapperspb.BytesValue]),
	}
}

func (d *S2Destination) Name() string { return "s2" }

// Accepts allows all feeds: S2 is eligible for both private and public.
func (d *S2Destination) Accepts(_ events.Routing) bool { return true }

func (d *S2Destination) Publish(ctx context.Context, req outbox.PublishRequest) error {
	target := stream.StreamFor(req.Routing)
	if req.Tenant != "" {
		target = stream.StreamForWithTenant(req.Routing, req.Tenant)
	}

	id, err := target.ID()
	if err != nil {
		return fmt.Errorf("s2 destination: stream id: %w", err)
	}

	writer, err := d.writerFor(id, target)
	if err != nil {
		return err
	}

	// Mirror the ids into headers so consumers can filter without decoding.
	headers := map[string]string{"event_id": req.EventID, "fqn": req.Routing.FQN}
	if req.Subject != "" {
		headers["subject"] = req.Subject
	}

	return writer.AppendOne(ctx, &stream.StreamEntity[*wrapperspb.BytesValue]{
		Record:  wrapperspb.Bytes(req.Record),
		Headers: headers,
	})
}

func (d *S2Destination) writerFor(id string, target stream.Stream) (stream.StreamWriter[*wrapperspb.BytesValue], error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if w, ok := d.writers[id]; ok {
		return w, nil
	}

	w, err := stream.NewS2StreamWriter[*wrapperspb.BytesValue](
		d.config, d.basinResolver, target, bytesSerializer{})
	if err != nil {
		return nil, fmt.Errorf("s2 destination: new writer for %s: %w", id, err)
	}

	d.writers[id] = w
	return w, nil
}

// bytesSerializer appends already-serialized record bytes verbatim. The outbox
// stores the binary-proto <Feed>Event; the S2 record body is exactly those bytes.
type bytesSerializer struct{}

func (bytesSerializer) Serialize(record *wrapperspb.BytesValue) ([]byte, error) {
	return record.GetValue(), nil
}

func (bytesSerializer) Deserialize(data []byte, record *wrapperspb.BytesValue) error {
	record.Value = data
	return nil
}
