package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvCredentialResolver(t *testing.T) {
	t.Run("resolves from environment", func(t *testing.T) {
		t.Setenv("SAFEDEP_API_KEY", "sk-test-key")
		t.Setenv("SAFEDEP_TENANT_ID", "tenant-123")

		resolver, err := NewEnvCredentialResolver()
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

	t.Run("missing API key returns error on resolve", func(t *testing.T) {
		t.Setenv("SAFEDEP_API_KEY", "")
		t.Setenv("SAFEDEP_TENANT_ID", "tenant-123")

		resolver, err := NewEnvCredentialResolver()
		require.NoError(t, err)

		_, err = resolver.Resolve()
		assert.Error(t, err)
	})
}

func TestChainCredentialResolver(t *testing.T) {
	t.Run("returns first successful result", func(t *testing.T) {
		t.Setenv("SAFEDEP_API_KEY", "")

		failing, _ := NewEnvCredentialResolver()

		working := &mockResolver{
			creds: mustAPIKeyCred("sk-from-chain", "tenant-chain"),
		}

		chain := NewChainCredentialResolver(failing, working)
		creds, err := chain.Resolve()
		require.NoError(t, err)

		apiKey, _ := creds.GetAPIKey()
		assert.Equal(t, "sk-from-chain", apiKey)
	})

	t.Run("all fail returns last error", func(t *testing.T) {
		t.Setenv("SAFEDEP_API_KEY", "")
		failing1, _ := NewEnvCredentialResolver()
		failing2, _ := NewEnvCredentialResolver()

		chain := NewChainCredentialResolver(failing1, failing2)
		_, err := chain.Resolve()
		assert.Error(t, err)
	})
}

type mockResolver struct {
	creds *Credentials
	err   error
}

func (m *mockResolver) Resolve() (*Credentials, error) {
	return m.creds, m.err
}

func mustAPIKeyCred(key, tenant string) *Credentials {
	c, _ := NewAPIKeyCredential(key, tenant)
	return c
}
