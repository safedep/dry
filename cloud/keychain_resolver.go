package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"github.com/safedep/dry/keychain"
)

// CloseableCredentialResolver combines CredentialResolver with io.Closer
// for resolvers that own underlying resources.
type CloseableCredentialResolver interface {
	CredentialResolver
	io.Closer
}

type keychainCredentialResolver struct {
	config   keychainConfig
	kc       keychain.Keychain
	ownsKc   bool
	credType CredentialType
}

// NewKeychainCredentialResolver creates a credential resolver backed by the keychain.
// The credType parameter specifies which credential type to resolve.
func NewKeychainCredentialResolver(credType CredentialType, opts ...KeychainOption) (CloseableCredentialResolver, error) {
	if credType == CredentialTypeUnspecified {
		return nil, fmt.Errorf("%w: credential type must be specified", ErrInvalidCredentialType)
	}

	cfg := buildKeychainConfig(opts)

	kc, ownsKc, err := resolveKeychain(cfg)
	if err != nil {
		return nil, err
	}

	return &keychainCredentialResolver{
		config:   cfg,
		kc:       kc,
		ownsKc:   ownsKc,
		credType: credType,
	}, nil
}

func (r *keychainCredentialResolver) Resolve() (*Credentials, error) {
	switch r.credType {
	case CredentialTypeAPIKey:
		return r.resolveAPIKey()
	case CredentialTypeToken:
		return r.resolveToken()
	default:
		return nil, fmt.Errorf("%w: unsupported credential type", ErrInvalidCredentialType)
	}
}

// resolveAttempts bounds how often resolveAPIKey reads the keychain again
// when a save replaces the key during the read.
const resolveAttempts = 3

// resolveAPIKey reads the key again after its other fields. A save that
// replaced the key in between would otherwise pair one key with the tenant of
// another, so the resolver reads again, and fails after resolveAttempts.
func (r *keychainCredentialResolver) resolveAPIKey() (*Credentials, error) {
	ctx := context.Background()

	for range resolveAttempts {
		apiKey, err := r.getField(ctx, fieldAPIKey)
		if err != nil {
			return nil, err
		}

		creds, err := r.readAPIKeyCredential(ctx, apiKey)
		if err != nil {
			return nil, err
		}

		again, err := r.getField(ctx, fieldAPIKey)
		if err != nil {
			return nil, err
		}
		if again == apiKey {
			return creds, nil
		}
	}

	return nil, errors.New("cloud: the API key in the keychain changed while it was read: retry")
}

// readAPIKeyCredential reads the tenant and the metadata of apiKey from its
// binding. A key with no binding resolves as an older dry saved it: the tenant
// from tenant_domain and no metadata.
func (r *keychainCredentialResolver) readAPIKeyCredential(ctx context.Context, apiKey string) (*Credentials, error) {
	stored, err := r.getOptionalField(ctx, fieldAPIKeyBindings)
	if err != nil {
		return nil, err
	}

	var meta APIKeyMetadata
	tenantDomain := ""
	if binding, ok := findAPIKeyBinding(decodeAPIKeyBindings(stored), apiKey); ok {
		tenantDomain = binding.TenantDomain
		meta = binding.metadata()
	} else {
		tenantDomain, err = r.getTenantDomain(ctx, fieldAPIKey)
		if err != nil {
			return nil, err
		}
	}

	creds, err := r.withSource(NewAPIKeyCredential(apiKey, tenantDomain))
	if err != nil {
		return nil, err
	}
	creds.apiKeyMeta = meta
	return creds, nil
}

// getOptionalField reads a field that may be absent. An absent field is "".
func (r *keychainCredentialResolver) getOptionalField(ctx context.Context, field string) (string, error) {
	value, err := r.getField(ctx, field)
	if errors.Is(err, ErrMissingCredentials) {
		return "", nil
	}
	return value, err
}

func (r *keychainCredentialResolver) resolveToken() (*Credentials, error) {
	ctx := context.Background()

	token, err := r.getField(ctx, fieldToken)
	if err != nil {
		return nil, err
	}

	refreshToken, err := r.getField(ctx, fieldRefreshToken)
	if err != nil && !errors.Is(err, ErrMissingCredentials) {
		return nil, err
	}

	tenantDomain, err := r.getTenantDomain(ctx, fieldToken)
	if err != nil {
		return nil, err
	}

	return r.withSource(NewTokenCredential(token, refreshToken, tenantDomain))
}

// getTenantDomain reads the tenant of a credential whose secret field is
// already present. A missing tenant then means half a credential.
func (r *keychainCredentialResolver) getTenantDomain(ctx context.Context, secretField string) (string, error) {
	tenantDomain, err := r.getField(ctx, fieldTenantDomain)
	if errors.Is(err, ErrMissingCredentials) || (err == nil && tenantDomain == "") {
		return "", fmt.Errorf("%w: keychain has %s but no %s", ErrIncompleteCredentials, secretField, fieldTenantDomain)
	}
	return tenantDomain, err
}

func (r *keychainCredentialResolver) withSource(creds *Credentials, err error) (*Credentials, error) {
	if err != nil {
		return nil, err
	}
	creds.source = CredentialSourceKeychain
	return creds, nil
}

func (r *keychainCredentialResolver) getField(ctx context.Context, field string) (string, error) {
	secret, err := r.kc.Get(ctx, r.config.keyForField(field))
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return "", fmt.Errorf("%w: %s", ErrMissingCredentials, field)
		}
		return "", fmt.Errorf("cloud: failed to read %s from keychain: %w", field, err)
	}
	return secret.Value, nil
}

func (r *keychainCredentialResolver) Close() error {
	if r.ownsKc {
		return r.kc.Close()
	}
	return nil
}
