package stream

import (
	"time"

	"github.com/safedep/dry/events"
)

// StreamFor renders the S2 stream identity for a global (non-tenant-scoped) event
// Routing. The namespace is the event's exposure and the name is the Routing Name
// ("<domain>.v<major>.<Message>").
func StreamFor(r events.Routing) Stream {
	return Stream{
		Namespace: string(r.Exposure),
		Name:      r.Name(),
	}
}

// StreamForWithTenant renders the S2 stream identity for a tenant-scoped event
// Routing. The returned Stream is multi-tenant, so Stream.ID prepends the tenant
// segment for per-tenant isolation.
func StreamForWithTenant(r events.Routing, tenant string) Stream {
	return StreamFor(r).WithTenant(tenant)
}

// StreamAccessRequestFor builds an access request scoped to exactly one event
// feed. It uses the single-Stream (exact-match) path deliberately: a scope-based
// NamePrefix would be a substring prefix and could over-grant a sibling feed
// whose name starts with this one (e.g. "...VerdictsEvent" matching
// "...VerdictsEventArchive"). For a whole-domain or all-exposure grant, construct
// a StreamScope explicitly (see StreamScope and s2ScopePrefix rules).
func StreamAccessRequestFor(r events.Routing, access StreamAccessRole, expiry time.Duration) StreamAccessRequest {
	return StreamAccessRequest{
		Stream: StreamFor(r),
		Access: access,
		Expiry: expiry,
	}
}

// StreamAccessRequestForWithTenant is StreamAccessRequestFor for a tenant-scoped
// feed: an exact-match grant on the per-tenant stream id ("<tenant>:<exposure>:<name>").
func StreamAccessRequestForWithTenant(r events.Routing, tenant string, access StreamAccessRole, expiry time.Duration) StreamAccessRequest {
	return StreamAccessRequest{
		Stream: StreamForWithTenant(r, tenant),
		Access: access,
		Expiry: expiry,
	}
}
