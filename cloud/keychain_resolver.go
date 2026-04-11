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

func (r *keychainCredentialResolver) resolveAPIKey() (*Credentials, error) {
	ctx := context.Background()

	apiKey, err := r.getField(ctx, fieldAPIKey)
	if err != nil {
		return nil, err
	}

	tenantDomain, err := r.getField(ctx, fieldTenantDomain)
	if err != nil {
		return nil, err
	}

	return NewAPIKeyCredential(apiKey, tenantDomain)
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

	tenantDomain, err := r.getField(ctx, fieldTenantDomain)
	if err != nil {
		return nil, err
	}

	return NewTokenCredential(token, refreshToken, tenantDomain)
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
