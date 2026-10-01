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

Earlier releases used `~/.config/<AppName>/creds.json` on Linux and
`~/Library/Application Support/<AppName>/creds.json` on macOS. On Windows the old and new paths are
the same, unless an absolute `XDG_STATE_HOME` is set.

When the old and new paths differ, a file at the old path moves to the new path on first use:

- The move never replaces a file at the new path.
- If both files exist, the provider uses the new file and logs a warning that names the old one.
- If the old file changes during the move, the provider keeps it and logs a warning.
- If the move fails, the provider uses the old path.

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

## Cloud Credential Store & Resolver

The `cloud` package provides a keychain-backed credential store and resolver for SafeDep Cloud. Configure once, use across all SafeDep tools.

```go
import "github.com/safedep/dry/cloud"

// Store credentials (e.g. during login)
store, err := cloud.NewKeychainCredentialStore()
defer store.Close()
store.SaveAPIKeyCredential("sk-abc123", "my-tenant")

// Resolve credentials (any tool)
resolver, err := cloud.NewKeychainCredentialResolver(cloud.CredentialTypeAPIKey)
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

### Multi-Tenancy

Use named profiles to work with multiple tenants. Each profile is an isolated credential context — both API key and token credentials within a profile share the same tenant.

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
