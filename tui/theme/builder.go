// tui/theme/builder.go
package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/icon"
)

// Option configures a Theme built by From.
type Option func(*builder)

type builder struct {
	name    string
	palette Palette
	icons   icon.Set
}

// From builds a new Theme by copying base and applying options. base is not
// mutated. Use this to tweak a handful of colors or icons without writing a
// full Theme implementation.
func From(base Theme, opts ...Option) Theme {
	b := &builder{
		name:    base.Name(),
		palette: base.Palette(),
		icons:   cloneIcons(base.Icons()),
	}
	for _, o := range opts {
		o(b)
	}
	return &basicTheme{
		name:    b.name,
		palette: b.palette,
		icons:   b.icons,
	}
}

// WithColor overrides one palette slot.
func WithColor(r Role, c lipgloss.AdaptiveColor) Option {
	return func(b *builder) { b.palette = b.palette.WithColorByRole(r, c) }
}

// WithIcon overrides one icon slot.
func WithIcon(k icon.IconKey, i icon.Icon) Option {
	return func(b *builder) { b.icons = b.icons.With(k, i) }
}

// WithName sets the Theme's Name(). Useful when a tool ships multiple derived
// themes and wants them identifiable in logs.
func WithName(name string) Option {
	return func(b *builder) { b.name = name }
}

func cloneIcons(s icon.Set) icon.Set {
	out := make(icon.Set, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}
