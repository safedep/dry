// tui/section/section.go
//
// Package section composes multi-part command output: titled blocks,
// next-step hints, empty states, and joining. Helpers degrade to
// undecorated text outside Rich mode.
package section

import (
	"strings"

	"github.com/safedep/dry/tui/icon"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
)

// Titled returns body under a styled section title. An empty body renders
// the title alone.
func Titled(title, body string) string {
	if body == "" {
		return style.Heading(title)
	}
	return style.Heading(title) + "\n" + body
}

// Hint returns a muted next-step guidance line prefixed with the theme's
// arrow glyph.
func Hint(text string) string {
	ic, _ := theme.Default().Icons().Get(icon.KeyArrow)
	return style.Faint(ic.Resolve(output.CurrentMode()) + " " + text)
}

// Empty returns a muted empty-state line for "no results" data output.
func Empty(text string) string {
	return style.Faint(text)
}

// Join concatenates non-blank parts with a blank line between them.
func Join(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		kept = append(kept, p)
	}
	return strings.Join(kept, "\n\n")
}
