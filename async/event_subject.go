package async

import (
	"fmt"
	"strings"

	"github.com/safedep/dry/events"
)

// maxTenantTokenLen bounds the tenant token of a subject.
const maxTenantTokenLen = 64

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
// The tenant is a canonical tenant id of 1 to 64 bytes from [A-Za-z0-9_-]. A
// dotted tenant domain such as "ab.cd.safedep.io" is an error, because each dot
// adds a token.
func TenantEventSubject(r events.Routing, tenant string) (string, error) {
	if err := validateTenantToken(tenant); err != nil {
		return "", fmt.Errorf("async: tenant %q: %w", tenant, err)
	}

	return r.FullName() + "." + tenant, nil
}

// TenantEventSubjects returns the subject that matches every tenant of a feed:
// "<full message name>.*". Use it for stream subjects and consumer filters.
func TenantEventSubjects(r events.Routing) string {
	return r.FullName() + ".*"
}

// ParseTenantEventSubject splits a subject of TenantEventSubject into the full
// message name of the feed and the tenant.
func ParseTenantEventSubject(subject string) (feed, tenant string, err error) {
	i := strings.LastIndexByte(subject, '.')
	if i < 0 {
		return "", "", fmt.Errorf("async: subject %q has no tenant token", subject)
	}

	feed, tenant = subject[:i], subject[i+1:]
	if err := validateTenantToken(tenant); err != nil {
		return "", "", fmt.Errorf("async: subject %q: tenant: %w", subject, err)
	}
	if _, err := events.RoutingForFullName(feed); err != nil {
		return "", "", fmt.Errorf("async: subject %q: %w", subject, err)
	}

	return feed, tenant, nil
}

func validateTenantToken(token string) error {
	if token == "" || len(token) > maxTenantTokenLen {
		return fmt.Errorf("length must be from 1 to %d bytes", maxTenantTokenLen)
	}

	for i := 0; i < len(token); i++ {
		c := token[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return fmt.Errorf("character %q is not in [A-Za-z0-9_-]", c)
		}
	}

	return nil
}
