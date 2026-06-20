package async

import "github.com/safedep/dry/events"

// EventSubject returns the NATS subject for an event Routing, per the SafeDep
// events convention: the subject is the fully-qualified message name verbatim.
//
// This is the new events-standard renderer; DomainEventTopicName remains for the
// frozen legacy service-event path.
func EventSubject(r events.Routing) string {
	return r.FQN
}
