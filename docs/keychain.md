# Keychain

Cross-platform secret storage for SafeDep CLI applications. Uses OS-native keychains (macOS Keychain, Linux Secret Service) with an optional insecure file-based fallback.

## Usage

```go
import "github.com/safedep/dry/keychain"

kc, err := keychain.New(keychain.Config{
    AppName: "vet",
})
if err != nil {
    log.Fatal(err)
}
defer kc.Close()

// Store a secret
err = kc.Set(ctx, "api-token", &keychain.Secret{Value: "sk-abc123"})

// Retrieve a secret
secret, err := kc.Get(ctx, "api-token")
if errors.Is(err, keychain.ErrNotFound) {
    // not stored yet
}

// Delete a secret
err = kc.Delete(ctx, "api-token")
```

## Insecure File Fallback

For environments without an OS keychain (CI, containers, headless servers), enable the plaintext file fallback:

```go
kc, err := keychain.New(keychain.Config{
    AppName:              "vet",
    InsecureFileFallback: true,
})
```

The file provider stores secrets in the per-user state directory:

| Platform | Default path |
|----------|--------------|
| Linux, macOS | `$XDG_STATE_HOME/<AppName>/creds.json`, default `~/.local/state/<AppName>/creds.json` |
| Windows | `%LOCALAPPDATA%\<AppName>\creds.json` |

An absolute `XDG_STATE_HOME` wins on every platform. A relative value is ignored.

Earlier releases used `$XDG_CONFIG_HOME/<AppName>/creds.json`, default
`~/.config/<AppName>/creds.json`, on Linux and `~/Library/Application Support/<AppName>/creds.json`
on macOS. On Windows the old and new paths are
the same, unless an absolute `XDG_STATE_HOME` is set.

When the old and new paths differ, a file at the old path moves to the new path on first use:

- The move never replaces a file at the new path.
- If both files exist, the provider uses the new file and logs a warning that names the old one.
- If the old file changes during the move, the provider keeps it and logs a warning.
- If the move fails, the provider uses the old path.
- If another user owns the old file, for example when root runs with sudo and keeps `HOME`, the
  provider uses the old path and does not move the file.
- If the state directory cannot be resolved, for example with `XDG_CONFIG_HOME` set and `HOME`
  unset, the provider uses the old path.

Override the path with `FilePath`:

```go
kc, err := keychain.New(keychain.Config{
    AppName:              "vet",
    InsecureFileFallback: true,
    FilePath:             "/custom/path/creds.json",
})
```

A warning is logged when the file provider is used.

## Platform Support

| Platform | Backend |
|----------|---------|
| macOS | Keychain (`/usr/bin/security`) |
| Linux | Secret Service (GNOME Keyring via D-Bus) |
| Windows | Windows Credential Manager |
| Others | File fallback only |

On Linux the OS keychain needs a D-Bus session bus that already runs. `New` finds it with the bus
discovery of godbus: `DBUS_SESSION_BUS_ADDRESS`, then `/run/user/<uid>/bus`, then
`/run/user/<uid>/dbus-session`. Without one, `New` treats the OS keychain as unavailable and uses
the file fallback when it is on. It does not start a bus. `dbus-launch` starts a daemon that
outlives the process, so a tool that runs many times on a headless host would leak one daemon each
time. A headless host that had a keyring only because each process started its own bus now uses
the file fallback.

## Cloud Credential Store & Resolver

The `cloud` package provides a keychain-backed credential store and resolver for SafeDep Cloud. Configure once, use across all SafeDep tools.

```go
import "github.com/safedep/dry/cloud"

// Store credentials (e.g. during login)
store, err := cloud.NewKeychainCredentialStore()
if err != nil {
    return err
}
defer store.Close()
store.SaveAPIKeyCredential("sk-abc123", "my-tenant")

// Resolve credentials (any tool)
resolver, err := cloud.NewKeychainCredentialResolver(cloud.CredentialTypeAPIKey)
if err != nil {
    return err
}
defer resolver.Close()
creds, err := resolver.Resolve()

// Chain with env fallback
chain := cloud.NewChainCredentialResolver(resolver, envResolver)
```

### Shared resolver

`NewDefaultCredentialResolver` builds the chain that SafeDep tools share, so that one sign-in
serves every tool:

```go
resolver, err := cloud.NewDefaultCredentialResolver(cloud.CredentialTypeAPIKey)
if err != nil {
    return err
}
defer resolver.Close()
creds, err := resolver.Resolve()
if err != nil {
    return err
}
creds.Source() // cloud.CredentialSourceEnvironment or cloud.CredentialSourceKeychain
```

