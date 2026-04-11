package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStoreAndResolver(t *testing.T, credType CredentialType, opts ...KeychainOption) (CredentialStore, CloseableCredentialResolver) {
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

	resolver, err := NewKeychainCredentialResolver(credType, allOpts...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resolver.Close()) })

	return store, resolver
}

func TestKeychainCredentialResolver_APIKey(t *testing.T) {
	t.Run("resolves saved API key credential", func(t *testing.T) {
		store, resolver := newTestStoreAndResolver(t, CredentialTypeAPIKey)

		err := store.SaveAPIKeyCredential("sk-test-key", "tenant-123")
		require.NoError(t, err)

		creds, err := resolver.Resolve()
		require.NoError(t, err)
		assert.True(t, creds.IsDataPlane())

		apiKey, err := creds.GetAPIKey()
		require.NoError(t, err)
		assert.Equal(t, "sk-test-key", apiKey)

		tenant, err := creds.GetTenantDomain()
		require.NoError(t, err)
		assert.Equal(t, "tenant-123", tenant)
	})

	t.Run("missing API key returns error", func(t *testing.T) {
		_, resolver := newTestStoreAndResolver(t, CredentialTypeAPIKey)

		_, err := resolver.Resolve()
		assert.ErrorIs(t, err, ErrMissingCredentials)
	})
}

func TestKeychainCredentialResolver_Token(t *testing.T) {
	t.Run("resolves saved token credential", func(t *testing.T) {
		store, resolver := newTestStoreAndResolver(t, CredentialTypeToken)

		err := store.SaveTokenCredential("jwt-token", "refresh-tok", "tenant-123")
		require.NoError(t, err)

		creds, err := resolver.Resolve()
		require.NoError(t, err)
		assert.True(t, creds.IsControlPlane())

		token, err := creds.GetToken()
		require.NoError(t, err)
		assert.Equal(t, "jwt-token", token)

		refresh, err := creds.GetRefreshToken()
		require.NoError(t, err)
		assert.Equal(t, "refresh-tok", refresh)

		tenant, err := creds.GetTenantDomain()
		require.NoError(t, err)
		assert.Equal(t, "tenant-123", tenant)
	})

	t.Run("missing token returns error", func(t *testing.T) {
		_, resolver := newTestStoreAndResolver(t, CredentialTypeToken)

		_, err := resolver.Resolve()
		assert.ErrorIs(t, err, ErrMissingCredentials)
	})
}

func TestKeychainCredentialResolver_Unspecified(t *testing.T) {
	t.Run("unspecified type returns error", func(t *testing.T) {
		_, err := NewKeychainCredentialResolver(CredentialTypeUnspecified,
			WithInsecureFileFallbackPath(t.TempDir()+"/creds.json"),
		)
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})
}

func TestKeychainCredentialResolver_ProfileIsolation(t *testing.T) {
	t.Run("different profiles do not interfere", func(t *testing.T) {
		tmpFile := t.TempDir() + "/creds.json"
		baseOpts := []KeychainOption{WithInsecureFileFallbackPath(tmpFile)}

		prodStore, err := NewKeychainCredentialStore(
			append(baseOpts, WithProfile("prod"))...,
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, prodStore.Close()) }()

		stagingStore, err := NewKeychainCredentialStore(
			append(baseOpts, WithProfile("staging"))...,
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, stagingStore.Close()) }()

		err = prodStore.SaveAPIKeyCredential("sk-prod", "prod-tenant")
		require.NoError(t, err)
		err = stagingStore.SaveAPIKeyCredential("sk-staging", "staging-tenant")
		require.NoError(t, err)

		prodResolver, err := NewKeychainCredentialResolver(CredentialTypeAPIKey,
			append(baseOpts, WithProfile("prod"))...,
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, prodResolver.Close()) }()

		stagingResolver, err := NewKeychainCredentialResolver(CredentialTypeAPIKey,
			append(baseOpts, WithProfile("staging"))...,
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, stagingResolver.Close()) }()

		prodCreds, err := prodResolver.Resolve()
		require.NoError(t, err)
		prodKey, _ := prodCreds.GetAPIKey()
		assert.Equal(t, "sk-prod", prodKey)
		prodTenant, _ := prodCreds.GetTenantDomain()
		assert.Equal(t, "prod-tenant", prodTenant)

		stagingCreds, err := stagingResolver.Resolve()
		require.NoError(t, err)
		stagingKey, _ := stagingCreds.GetAPIKey()
		assert.Equal(t, "sk-staging", stagingKey)
		stagingTenant, _ := stagingCreds.GetTenantDomain()
		assert.Equal(t, "staging-tenant", stagingTenant)
	})
}

