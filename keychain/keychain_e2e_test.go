package keychain

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipUnlessE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("KEYCHAIN_ENABLE_E2E_TEST") == "" {
		t.Skip("skipping E2E test: set KEYCHAIN_ENABLE_E2E_TEST=true to run")
	}
}

func TestE2EKeychain(t *testing.T) {
	skipUnlessE2E(t)

	kc, err := New(Config{
		AppName: "dry-keychain-e2e-test",
	})
	require.NoError(t, err)
	defer func() { assert.NoError(t, kc.Close()) }()

	ctx := context.Background()
	key := "e2e-test-secret"

	// Clean up any leftover from previous runs
	_ = kc.Delete(ctx, key)

	// Get non-existent key
	_, err = kc.Get(ctx, key)
	assert.ErrorIs(t, err, ErrNotFound)

	// Set and Get
	err = kc.Set(ctx, key, &Secret{Value: "e2e-test-value"})
	require.NoError(t, err)

	secret, err := kc.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "e2e-test-value", secret.Value)

	// Overwrite
	err = kc.Set(ctx, key, &Secret{Value: "updated-value"})
	require.NoError(t, err)

	secret, err = kc.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "updated-value", secret.Value)

	// Delete
	err = kc.Delete(ctx, key)
	require.NoError(t, err)

	_, err = kc.Get(ctx, key)
	assert.ErrorIs(t, err, ErrNotFound)

	// Delete non-existent
	err = kc.Delete(ctx, key)
	assert.ErrorIs(t, err, ErrNotFound)
}
