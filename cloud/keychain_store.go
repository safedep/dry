package cloud

import (
	"context"
	"fmt"
	"io"

	"github.com/safedep/dry/keychain"
)

const (
	fieldAPIKey       = "api_key"
	fieldToken        = "token"
	fieldRefreshToken = "refresh_token"
	fieldTenantDomain = "tenant_domain"
)

// CredentialStore writes SafeDep Cloud credentials to the keychain.
type CredentialStore interface {
	SaveAPIKeyCredential(apiKey, tenantDomain string) error
	SaveTokenCredential(token, refreshToken, tenantDomain string) error
	Clear() error
	io.Closer
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

func (s *keychainCredentialStore) SaveAPIKeyCredential(apiKey, tenantDomain string) error {
	if apiKey == "" {
		return fmt.Errorf("%w: API key is required", ErrMissingCredentials)
	}
	if tenantDomain == "" {
		return fmt.Errorf("%w: tenant domain is required", ErrMissingCredentials)
	}

	ctx := context.Background()
	if err := s.kc.Set(ctx, s.config.keyForField(fieldAPIKey), &keychain.Secret{Value: apiKey}); err != nil {
		return fmt.Errorf("cloud: failed to save API key: %w", err)
	}
	if err := s.kc.Set(ctx, s.config.keyForField(fieldTenantDomain), &keychain.Secret{Value: tenantDomain}); err != nil {
		return fmt.Errorf("cloud: failed to save tenant domain: %w", err)
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
	fields := []string{fieldAPIKey, fieldToken, fieldRefreshToken, fieldTenantDomain}
	for _, field := range fields {
		err := s.kc.Delete(ctx, s.config.keyForField(field))
		if err != nil && err != keychain.ErrNotFound {
			return fmt.Errorf("cloud: failed to clear %s: %w", field, err)
		}
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
