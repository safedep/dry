package async

import (
	"strings"
	"testing"

	"github.com/safedep/dry/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventSubject(t *testing.T) {
	// FQN set (the RoutingForFullName path) is rendered verbatim.
	withFQN := events.Routing{FQN: "safedep.events.public.threatintel.v1.VerdictsEvent"}
	assert.Equal(t, "safedep.events.public.threatintel.v1.VerdictsEvent", EventSubject(withFQN))

	// A literal Routing without FQN still renders a valid subject (not empty).
	literal := events.Routing{
		Exposure: events.ExposurePublic,
		Domain:   "threatintel",
		Major:    1,
		Message:  "VerdictsEvent",
	}
	assert.Equal(t, "safedep.events.public.threatintel.v1.VerdictsEvent", EventSubject(literal))
}

func TestNewTenantStreamSegmentID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "ULID", id: "01JDQ7Z8X4M2K9V3N5P6R7S8T9"},
		{name: "underscore and dash", id: "T_42-a"},
		{name: "64 bytes", id: strings.Repeat("a", 64)},
		{name: "65 bytes", id: strings.Repeat("a", 65), wantErr: true},
		{name: "empty", id: "", wantErr: true},
		{name: "tenant domain", id: "ab.cd.safedep.io", wantErr: true},
		{name: "star wildcard", id: "*", wantErr: true},
		{name: "full wildcard", id: ">", wantErr: true},
		{name: "space", id: "a b", wantErr: true},
		{name: "NUL", id: "a\x00b", wantErr: true},
		{name: "colon", id: "a:b", wantErr: true},
		{name: "non-ASCII letter", id: "caf\u00e9", wantErr: true},
		{name: "invalid UTF-8", id: "a\xffb", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewTenantStreamSegmentID(tc.id)
			if tc.wantErr {
				assert.Error(t, err)
				assert.True(t, got.IsZero())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, got.String())
		})
	}
}

func TestTenantEventSubject(t *testing.T) {
	routing := events.Routing{
		Exposure: events.ExposurePrivate,
		Domain:   "endpoint",
		Major:    1,
		Message:  "ToolEvent",
	}
	segment, err := NewTenantStreamSegmentID("01JDQ7Z8X4M2K9V3N5P6R7S8T9")
	require.NoError(t, err)

	got, err := TenantEventSubject(routing, segment)
	require.NoError(t, err)
	assert.Equal(t, "safedep.events.private.endpoint.v1.ToolEvent.01JDQ7Z8X4M2K9V3N5P6R7S8T9", got)

	_, err = TenantEventSubject(routing, TenantStreamSegmentID{})
	assert.Error(t, err, "the zero value is refused")
}

func TestParseTenantEventSubject(t *testing.T) {
	const feed = "safedep.events.private.endpoint.v1.ToolEvent"

	cases := []struct {
		name        string
		subject     string
		wantFeed    string
		wantSegment string
		wantErr     bool
	}{
		{name: "segment subject", subject: feed + ".tenant-01", wantFeed: feed, wantSegment: "tenant-01"},
		{name: "no segment token", subject: "plain", wantErr: true},
		{name: "bare feed", subject: feed, wantErr: true},
		{name: "empty segment", subject: feed + ".", wantErr: true},
		{name: "not an event feed", subject: "orders.created.tenant-01", wantErr: true},
		{name: "wildcard segment", subject: feed + ".*", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFeed, gotSegment, err := ParseTenantEventSubject(tc.subject)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFeed, gotFeed)
			assert.Equal(t, tc.wantSegment, gotSegment.String())
		})
	}
}

func TestTenantEventSubjects(t *testing.T) {
	routing := events.Routing{FQN: "safedep.events.private.endpoint.v1.ToolEvent"}
	assert.Equal(t, "safedep.events.private.endpoint.v1.ToolEvent.*", TenantEventSubjects(routing))
}
