// Package events provides the first-class, transport-neutral identity of a
// SafeDep platform event — its Routing — derived from the fully-qualified
// protobuf message name. It is the single source of truth for *where* an event
// is published. Transport-native addresses are rendered from a Routing at the
// edges (e.g. stream.StreamFor for S2, async.EventSubject for NATS), so this
// package stays transport-free.
//
// See the SafeDep events convention (api repo EVENTS.md): an event message is
//
//	safedep.events.<exposure>.<domain>.v<major>.<Message>
package events

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
)

// Exposure is the audience class of an event, encoded as the third package
// segment (safedep.events.<exposure>.<domain>.v<major>).
type Exposure string

const (
	// ExposurePrivate events stay within the SafeDep bounded context; never
	// customer-grantable. The token is "private" (not "internal") because a proto
	// package segment named "internal" generates a Go internal/ directory that
	// application code cannot import.
	ExposurePrivate Exposure = "private"
	// ExposurePublic events cross to customers; the only customer-exposable class.
	ExposurePublic Exposure = "public"
)

// Routing is the transport-neutral identity of an event feed.
type Routing struct {
	Exposure Exposure // internal | public
	Domain   string   // e.g. "packageregistry"
	Major    uint32   // e.g. 1
	Message  string   // e.g. "PackageVersionObservationEvent"
	FQN      string   // safedep.events.<exposure>.<domain>.v<major>.<Message>
}

// IsPublic reports whether the event is customer-exposable.
func (r Routing) IsPublic() bool { return r.Exposure == ExposurePublic }

// Name is the FQN suffix after the exposure segment — "<domain>.v<major>.<Message>".
// It is the S2 stream Name (the namespace carries the exposure).
func (r Routing) Name() string {
	return fmt.Sprintf("%s.v%d.%s", r.Domain, r.Major, r.Message)
}

const (
	eventsRoot     = "safedep.events."
	fqnSegmentsLen = 6 // safedep . events . <exposure> . <domain> . v<major> . <Message>
)

// RoutingFor derives Routing from a generated protobuf message.
func RoutingFor(m proto.Message) (Routing, error) {
	if m == nil {
		return Routing{}, fmt.Errorf("events: nil message")
	}

	return RoutingForFullName(string(m.ProtoReflect().Descriptor().FullName()))
}

// RoutingForFullName derives Routing from a fully-qualified protobuf message
// name. It validates the SafeDep events convention and is the enforcement seam:
// a name that is not a conforming event message is an error, never a guess.
func RoutingForFullName(fqn string) (Routing, error) {
	parts := strings.Split(fqn, ".")
	if len(parts) != fqnSegmentsLen || parts[0] != "safedep" || parts[1] != "events" {
		return Routing{}, fmt.Errorf(
			"events: %q is not a SafeDep event (want safedep.events.<exposure>.<domain>.v<major>.<Message>)", fqn)
	}

	exposure := Exposure(parts[2])
	if exposure != ExposurePrivate && exposure != ExposurePublic {
		return Routing{}, fmt.Errorf("events: %q has invalid exposure %q (want private|public)", fqn, parts[2])
	}

	domain := parts[3]
	if domain == "" {
		return Routing{}, fmt.Errorf("events: %q has an empty domain", fqn)
	}

	major, err := parseMajor(parts[4])
	if err != nil {
		return Routing{}, fmt.Errorf("events: %q has an invalid version %q (want v<major>)", fqn, parts[4])
	}

	message := parts[5]
	if message == "" {
		return Routing{}, fmt.Errorf("events: %q has an empty message", fqn)
	}

	return Routing{
		Exposure: exposure,
		Domain:   domain,
		Major:    major,
		Message:  message,
		FQN:      fqn,
	}, nil
}

func parseMajor(seg string) (uint32, error) {
	if len(seg) < 2 || seg[0] != 'v' {
		return 0, fmt.Errorf("missing v prefix")
	}

	n, err := strconv.ParseUint(seg[1:], 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(n), nil
}
