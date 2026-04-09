package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	assert.Error(t, ErrInvalidCredentialType)
	assert.Error(t, ErrMissingCredentials)
	assert.ErrorIs(t, ErrInvalidCredentialType, ErrInvalidCredentialType)
	assert.NotErrorIs(t, ErrInvalidCredentialType, ErrMissingCredentials)
}
