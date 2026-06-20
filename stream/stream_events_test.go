package stream

import (
	"testing"
	"time"

	"github.com/safedep/dry/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamFor(t *testing.T) {
	cases := []struct {
		name     string
		routing  events.Routing
		wantNS   string
		wantName string
		wantID   string
	}{
		{
			name:     "public threatintel",
			routing:  events.Routing{Exposure: events.ExposurePublic, Domain: "threatintel", Major: 1, Message: "VerdictsEvent"},
			wantNS:   "public",
			wantName: "threatintel.v1.VerdictsEvent",
			wantID:   "public:threatintel.v1.VerdictsEvent",
		},
		{
			name:     "private packageregistry",
			routing:  events.Routing{Exposure: events.ExposurePrivate, Domain: "packageregistry", Major: 2, Message: "PackageVersionObservationEvent"},
			wantNS:   "private",
			wantName: "packageregistry.v2.PackageVersionObservationEvent",
			wantID:   "private:packageregistry.v2.PackageVersionObservationEvent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := StreamFor(tc.routing)

			assert.Equal(t, tc.wantNS, s.Namespace)
			assert.Equal(t, tc.wantName, s.Name)
			assert.False(t, s.IsMultiTenant, "global stream must not be multi-tenant")
			assert.Empty(t, s.TenantID)

			id, err := s.ID()
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, id)
		})
	}
}

func TestStreamForWithTenant(t *testing.T) {
	r := events.Routing{Exposure: events.ExposurePublic, Domain: "scans", Major: 1, Message: "ResultsEvent"}

	s := StreamForWithTenant(r, "tenant-acme")

	assert.Equal(t, "public", s.Namespace)
	assert.Equal(t, "scans.v1.ResultsEvent", s.Name)
	assert.True(t, s.IsMultiTenant)
	assert.Equal(t, "tenant-acme", s.TenantID)

	id, err := s.ID()
	require.NoError(t, err)
	assert.Equal(t, "tenant-acme:public:scans.v1.ResultsEvent", id)
}

func TestStreamForWithTenant_Isolation(t *testing.T) {
	r := events.Routing{Exposure: events.ExposurePublic, Domain: "scans", Major: 1, Message: "ResultsEvent"}

	idA, err := StreamForWithTenant(r, "tenant-a").ID()
	require.NoError(t, err)
	idB, err := StreamForWithTenant(r, "tenant-b").ID()
	require.NoError(t, err)

	assert.NotEqual(t, idA, idB, "different tenants must not collide")
}

func TestStreamForWithTenant_EmptyTenantIsInvalid(t *testing.T) {
	r := events.Routing{Exposure: events.ExposurePrivate, Domain: "packageregistry", Major: 1, Message: "PackageVersionObservationEvent"}

	// A multi-tenant stream with no tenant must fail ID composition rather than
	// silently fall back to a global id.
	_, err := StreamForWithTenant(r, "").ID()
	require.ErrorIs(t, err, ErrMissingTenantID)
}

func TestStreamAccessRequestFor(t *testing.T) {
	r := events.Routing{Exposure: events.ExposurePublic, Domain: "threatintel", Major: 1, Message: "VerdictsEvent"}

	req := StreamAccessRequestFor(r, StreamAccessRead, time.Hour)

	// Exact-match path: a single Stream, no Scope (so the matcher is exact, not
	// a prefix that could over-grant a sibling feed).
	assert.Nil(t, req.Scope)
	assert.Equal(t, StreamAccessRead, req.Access)
	assert.Equal(t, time.Hour, req.Expiry)
	require.NoError(t, req.Validate())

	id, err := req.Stream.ID()
	require.NoError(t, err)
	assert.Equal(t, "public:threatintel.v1.VerdictsEvent", id)
}

func TestStreamAccessRequestForWithTenant(t *testing.T) {
	r := events.Routing{Exposure: events.ExposurePublic, Domain: "scans", Major: 1, Message: "ResultsEvent"}

	req := StreamAccessRequestForWithTenant(r, "tenant-acme", StreamAccessReadWrite, time.Minute)

	assert.Nil(t, req.Scope)
	assert.True(t, req.Stream.IsMultiTenant)
	require.NoError(t, req.Validate())

	id, err := req.Stream.ID()
	require.NoError(t, err)
	assert.Equal(t, "tenant-acme:public:scans.v1.ResultsEvent", id)
}
