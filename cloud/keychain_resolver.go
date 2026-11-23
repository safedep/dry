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

// readAPIKeyCredential reads the tenant and the metadata of apiKey. It reads
// the fingerprint after the fields that it binds, and the save writes the
// fingerprint last, so a matching fingerprint means that every field belongs
// to apiKey. Without a match, the key resolves as an older dry saved it: the
// tenant from tenant_domain and no metadata.
func (r *keychainCredentialResolver) readAPIKeyCredential(ctx context.Context, apiKey string) (*Credentials, error) {
	bound := make(map[string]string, len(apiKeyBoundFields))
	for _, field := range apiKeyBoundFields {
		value, err := r.getOptionalField(ctx, field)
		if err != nil {
			return nil, err
		}
		bound[field] = value
	}

	fingerprint, err := r.getOptionalField(ctx, fieldAPIKeyFingerprint)
	if err != nil {
		return nil, err
	}

	var meta APIKeyMetadata
	tenantDomain := ""
	if fingerprint != "" && fingerprint == apiKeyFingerprint(apiKey) {
		tenantDomain = bound[fieldAPIKeyTenantDomain]
		meta = parseAPIKeyMetadata(bound[fieldAPIKeyID], bound[fieldAPIKeyExpiresAt])
	}
	if tenantDomain == "" {
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

// parseAPIKeyMetadata builds the metadata of a key. An expiry that does not
// parse is logged and treated as unknown: the key may still be valid, and
// only SafeDep Cloud can tell.
func parseAPIKeyMetadata(keyID, expiresAt string) APIKeyMetadata {
	meta := APIKeyMetadata{KeyID: keyID}
	if expiresAt == "" {
		return meta
	}

	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		log.Warnf("cloud: ignoring the API key expiry %q in the keychain: %v", expiresAt, err)
		return meta
	}
	meta.ExpiresAt = parsed
	return meta
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
