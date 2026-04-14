// tui/prompt/errors_test.go
package prompt

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelsAreStable(t *testing.T) {
	assert.True(t, errors.Is(ErrCancelled, ErrCancelled))
	assert.True(t, errors.Is(ErrAgentMode, ErrAgentMode))
	assert.True(t, errors.Is(ErrNoTTY, ErrNoTTY))

	assert.False(t, errors.Is(ErrCancelled, ErrNoTTY))
}
