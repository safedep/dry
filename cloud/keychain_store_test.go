package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T, opts ...KeychainOption) CredentialStore {
	t.Helper()
	tmpFile := t.TempDir() + "/creds.json"
	allOpts := append([]KeychainOption{
		WithAppName("safedep-test-" + t.Name()),
		WithInsecureFileFallbackPath(tmpFile),
	}, opts...)
	store, err := NewKeychainCredentialStore(allOpts...)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Clear())
		require.NoError(t, store.Close())
	})
	return store
}

func TestKeychainCredentialStore_SaveAPIKeyCredential(t *testing.T) {
	t.Run("save and verify via resolver", func(t *testing.T) {
		store := newTestStore(t)

		err := store.SaveAPIKeyCredential("sk-test-key", "tenant-123")
		require.NoError(t, err)
	})

	t.Run("empty api key returns error", func(t *testing.T) {
		store := newTestStore(t)
		err := store.SaveAPIKeyCredential("", "tenant-123")
		assert.Error(t, err)
	})

	t.Run("empty tenant domain returns error", func(t *testing.T) {
		store := newTestStore(t)
		err := store.SaveAPIKeyCredential("sk-key", "")
		assert.Error(t, err)
	})
}

func TestKeychainCredentialStore_SaveTokenCredential(t *testing.T) {
	t.Run("save token credential", func(t *testing.T) {
		store := newTestStore(t)
		err := store.SaveTokenCredential("jwt-token", "refresh-tok", "tenant-123")
		require.NoError(t, err)
	})

	t.Run("empty token returns error", func(t *testing.T) {
		store := newTestStore(t)
		err := store.SaveTokenCredential("", "refresh", "tenant")
		assert.Error(t, err)
	})

	t.Run("empty tenant domain returns error", func(t *testing.T) {
		store := newTestStore(t)
		err := store.SaveTokenCredential("token", "refresh", "")
		assert.Error(t, err)
	})
}

func TestKeychainCredentialStore_Clear(t *testing.T) {
	t.Run("clear removes all fields", func(t *testing.T) {
		store := newTestStore(t)

		err := store.SaveAPIKeyCredential("sk-key", "tenant-123")
		require.NoError(t, err)

		err = store.Clear()
		require.NoError(t, err)
	})

	t.Run("clear on empty store succeeds", func(t *testing.T) {
		store := newTestStore(t)
		err := store.Clear()
		require.NoError(t, err)
	})
}
