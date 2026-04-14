// tui/theme/theme.go
package theme

import "github.com/safedep/dry/tui/icon"

// Theme is the unit of design language injection. Every component reads the
// installed Theme at render time via Default(); no component caches the
// resolved theme across renders.
type Theme interface {
	Palette() Palette
	Icons() icon.Set
	Name() string
}

// basicTheme is a simple Theme-conforming struct used internally for the
// default SafeDep theme and as the output of From().
type basicTheme struct {
	name    string
	palette Palette
	icons   icon.Set
}

func (t *basicTheme) Palette() Palette { return t.palette }
func (t *basicTheme) Icons() icon.Set  { return t.icons }
func (t *basicTheme) Name() string     { return t.name }

// SafeDep returns the canonical SafeDep Theme.
//
// The returned value is a fresh copy on each call; callers may mutate it
// freely without affecting the global default.
func SafeDep() Theme {
	return &basicTheme{
		name:    "safedep",
		palette: safeDepPalette(),
		icons:   icon.DefaultSet(),
	}
}
