// tui/theme/palette_test.go
package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeDepPaletteNonEmpty(t *testing.T) {
	p := safeDepPalette()
	// Spot-check that all required fields are populated. We assert on the
	// Dark half of each AdaptiveColor; Light is exercised by integration
	// tests that force --light mode.
	assert.NotEmpty(t, p.Info.Dark)
	assert.NotEmpty(t, p.Success.Dark)
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
