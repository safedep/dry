package tui

import (
	"os"
	"sync"

	"github.com/charmbracelet/colorprofile"
	"github.com/fatih/color"
)

// ColorConfig holds the terminal color configuration and context
type ColorConfig struct {
	profile colorprofile.Profile
}

// NewColorConfig creates a ColorConfig with the specified color profile.
// Use this to create a config with a specific profile (e.g., for testing
// or to force a particular color mode like colorprofile.Ascii for CI environments).
func NewColorConfig(profile colorprofile.Profile) *ColorConfig {
	return &ColorConfig{profile: profile}
}

var (
	globalColorConfig *ColorConfig
	colorConfigMu     sync.RWMutex
)

func init() {
	globalColorConfig = &ColorConfig{
		profile: colorprofile.Detect(os.Stdout, os.Environ()),
	}
}

// GetColorConfig returns the global color configuration.
// It is safe for concurrent use.
func GetColorConfig() *ColorConfig {
	colorConfigMu.RLock()
	defer colorConfigMu.RUnlock()
	return globalColorConfig
}

// SetColorConfig sets the global color configuration.
// If colorCfg is nil, the function does nothing.
// It is safe for concurrent use.
func SetColorConfig(colorCfg *ColorConfig) {
	if colorCfg == nil {
		return
	}
	colorConfigMu.Lock()
	defer colorConfigMu.Unlock()
	globalColorConfig = colorCfg
}

// IsColorEnabled returns true if colors are enabled for the current profile.
func (c *ColorConfig) IsColorEnabled() bool {
	return c.profile != colorprofile.NoTTY && c.profile != colorprofile.Ascii
}

type colorFn func(format string, a ...interface{}) string

// TerminalColors defines a collection of color formatting functions for terminal output.
// It provides both foreground text colors (Normal, Red, Yellow, etc.) and background
// "badge" styles (ErrorBg, WarningBg, etc.) for creating visually distinct terminal output.
type TerminalColors struct {
	Normal    colorFn
	Red       colorFn
	Yellow    colorFn
	Cyan      colorFn
	Green     colorFn
	Bold      colorFn
	Dim       colorFn
	ErrorCode colorFn

	// Background "badge" styles
	ErrorBg   colorFn
	WarningBg colorFn
	InfoBg    colorFn
	SuccessBg colorFn
	NeutralBg colorFn
}

var colors = TerminalColors{
	Normal:    color.New().SprintfFunc(),
	Red:       color.New(color.FgRed, color.Bold).SprintfFunc(),
	Yellow:    color.New(color.FgYellow).SprintfFunc(),
	Cyan:      color.New(color.FgCyan).SprintfFunc(),
	Green:     color.New(color.FgGreen).SprintfFunc(),
	Bold:      color.New(color.Bold).SprintfFunc(),
	Dim:       color.New(color.Faint).SprintfFunc(),
	ErrorCode: color.New(color.BgRed, color.FgBlack, color.Bold).SprintfFunc(),

	// Background "badge" color functions
	ErrorBg:   color.New(color.BgRed, color.FgBlack, color.Bold).SprintfFunc(),
	WarningBg: color.New(color.BgYellow, color.FgBlack).SprintfFunc(),
	InfoBg:    color.New(color.BgCyan, color.FgBlack).SprintfFunc(),
	SuccessBg: color.New(color.BgGreen, color.FgBlack).SprintfFunc(),
	NeutralBg: color.New(color.BgWhite, color.FgBlack).SprintfFunc(),
}

// Semantic foreground functions
//
// Rule of thumb:
// - ANSI/ANSI256/TrueColor: use the base colors definitions.

// InfoText returns informational foreground text.
func (c *ColorConfig) InfoText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.Cyan("%s", s)
}

// WarningText returns a warning foreground text in muted yellow.
func (c *ColorConfig) WarningText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.Yellow("%s", s)
}

// ErrorText returns an error foreground text in restrained red.
func (c *ColorConfig) ErrorText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.Red("%s", s)
}

// SuccessText returns a success foreground text in gentle green.
func (c *ColorConfig) SuccessText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.Green("%s", s)
}

// FaintText returns text with faint/dim styling (secondary information).
func (c *ColorConfig) FaintText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.Dim("%s", s)
}

// BoldText returns text with bold styling (use sparingly, e.g., titles).
func (c *ColorConfig) BoldText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.Bold("%s", s)
}

// ErrorCodeText returns text formatted as an error code badge.
func (c *ColorConfig) ErrorCodeText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.ErrorCode(" %s ", s)
}

// Background "badge" functions
//
// Use for compact labels such as error codes or status tags.
// Ensure high contrast without aggressive saturation.

// ErrorBgText returns text with an error badge background (high-contrast).
func (c *ColorConfig) ErrorBgText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.ErrorBg("%s", s)
}

// WarningBgText returns text with a warning badge background.
func (c *ColorConfig) WarningBgText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.WarningBg("%s", s)
}

// InfoBgText returns text with an informational badge background.
func (c *ColorConfig) InfoBgText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.InfoBg("%s", s)
}

// SuccessBgText returns text with a success badge background.
func (c *ColorConfig) SuccessBgText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.SuccessBg("%s", s)
}

// NeutralBgText returns text with a neutral (white) badge background, useful for
// non-severity labels. Keep it high-contrast but subtle.
func (c *ColorConfig) NeutralBgText(s string) string {
	if !c.IsColorEnabled() {
		return s
	}
	return colors.NeutralBg("%s", s)
}

// Global convenience functions that proxy to the global ColorConfig
//
// Foreground

func InfoText(s string) string      { return GetColorConfig().InfoText(s) }
func WarningText(s string) string   { return GetColorConfig().WarningText(s) }
func ErrorText(s string) string     { return GetColorConfig().ErrorText(s) }
func SuccessText(s string) string   { return GetColorConfig().SuccessText(s) }
func FaintText(s string) string     { return GetColorConfig().FaintText(s) }
func BoldText(s string) string      { return GetColorConfig().BoldText(s) }
func ErrorCodeText(s string) string { return GetColorConfig().ErrorCodeText(s) }

// Background badges

func ErrorBgText(s string) string   { return GetColorConfig().ErrorBgText(s) }
func WarningBgText(s string) string { return GetColorConfig().WarningBgText(s) }
func InfoBgText(s string) string    { return GetColorConfig().InfoBgText(s) }
func SuccessBgText(s string) string { return GetColorConfig().SuccessBgText(s) }
func NeutralBgText(s string) string { return GetColorConfig().NeutralBgText(s) }

// IsColorEnabled returns true if colors are enabled for the global color config.
func IsColorEnabled() bool { return GetColorConfig().IsColorEnabled() }
