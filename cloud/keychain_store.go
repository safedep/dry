package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/safedep/dry/keychain"
)

const (
	fieldAPIKey       = "api_key"
	fieldToken        = "token"
	fieldRefreshToken = "refresh_token"
	fieldTenantDomain = "tenant_domain"
)

// allCredentialFields lists every field that Clear deletes.
var allCredentialFields = []string{
	fieldAPIKey, fieldToken, fieldRefreshToken, fieldTenantDomain, fieldAPIKeyMetadata,
}

// CredentialStore writes SafeDep Cloud credentials to the keychain.
type CredentialStore interface {
	SaveAPIKeyCredential(apiKey, tenantDomain string) error
	SaveTokenCredential(token, refreshToken, tenantDomain string) error
	Clear() error
	io.Closer
}

// APIKeyMetadata is what SafeDep Cloud returns about an API key when it
// creates the key. A zero field means that the value is not known.
type APIKeyMetadata struct {
	// KeyID is the id of the key in SafeDep Cloud. A tool needs it to revoke
	// the key.
	KeyID string

	// ExpiresAt is when SafeDep Cloud stops accepting the key.
	ExpiresAt time.Time
}

// APIKeyMetadataStore saves an API key together with its metadata, so that a
// tool can tell an expired key from a valid one before it calls SafeDep
// Cloud. The store that NewKeychainCredentialStore returns implements it.
// CredentialStore does not include it, so other implementations of
// CredentialStore keep compiling.
type APIKeyMetadataStore interface {
	SaveAPIKeyCredentialWithMetadata(apiKey, tenantDomain string, meta APIKeyMetadata) error
}

type keychainCredentialStore struct {
	config keychainConfig
	kc     keychain.Keychain
	ownsKc bool
}

// NewKeychainCredentialStore creates a credential store backed by the keychain.
func NewKeychainCredentialStore(opts ...KeychainOption) (CredentialStore, error) {
	cfg := buildKeychainConfig(opts)

	kc, ownsKc, err := resolveKeychain(cfg)
	if err != nil {
		return nil, err
	}

	return &keychainCredentialStore{
		config: cfg,
		kc:     kc,
		ownsKc: ownsKc,
	}, nil
}

// SaveAPIKeyCredential saves an API key with no metadata. It deletes the
// metadata of the key that it replaces, so the new key never carries the id
// or the expiry of the old one.
func (s *keychainCredentialStore) SaveAPIKeyCredential(apiKey, tenantDomain string) error {
	return s.SaveAPIKeyCredentialWithMetadata(apiKey, tenantDomain, APIKeyMetadata{})
}

// SaveAPIKeyCredentialWithMetadata saves an API key, its tenant and its
// metadata. It also writes tenant_domain, which older readers use as the
// tenant of the API key. The keychain has no transaction, so a read during
// the save can see the new key before its metadata. The fingerprint does not
// match then, and the key resolves as an older dry saved it.
func (s *keychainCredentialStore) SaveAPIKeyCredentialWithMetadata(apiKey, tenantDomain string,
	meta APIKeyMetadata,
) error {
	if apiKey == "" {
		return fmt.Errorf("%w: API key is required", ErrMissingCredentials)
	}
	if tenantDomain == "" {
		return fmt.Errorf("%w: tenant domain is required", ErrMissingCredentials)
	}

	value, err := json.Marshal(newStoredAPIKeyMetadata(apiKey, tenantDomain, meta))
	if err != nil {
		return fmt.Errorf("cloud: failed to encode %s: %w", fieldAPIKeyMetadata, err)
	}

	ctx := context.Background()
	for _, w := range []struct{ field, value string }{
		{fieldAPIKey, apiKey},
		{fieldAPIKeyMetadata, string(value)},
		{fieldTenantDomain, tenantDomain},
	} {
		if err := s.kc.Set(ctx, s.config.keyForField(w.field), &keychain.Secret{Value: w.value}); err != nil {
			return fmt.Errorf("cloud: failed to save %s: %w", w.field, err)
		}
	}
	return nil
}

func (s *keychainCredentialStore) SaveTokenCredential(token, refreshToken, tenantDomain string) error {
	if token == "" {
		return fmt.Errorf("%w: token is required", ErrMissingCredentials)
	}
	if tenantDomain == "" {
		return fmt.Errorf("%w: tenant domain is required", ErrMissingCredentials)
	}

	ctx := context.Background()
	if err := s.kc.Set(ctx, s.config.keyForField(fieldToken), &keychain.Secret{Value: token}); err != nil {
		return fmt.Errorf("cloud: failed to save token: %w", err)
	}
	if err := s.kc.Set(ctx, s.config.keyForField(fieldRefreshToken), &keychain.Secret{Value: refreshToken}); err != nil {
		return fmt.Errorf("cloud: failed to save refresh token: %w", err)
	}
	if err := s.kc.Set(ctx, s.config.keyForField(fieldTenantDomain), &keychain.Secret{Value: tenantDomain}); err != nil {
		return fmt.Errorf("cloud: failed to save tenant domain: %w", err)
	}
	return nil
}

func (s *keychainCredentialStore) Clear() error {
	ctx := context.Background()
	for _, field := range allCredentialFields {
		if err := s.delete(ctx, field); err != nil {
			return err
		}
	}
	return nil
}

// delete removes a field. A field that is not there is not an error.
func (s *keychainCredentialStore) delete(ctx context.Context, field string) error {
	err := s.kc.Delete(ctx, s.config.keyForField(field))
	if err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return fmt.Errorf("cloud: failed to clear %s: %w", field, err)
	}
	return nil
}

func (s *keychainCredentialStore) Close() error {
	if s.ownsKc {
		return s.kc.Close()
	}
	return nil
}

// resolveKeychain returns a keychain instance and whether the caller owns it.
func resolveKeychain(cfg keychainConfig) (keychain.Keychain, bool, error) {
	if cfg.keychain != nil {
		return cfg.keychain, false, nil
	}

	kc, err := keychain.New(keychain.Config{
		AppName:              cfg.appName,
		InsecureFileFallback: cfg.insecureFileFallback,
		FilePath:             cfg.insecureFileFallbackPath,
	})
	if err != nil {
		return nil, false, fmt.Errorf("cloud: failed to create keychain: %w", err)
	}

	return kc, true, nil
}
