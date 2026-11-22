package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/safedep/dry/keychain"
	"github.com/safedep/dry/log"
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

func (r *keychainCredentialResolver) resolveAPIKey() (*Credentials, error) {
	ctx := context.Background()

	apiKey, err := r.getField(ctx, fieldAPIKey)
	if err != nil {
		return nil, err
	}

	tenantDomain, err := r.getAPIKeyTenantDomain(ctx)
	if err != nil {
		return nil, err
	}

	meta, err := r.getAPIKeyMetadata(ctx)
	if err != nil {
		return nil, err
	}

	creds, err := r.withSource(NewAPIKeyCredential(apiKey, tenantDomain))
	if err != nil {
		return nil, err
	}
	creds.apiKeyMeta = meta
	return creds, nil
}

// getAPIKeyTenantDomain reads the tenant of the API key. A key saved before
// api_key_tenant_domain existed has only tenant_domain, so the resolver falls
// back to it.
func (r *keychainCredentialResolver) getAPIKeyTenantDomain(ctx context.Context) (string, error) {
	tenantDomain, err := r.getField(ctx, fieldAPIKeyTenantDomain)
	if err == nil && tenantDomain != "" {
		return tenantDomain, nil
	}
	if err != nil && !errors.Is(err, ErrMissingCredentials) {
		return "", err
	}
	return r.getTenantDomain(ctx, fieldAPIKey)
}

// getAPIKeyMetadata reads the id and the expiry of the API key. Both are
// optional. An expiry that does not parse is logged and treated as unknown:
// the key may still be valid, and only SafeDep Cloud can tell.
func (r *keychainCredentialResolver) getAPIKeyMetadata(ctx context.Context) (APIKeyMetadata, error) {
	var meta APIKeyMetadata

	keyID, err := r.getField(ctx, fieldAPIKeyID)
	switch {
	case err == nil:
		meta.KeyID = keyID
	case !errors.Is(err, ErrMissingCredentials):
		return APIKeyMetadata{}, err
	}

	expiresAt, err := r.getField(ctx, fieldAPIKeyExpiresAt)
	switch {
	case err == nil && expiresAt != "":
		parsed, perr := time.Parse(time.RFC3339, expiresAt)
		if perr != nil {
			log.Warnf("cloud: ignoring the API key expiry %q in the keychain: %v", expiresAt, perr)
			break
		}
		meta.ExpiresAt = parsed
	case err != nil && !errors.Is(err, ErrMissingCredentials):
		return APIKeyMetadata{}, err
	}

	return meta, nil
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
