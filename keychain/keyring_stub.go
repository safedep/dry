//go:build !darwin && !linux

package keychain

import (
	"context"
	"fmt"
)

type keyringProvider struct{}

func newKeyringProvider(appName string) (*keyringProvider, error) {
	return nil, fmt.Errorf("keychain: OS keychain is not supported on this platform")
}

func (k *keyringProvider) get(_ context.Context, _ string) (*Secret, error) {
	return nil, fmt.Errorf("keychain: OS keychain is not supported on this platform")
}

func (k *keyringProvider) set(_ context.Context, _ string, _ *Secret) error {
	return fmt.Errorf("keychain: OS keychain is not supported on this platform")
}

func (k *keyringProvider) delete(_ context.Context, _ string) error {
	return fmt.Errorf("keychain: OS keychain is not supported on this platform")
}

func (k *keyringProvider) close() error {
	return nil
}
