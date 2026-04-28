// tui/console.go
package tui

import (
	"fmt"
	"io"

	"github.com/safedep/dry/tui/icon"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// Console is a value-typed wrapper that can override theme, mode, and writer
// without touching global state. Intended for tests and exceptional callers;
// production code should use the package-level functions (Info, Success, ...).
type Console struct {
	theme  theme.Theme
	mode   output.Mode
	writer io.Writer
}

// ConsoleOption configures a Console built by NewConsole.
type ConsoleOption func(*Console)

// WithTheme overrides the theme used by this Console.
func WithTheme(t theme.Theme) ConsoleOption { return func(c *Console) { c.theme = t } }

// WithMode overrides the output mode used by this Console.
func WithMode(m output.Mode) ConsoleOption { return func(c *Console) { c.mode = m } }

// WithWriter overrides the writer used by this Console. The writer is NOT
// mutex-serialized automatically — callers are responsible for synchronizing
// if they share it across goroutines. (Contrast with the global output.Stderr
// which wraps in a lockedWriter.)
func WithWriter(w io.Writer) ConsoleOption { return func(c *Console) { c.writer = w } }

// NewConsole builds a Console with the given options, defaulting to the
// current global theme/mode/writer for anything unset.
func NewConsole(opts ...ConsoleOption) *Console {
	c := &Console{
		theme:  theme.Default(),
		mode:   output.CurrentMode(),
		writer: output.Stderr(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Info mirrors the package-level Info but uses this Console's overrides.
func (c *Console) Info(format string, a ...any) {
	c.writeln(c.renderLine(lineInfo, fmt.Sprintf(format, a...)))
}

// Success mirrors the package-level Success.
func (c *Console) Success(format string, a ...any) {
	c.writeln(c.renderLine(lineSuccess, fmt.Sprintf(format, a...)))
}

// Warning mirrors the package-level Warning.
func (c *Console) Warning(format string, a ...any) {
	c.writeln(c.renderLine(lineWarning, fmt.Sprintf(format, a...)))
}

// Error mirrors the package-level Error.
func (c *Console) Error(format string, a ...any) {
	c.writeln(c.renderLine(lineError, fmt.Sprintf(format, a...)))
}

func (c *Console) writeln(s string) {
	_, _ = fmt.Fprintln(c.writer, s)
}

// lineRole identifies the semantic role of a Console output line.
type lineRole int

const (
	lineInfo lineRole = iota
	lineSuccess
	lineWarning
	lineError
)

func (c *Console) renderLine(r lineRole, text string) string {
	return consoleRender(c.theme, c.mode, r, text)
}

// consoleRender intentionally mirrors the package helpers' icon/prefix
// behavior using explicit (theme, mode). Console is primarily a testing seam,
// so this avoids reaching back into global mode/theme state without promising
// byte-for-byte parity with the richer style package internals.
func consoleRender(t theme.Theme, m output.Mode, r lineRole, text string) string {
	ic, _ := t.Icons().Get(iconKeyForLine(r))
	glyph := ic.Resolve(m)
	return fmt.Sprintf("%s %s", glyph, text)
}

func iconKeyForLine(r lineRole) icon.IconKey {
	switch r {
	case lineSuccess:
		return icon.KeySuccess
	case lineWarning:
		return icon.KeyWarning
	case lineError:
		return icon.KeyError
	default:
		return icon.KeyInfo
	}
}
