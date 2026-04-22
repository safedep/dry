// tui/output/verbosity_test.go
package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerbosityDefault(t *testing.T) {
	ResetVerbosityForTest()
	assert.Equal(t, Normal, CurrentVerbosity())
}

func TestVerbositySet(t *testing.T) {
	SetVerbosity(Silent)
	assert.Equal(t, Silent, CurrentVerbosity())
	SetVerbosity(Verbose)
	assert.Equal(t, Verbose, CurrentVerbosity())
}
