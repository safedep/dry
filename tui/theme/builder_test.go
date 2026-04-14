// tui/theme/builder_test.go
package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/icon"
)

func TestFromWithColor(t *testing.T) {
	base := SafeDep()
	// Reuse an existing palette color as the override value — this keeps all
	// hex literals confined to palette.go (CI-enforced discipline).
	override := base.Palette().BrandAccent
	custom := From(base, WithColor(RoleBrandPrimary, override))

	assert.Equal(t, override, custom.Palette().BrandPrimary)
	// Base untouched: its BrandPrimary is still the SafeDep pink, not the accent.
	assert.NotEqual(t, override, base.Palette().BrandPrimary)
}

func TestFromWithName(t *testing.T) {
	custom := From(SafeDep(), WithName("my-theme"))
	assert.Equal(t, "my-theme", custom.Name())
}

func TestFromWithIcon(t *testing.T) {
	tick := icon.Icon{Unicode: "✔", Ascii: "[DONE]", Agent: "DONE:"}
	custom := From(SafeDep(), WithIcon(icon.KeySuccess, tick))

	got, ok := custom.Icons().Get(icon.KeySuccess)
	assert.True(t, ok)
	assert.Equal(t, "✔", got.Unicode)
}
