package async

import "github.com/safedep/dry/events"

// EventSubject returns the NATS subject for an event Routing, per the SafeDep
// events convention: the subject is the fully-qualified message name. It uses
// Routing.FullName so a Routing built as a literal (without FQN set) still renders
// a valid subject rather than an empty string.
//
// This is the new events-standard renderer; DomainEventTopicName remains for the
// frozen legacy service-event path.
func EventSubject(r events.Routing) string {
	return r.FullName()
}
