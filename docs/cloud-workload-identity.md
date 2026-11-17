# cloud: workload identity

A SafeDep tool in CI can call SafeDep Cloud with no API key. The tool trades a token that the CI
platform signs for the job, such as a GitHub Actions OIDC ID token, for a short-lived workload
credential. The API is `WorkloadIdentityService` in safedep/api. The design is
`docs/specs/2026-11-07-workload-identity-design.md` in safedep/control-tower.

## Which credentials the default resolver selects

`cloud.NewDefaultCredentialResolver(cloud.CredentialTypeAPIKey)` reads the environment in this order:

1. `SAFEDEP_API_KEY` with `SAFEDEP_TENANT_ID`: the API key. An explicit API key always wins.
2. `SAFEDEP_TENANT_ID` with a workload identity in the environment: the exchange. The credentials
   have the source `CredentialSourceWorkloadIdentity`.
3. `SAFEDEP_TENANT_ID` alone: `ErrIncompleteCredentials`. The message tells the user to give the job
   `permissions: id-token: write`, or to set `SAFEDEP_API_KEY`.
4. Neither variable: the keychain profile, as before.

A GitHub Actions job has a workload identity when GitHub sets `ACTIONS_ID_TOKEN_REQUEST_URL` and
`ACTIONS_ID_TOKEN_REQUEST_TOKEN`. GitHub sets both for a job with `id-token: write`.

`Resolve` makes no network call. The first exchange runs on the first RPC.

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
dials its own connection passes `creds.Workload()` to `grpc.WithPerRPCCredentials`. `GetAPIKey`
returns the current access token, which expires, so use it only for a short-lived connection.

## Rules

- **A new ID token for each exchange.** The server accepts a token one time only, and a GitHub
  Actions ID token lives 5 minutes. `WorkloadExchangeClient.Exchange` gets the token from the source
  right before it calls `ExchangeToken`, and keeps no token.
- **Refresh before expiry.** `RefreshingWorkloadCredential` exchanges again 5 minutes before the
  credential expires, or at half its life when it lives less than 10 minutes. A refresh that fails
  keeps the current credential until it expires, and tries again after 30 seconds. So a job that can
  no longer get an ID token works to the end of the credential life.
- **Revoke at the end.** `Revoke` revokes the current credential, and the credential then refuses to
  exchange again. A credential that a refresh replaced expires within the refresh margin.
- **The exchange is keyless.** The exchange connection sends no API key, no JWT and no tenant header.
  The tenant is in the request body.
- **No secret in a log.** Do not log `WorkloadToken.Value` or `WorkloadCredential.AccessToken`.
  `WorkloadCredential.String` leaves out the access token.

## Add a platform

A new platform, such as GitLab `id_tokens` or a Kubernetes projected token, adds one
`WorkloadIdentitySource` and its detector in `DetectWorkloadIdentitySource`. Its token type is a value
of `SubjectTokenType` in safedep/api. Callers do not change.
