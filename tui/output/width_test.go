// tui/output/width_test.go
package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWidthFallback(t *testing.T) {
	// When called in a non-TTY test env, width falls back to DefaultWidth.
	assert.Equal(t, DefaultWidth, Width())
}

func TestWidthOverride(t *testing.T) {
	SetWidthOverride(60)
	defer SetWidthOverride(0)
	assert.Equal(t, 60, Width())
}
