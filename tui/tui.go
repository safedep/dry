// tui/tui.go
//
// Package tui is SafeDep's unified terminal output library. Callers typically
// only use the top-level helpers declared here; finer-grained control lives
// in the subpackages (theme, output, icon, ...).
package tui

import (
	"fmt"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
)

// Info prints a formatted info-level line to stderr, honoring the active
// theme, mode, and verbosity. Suppressed when verbosity is Silent.
func Info(format string, a ...any) {
	if output.CurrentVerbosity() <= output.Silent {
		return
	}
	fmt.Fprintln(output.Stderr(), style.Info(fmt.Sprintf(format, a...)))
}

// Success prints a formatted success-level line to stderr. Suppressed when
// verbosity is Silent.
func Success(format string, a ...any) {
	if output.CurrentVerbosity() <= output.Silent {
		return
	}
	fmt.Fprintln(output.Stderr(), style.Success(fmt.Sprintf(format, a...)))
}

// Warning prints a formatted warning-level line to stderr. Always shown,
// regardless of verbosity (warnings must not be hidden by --silent).
func Warning(format string, a ...any) {
	fmt.Fprintln(output.Stderr(), style.Warning(fmt.Sprintf(format, a...)))
}

// Error prints a formatted error-level line to stderr. Always shown.
func Error(format string, a ...any) {
	fmt.Fprintln(output.Stderr(), style.Error(fmt.Sprintf(format, a...)))
}

// Faint prints a muted line to stderr. Only shown when verbosity is Verbose.
func Faint(format string, a ...any) {
	if output.CurrentVerbosity() < output.Verbose {
		return
	}
	fmt.Fprintln(output.Stderr(), style.Faint(fmt.Sprintf(format, a...)))
}

// Heading prints a bold accent line to stderr.
func Heading(text string) {
	fmt.Fprintln(output.Stderr(), style.Heading(text))
}

// Badge returns a pre-styled inline badge for embedding in cells or messages.
// Does not print; callers pass the returned string into Info/Table/etc.
func Badge(role theme.Role, text string) string {
	return style.Badge(role, text)
}

// Print writes a Renderable to stderr using the current theme and mode.
// Convenience dispatch for callers who implement Renderable on their types.
func Print(r Renderable) {
	fmt.Fprintln(output.Stderr(), r.Render(theme.Default(), output.CurrentMode()))
}
