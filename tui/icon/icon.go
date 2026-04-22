// tui/icon/icon.go
package icon

import "github.com/safedep/dry/tui/output"

// Icon is a glyph triple — one form per output mode.
//
// Unicode is used in Rich mode. Ascii replaces it in Plain mode (no UTF-8
// dependency). Agent is a parseable prefix used in Agent mode; it SHOULD
// end with a separator (':' by convention) so downstream log tailers can
// pattern-match messages.
type Icon struct {
	Unicode string
	Ascii   string
	Agent   string
}

// Resolve returns the form appropriate for the given mode.
func (i Icon) Resolve(m output.Mode) string {
	switch m {
	case output.Agent:
		return i.Agent
	case output.Plain:
		return i.Ascii
	default:
		return i.Unicode
	}
}

// IconKey names a canonical icon role.
type IconKey int

const (
	KeySuccess IconKey = iota
	KeyError
	KeyWarning
	KeyInfo
	KeyBullet
	KeyArrow
	KeySpinnerFrames
)

func AllKeys() []IconKey {
	return []IconKey{
		KeySuccess, KeyError, KeyWarning, KeyInfo,
		KeyBullet, KeyArrow, KeySpinnerFrames,
	}
}

// Set maps IconKey to Icon. Themes supply one of these.
type Set map[IconKey]Icon

// Get returns the icon for the key, or the zero Icon plus false.
func (s Set) Get(k IconKey) (Icon, bool) {
	i, ok := s[k]
	return i, ok
}

// With returns a copy of s with key k overridden by i.
func (s Set) With(k IconKey, i Icon) Set {
	out := make(Set, len(s)+1)
	for kk, vv := range s {
		out[kk] = vv
	}
	out[k] = i
	return out
}
