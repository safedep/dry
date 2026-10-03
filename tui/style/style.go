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
	bgRole := badgeBgFor(r)
	bg, ok := pal.ColorByRole(bgRole)
	if !ok {
		return fmt.Sprintf("[%s]", text)
	}
	fg, _ := pal.ColorByRole(theme.RoleBadgeText)
	bgColor, fgColor := badgeColors(bgRole, bg, fg)
	return lipgloss.NewStyle().
		Background(bgColor).
		Foreground(fgColor).
		Padding(0, 1).
		Bold(true).
		Render(text)
}

// badgeCodes are the 256-colour and 16-colour codes of the SafeDep badge
// backgrounds, with a text colour that a person can read on each.
// lipgloss maps a hex colour to the nearest code, and the orange of high
// and the amber of medium map to the same code in both tables. A badge
// with its own codes keeps the severities apart on every terminal.
var badgeCodes = map[theme.Role]struct{ bg256, bg16, fg16 string }{
	theme.RoleBgCritical: {bg256: "124", bg16: "1", fg16: "15"},
	theme.RoleBgHigh:     {bg256: "166", bg16: "9", fg16: "15"},
	theme.RoleBgMedium:   {bg256: "136", bg16: "3", fg16: "0"},
	theme.RoleBgLow:      {bg256: "240", bg16: "8", fg16: "15"},
	theme.RoleBgInfo:     {bg256: "30", bg16: "6", fg16: "0"},
	theme.RoleBgSuccess:  {bg256: "28", bg16: "2", fg16: "0"},
}

// badgeColors returns the colours of a badge. A badge of the SafeDep
// palette gets its own 256-colour and 16-colour codes. A badge colour that
// a theme sets keeps the nearest code of lipgloss. A badge background is
// the same on a light and a dark terminal, so the colours do not need the
// background of the terminal, and lipgloss does not ask the terminal for it.
func badgeColors(role theme.Role, bg, fg lipgloss.AdaptiveColor) (lipgloss.TerminalColor, lipgloss.TerminalColor) {
	codes, ok := badgeCodes[role]
	base, _ := theme.SafeDep().Palette().ColorByRole(role)
	baseFG, _ := theme.SafeDep().Palette().ColorByRole(theme.RoleBadgeText)
	if !ok || bg != base || fg != baseFG || bg.Light != bg.Dark || fg.Light != fg.Dark {
		return bg, fg
	}
	return lipgloss.CompleteColor{TrueColor: bg.Dark, ANSI256: codes.bg256, ANSI: codes.bg16},
		lipgloss.CompleteColor{TrueColor: fg.Dark, ANSI256: "15", ANSI: codes.fg16}
}

// badgeBgFor maps a semantic/severity role to its matching Bg* role.
// Callers pass the semantic role (RoleCritical); we return the bg role (RoleBgCritical).
func badgeBgFor(r theme.Role) theme.Role {
	switch r {
	case theme.RoleError:
		return theme.RoleBgCritical
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
