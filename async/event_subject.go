package async

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/safedep/dry/events"
)

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
// "<full message name>.<tenant>". The tenant is the last token, so a purge by
// subject removes the events of one tenant only.
//
// The tenant must be one printable subject token. A tenant with a dot, a
// wildcard, whitespace or a control character is an error, because it adds a
// token or matches the events of other tenants.
func TenantEventSubject(r events.Routing, tenant string) (string, error) {
	if err := validateSubjectToken(tenant); err != nil {
		return "", fmt.Errorf("async: tenant %q: %w", tenant, err)
	}

	return r.FullName() + "." + tenant, nil
}

// TenantEventSubjects returns the subject that matches every tenant of a feed:
// "<full message name>.*". Use it for stream subjects and consumer filters.
func TenantEventSubjects(r events.Routing) string {
	return r.FullName() + ".*"
}

func validateSubjectToken(token string) error {
	if token == "" {
		return fmt.Errorf("empty subject token")
	}
	if !utf8.ValidString(token) {
		return fmt.Errorf("invalid UTF-8")
	}

	for _, c := range token {
		switch {
		case c == '.' || c == '*' || c == '>':
			return fmt.Errorf("reserved subject character %q", c)
		case unicode.IsSpace(c), unicode.IsControl(c), !unicode.IsPrint(c):
			return fmt.Errorf("non-printable or whitespace character %q", c)
		}
	}

	return nil
}
