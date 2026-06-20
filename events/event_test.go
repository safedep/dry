package events

import (
	"testing"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func validObservation() *pkgregv1.PackageVersionObservationEvent {
	return pkgregv1.PackageVersionObservationEvent_builder{
		PackageVersion: packagev1.PackageVersion_builder{
			Package: packagev1.Package_builder{
				Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
				Name:      "test-package",
			}.Build(),
			Version: "1.0.0",
		}.Build(),
		Kind: pkgregv1.PackageVersionObservationEvent_KIND_PUBLISHED,
	}.Build()
}

func TestNewMeta_Defaults(t *testing.T) {
	m := NewMeta()
	assert.NotEmpty(t, m.GetEventId(), "event_id defaults to a ULID")
	assert.NotNil(t, m.GetOccurredAt(), "occurred_at defaults to now")
	assert.Empty(t, m.GetSubject())
	assert.Nil(t, m.GetTenant())
}

func TestNewMeta_Options(t *testing.T) {
	at := timestamppb.New(time.Unix(1700000000, 0))
	m := NewMeta(
		WithEventID("01EVENTIDULID"), WithOccurredAt(at), WithSubject("pkg:npm/foo"),
		WithRevision(7), WithTenant("tenant-acme"), WithProducer("malysis"),
		WithSchemaVersion("1"), WithTraceID("trace-1"), WithCorrelationID("corr-1"), WithCausationID("cause-1"),
	)
	assert.Equal(t, "01EVENTIDULID", m.GetEventId())
	assert.Equal(t, at, m.GetOccurredAt())
	assert.Equal(t, "pkg:npm/foo", m.GetSubject())
	assert.Equal(t, uint64(7), m.GetRevision())
	assert.Equal(t, "tenant-acme", m.GetTenant().GetTenantId())
	assert.Equal(t, "malysis", m.GetProducer())
	assert.Equal(t, "1", m.GetSchemaVersion())
	assert.Equal(t, "trace-1", m.GetTraceId())
	assert.Equal(t, "corr-1", m.GetCorrelationId())
	assert.Equal(t, "cause-1", m.GetCausationId())
}

func TestNewMeta_EventIDOverrideAndUnique(t *testing.T) {
	assert.Equal(t, "custom-id", NewMeta(WithEventID("custom-id")).GetEventId())
	assert.NotEqual(t, NewMeta().GetEventId(), NewMeta().GetEventId())
}

func TestNew_StampsEnvelopeAndValidates(t *testing.T) {
	obs, err := New(validObservation(), WithSubject("pkg:npm/test-package"), WithProducer("malysis"))
	require.NoError(t, err)

	// Envelope stamped at field 1 with defaults + options.
	assert.NotEmpty(t, obs.GetMeta().GetEventId())
	assert.NotNil(t, obs.GetMeta().GetOccurredAt())
	assert.Equal(t, "pkg:npm/test-package", obs.GetMeta().GetSubject())
	assert.Equal(t, "malysis", obs.GetMeta().GetProducer())
	// Payload untouched.
	assert.Equal(t, "test-package", obs.GetPackageVersion().GetPackage().GetName())
}

func TestNew_EventIDOption(t *testing.T) {
	obs, err := New(validObservation(), WithEventID("01CUSTOMEVENTID"))
	require.NoError(t, err)
	assert.Equal(t, "01CUSTOMEVENTID", obs.GetMeta().GetEventId())
}

func TestNew_ValidationFails(t *testing.T) {
	// Missing the required package_version payload field — protovalidate must
	// reject it even though the envelope is well-formed.
	incomplete := pkgregv1.PackageVersionObservationEvent_builder{
		Kind: pkgregv1.PackageVersionObservationEvent_KIND_PUBLISHED,
	}.Build()

	_, err := New(incomplete)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestNew_NotAnEvent(t *testing.T) {
	// A message whose field 1 is not EventMeta is rejected (enforcement seam).
	_, err := New(packagev1.PackageVersion_builder{Version: "1.0.0"}.Build())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a SafeDep event")
}
