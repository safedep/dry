// tui/style/style.go
package style

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/icon"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// Info returns a styled "info"-role string: colored+iconned in Rich,
// ASCII-iconned in Plain, agent-prefixed in Agent.
func Info(s string) string    { return render(icon.KeyInfo, theme.RoleInfo, s) }
func Success(s string) string { return render(icon.KeySuccess, theme.RoleSuccess, s) }
func Warning(s string) string { return render(icon.KeyWarning, theme.RoleWarning, s) }
func Error(s string) string   { return render(icon.KeyError, theme.RoleError, s) }

// Faint returns muted text with no icon prefix.
func Faint(s string) string {
	if !output.IsColorEnabled() || output.CurrentMode() != output.Rich {
		return s
	}
	c, _ := theme.Default().Palette().ColorByRole(theme.RoleMuted)
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

// Heading returns bold accent-colored text with no icon prefix.
func Heading(s string) string {
	if !output.IsColorEnabled() || output.CurrentMode() != output.Rich {
		return s
	}
	c, _ := theme.Default().Palette().ColorByRole(theme.RoleHeading)
	return lipgloss.NewStyle().Bold(true).Foreground(c).Render(s)
}

// Path returns file-path-styled text. No icon.
func Path(s string) string {
	if !output.IsColorEnabled() || output.CurrentMode() != output.Rich {
		return s
	}
	c, _ := theme.Default().Palette().ColorByRole(theme.RolePath)
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

// Badge returns a padded, background-filled inline badge for use inside cells
// or prose (e.g., severity labels). In Plain/Agent it degrades to bracketed text.
func Badge(r theme.Role, text string) string {
	mode := output.CurrentMode()
	if mode != output.Rich || !output.IsColorEnabled() {
		return fmt.Sprintf("[%s]", text)
	}
	pal := theme.Default().Palette()
	bg, ok := pal.ColorByRole(badgeBgFor(r))
	if !ok {
		return fmt.Sprintf("[%s]", text)
	}
	fg, _ := pal.ColorByRole(theme.RoleBadgeText)
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(fg).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// badgeBgFor maps a semantic/severity role to its matching Bg* role.
// Callers pass the semantic role (RoleCritical); we return the bg role (RoleBgCritical).
func badgeBgFor(r theme.Role) theme.Role {
	switch r {
	case theme.RoleCritical:
		return theme.RoleBgCritical
	case theme.RoleHigh:
		return theme.RoleBgHigh
	case theme.RoleMedium:
		return theme.RoleBgMedium
	case theme.RoleLow:
		return theme.RoleBgLow
	case theme.RoleInfo:
		return theme.RoleBgInfo
	case theme.RoleSuccess:
		return theme.RoleBgSuccess
	}
	return r
}

// render is the shared implementation for Info/Success/Warning/Error.
func render(k icon.IconKey, r theme.Role, text string) string {
	mode := output.CurrentMode()
	ic, _ := theme.Default().Icons().Get(k)
	glyph := ic.Resolve(mode)

	switch mode {
	case output.Agent:
		return fmt.Sprintf("%s %s", glyph, text)
	case output.Plain:
		return fmt.Sprintf("%s %s", glyph, text)
	}
	// Rich.
	if !output.IsColorEnabled() {
		return fmt.Sprintf("%s %s", glyph, text)
	}
	c, _ := theme.Default().Palette().ColorByRole(r)
	styled := lipgloss.NewStyle().Foreground(c).Render(glyph + " " + text)
	return styled
}
