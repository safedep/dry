package destinations

import (
	"testing"

	"github.com/safedep/dry/events"
	"github.com/safedep/dry/stream"
	"github.com/stretchr/testify/assert"
)

func TestNatsDestination_Accepts(t *testing.T) {
	d := NewNATS(nil)

	public := events.Routing{Exposure: events.ExposurePublic, Domain: "threatintel", Major: 1, Message: "VerdictsEvent"}
	private := events.Routing{Exposure: events.ExposurePrivate, Domain: "packageregistry", Major: 1, Message: "PackageVersionObservationEvent"}

	assert.False(t, d.Accepts(public), "NATS must reject public feeds")
	assert.True(t, d.Accepts(private), "NATS accepts private feeds")
}

func TestS2Destination_Accepts(t *testing.T) {
	d := NewS2(stream.S2StreamProviderConfig{ApiKey: "test"}, nil)

	assert.True(t, d.Accepts(events.Routing{Exposure: events.ExposurePublic}))
	assert.True(t, d.Accepts(events.Routing{Exposure: events.ExposurePrivate}))
}
