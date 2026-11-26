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

func TestTenantEventSubject(t *testing.T) {
	routing := events.Routing{
		Exposure: events.ExposurePrivate,
		Domain:   "endpoint",
		Major:    1,
		Message:  "ToolEvent",
	}

	cases := []struct {
		name    string
		tenant  string
		want    string
		wantErr bool
	}{
		{name: "plain tenant", tenant: "tenant-01", want: "safedep.events.private.endpoint.v1.ToolEvent.tenant-01"},
		{name: "underscore and capitals", tenant: "T_42", want: "safedep.events.private.endpoint.v1.ToolEvent.T_42"},
		{name: "64 bytes", tenant: strings.Repeat("a", 64), want: "safedep.events.private.endpoint.v1.ToolEvent." + strings.Repeat("a", 64)},
		{name: "65 bytes", tenant: strings.Repeat("a", 65), wantErr: true},
		{name: "empty", tenant: "", wantErr: true},
		{name: "tenant domain", tenant: "ab.cd.safedep.io", wantErr: true},
		{name: "star wildcard", tenant: "*", wantErr: true},
		{name: "full wildcard", tenant: ">", wantErr: true},
		{name: "space", tenant: "a b", wantErr: true},
		{name: "NUL", tenant: "a\x00b", wantErr: true},
		{name: "colon", tenant: "a:b", wantErr: true},
		{name: "non-ASCII letter", tenant: "caf\u00e9", wantErr: true},
		{name: "invalid UTF-8", tenant: "a\xffb", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TenantEventSubject(routing, tc.tenant)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseTenantEventSubject(t *testing.T) {
	const feed = "safedep.events.private.endpoint.v1.ToolEvent"

	cases := []struct {
		name       string
		subject    string
		wantFeed   string
		wantTenant string
		wantErr    bool
	}{
		{name: "tenant subject", subject: feed + ".tenant-01", wantFeed: feed, wantTenant: "tenant-01"},
		{name: "no tenant token", subject: "plain", wantErr: true},
		{name: "bare feed", subject: feed, wantErr: true},
		{name: "empty tenant", subject: feed + ".", wantErr: true},
		{name: "not an event feed", subject: "orders.created.tenant-01", wantErr: true},
		{name: "wildcard tenant", subject: feed + ".*", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFeed, gotTenant, err := ParseTenantEventSubject(tc.subject)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFeed, gotFeed)
			assert.Equal(t, tc.wantTenant, gotTenant)
		})
	}
}

func TestTenantEventSubjects(t *testing.T) {
	routing := events.Routing{FQN: "safedep.events.private.endpoint.v1.ToolEvent"}
	assert.Equal(t, "safedep.events.private.endpoint.v1.ToolEvent.*", TenantEventSubjects(routing))
}
