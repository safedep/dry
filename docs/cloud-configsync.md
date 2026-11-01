# cloud/configsync

`cloud/configsync` fetches the resolved policy of an endpoint from SafeDep Cloud and keeps it in a
local cache. pmg uses it now. The SafeDep daemon of M2 uses it with no change. A tool never writes the
policy cache outside this package.

The design is Design 2 and 3 of the spec `docs/specs/2026-10-30-pmg-cloud-policy-design.md` in
control-tower. The API is `PolicySyncService.GetResolvedPolicy` in safedep/api.

## Flow in a tool

```go
client, err := cloud.NewDataPlaneClient("pmg", creds)
if err != nil {
	return err
}
defer client.Close()

store, err := configsync.NewStore(cacheDir) // pmg: <pmg config dir>/cloud-policy
if err != nil {
	return err
}
fetcher, err := configsync.NewFetcher(store, configsync.NewGrpcTransport(client.Connection()))
if err != nil {
	return err
}

// The tool holds its writer lock across Fetch, as pmg does with cloud-sync.lock.
res, err := fetcher.Fetch(ctx, configsync.FetchRequest{
	TenantDomain: tenant,
	Endpoint:     endpoint, // the identity of the check-in
	Labels:       endpointsync.ReportedLabels(),
	ToolName:     "pmg",
	ToolVersion:  version,
	Validate:     validate, // the header check and the projection of the tool
})
```

At start, the tool reads the cache:

```go
loaded, err := store.Load(headerCheck)
if errors.Is(err, configsync.ErrNoCache) {
	// The tool baseline applies.
}
if loaded.FromPrevious {
	// The current file failed. loaded.CurrentErr tells why. Report APPLY_STATE_REJECTED.
}
```

## The fetch

`Fetch` sends the digest of the current cache. It sends an empty digest when the cache is empty, holds
another tenant, or when the current file fails its SHA-256 check, so the server sends the policy again
and the cache repairs itself.

The reply goes through these checks, in order. A failed check writes no policy bytes:

1. `NOT_MODIFIED`: the reply digest must equal the digest that the fetch sent. Then only `fetched_at`
   moves. Another digest, or a reply to an empty digest, is a failed fetch (`ErrDigestMismatch`).
2. `MODIFIED`: the reply digest must be `sha256:` and the hex SHA-256 of the received bytes
   (`ErrDigestMismatch`). The fetcher never computes it from a decoded message.
3. The revision must not be lower than `highest_revision` of the tenant (`ErrLowerRevision`). The same
   revision with a new digest is accepted.
4. The `Validate` hook of the tool must pass (`ErrRejected`). The fetch records the rejection in
   `rejected`.

The fetcher never decodes the bytes. Only the validator of the tool does.

A failed fetch, also a gRPC error, records `last_attempt_at` and `last_error`, keeps the cache, and
returns the error. `status.Code` reads the code of a gRPC error, so a tool can treat `UNAVAILABLE` as
"try later".

A change of tenant resets `highest_revision`, and the new tenant gets no previous file and no
capability sources: a policy of another tenant fails the header check, so it is no fallback. A failed
fetch for a new tenant keeps the cache of the old tenant.

`Validate` is required. `CapabilitySources` is optional. It runs after `Validate` passes, on the same
bytes, and returns the `capability_sources` of the new commit (see below). The tool reads
`Store.State` to compute it.

## The cache layout

```text
<dir>/state.json          the state, and the commit point
<dir>/policy-<hex>.pb     the ResolvedPolicy bytes as received, named by their digest
```

A commit writes the policy file, then `state.json`. It writes each one to a temp file in the folder,
syncs it, and renames it. The rename of `state.json` is the one commit point. A crash between the two
renames leaves the previous commit, and the next commit deletes the policy file that no state names.

After a commit, the store deletes each policy file that `state.json` does not name, and the temp files
of a writer that stopped. It keeps the current file, the previous file, and each file that
`capability_sources` names. It deletes no other file in the folder.

The store creates the folder with mode `0700` and the files with mode `0600`. On Windows it relies on
the ACL of the per-user config folder.

Readers need no lock, because the commit is one rename. `Store` serializes the writers of one process.
Writers in more than one process hold a lock of the tool across `Fetch`.

### `state.json`

```json
{
  "format_version": 1,
  "tenant": "platform-eng.acme.safedep.io",
  "digest": "sha256:9f2c...",
  "revision": 48,
  "previous_digest": "sha256:41ab...",
  "previous_revision": 47,
  "highest_revision": 48,
  "stored_at": "2026-10-30T10:02:11Z",
  "fetched_at": "2026-10-30T10:07:15Z",
  "last_attempt_at": "2026-10-30T10:07:15Z",
  "last_error": "",
  "rejected": {"digest": "sha256:77d0...", "revision": 49, "reason": "tenant mismatch", "at": "2026-10-30T09:40:02Z"},
  "capability_sources": {"package_admission": "sha256:41ab..."}
}
```

| Field | Meaning |
|---|---|
| `format_version` | The format. A reader that finds a higher version reports `ErrNoCache` |
| `tenant` | The tenant domain of the policy files |
| `digest`, `revision` | The current policy file |
| `previous_digest`, `previous_revision` | The file that was current before. `Load` falls back to it |
| `highest_revision` | The highest revision of the tenant that a fetch accepted |
| `stored_at` | The time of the commit of the current file |
| `fetched_at` | The time of the last fetch that ended with the policy of the server in the cache |
| `last_attempt_at`, `last_error` | The last failed fetch, or the last commit, which clears `last_error` |
| `rejected` | The last policy that the validator refused. A commit clears it |
| `capability_sources` | For each capability that does not come from the current file, the digest of the file that it comes from |

The time fields are absent until they are set. `rejected` and `capability_sources` are absent when
they are empty.

`State.LastFetchFailed()` is true when `last_attempt_at` is after `fetched_at`.
`State.LastFetchRejected()` is true when the last failed fetch was a rejection. A `NOT_MODIFIED` reply
moves only `fetched_at`, so it ends a failure without a change to `last_error` or `rejected`.

## `Store.Load`

`Load(validate)` reads `state.json` and the current policy file. It checks that the SHA-256 of the file
equals the digest, then calls `validate`. A tool passes its header check, because only the tool decodes
the policy.

When the current file fails a check, `Load` loads the previous file with the same checks. It sets
`Loaded.FromPrevious` and gives the failure in `Loaded.CurrentErr`. The result is `ErrNoCache` only
when both files fail, when the state names no policy, or when `state.json` is missing, does not parse
or has a higher `format_version`.

When `Load` falls back, `State.CapabilitySources` describes the current file, not the loaded one.
`Store.ReadPolicy(digest)` reads the file of one capability source, with the SHA-256 check.

## Tests

The `Transport` interface has a mockery mock, `MockTransport`. A tool test that needs a cache commits
it through a `Fetcher` with a stub `Transport`, so the test uses the layout of this package and does not
write the files itself.
