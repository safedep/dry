package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIKeyCredential(t *testing.T) {
	t.Run("valid API key credential", func(t *testing.T) {
		cred, err := NewAPIKeyCredential("sk-test-key", "tenant-123")
		require.NoError(t, err)
		assert.True(t, cred.IsDataPlane())
		assert.False(t, cred.IsControlPlane())

		apiKey, err := cred.GetAPIKey()
		require.NoError(t, err)
		assert.Equal(t, "sk-test-key", apiKey)

		tenant, err := cred.GetTenantDomain()
		require.NoError(t, err)
		assert.Equal(t, "tenant-123", tenant)
	})

	t.Run("empty API key returns error", func(t *testing.T) {
		_, err := NewAPIKeyCredential("", "tenant-123")
		assert.ErrorIs(t, err, ErrMissingCredentials)
	})

	t.Run("empty tenant domain returns error from getter", func(t *testing.T) {
		cred, err := NewAPIKeyCredential("sk-test-key", "")
		require.NoError(t, err)
		_, err = cred.GetTenantDomain()
		assert.ErrorIs(t, err, ErrMissingCredentials)
	})
}

func TestNewTokenCredential(t *testing.T) {
	t.Run("valid token credential", func(t *testing.T) {
		cred, err := NewTokenCredential("jwt-token", "refresh-token", "tenant-123")
		require.NoError(t, err)
		assert.True(t, cred.IsControlPlane())
		assert.False(t, cred.IsDataPlane())

		token, err := cred.GetToken()
		require.NoError(t, err)
		assert.Equal(t, "jwt-token", token)

		refresh, err := cred.GetRefreshToken()
		require.NoError(t, err)
		assert.Equal(t, "refresh-token", refresh)
	})

	t.Run("empty token returns error", func(t *testing.T) {
		_, err := NewTokenCredential("", "refresh", "tenant")
		assert.ErrorIs(t, err, ErrMissingCredentials)
	})
}

func TestCredentialTypeValidation(t *testing.T) {
	t.Run("API key credential rejects token getter", func(t *testing.T) {
		cred, _ := NewAPIKeyCredential("sk-key", "tenant")
		_, err := cred.GetToken()
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
		_, err = cred.GetRefreshToken()
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})

	t.Run("token credential rejects API key getter", func(t *testing.T) {
		cred, _ := NewTokenCredential("jwt", "refresh", "tenant")
		_, err := cred.GetAPIKey()
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})
}
