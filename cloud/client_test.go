package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDataPlaneClient(t *testing.T) {
	t.Run("rejects control plane credentials", func(t *testing.T) {
		cred, _ := NewTokenCredential("jwt", "refresh", "tenant")
		_, err := NewDataPlaneClient("test", cred)
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})

	t.Run("rejects nil credentials", func(t *testing.T) {
		_, err := NewDataPlaneClient("test", nil)
		assert.Error(t, err)
	})
}

func TestNewControlPlaneClient(t *testing.T) {
	t.Run("rejects data plane credentials", func(t *testing.T) {
		cred, _ := NewAPIKeyCredential("sk-key", "tenant")
		_, err := NewControlPlaneClient("test", cred)
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	})

	t.Run("rejects nil credentials", func(t *testing.T) {
		_, err := NewControlPlaneClient("test", nil)
		assert.Error(t, err)
	})
}
