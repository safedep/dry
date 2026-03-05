package keychain

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequiresAppName(t *testing.T) {
	_, err := New(Config{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AppName is required")
}

func TestNewWithInsecureFileFallback(t *testing.T) {
	tmpDir := t.TempDir()

	kc, err := New(Config{
		AppName:              "test-app",
		InsecureFileFallback: true,
		FilePath:             filepath.Join(tmpDir, "creds.json"),
	})
	require.NoError(t, err)
	defer func() { assert.NoError(t, kc.Close()) }()

	ctx := context.Background()

	// Set and Get
	err = kc.Set(ctx, "token", &Secret{Value: "abc"})
	require.NoError(t, err)

	secret, err := kc.Get(ctx, "token")
	require.NoError(t, err)
	assert.Equal(t, "abc", secret.Value)

	// Delete
	err = kc.Delete(ctx, "token")
	require.NoError(t, err)

	_, err = kc.Get(ctx, "token")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestNewWithoutFallbackFailsOnUnsupportedPlatform(t *testing.T) {
	// This test verifies that when InsecureFileFallback is false
	// and the OS keychain is unavailable, New returns an error.
	// On platforms with a working keychain (macOS with unlocked keychain),
	// this test may succeed instead - which is correct behavior.
	kc, err := New(Config{
		AppName:              "test-app",
		InsecureFileFallback: false,
	})
	if err != nil {
		assert.Contains(t, err.Error(), "OS keychain unavailable")
	} else {
		// OS keychain is available, which is fine
		assert.NoError(t, kc.Close())
	}
}

func TestSetNilSecretReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	kc, err := New(Config{
		AppName:              "test-app",
		InsecureFileFallback: true,
		FilePath:             filepath.Join(tmpDir, "creds.json"),
	})
	require.NoError(t, err)
	defer func() { assert.NoError(t, kc.Close()) }()

	err = kc.Set(context.Background(), "key", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret must not be nil")
}

func TestKeychainInterfaceThroughFileProvider(t *testing.T) {
	tmpDir := t.TempDir()

	kc, err := New(Config{
		AppName:              "integration-test",
		InsecureFileFallback: true,
		FilePath:             filepath.Join(tmpDir, "creds.json"),
	})
	require.NoError(t, err)
	defer func() { assert.NoError(t, kc.Close()) }()

	ctx := context.Background()

	// Get non-existent key
	_, err = kc.Get(ctx, "missing")
	assert.ErrorIs(t, err, ErrNotFound)

	// Set multiple keys
	err = kc.Set(ctx, "key1", &Secret{Value: "val1"})
	require.NoError(t, err)
	err = kc.Set(ctx, "key2", &Secret{Value: "val2"})
	require.NoError(t, err)

	// Get both
	s1, err := kc.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "val1", s1.Value)

	s2, err := kc.Get(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, "val2", s2.Value)

	// Overwrite
	err = kc.Set(ctx, "key1", &Secret{Value: "updated"})
	require.NoError(t, err)

	s1, err = kc.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "updated", s1.Value)

	// Delete non-existent
	err = kc.Delete(ctx, "nope")
	assert.ErrorIs(t, err, ErrNotFound)

	// Delete existing
	err = kc.Delete(ctx, "key1")
	require.NoError(t, err)

	_, err = kc.Get(ctx, "key1")
	assert.ErrorIs(t, err, ErrNotFound)

	// key2 still exists
	s2, err = kc.Get(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, "val2", s2.Value)
}
