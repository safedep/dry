package async

import (
	"testing"

	"github.com/safedep/dry/events"
	"github.com/stretchr/testify/assert"
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
