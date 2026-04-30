// tui/theme/palette.go
package theme

import "github.com/charmbracelet/lipgloss"

// Palette is the exhaustive set of named colors used across tui components.
// It is intentionally CLOSED — no Custom map escape hatch. Runtime-keyed or
// product-specific palettes stay in the consuming tool.
type Palette struct {
	// Semantic
	Info    lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Muted   lipgloss.AdaptiveColor
	Text    lipgloss.AdaptiveColor
	Heading lipgloss.AdaptiveColor
	Path    lipgloss.AdaptiveColor

	// Diff
	DiffAdd lipgloss.AdaptiveColor // added lines (+); separate from Success so Success can be uncolored

	// Severity
	Critical lipgloss.AdaptiveColor
	High     lipgloss.AdaptiveColor
	Medium   lipgloss.AdaptiveColor
	Low      lipgloss.AdaptiveColor

	// Brand (SafeDep identity)
	BrandPrimary lipgloss.AdaptiveColor
	BrandAccent  lipgloss.AdaptiveColor

	// Background surfaces (for badges)
	BgCritical lipgloss.AdaptiveColor
	BgHigh     lipgloss.AdaptiveColor
	BgMedium   lipgloss.AdaptiveColor
	BgLow      lipgloss.AdaptiveColor
	BgInfo     lipgloss.AdaptiveColor
	BgSuccess  lipgloss.AdaptiveColor

	// BadgeText is the foreground color used inside saturated-background badges.
	// Kept as an explicit palette slot (rather than a hardcoded constant in the
	// style package) so high-contrast themes can override it.
	BadgeText lipgloss.AdaptiveColor
}

// Role enumerates every named color slot. Used by theme.WithColor.
type Role int

const (
	RoleInfo Role = iota
	RoleSuccess
	RoleWarning
	RoleError
	RoleMuted
	RoleText
	RoleHeading
	RolePath
	RoleDiffAdd
	RoleCritical
	RoleHigh
	RoleMedium
	RoleLow
	RoleBrandPrimary
	RoleBrandAccent
	RoleBgCritical
	RoleBgHigh
	RoleBgMedium
	RoleBgLow
	RoleBgInfo
	RoleBgSuccess
	RoleBadgeText
)

// ColorByRole returns the AdaptiveColor for a role, or (zero, false).
func (p Palette) ColorByRole(r Role) (lipgloss.AdaptiveColor, bool) {
	switch r {
	case RoleInfo:
		return p.Info, true
	case RoleSuccess:
		return p.Success, true
	case RoleWarning:
		return p.Warning, true
	case RoleError:
		return p.Error, true
	case RoleMuted:
		return p.Muted, true
	case RoleText:
		return p.Text, true
	case RoleHeading:
		return p.Heading, true
	case RolePath:
		return p.Path, true
	case RoleDiffAdd:
		return p.DiffAdd, true
	case RoleCritical:
		return p.Critical, true
	case RoleHigh:
		return p.High, true
	case RoleMedium:
		return p.Medium, true
	case RoleLow:
		return p.Low, true
	case RoleBrandPrimary:
		return p.BrandPrimary, true
	case RoleBrandAccent:
		return p.BrandAccent, true
	case RoleBgCritical:
		return p.BgCritical, true
	case RoleBgHigh:
		return p.BgHigh, true
	case RoleBgMedium:
		return p.BgMedium, true
	case RoleBgLow:
		return p.BgLow, true
	case RoleBgInfo:
		return p.BgInfo, true
	case RoleBgSuccess:
		return p.BgSuccess, true
	case RoleBadgeText:
		return p.BadgeText, true
	}
	return lipgloss.AdaptiveColor{}, false
}

// WithColorByRole returns a copy of p with the given role replaced.
func (p Palette) WithColorByRole(r Role, c lipgloss.AdaptiveColor) Palette {
	out := p
	switch r {
	case RoleInfo:
		out.Info = c
	case RoleSuccess:
		out.Success = c
	case RoleWarning:
		out.Warning = c
	case RoleError:
		out.Error = c
	case RoleMuted:
		out.Muted = c
	case RoleText:
		out.Text = c
	case RoleHeading:
		out.Heading = c
	case RolePath:
		out.Path = c
	case RoleDiffAdd:
		out.DiffAdd = c
	case RoleCritical:
		out.Critical = c
	case RoleHigh:
		out.High = c
	case RoleMedium:
		out.Medium = c
	case RoleLow:
		out.Low = c
	case RoleBrandPrimary:
		out.BrandPrimary = c
	case RoleBrandAccent:
		out.BrandAccent = c
	case RoleBgCritical:
		out.BgCritical = c
	case RoleBgHigh:
		out.BgHigh = c
	case RoleBgMedium:
		out.BgMedium = c
	case RoleBgLow:
		out.BgLow = c
	case RoleBgInfo:
		out.BgInfo = c
	case RoleBgSuccess:
		out.BgSuccess = c
	case RoleBadgeText:
		out.BadgeText = c
	}
	return out
}

// safeDepPalette returns the canonical SafeDep palette.
//
// CRITICAL: this is the ONLY place in the codebase that should contain hex
// color literals. A CI grep check enforces this.
func safeDepPalette() Palette {
	return Palette{
		// Semantic — only warnings and errors carry color; info and success use the
		// terminal's default foreground so routine messages don't compete visually
		// with actionable ones.
		Info:    lipgloss.AdaptiveColor{Light: "", Dark: ""},
		Success: lipgloss.AdaptiveColor{Light: "", Dark: ""},
		Warning: lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"}, // amber-700 / amber-300
		Error:   lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}, // red-700 / red-300
		Muted:   lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}, // slate-500 / slate-400
		Text:    lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"}, // gray-800 / gray-200
		Heading: lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F3F4F6"}, // gray-900 / gray-100
		Path:    lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#93C5FD"}, // blue-700 / blue-300

		// Diff — green for additions (functional color, not decorative).
		DiffAdd: lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#86EFAC"}, // green-700 / green-300

		// Severity — deeper tones on light, brighter on dark.
		Critical: lipgloss.AdaptiveColor{Light: "#991B1B", Dark: "#F87171"}, // red-800 / red-400
		High:     lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FDBA74"}, // orange-700 / orange-300
		Medium:   lipgloss.AdaptiveColor{Light: "#A16207", Dark: "#FDE68A"}, // yellow-700 / yellow-200
		Low:      lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#D1D5DB"}, // gray-600 / gray-300

		// Brand — SafeDep pink-red (origin: pmg banner).
		BrandPrimary: lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"}, // pink-700 / pink-400
		BrandAccent:  lipgloss.AdaptiveColor{Light: "#A21CAF", Dark: "#E879F9"}, // fuchsia-700 / fuchsia-400

		// Badge backgrounds — deep, saturated; foreground text on badges is always white.
		BgCritical: lipgloss.AdaptiveColor{Light: "#991B1B", Dark: "#991B1B"},
		BgHigh:     lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#C2410C"},
		BgMedium:   lipgloss.AdaptiveColor{Light: "#A16207", Dark: "#A16207"},
		BgLow:      lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#4B5563"},
		BgInfo:     lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#0E7490"},
		BgSuccess:  lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#15803D"},

		// Badge foreground — white on both light and dark, because badge
		// backgrounds are saturated in both modes.
		BadgeText: lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
	}
}
