package cloud

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/dry/keychain"
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
		assert.Equal(t, CredentialSourceKeychain, creds.Source())
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
		appName := "safedep-test-" + t.Name()
		baseOpts := []KeychainOption{WithAppName(appName), WithInsecureFileFallbackPath(tmpFile)}

		prodStore, err := NewKeychainCredentialStore(
			append(baseOpts, WithProfile("prod"))...,
		)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, prodStore.Clear())
			require.NoError(t, prodStore.Close())
		}()

		stagingStore, err := NewKeychainCredentialStore(
			append(baseOpts, WithProfile("staging"))...,
		)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, stagingStore.Clear())
			require.NoError(t, stagingStore.Close())
		}()

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
		opts := []KeychainOption{WithAppName("safedep-test-" + t.Name()), WithInsecureFileFallbackPath(tmpFile)}

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
			WithAppName("safedep-test-"+t.Name()),
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
		opts := []KeychainOption{WithAppName("safedep-test-" + t.Name()), WithInsecureFileFallbackPath(tmpFile)}

		store, err := NewKeychainCredentialStore(opts...)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, store.Clear())
			require.NoError(t, store.Close())
		}()

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

// newSharedKeychain returns one keychain for a store, a resolver and the raw
// writes that stand in for an older dry or another tool.
func newSharedKeychain(t *testing.T) keychain.Keychain {
	t.Helper()
	kc, err := keychain.New(keychain.Config{
		AppName:              "safedep-test-" + t.Name(),
		InsecureFileFallback: true,
		FilePath:             t.TempDir() + "/creds.json",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, kc.Close()) })
	return kc
}

func TestKeychainCredentialResolver_APIKeyMetadata(t *testing.T) {
	expiresAt := time.Date(2027, 2, 20, 10, 30, 0, 0, time.UTC)

	setRaw := func(t *testing.T, kc keychain.Keychain, field, value string) {
		t.Helper()
		require.NoError(t, kc.Set(context.Background(), DefaultProfile+"/"+field, &keychain.Secret{Value: value}))
	}

	tests := []struct {
		name string
		// write stores the credentials under test.
		write          func(t *testing.T, kc keychain.Keychain, store CredentialStore)
		wantTenant     string
		wantKeyID      string
		wantExpiresAt  time.Time
		wantNoMetadata bool
	}{
		{
			name: "returns the id and the expiry saved with the key",
			write: func(t *testing.T, _ keychain.Keychain, store CredentialStore) {
				t.Helper()
				require.NoError(t, store.(APIKeyMetadataStore).SaveAPIKeyCredentialWithMetadata(
					"sk-new", "tenant-a", APIKeyMetadata{
						KeyID:     "key-1",
						ExpiresAt: expiresAt.In(time.FixedZone("IST", 5*3600+1800)),
					}))
			},
			wantTenant:    "tenant-a",
			wantKeyID:     "key-1",
			wantExpiresAt: expiresAt,
		},
		{
			name: "a key saved with no metadata drops the metadata of the key it replaces",
			write: func(t *testing.T, _ keychain.Keychain, store CredentialStore) {
				t.Helper()
				require.NoError(t, store.(APIKeyMetadataStore).SaveAPIKeyCredentialWithMetadata(
					"sk-old", "tenant-a", APIKeyMetadata{KeyID: "key-1", ExpiresAt: expiresAt}))
				require.NoError(t, store.SaveAPIKeyCredential("sk-pasted", "tenant-a"))
			},
			wantTenant:     "tenant-a",
			wantNoMetadata: true,
		},
		{
			name: "a sign-in to another tenant keeps the tenant of the API key",
			write: func(t *testing.T, _ keychain.Keychain, store CredentialStore) {
				t.Helper()
				require.NoError(t, store.SaveAPIKeyCredential("sk-a", "tenant-a"))
				require.NoError(t, store.SaveTokenCredential("jwt", "refresh", "tenant-b"))
			},
			wantTenant:     "tenant-a",
			wantNoMetadata: true,
		},
		{
			name: "a key written before api_key_tenant_domain existed uses tenant_domain",
			write: func(t *testing.T, kc keychain.Keychain, _ CredentialStore) {
				t.Helper()
				setRaw(t, kc, fieldAPIKey, "sk-legacy")
				setRaw(t, kc, fieldTenantDomain, "tenant-legacy")
			},
			wantTenant:     "tenant-legacy",
			wantNoMetadata: true,
		},
		{
			name: "an expiry that does not parse is unknown",
			write: func(t *testing.T, kc keychain.Keychain, store CredentialStore) {
				t.Helper()
				require.NoError(t, store.SaveAPIKeyCredential("sk-a", "tenant-a"))
				setRaw(t, kc, fieldAPIKeyMetadata, fmt.Sprintf(
					`{"fingerprint":%q,"tenant_domain":"tenant-a","expires_at":"next tuesday"}`,
					apiKeyFingerprint("sk-a")))
			},
			wantTenant:     "tenant-a",
			wantNoMetadata: true,
		},
		{
			name: "a key that an older dry saves over a new key does not take its tenant or metadata",
			write: func(t *testing.T, kc keychain.Keychain, store CredentialStore) {
				t.Helper()
				require.NoError(t, store.(APIKeyMetadataStore).SaveAPIKeyCredentialWithMetadata(
					"sk-old", "tenant-a", APIKeyMetadata{KeyID: "key-1", ExpiresAt: expiresAt}))
				setRaw(t, kc, fieldAPIKey, "sk-pasted")
				setRaw(t, kc, fieldTenantDomain, "tenant-b")
			},
			wantTenant:     "tenant-b",
			wantNoMetadata: true,
		},
		{
			// The same key saved again by an older dry matches the
			// fingerprint, so only Clear keeps the old metadata off it.
			name: "clear removes the metadata, so a key written by an older dry has none",
			write: func(t *testing.T, kc keychain.Keychain, store CredentialStore) {
				t.Helper()
				require.NoError(t, store.(APIKeyMetadataStore).SaveAPIKeyCredentialWithMetadata(
					"sk-new", "tenant-a", APIKeyMetadata{KeyID: "key-1", ExpiresAt: expiresAt}))
				require.NoError(t, store.Clear())
				setRaw(t, kc, fieldAPIKey, "sk-new")
				setRaw(t, kc, fieldTenantDomain, "tenant-legacy")
			},
			wantTenant:     "tenant-legacy",
			wantNoMetadata: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kc := newSharedKeychain(t)
			store, err := NewKeychainCredentialStore(WithKeychainHandle(kc))
			require.NoError(t, err)
			resolver, err := NewKeychainCredentialResolver(CredentialTypeAPIKey, WithKeychainHandle(kc))
			require.NoError(t, err)

			tc.write(t, kc, store)

			creds, err := resolver.Resolve()
			require.NoError(t, err)

			tenant, err := creds.GetTenantDomain()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTenant, tenant)

			keyID, hasKeyID := creds.APIKeyID()
			gotExpiresAt, hasExpiresAt := creds.APIKeyExpiresAt()
			if tc.wantNoMetadata {
				assert.False(t, hasKeyID, "unexpected key id %q", keyID)
				assert.False(t, hasExpiresAt, "unexpected expiry %v", gotExpiresAt)
				return
			}

			assert.True(t, hasKeyID)
			assert.Equal(t, tc.wantKeyID, keyID)
			require.True(t, hasExpiresAt)
			assert.True(t, tc.wantExpiresAt.Equal(gotExpiresAt), "expiry %v, want %v", gotExpiresAt, tc.wantExpiresAt)
		})
	}
}