1. For `CredentialTypeAPIKey`, it reads `SAFEDEP_API_KEY` and `SAFEDEP_TENANT_ID` first. Both must
   be set, or neither.
2. Then it reads the keychain profile from `cloud.ResolveProfile("")`: `SAFEDEP_PROFILE`, else
   `default`. Pass `cloud.WithProfile(cloud.ResolveProfile(flagValue))` to let a `--profile` flag
   win.

A source that holds half a credential (an API key with no tenant) returns
`ErrIncompleteCredentials` and stops the chain. `ErrIncompleteCredentials` wraps
`ErrMissingCredentials`, so existing `errors.Is` checks still match.

A keychain that cannot open does not fail construction. The environment still works on a machine
with no keychain. `Resolve` reports the keychain error only when the environment has no
credentials.

Options: `WithProfile("staging")`, `WithAppName("custom")`, `WithInsecureFileFallback()`, `WithInsecureFileFallbackPath("/path")`, `WithKeychainHandle(kc)`.

### API key metadata

SafeDep Cloud returns the id and the expiry of an API key when it creates the key. Save them with
the key, so that a tool can tell an expired key from a valid one before it calls SafeDep Cloud, and
can revoke the key that it replaces:

```go
meta := cloud.APIKeyMetadata{KeyID: resp.GetKeyId()}
// AsTime on a missing timestamp returns 1970-01-01, which would read as expired.
if resp.GetExpiresAt() != nil {
    meta.ExpiresAt = resp.GetExpiresAt().AsTime()
}

saver, ok := store.(cloud.APIKeyMetadataStore) // the keychain store implements it
if ok {
    err = saver.SaveAPIKeyCredentialWithMetadata(apiKey, tenant, meta)
}

creds, err := resolver.Resolve()
if expiresAt, ok := creds.APIKeyExpiresAt(); ok && !expiresAt.After(time.Now()) {
    // The key has expired. Create a new one.
}
id, ok := creds.APIKeyID() // ok is false when the id is not known
```

- `SaveAPIKeyCredential` saves a key with no metadata. A key that a user pastes never carries the
  id or the expiry of an older key.
- An expiry that is not known (`ok` is false) does not mean that the key is valid or expired. Only
  SafeDep Cloud can tell. Keys from the environment, keys saved by `SaveAPIKeyCredential`, and keys
  saved by an older dry have no metadata.
- `APIKeyMetadataStore` is a separate interface, so other implementations of `CredentialStore`
  keep compiling.

The keychain fields of a profile:

| Field | Written by | Value |
| --- | --- | --- |
| `api_key` | API key saves | the API key |
| `api_key_metadata` | API key saves | JSON: the SHA-256 of `api_key`, its tenant, id and expiry |
| `token`, `refresh_token` | `SaveTokenCredential` | the OAuth tokens |
| `tenant_domain` | API key and token saves | the tenant of the last save |

The resolver reads the tenant and the metadata of the API key from `api_key_metadata` while its
SHA-256 matches `api_key`. Otherwise the key resolves as an older dry saved it: the tenant from
`tenant_domain` and no metadata. So:

- A token saved for another tenant rewrites `tenant_domain` but not the tenant of the API key.
- An older dry that replaces `api_key` leaves `api_key_metadata`. The SHA-256 no longer matches, so
  the new key never takes the tenant, the id or the expiry of the old key.
- API key saves still write `tenant_domain`, because older readers take the tenant of the API key
  from it.

The keychain has no transactions, so reads and writes are best effort. A read during a save, or two
saves at the same moment, can see the fields of two saves. A key whose SHA-256 does not match then
resolves as an older dry saved it, and the next read after the save sees one consistent key.

### Multi-Tenancy

Use named profiles to work with multiple tenants. Each profile is an isolated credential context. Inside a profile, only the API key keeps its own tenant (see [API key metadata](#api-key-metadata)). The token takes the tenant of the last save, and an API key save also writes that tenant.

```go
// Store credentials for different tenants
prodStore, _ := cloud.NewKeychainCredentialStore(cloud.WithProfile("prod"))
prodStore.SaveAPIKeyCredential("sk-prod-key", "prod.safedep.io")

stagingStore, _ := cloud.NewKeychainCredentialStore(cloud.WithProfile("staging"))
stagingStore.SaveAPIKeyCredential("sk-staging-key", "staging.safedep.io")

// Resolve from a specific profile
resolver, _ := cloud.NewKeychainCredentialResolver(
    cloud.CredentialTypeAPIKey,
    cloud.WithProfile("prod"),
)
```

The default profile is `"default"` when `WithProfile` is not specified.

## Security

The security boundary is the OS user session.
