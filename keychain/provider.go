package keychain

import "context"

type provider interface {
	get(ctx context.Context, key string) (*Secret, error)
	set(ctx context.Context, key string, secret *Secret) error
	delete(ctx context.Context, key string) error
	close() error
}
