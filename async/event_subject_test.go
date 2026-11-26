package async

import (
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
		{name: "tenant with underscore and digits", tenant: "t_42", want: "safedep.events.private.endpoint.v1.ToolEvent.t_42"},
		{name: "empty", tenant: "", wantErr: true},
		{name: "dot adds a token", tenant: "ab.cd", wantErr: true},
		{name: "star wildcard", tenant: "*", wantErr: true},
		{name: "full wildcard", tenant: ">", wantErr: true},
		{name: "embedded wildcard", tenant: "a*b", wantErr: true},
		{name: "space", tenant: "a b", wantErr: true},
		{name: "tab", tenant: "a\tb", wantErr: true},
		{name: "newline", tenant: "a\nb", wantErr: true},
		{name: "NUL", tenant: "a\x00b", wantErr: true},
		{name: "DEL", tenant: "a\x7fb", wantErr: true},
		{name: "non-breaking space", tenant: "a\u00a0b", wantErr: true},
		{name: "zero-width space", tenant: "a\u200bb", wantErr: true},
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

func TestTenantEventSubjects(t *testing.T) {
	routing := events.Routing{FQN: "safedep.events.private.endpoint.v1.ToolEvent"}
	assert.Equal(t, "safedep.events.private.endpoint.v1.ToolEvent.*", TenantEventSubjects(routing))
}
