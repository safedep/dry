// tui/output/profile_test.go
package output

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
)

func TestIsColorEnabled_NoColorStrips(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assert.False(t, IsColorEnabled())
}

func TestIsColorEnabled_AsciiProfileStrips(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	setProfileForTest(colorprofile.Ascii)
	defer resetProfileForTest()
	assert.False(t, IsColorEnabled())
}

func TestIsColorEnabled_TrueColorEnables(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	setProfileForTest(colorprofile.TrueColor)
	defer resetProfileForTest()
	assert.True(t, IsColorEnabled())
}
