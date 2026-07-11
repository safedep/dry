// tui/panel/panel.go
//
// Package panel renders a titled key-value detail card. It is the standard
// presentation for "show one thing" command output (a credential profile,
// an endpoint, a configuration). Rich mode draws a bordered card with
// aligned muted labels; Plain and Agent modes degrade to aligned
// label/value lines with no decoration.
package panel

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
)

type field struct {
	label string
	value string
}

// Panel is a fluent builder for a titled key-value card.
type Panel struct {
	title  string
	fields []field
}

// New returns an empty Panel. An empty title renders a card without a
// heading.
func New(title string) *Panel { return &Panel{title: title} }

// Field appends one label/value pair. Values may be pre-styled (badges,
// colored text); alignment is ANSI-aware.
func (p *Panel) Field(label, value string) *Panel {
	p.fields = append(p.fields, field{label: label, value: value})
	return p
}

// FieldIf appends the pair only when present is true. Convenience for
// optional fields so call sites stay fluent.
func (p *Panel) FieldIf(present bool, label, value string) *Panel {
	if present {
		return p.Field(label, value)
	}
	return p
}

// Render returns the panel as a string, respecting the active output mode.
func (p *Panel) Render() string {
	labelW := 0
	for _, f := range p.fields {
		if w := lipgloss.Width(f.label); w > labelW {
			labelW = w
		}
	}
	if output.CurrentMode() != output.Rich {
		return p.renderBasic(labelW)
	}
	return p.renderRich(labelW)
}

func (p *Panel) renderBasic(labelW int) string {
	var b strings.Builder
	if p.title != "" {
		b.WriteString(p.title + "\n")
	}
	for _, f := range p.fields {
		b.WriteString(pad(f.label+":", labelW+1) + "  " + f.value + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (p *Panel) renderRich(labelW int) string {
	mutedC, _ := theme.Default().Palette().ColorByRole(theme.RoleMuted)
	labelStyle := lipgloss.NewStyle().Foreground(mutedC)

	var lines []string
	if p.title != "" {
		lines = append(lines, style.Heading(p.title))
		if len(p.fields) > 0 {
			lines = append(lines, "")
		}
	}
	for _, f := range p.fields {
		lines = append(lines, labelStyle.Render(pad(f.label, labelW))+"  "+f.value)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(mutedC).
		Padding(0, 2)
	return box.Render(strings.Join(lines, "\n"))
}

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
