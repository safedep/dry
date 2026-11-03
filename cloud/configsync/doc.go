// Package configsync fetches the resolved policy of an endpoint from SafeDep
// Cloud and keeps it in a local cache.
//
// A tool never writes the policy cache outside this package.
//
// Fetcher.Fetch calls PolicySyncService.GetResolvedPolicy and commits a new
// policy only when every check passes:
//   - A NOT_MODIFIED reply moves fetched_at and clears last_error. It
//     changes no policy field. A NOT_MODIFIED reply with another digest is a
//     failed fetch.
//   - A MODIFIED reply must carry "sha256:" and the hex SHA-256 of the
//     received bytes. A revision lower than highest_revision of the tenant
//     is ignored. Then the Validator of the tool runs.
//
// The fetcher never decodes the policy bytes. Only the validator of the tool
// does.
//
// Store owns the cache layout:
//
//	<dir>/state.json        the state, and the commit point
//	<dir>/policy-<hex>.pb   the ResolvedPolicy bytes as received
//
// A commit writes the policy file, then state.json, each to a temp file that
// it syncs and renames. The rename of state.json is the one commit point, so
// a crash between the two renames leaves the previous commit. Readers need no
// lock. Writers that do not share one Store, in one process or in more than
// one, must hold a lock of the tool across Fetch.
//
// WithTenant binds a Store to the tenant of the credentials of the tool.
// NewFetcher requires it. Load then refuses a cache of another tenant.
package configsync
