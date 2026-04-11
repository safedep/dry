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

Secrets are stored in `$HOME/.config/<AppName>/creds.json`. Override the path with `FilePath`:

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

Options: `WithProfile("staging")`, `WithAppName("custom")`, `WithInsecureFileFallback()`, `WithInsecureFileFallbackPath("/path")`, `WithKeychainHandle(kc)`.

## Security

The security boundary is the OS user session.
