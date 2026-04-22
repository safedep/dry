// tui/renderable.go
package tui

import (
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// Theme re-exports theme.Theme so Renderable users don't need a second import.
type Theme = theme.Theme

// Renderable is the extension point for external types that want to flow
// through tui.Print. Implementations MUST be pure: no I/O, no global reads
// beyond the passed Theme and Mode, no time-dependent output.
//
// Every built-in component (Table, Banner, Diff, Badge) produces its rendered
// form from (theme, mode); Renderable formalizes that contract so callers can
// plug in their own domain types (e.g., a vet Finding that renders itself).
type Renderable interface {
	Render(t Theme, m output.Mode) string
}
