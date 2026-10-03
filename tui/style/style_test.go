// tui/style/style_test.go
package style

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStyleStripsColorInPlainMode(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	got := Success("done")
	// Plain mode: icon prefix + space + text, no ANSI escapes.
	assert.Equal(t, "[OK] done", got)
	assert.NotContains(t, got, "\x1b[")
}

func TestStyleAgentPrefixInAgentMode(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	got := Error("boom")
	assert.True(t, strings.HasPrefix(got, "ERR: "))
	assert.NotContains(t, got, "\x1b[")
}

func TestStyleRichEmitsAnsiUnlessNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	output.SetMode(output.Rich)
	defer output.SetMode(output.Rich)

	got := Success("done")
	// In Rich we expect the unicode icon at least.
	assert.True(t, strings.Contains(got, "✓"))
}

func TestErrorBadgeUsesDarkBackground(t *testing.T) {
	assert.Equal(t, theme.RoleBgCritical, badgeBgFor(theme.RoleError))
}

func TestBadgeColorsKeepTheSeveritiesApart(t *testing.T) {
	pal := theme.SafeDep().Palette()
	fg, _ := pal.ColorByRole(theme.RoleBadgeText)
	seen256, seen16 := map[string]theme.Role{}, map[string]theme.Role{}
	for _, role := range []theme.Role{theme.RoleBgCritical, theme.RoleBgHigh, theme.RoleBgMedium, theme.RoleBgLow, theme.RoleBgInfo} {
		bg, _ := pal.ColorByRole(role)
		bgColor, fgColor := badgeColors(role, bg, fg)
		c, ok := bgColor.(lipgloss.CompleteColor)
		require.True(t, ok, "role %d gets its own codes", role)
		assert.Equal(t, bg.Dark, c.TrueColor, "the true colour stays the palette colour")
		assert.NotContains(t, seen256, c.ANSI256, "role %d shares a 256-colour code with role %d", role, seen256[c.ANSI256])
		assert.NotContains(t, seen16, c.ANSI, "role %d shares a 16-colour code with role %d", role, seen16[c.ANSI])
		seen256[c.ANSI256], seen16[c.ANSI] = role, role
		_, ok = fgColor.(lipgloss.CompleteColor)
		assert.True(t, ok)
	}
}

func TestBadgeColorsKeepAThemeColor(t *testing.T) {
	custom := lipgloss.AdaptiveColor{Light: "#123456", Dark: "#123456"}
	fg, _ := theme.SafeDep().Palette().ColorByRole(theme.RoleBadgeText)
	bgColor, _ := badgeColors(theme.RoleBgHigh, custom, fg)
	assert.Equal(t, custom, bgColor)
}
