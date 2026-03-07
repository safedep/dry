//go:build darwin || linux || windows

package keychain

import (
	"context"
	"errors"

	gokeyring "github.com/zalando/go-keyring"
)

type keyringProvider struct {
	appName string
}

func newKeyringProvider(appName string) (*keyringProvider, error) {
	// Verify the keyring is accessible by attempting a get
	// for a key that almost certainly doesn't exist.
	// If the keyring service itself is unavailable, this will
	// return an error other than ErrNotFound.
	_, err := gokeyring.Get(appName, "keychain-availability-probe")
	if err != nil && !errors.Is(err, gokeyring.ErrNotFound) {
		return nil, err
	}

	return &keyringProvider{appName: appName}, nil
}

func (k *keyringProvider) get(_ context.Context, key string) (*Secret, error) {
	val, err := gokeyring.Get(k.appName, key)
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &Secret{Value: val}, nil
}

func (k *keyringProvider) set(_ context.Context, key string, secret *Secret) error {
	return gokeyring.Set(k.appName, key, secret.Value)
}

func (k *keyringProvider) delete(_ context.Context, key string) error {
	err := gokeyring.Delete(k.appName, key)
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (k *keyringProvider) close() error {
	return nil
}
