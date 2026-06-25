package events

import (
	"fmt"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	"buf.build/go/protovalidate"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// eventMetaFullName is the descriptor name a feed message's field 1 must carry.
var eventMetaFullName = (&commonv1.EventMeta{}).ProtoReflect().Descriptor().FullName()

// Option mutates the envelope being built. Callers use the With* helpers.
type Option func(*commonv1.EventMeta_builder)

// WithEventID overrides the default ULID event id. Empty values are honored
// (and will fail validation) — pass a real id or omit the option.
func WithEventID(id string) Option {
	return func(b *commonv1.EventMeta_builder) { b.EventId = id }
}

// WithOccurredAt overrides the default occurred_at (time.Now at build).
func WithOccurredAt(t *timestamppb.Timestamp) Option {
	return func(b *commonv1.EventMeta_builder) { b.OccurredAt = t }
}

// WithSubject sets the per-key ordering domain (e.g. a package URN).
func WithSubject(subject string) Option {
	return func(b *commonv1.EventMeta_builder) { b.Subject = &subject }
}

// WithRevision sets the per-subject monotonic revision (projection feeds).
func WithRevision(revision uint64) Option {
	return func(b *commonv1.EventMeta_builder) { b.Revision = &revision }
}

// WithTenant sets the in-band tenancy context.
func WithTenant(tenantID string) Option {
	return func(b *commonv1.EventMeta_builder) {
		b.Tenant = commonv1.TenantContext_builder{TenantId: tenantID}.Build()
	}
}

// WithProducer sets the producing system (e.g. "malysis").
func WithProducer(producer string) Option {
	return func(b *commonv1.EventMeta_builder) { b.Producer = &producer }
}

// WithSchemaVersion sets the observability-only schema version.
func WithSchemaVersion(version string) Option {
	return func(b *commonv1.EventMeta_builder) { b.SchemaVersion = &version }
}

// WithTraceID, WithCorrelationID, WithCausationID set observability/causation ids.
func WithTraceID(id string) Option {
	return func(b *commonv1.EventMeta_builder) { b.TraceId = &id }
}

func WithCorrelationID(id string) Option {
	return func(b *commonv1.EventMeta_builder) { b.CorrelationId = &id }
}

func WithCausationID(id string) Option {
	return func(b *commonv1.EventMeta_builder) { b.CausationId = &id }
}

// NewMeta builds an EventMeta envelope. Defaults event_id to a fresh ULID and
// occurred_at to now; options override. The two required envelope fields are thus
// always set by construction.
func NewMeta(opts ...Option) *commonv1.EventMeta {
	b := commonv1.EventMeta_builder{
		EventId:    ulid.Make().String(),
		OccurredAt: timestamppb.Now(),
	}
	for _, opt := range opts {
		opt(&b)
	}

	return b.Build()
}

// New stamps a freshly-built envelope onto field 1 of a feed message and returns
// the validated event. msg must be a SafeDep event message (EventMeta at field 1,
// per the events convention); otherwise New errors — the write-side dual of
// RoutingFor. The whole message is run through protovalidate, so a returned event
// is structurally valid (envelope + payload constraints) and ready to publish.
//
// New mutates msg's field 1 in place and returns it.
func New[T proto.Message](msg T, opts ...Option) (T, error) {
	refl := msg.ProtoReflect()

	// Enforce the events convention on the message name itself, so New only
	// accepts what RoutingFor can route — a message with EventMeta at field 1 but
	// a non-conforming name would otherwise stamp+publish to an unroutable feed.
	if _, err := RoutingForFullName(string(refl.Descriptor().FullName())); err != nil {
		return msg, err
	}

	fd := refl.Descriptor().Fields().ByNumber(1)
	if fd == nil || fd.Message() == nil || fd.Message().FullName() != eventMetaFullName {
		return msg, fmt.Errorf("events: %s is not a SafeDep event (no %s at field 1)",
			refl.Descriptor().FullName(), eventMetaFullName)
	}

	meta := NewMeta(opts...)
	refl.Set(fd, protoreflect.ValueOfMessage(meta.ProtoReflect()))

	if err := protovalidate.Validate(msg); err != nil {
		return msg, fmt.Errorf("events: validation failed: %w", err)
	}

	return msg, nil
}

// MetaOf extracts the EventMeta envelope from a feed message's field 1. It is the
// read-side dual of New: a message that is not a SafeDep event (no EventMeta at
// field 1) is an error. When the envelope is unset, it returns a zero EventMeta.
func MetaOf(m proto.Message) (*commonv1.EventMeta, error) {
	if m == nil {
		return nil, fmt.Errorf("events: nil message")
	}

	refl := m.ProtoReflect()
	fd := refl.Descriptor().Fields().ByNumber(1)
	if fd == nil || fd.Message() == nil || fd.Message().FullName() != eventMetaFullName {
		return nil, fmt.Errorf("events: %s has no %s at field 1",
			refl.Descriptor().FullName(), eventMetaFullName)
	}

	// Unset envelope: return a zero EventMeta explicitly rather than relying on
	// reflect Get's default-message behavior.
	if !refl.Has(fd) {
		return commonv1.EventMeta_builder{}.Build(), nil
	}

	meta, ok := refl.Get(fd).Message().Interface().(*commonv1.EventMeta)
	if !ok {
		return nil, fmt.Errorf("events: field 1 of %s is not an EventMeta", refl.Descriptor().FullName())
	}

	return meta, nil
}