func TestKeychainCredentialResolver_ClearThenResolve(t *testing.T) {
	t.Run("clear removes credentials so resolve fails", func(t *testing.T) {
		tmpFile := t.TempDir() + "/creds.json"
		opts := []KeychainOption{WithInsecureFileFallbackPath(tmpFile)}

		store, err := NewKeychainCredentialStore(opts...)
		require.NoError(t, err)
		defer func() { require.NoError(t, store.Close()) }()

		err = store.SaveAPIKeyCredential("sk-key", "tenant-123")
		require.NoError(t, err)

		resolver, err := NewKeychainCredentialResolver(CredentialTypeAPIKey, opts...)
		require.NoError(t, err)
		defer func() { require.NoError(t, resolver.Close()) }()

		creds, err := resolver.Resolve()
		require.NoError(t, err)
		apiKey, _ := creds.GetAPIKey()
		assert.Equal(t, "sk-key", apiKey)

		err = store.Clear()
		require.NoError(t, err)

		_, err = resolver.Resolve()
		assert.ErrorIs(t, err, ErrMissingCredentials)
	})
}

func TestKeychainCredentialResolver_ChainIntegration(t *testing.T) {
	t.Run("keychain resolver in chain with env fallback", func(t *testing.T) {
		tmpFile := t.TempDir() + "/creds.json"

		keychainResolver, err := NewKeychainCredentialResolver(
			CredentialTypeAPIKey,
			WithInsecureFileFallbackPath(tmpFile),
		)
		require.NoError(t, err)
		defer func() { require.NoError(t, keychainResolver.Close()) }()

		t.Setenv("SAFEDEP_API_KEY", "sk-env-key")
		t.Setenv("SAFEDEP_TENANT_ID", "env-tenant")
		envResolver, err := NewEnvCredentialResolver()
		require.NoError(t, err)

		chain := NewChainCredentialResolver(keychainResolver, envResolver)

		// Keychain is empty, should fall through to env
		creds, err := chain.Resolve()
		require.NoError(t, err)
		apiKey, _ := creds.GetAPIKey()
		assert.Equal(t, "sk-env-key", apiKey)
	})

	t.Run("keychain takes priority over env in chain", func(t *testing.T) {
		tmpFile := t.TempDir() + "/creds.json"
		opts := []KeychainOption{WithInsecureFileFallbackPath(tmpFile)}

		store, err := NewKeychainCredentialStore(opts...)
		require.NoError(t, err)
		defer func() { require.NoError(t, store.Close()) }()

		err = store.SaveAPIKeyCredential("sk-keychain-key", "keychain-tenant")
		require.NoError(t, err)

		keychainResolver, err := NewKeychainCredentialResolver(CredentialTypeAPIKey, opts...)
		require.NoError(t, err)
		defer func() { require.NoError(t, keychainResolver.Close()) }()

		t.Setenv("SAFEDEP_API_KEY", "sk-env-key")
		t.Setenv("SAFEDEP_TENANT_ID", "env-tenant")
		envResolver, err := NewEnvCredentialResolver()
		require.NoError(t, err)

		chain := NewChainCredentialResolver(keychainResolver, envResolver)

		creds, err := chain.Resolve()
		require.NoError(t, err)
		apiKey, _ := creds.GetAPIKey()
		assert.Equal(t, "sk-keychain-key", apiKey)
		tenant, _ := creds.GetTenantDomain()
		assert.Equal(t, "keychain-tenant", tenant)
	})
}
