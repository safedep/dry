// Package natsjs is the NATS JetStream transport of SafeDep events. It sets up
// a stream and a durable pull consumer, publishes events with publish-side
// deduplication, and purges the events of one tenant.
//
// A record subject is "<full message name>.<tenant>" (async.TenantEventSubject).
// The inbox reads the stream with events/inbox/sources.NewJetStream and a
// BatchRouter. NATS is intra-platform only, so this package refuses public feeds.
package natsjs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// ErrDrift is the error of EnsureStream and EnsureConsumer when the server has
// a config that differs from the one in code.
var ErrDrift = errors.New("natsjs: config drift")

// The JetStream defaults, set explicitly so that a drift check compares like
// with like.
const (
	maxReplicas            = 5
	defaultDuplicateWindow = 2 * time.Minute
	defaultAckWait         = 30 * time.Second
	defaultMaxAckPending   = 1000
)

// StreamConfig is the stream of one feed family. MaxAge and MaxBytes are
// required: an unbounded file stream fills the disk of the server.
type StreamConfig struct {
	Name     string
	Subjects []string
	MaxAge   time.Duration
	MaxBytes int64
	// Replicas is 1 when zero.
	Replicas int
	// DuplicateWindow is how long the server remembers a Nats-Msg-Id. It is 2m
	// when zero.
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

func (c StreamConfig) native() jetstream.StreamConfig {
	replicas := c.Replicas
	if replicas == 0 {
		replicas = 1
	}
	duplicates := c.DuplicateWindow
	if duplicates == 0 {
		duplicates = min(defaultDuplicateWindow, c.MaxAge)
	}

	return jetstream.StreamConfig{
		Name:              c.Name,
		Subjects:          c.Subjects,
		Storage:           jetstream.FileStorage,
		Retention:         jetstream.LimitsPolicy,
		Discard:           jetstream.DiscardOld,
		MaxAge:            c.MaxAge,
		MaxBytes:          c.MaxBytes,
		MaxMsgs:           -1,
		MaxMsgsPerSubject: -1,
		MaxMsgSize:        -1,
		Replicas:          replicas,
		Duplicates:        duplicates,
	}
}

// EnsureStream creates the stream when it is missing. The stream uses file
// storage and limits retention, and discards the oldest records at a limit.
//
// EnsureStream never changes a stream that exists. An older replica would revert
// a newer config, and a lower MaxAge or MaxBytes discards records. When the
// stream differs from cfg, EnsureStream returns an ErrDrift error that names
// each field. Use UpdateStream for a deliberate change.
func EnsureStream(ctx context.Context, js jetstream.JetStream, cfg StreamConfig) (jetstream.Stream, error) {
	if js == nil {
		return nil, errors.New("natsjs: stream: JetStream is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("natsjs: stream %q: %w", cfg.Name, err)
	}
	want := cfg.native()

	stream, err := js.Stream(ctx, cfg.Name)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		stream, err = js.CreateStream(ctx, want)
	}
	if err != nil {
		return nil, fmt.Errorf("natsjs: stream %q: %w", cfg.Name, err)
	}

	got := stream.CachedInfo().Config
	var d drift
	d.check("subjects", sorted(got.Subjects), sorted(want.Subjects))
	d.check("storage", got.Storage, want.Storage)
	d.check("retention", got.Retention, want.Retention)
	d.check("discard", got.Discard, want.Discard)
	d.check("max age", got.MaxAge, want.MaxAge)
	d.check("max bytes", got.MaxBytes, want.MaxBytes)
	d.check("max msgs", got.MaxMsgs, want.MaxMsgs)
	d.check("max msgs per subject", got.MaxMsgsPerSubject, want.MaxMsgsPerSubject)
	d.check("max msg size", got.MaxMsgSize, want.MaxMsgSize)
	d.check("replicas", got.Replicas, want.Replicas)
	d.check("duplicate window", got.Duplicates, want.Duplicates)
	if err := d.err("stream", cfg.Name); err != nil {
		return nil, err
	}

	return stream, nil
}

// UpdateStream changes an existing stream to cfg. A lower MaxAge or MaxBytes
// discards records at once.
func UpdateStream(ctx context.Context, js jetstream.JetStream, cfg StreamConfig) (jetstream.Stream, error) {
	if js == nil {
		return nil, errors.New("natsjs: stream: JetStream is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("natsjs: stream %q: %w", cfg.Name, err)
	}

	stream, err := js.UpdateStream(ctx, cfg.native())
	if err != nil {
		return nil, fmt.Errorf("natsjs: update stream %q: %w", cfg.Name, err)
	}

	return stream, nil
}

// ConsumerConfig is a durable pull consumer. Writer replicas share it.
//
// The consumer has no delivery limit. The BatchRouter owns the delivery budget.
// It redelivers a record without a limit during an outage, or while the
// dead-letter store fails, so a server limit would drop such a record. An
// existing consumer with a limit is a drift.
type ConsumerConfig struct {
	Durable string
	// FilterSubjects selects the subjects of the stream. None selects the whole
	// stream. More than one needs NATS server 2.10 or later.
	FilterSubjects []string
	// AckWait is 30s when zero.
	AckWait time.Duration
	// MaxAckPending is 1000 when zero.
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
	if c.MaxAckPending < -1 {
		errs = append(errs, errors.New("max ack pending must be -1 or more"))
	}

	return errors.Join(errs...)
}

func (c ConsumerConfig) native() jetstream.ConsumerConfig {
	cfg := jetstream.ConsumerConfig{
		Durable:       c.Durable,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckWait:       c.AckWait,
		MaxDeliver:    -1,
		MaxAckPending: c.MaxAckPending,
	}
	if cfg.AckWait == 0 {
		cfg.AckWait = defaultAckWait
	}
	if cfg.MaxAckPending == 0 {
		cfg.MaxAckPending = defaultMaxAckPending
	}
	// One filter goes in FilterSubject, which NATS servers before 2.10 also read.
	switch len(c.FilterSubjects) {
	case 0:
	case 1:
		cfg.FilterSubject = c.FilterSubjects[0]
	default:
		cfg.FilterSubjects = c.FilterSubjects
	}

	return cfg
}

// EnsureConsumer creates the durable pull consumer when it is missing. The
// consumer acks explicitly and starts at the first record of the stream.
//
// EnsureConsumer never changes a consumer that exists, so that an older replica
// cannot revert a newer config. When the consumer differs from cfg, it returns
// an ErrDrift error that names each field. Use UpdateConsumer for a deliberate
// change.
func EnsureConsumer(ctx context.Context, stream jetstream.Stream, cfg ConsumerConfig) (jetstream.Consumer, error) {
	if stream == nil {
		return nil, errors.New("natsjs: consumer: stream is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("natsjs: consumer %q: %w", cfg.Durable, err)
	}
	want := cfg.native()

	consumer, err := stream.Consumer(ctx, cfg.Durable)
	if errors.Is(err, jetstream.ErrConsumerNotFound) {
		consumer, err = stream.CreateConsumer(ctx, want)
	}
	if err != nil {
		return nil, fmt.Errorf("natsjs: consumer %q: %w", cfg.Durable, err)
	}

	got := consumer.CachedInfo().Config
	var d drift
	d.check("filter subjects", filters(got), filters(want))
	d.check("ack policy", got.AckPolicy, want.AckPolicy)
	d.check("deliver policy", got.DeliverPolicy, want.DeliverPolicy)
	d.check("ack wait", got.AckWait, want.AckWait)
	d.check("max deliver", got.MaxDeliver, want.MaxDeliver)
	d.check("max ack pending", got.MaxAckPending, want.MaxAckPending)
	d.check("headers only", got.HeadersOnly, want.HeadersOnly)
	if err := d.err("consumer", cfg.Durable); err != nil {
		return nil, err
	}

	return consumer, nil
}

// UpdateConsumer changes an existing durable consumer to cfg.
func UpdateConsumer(ctx context.Context, stream jetstream.Stream, cfg ConsumerConfig) (jetstream.Consumer, error) {
	if stream == nil {
		return nil, errors.New("natsjs: consumer: stream is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("natsjs: consumer %q: %w", cfg.Durable, err)
	}

	consumer, err := stream.UpdateConsumer(ctx, cfg.native())
	if err != nil {
		return nil, fmt.Errorf("natsjs: update consumer %q: %w", cfg.Durable, err)
	}

	return consumer, nil
}

type drift []string

func (d *drift) check(field string, got, want any) {
	if !reflect.DeepEqual(got, want) {
		*d = append(*d, fmt.Sprintf("%s is %v, want %v", field, got, want))
	}
}

func (d drift) err(kind, name string) error {
	if len(d) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s %q: %s", ErrDrift, kind, name, strings.Join(d, "; "))
}

func filters(c jetstream.ConsumerConfig) []string {
	if c.FilterSubject != "" {
		return []string{c.FilterSubject}
	}
	return sorted(c.FilterSubjects)
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	if out == nil {
		out = []string{}
	}
	return out
}
