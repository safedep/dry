package tui

import (
	"fmt"
	"os"
	"strings"
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

// GetColorConfig returns a copy of the global color configuration.
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

/*
Semantic foreground functions

Rule of thumb:
- ANSI/ANSI256/TrueColor: use the base colors definitions.
*/

// InfoText returns informational foreground text.
func (c *ColorConfig) InfoText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.Cyan("%s", s)
}

// WarningText returns a warning foreground text in muted yellow.
func (c *ColorConfig) WarningText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.Yellow("%s", s)
}

// ErrorText returns an error foreground text in restrained red.
func (c *ColorConfig) ErrorText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.Red("%s", s)
}

// SuccessText returns a success foreground text in gentle green.
func (c *ColorConfig) SuccessText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.Green("%s", s)
}

// FaintText returns text with faint/dim styling (secondary information).
func (c *ColorConfig) FaintText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.Dim("%s", s)
}

// BoldText returns text with bold styling (use sparingly, e.g., titles).
func (c *ColorConfig) BoldText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.Bold("%s", s)
}

// MinimalError formats an error message with code, message, and optional hint.
// It returns a formatted string suitable for minimal error output, respecting
// the terminal's color profile (no colors for NoTTY/Ascii terminals).
func (c *ColorConfig) MinimalError(code, msg, hint string) string {
	var sb strings.Builder
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		sb.WriteString(fmt.Sprintf("%s  %s\n", code, msg))

		if hint != "" {
			sb.WriteString(fmt.Sprintf(" %s %s\n", "→", hint))
		}
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("%s  %s\n", colors.ErrorCode(" %s ", code), colors.Red(msg)))
	if hint != "" {
		sb.WriteString(fmt.Sprintf(" %s %s\n", colors.Dim("→"), colors.Dim(hint)))
	}

	return sb.String()
}

// VerboseError formats a detailed error message with code, message, hint,
// additional help text, and the original error string. It returns a formatted
// string suitable for verbose error output, respecting the terminal's color
// profile (no colors for NoTTY/Ascii terminals).
func (c *ColorConfig) VerboseError(code, msg, hint, additionalHelp, originalError string) string {
	var sb strings.Builder
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		sb.WriteString(fmt.Sprintf("%s  %s\n", code, msg))

		if hint != "" {
			sb.WriteString(fmt.Sprintf(" %s %s\n", "→", hint))
		}

		if additionalHelp != "" {
			sb.WriteString(fmt.Sprintf(" %s %s\n", "→", additionalHelp))
		}

		if originalError != "" {
			sb.WriteString(fmt.Sprintf(" %s %s\n", "┄", originalError))
		}

		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("%s  %s\n", colors.ErrorCode(" %s ", code), colors.Red(msg)))

	if hint != "" {
		sb.WriteString(fmt.Sprintf(" %s %s\n", colors.Dim("→"), colors.Dim(hint)))
	}

	if additionalHelp != "" {
		sb.WriteString(fmt.Sprintf(" %s %s\n", colors.Dim("→"), colors.Dim(additionalHelp)))
	}

	if originalError != "" {
		sb.WriteString(fmt.Sprintf(" %s %s\n", colors.Dim("┄"), colors.Dim(originalError)))
	}

	return sb.String()
}

/*
Background “badge” functions

Use for compact labels such as error codes or status tags.
Ensure high contrast without aggressive saturation.
*/

// ErrorBgText returns text with an error badge background (high-contrast).
func (c *ColorConfig) ErrorBgText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.ErrorBg("%s", s)
}

// WarningBgText returns text with a warning badge background.
func (c *ColorConfig) WarningBgText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.WarningBg("%s", s)
}

// InfoBgText returns text with an informational badge background.
func (c *ColorConfig) InfoBgText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.InfoBg("%s", s)
}

// SuccessBgText returns text with a success badge background.
func (c *ColorConfig) SuccessBgText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.SuccessBg("%s", s)
}

// NeutralBgText returns text with a neutral (white) badge background, useful for
// non-severity labels. Keep it high-contrast but subtle.
func (c *ColorConfig) NeutralBgText(s string) string {
	if c.profile == colorprofile.NoTTY || c.profile == colorprofile.Ascii {
		return s
	}
	return colors.NeutralBg("%s", s)
}

/*
Global convenience functions that proxy to the global ColorConfig

Foreground
*/

func InfoText(s string) string    { return GetColorConfig().InfoText(s) }
func WarningText(s string) string { return GetColorConfig().WarningText(s) }
func ErrorText(s string) string   { return GetColorConfig().ErrorText(s) }
func SuccessText(s string) string { return GetColorConfig().SuccessText(s) }
func FaintText(s string) string   { return GetColorConfig().FaintText(s) }
func BoldText(s string) string    { return GetColorConfig().BoldText(s) }

/*
 * Internal
 */
func printMinimalError(code, message, hint string) {
	str := GetColorConfig().MinimalError(code, message, hint)

	fmt.Print(str)
}

func printVerboseError(code, message, hint, additionalHelp, originalError string) {
	str := GetColorConfig().VerboseError(code, message, hint, additionalHelp, originalError)

	fmt.Print(str)
}

/*
Background badges
*/

func ErrorBgText(s string) string   { return GetColorConfig().ErrorBgText(s) }
func WarningBgText(s string) string { return GetColorConfig().WarningBgText(s) }
func InfoBgText(s string) string    { return GetColorConfig().InfoBgText(s) }
func SuccessBgText(s string) string { return GetColorConfig().SuccessBgText(s) }
func NeutralBgText(s string) string { return GetColorConfig().NeutralBgText(s) }
