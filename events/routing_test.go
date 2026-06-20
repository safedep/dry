package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRoutingForFullName(t *testing.T) {
	cases := []struct {
		name    string
		fqn     string
		want    Routing
		wantErr bool
	}{
		{
			name: "private event",
			fqn:  "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent",
			want: Routing{
				Exposure: ExposurePrivate,
				Domain:   "packageregistry",
				Major:    1,
				Message:  "PackageVersionObservationEvent",
				FQN:      "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent",
			},
		},
		{
			name: "public event, multi-digit major",
			fqn:  "safedep.events.public.threatintel.v12.VerdictsEvent",
			want: Routing{
				Exposure: ExposurePublic,
				Domain:   "threatintel",
				Major:    12,
				Message:  "VerdictsEvent",
				FQN:      "safedep.events.public.threatintel.v12.VerdictsEvent",
			},
		},
		{name: "not an events package", fqn: "safedep.messages.package.v1.PackageVersion", wantErr: true},
		{name: "wrong root", fqn: "google.protobuf.Timestamp", wantErr: true},
		{name: "invalid exposure", fqn: "safedep.events.protected.threatintel.v1.VerdictsEvent", wantErr: true},
		{name: "missing version prefix", fqn: "safedep.events.public.threatintel.1.VerdictsEvent", wantErr: true},
		{name: "non-numeric version", fqn: "safedep.events.public.threatintel.vX.VerdictsEvent", wantErr: true},
		{name: "too few segments", fqn: "safedep.events.public.threatintel.v1", wantErr: true},
		{name: "nested message (too many segments)", fqn: "safedep.events.public.threatintel.v1.VerdictsEvent.Inner", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RoutingForFullName(tc.fqn)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRoutingMethods(t *testing.T) {
	r := Routing{Exposure: ExposurePublic, Domain: "threatintel", Major: 1, Message: "VerdictsEvent"}
	assert.Equal(t, "threatintel.v1.VerdictsEvent", r.Name())
	assert.True(t, r.IsPublic())
	assert.False(t, Routing{Exposure: ExposurePrivate}.IsPublic())
}

func TestRoutingFullName(t *testing.T) {
	// FQN is returned verbatim when set (the RoutingForFullName path).
	set := Routing{FQN: "safedep.events.public.threatintel.v1.VerdictsEvent"}
	assert.Equal(t, "safedep.events.public.threatintel.v1.VerdictsEvent", set.FullName())

	// A literal Routing without FQN recomputes the fully-qualified name from parts.
	literal := Routing{Exposure: ExposurePublic, Domain: "threatintel", Major: 1, Message: "VerdictsEvent"}
	assert.Equal(t, "safedep.events.public.threatintel.v1.VerdictsEvent", literal.FullName())
}

func TestRoutingFor_Message(t *testing.T) {
	// A real generated message that is NOT a SafeDep event must error (the
	// enforcement seam), exercising the descriptor extraction path.
	_, err := RoutingFor(timestamppb.New(timestamppb.Now().AsTime()))
	require.Error(t, err)

	_, err = RoutingFor(nil)
	require.Error(t, err)
}
