// tui/theme/palette_test.go
package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeDepPaletteNonEmpty(t *testing.T) {
	p := safeDepPalette()
	// Info and Success intentionally have no color — they use the terminal's
	// default foreground so routine messages don't compete with actionable ones.
	assert.Empty(t, p.Info.Dark)
	assert.Empty(t, p.Success.Dark)

	// Actionable roles must always carry color.
	assert.NotEmpty(t, p.Warning.Dark)
	assert.NotEmpty(t, p.Error.Dark)
	assert.NotEmpty(t, p.Muted.Dark)
	assert.NotEmpty(t, p.Text.Dark)
	assert.NotEmpty(t, p.Heading.Dark)
	assert.NotEmpty(t, p.Path.Dark)
	assert.NotEmpty(t, p.Critical.Dark)
	assert.NotEmpty(t, p.High.Dark)
	assert.NotEmpty(t, p.Medium.Dark)
	assert.NotEmpty(t, p.Low.Dark)
	assert.NotEmpty(t, p.BrandPrimary.Dark)
	assert.NotEmpty(t, p.BrandAccent.Dark)
}

func TestPaletteColorByRole(t *testing.T) {
	p := safeDepPalette()
	c, ok := p.ColorByRole(RoleBrandPrimary)
	assert.True(t, ok)
	assert.Equal(t, p.BrandPrimary, c)

	_, ok = p.ColorByRole(Role(999))
	assert.False(t, ok)
}
