// Package jetstream is the NATS JetStream transport of SafeDep events. It sets
// up a stream and a durable pull consumer, publishes events with publish-side
// deduplication, and purges the events of one tenant.
//
// A record subject is "<full message name>.<tenant>" (async.TenantEventSubject).
// The inbox reads the stream with events/inbox/sources.NewJetStream and a
// BatchRouter. NATS is intra-platform only, so this package refuses public feeds.
package jetstream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// maxReplicas is the largest replica count that JetStream accepts.
const maxReplicas = 5

// StreamConfig is the stream of one feed family. MaxAge and MaxBytes are
// required: an unbounded file stream fills the disk of the server.
type StreamConfig struct {
	Name     string
	Subjects []string
	MaxAge   time.Duration
	MaxBytes int64
	// Replicas is 1 when zero.
	Replicas int
	// DuplicateWindow is how long the server remembers a Nats-Msg-Id. The server
	// default (2m) applies when zero.
	DuplicateWindow time.Duration
}

func (c StreamConfig) validate() error {
	var errs []error
	if c.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if len(c.Subjects) == 0 {
		errs = append(errs, errors.New("at least one subject is required"))
	}
	for _, s := range c.Subjects {
		if s == "" {
			errs = append(errs, errors.New("empty subject"))
		}
	}
	if c.MaxAge <= 0 {
		errs = append(errs, errors.New("max age must be positive"))
	}
	if c.MaxBytes <= 0 {
		errs = append(errs, errors.New("max bytes must be positive"))
	}
	if c.Replicas < 0 || c.Replicas > maxReplicas {
		errs = append(errs, fmt.Errorf("replicas must be from 0 to %d", maxReplicas))
	}
	if c.DuplicateWindow < 0 {
		errs = append(errs, errors.New("duplicate window must not be negative"))
	}
	if c.MaxAge > 0 && c.DuplicateWindow > c.MaxAge {
		errs = append(errs, errors.New("duplicate window must not exceed max age"))
	}

	return errors.Join(errs...)
}

// EnsureStream creates the stream, or updates it to cfg. The stream uses file
// storage and limits retention, and discards the oldest records at a limit.
func EnsureStream(ctx context.Context, js jetstream.JetStream, cfg StreamConfig) (jetstream.Stream, error) {
	if js == nil {
		return nil, errors.New("jetstream: stream: JetStream is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("jetstream: stream %q: %w", cfg.Name, err)
	}

	replicas := cfg.Replicas
	if replicas == 0 {
		replicas = 1
	}

	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       cfg.Name,
		Subjects:   cfg.Subjects,
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		Discard:    jetstream.DiscardOld,
		MaxAge:     cfg.MaxAge,
		MaxBytes:   cfg.MaxBytes,
		Replicas:   replicas,
		Duplicates: cfg.DuplicateWindow,
	})
	if err != nil {
		return nil, fmt.Errorf("jetstream: stream %q: %w", cfg.Name, err)
	}

	return stream, nil
}

// ConsumerConfig is a durable pull consumer. Writer replicas share it.
type ConsumerConfig struct {
	Durable string
	// FilterSubjects selects the subjects of the stream. None selects the whole
	// stream. More than one needs NATS server 2.10 or later.
	FilterSubjects []string
	// AckWait is the server default (30s) when zero.
	AckWait time.Duration
	// MaxDeliver is unlimited when zero or -1. Set it to at least the delivery
	// budget of the router (inbox.WithBatchRetry). Otherwise the server stops a
	// record before the router can dead-letter it.
	MaxDeliver int
	// MaxAckPending is the server default (1000) when zero.
	MaxAckPending int
}

func (c ConsumerConfig) validate() error {
	var errs []error
	if c.Durable == "" {
		errs = append(errs, errors.New("durable name is required"))
	}
	for _, s := range c.FilterSubjects {
		if s == "" {
			errs = append(errs, errors.New("empty filter subject"))
		}
	}
	if c.AckWait < 0 {
		errs = append(errs, errors.New("ack wait must not be negative"))
	}
	if c.MaxDeliver < -1 {
		errs = append(errs, errors.New("max deliver must be -1 or more"))
	}
	if c.MaxAckPending < -1 {
		errs = append(errs, errors.New("max ack pending must be -1 or more"))
	}

	return errors.Join(errs...)
}

// EnsureConsumer creates the durable pull consumer, or updates it to cfg. The
// consumer acks explicitly and starts at the first record of the stream.
func EnsureConsumer(ctx context.Context, stream jetstream.Stream, cfg ConsumerConfig) (jetstream.Consumer, error) {
	if stream == nil {
		return nil, errors.New("jetstream: consumer: stream is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("jetstream: consumer %q: %w", cfg.Durable, err)
	}

	jcfg := jetstream.ConsumerConfig{
		Durable:       cfg.Durable,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
		MaxAckPending: cfg.MaxAckPending,
	}
	// One filter goes in FilterSubject, which NATS servers before 2.10 also read.
	switch len(cfg.FilterSubjects) {
	case 0:
	case 1:
		jcfg.FilterSubject = cfg.FilterSubjects[0]
	default:
		jcfg.FilterSubjects = cfg.FilterSubjects
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, jcfg)
	if err != nil {
		return nil, fmt.Errorf("jetstream: consumer %q: %w", cfg.Durable, err)
	}

	return consumer, nil
}
