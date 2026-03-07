package keychain

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var ErrNotFound = errors.New("keychain: secret not found")

type Secret struct {
	Value string
}

type Config struct {
	// AppName is required. Scopes secrets by application.
	// Used as the service name in OS keychains and as the
	// directory name for file storage.
	AppName string

	// InsecureFileFallback enables plaintext JSON file storage
	// when the OS keychain is unavailable.
	InsecureFileFallback bool

	// FilePath overrides the default file path for the insecure
	// file provider. Defaults to $HOME/.config/<AppName>/creds.json.
	FilePath string
}

type Keychain interface {
	// Get retrieves the secret associated with the given key.
	Get(ctx context.Context, key string) (*Secret, error)

	// Set stores the secret associated with the given key.
	Set(ctx context.Context, key string, secret *Secret) error

	// Delete removes the secret associated with the given key.
	Delete(ctx context.Context, key string) error

	// Close releases any resources held by the keychain.
	io.Closer
}

type keychainImpl struct {
	p provider
}

func New(config Config) (Keychain, error) {
	if config.AppName == "" {
		return nil, fmt.Errorf("keychain: AppName is required")
	}

	p, err := newKeyringProvider(config.AppName)
	if err == nil {
		return &keychainImpl{p: p}, nil
	}

	if !config.InsecureFileFallback {
		return nil, fmt.Errorf("keychain: OS keychain unavailable: %w", err)
	}

	fp, err := newFileProvider(config.AppName, config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("keychain: failed to create file provider: %w", err)
	}

	return &keychainImpl{p: fp}, nil
}

func (k *keychainImpl) Get(ctx context.Context, key string) (*Secret, error) {
	return k.p.get(ctx, key)
}

func (k *keychainImpl) Set(ctx context.Context, key string, secret *Secret) error {
	if secret == nil {
		return fmt.Errorf("keychain: secret must not be nil")
	}
	return k.p.set(ctx, key, secret)
}

func (k *keychainImpl) Delete(ctx context.Context, key string) error {
	return k.p.delete(ctx, key)
}

func (k *keychainImpl) Close() error {
	return k.p.close()
}
