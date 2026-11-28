package async

import (
	"errors"
	"fmt"
	"strings"

	"github.com/safedep/dry/events"
)

// maxSegmentLen bounds a tenant stream segment id.
const maxSegmentLen = 64

// TenantStreamSegmentID names the stream segment of one tenant: the last token
// of a JetStream subject. It is 1 to 64 bytes from [A-Za-z0-9_-], so it never
// adds a token or a wildcard. The producer picks a stable value, because a purge
// or a replay by segment must find every record of the tenant. The zero value is
// not valid.
type TenantStreamSegmentID struct {
	id string
}

// NewTenantStreamSegmentID checks id and returns it as a segment id.
func NewTenantStreamSegmentID(id string) (TenantStreamSegmentID, error) {
	if err := validateSegment(id); err != nil {
		return TenantStreamSegmentID{}, fmt.Errorf("async: tenant stream segment id %q: %w", id, err)
	}
	return TenantStreamSegmentID{id: id}, nil
}

// String returns the segment id as the subject token.
func (s TenantStreamSegmentID) String() string { return s.id }

// IsZero reports the zero value.
func (s TenantStreamSegmentID) IsZero() bool { return s.id == "" }

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

// TenantEventSubject returns the JetStream subject of one tenant of a feed:
// "<full message name>.<segment>". The segment is the last token, so a purge by
// subject removes the events of one tenant only.
func TenantEventSubject(r events.Routing, segment TenantStreamSegmentID) (string, error) {
	if segment.IsZero() {
		return "", errors.New("async: tenant event subject: the tenant stream segment id is empty")
	}

	return r.FullName() + "." + segment.String(), nil
}

// TenantEventSubjects returns the subject that matches every tenant of a feed:
// "<full message name>.*". Use it for stream subjects and consumer filters.
func TenantEventSubjects(r events.Routing) string {
	return r.FullName() + ".*"
}

// ParseTenantEventSubject splits a subject of TenantEventSubject into the full
// message name of the feed and the tenant stream segment id.
func ParseTenantEventSubject(subject string) (string, TenantStreamSegmentID, error) {
	i := strings.LastIndexByte(subject, '.')
	if i < 0 {
		return "", TenantStreamSegmentID{}, fmt.Errorf("async: subject %q has no segment token", subject)
	}

	feed := subject[:i]
	segment, err := NewTenantStreamSegmentID(subject[i+1:])
	if err != nil {
		return "", TenantStreamSegmentID{}, fmt.Errorf("async: subject %q: %w", subject, err)
	}
	if _, err := events.RoutingForFullName(feed); err != nil {
		return "", TenantStreamSegmentID{}, fmt.Errorf("async: subject %q: %w", subject, err)
	}

	return feed, segment, nil
}

func validateSegment(id string) error {
	if id == "" || len(id) > maxSegmentLen {
		return fmt.Errorf("length must be from 1 to %d bytes", maxSegmentLen)
	}

	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return fmt.Errorf("character %q is not in [A-Za-z0-9_-]", c)
		}
	}

	return nil
}
