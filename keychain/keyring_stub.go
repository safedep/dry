//go:build !darwin && !linux && !windows

package keychain

import (
	"context"
	"errors"
)

var errUnsupportedPlatform = errors.New("keychain: OS keychain is not supported on this platform")

type keyringProvider struct{}

func newKeyringProvider(_ string) (*keyringProvider, error) {
	return nil, errUnsupportedPlatform
}

func (k *keyringProvider) get(_ context.Context, _ string) (*Secret, error) {
	return nil, errUnsupportedPlatform
}

func (k *keyringProvider) set(_ context.Context, _ string, _ *Secret) error {
	return errUnsupportedPlatform
}

func (k *keyringProvider) delete(_ context.Context, _ string) error {
	return errUnsupportedPlatform
}

func (k *keyringProvider) close() error {
	return nil
}
