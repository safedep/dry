# cloud: workload identity

A SafeDep tool in CI can call SafeDep Cloud with no API key. The tool trades a token that the CI
platform signs for the job, such as a GitHub Actions OIDC ID token, for a short-lived workload
credential. The API is `WorkloadIdentityService` in safedep/api. The design is
`docs/specs/2026-11-07-workload-identity-design.md` in safedep/control-tower.

## Which credentials the default resolver selects

`cloud.NewDefaultCredentialResolver(cloud.CredentialTypeAPIKey)` reads the environment in this order:

1. `SAFEDEP_API_KEY` with `SAFEDEP_TENANT_ID`: the API key. An explicit API key always wins.
2. `SAFEDEP_TENANT_ID` with a workload identity in the environment, and `SAFEDEP_API_KEY` not set:
   the exchange. The credentials have the source `CredentialSourceWorkloadIdentity`.
3. `SAFEDEP_TENANT_ID` with a workload identity, and `SAFEDEP_API_KEY` set to an empty value:
   `ErrIncompleteCredentials`. A workflow that maps a missing secret to `SAFEDEP_API_KEY` sets it to
   empty, and the resolver does not hide that with an exchange.
4. `SAFEDEP_TENANT_ID` with no workload identity: `ErrIncompleteCredentials`. The message tells the
   user to give the job `permissions: id-token: write`, or to set `SAFEDEP_API_KEY`. When only one of
   the two GitHub variables is set, the message names the missing one.
5. Neither variable: the keychain profile, as before.

A GitHub Actions job has a workload identity when GitHub sets `ACTIONS_ID_TOKEN_REQUEST_URL` and
`ACTIONS_ID_TOKEN_REQUEST_TOKEN`. GitHub sets both for a job with `id-token: write`.

`Resolve` makes no network call. The first exchange runs on the first RPC. Each `Resolve` of one
resolver returns the same workload identity credentials.

`NewEnvCredentialResolver` does not select a workload identity. It reads only `SAFEDEP_API_KEY` and
`SAFEDEP_TENANT_ID`.

## Use in a tool

```go
creds, err := resolver.Resolve()
if err != nil {
	return err
}

client, err := cloud.NewDataPlaneClient("vet", creds)
if err != nil {
	return err
}
defer client.Close()

// At the end of the job, revoke the workload credential.
if w, ok := creds.Workload(); ok {
	defer func() {
		if err := w.Revoke(context.Background()); err != nil {
			log.Warnf("revoke the workload credential: %v", err)
		}
	}()
}
```

`NewDataPlaneClient` sends the workload credential as a per-RPC credential that refreshes. A tool that
dials its own connection passes `creds.Workload()` to `grpc.WithPerRPCCredentials`. Workload identity
credentials hold no fixed API key: `GetAPIKey` returns `ErrWorkloadCredential`, which wraps
`ErrInvalidCredentialType`, and makes no network call.

## Rules

- **A new ID token for each exchange.** The server accepts a token one time only, and a GitHub
  Actions ID token lives 5 minutes. `WorkloadExchangeClient.Exchange` gets the token from the source
  right before it calls `ExchangeToken`, and keeps no token.
- **Refresh before expiry.** `RefreshingWorkloadCredential` exchanges again 5 minutes before the
  credential expires, or at half its life when it lives less than 10 minutes.
- **Clock skew.** The server issues a credential for at least a minute. When the server expiry is
  less than 30 seconds after the local clock, the local clock is ahead of the server. The holder then
  logs a warning and uses the credential for 30 seconds by the local clock. Without this, every RPC
  would run an exchange.
- **One exchange at a time, outside the lock.** An RPC that holds a valid credential never waits for
  an exchange. An RPC with no valid credential waits for the exchange in flight, or for its own
  context.
- **A failed exchange holds for 30 seconds.** In that time an RPC with no valid credential gets the
  same error, and no new exchange starts. An RPC with a valid credential keeps it until it expires. So
  a job that can no longer get an ID token works to the end of the credential life.
- **Revoke at the end.** `Revoke` revokes the current credential, and the credential then refuses to
  exchange again. Revoke waits for an exchange in flight, or for its context. A failed `Revoke` keeps
  the credential, so a second call tries again. A credential that a refresh replaced expires within
  the refresh margin. A `Resolve` after `Revoke` returns new credentials, so later work in the same
  process exchanges again.
- **The exchange is keyless.** The exchange connection sends no API key, no JWT and no tenant header.
  The tenant is in the request body. The resolver opens the connection for one call and closes it.
- **The ID token request stays on GitHub.** The request URL must use https, and the source follows no
  redirect, because the request carries the request token.
- **No secret in a log.** `WorkloadToken`, `WorkloadCredential`, `RefreshingWorkloadCredential` and
  the GitHub source leave their secrets out of `%v`, `%+v`, `%#v` and `%s`. JSON leaves out
  `apiguard.KeyInfo.Binding`.

## Add a platform

A new platform, such as GitLab `id_tokens` or a Kubernetes projected token, adds one
`WorkloadIdentitySource` and its detector in `DetectWorkloadIdentitySource`. Its token type is a value
of `SubjectTokenType` in safedep/api. Callers do not change.
