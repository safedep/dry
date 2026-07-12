// tui/panel/panel.go
//
// Package panel renders a titled key-value detail card. It is the standard
// presentation for "show one thing" command output (a credential profile,
// an endpoint, a configuration). Rich mode draws a bordered card with the
// title embedded in the top border, vertical breathing room, and aligned
// muted labels; Plain and Agent modes degrade to aligned label/value lines
// with no decoration.
package panel

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// Layout constants for the Rich card. hpad is the space between the border
// and content on each side. gutter separates the label column from values.
const (
	hpad   = 3
	gutter = 3
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

// renderRich draws the card manually rather than via lipgloss borders so
// the title can live on the top border line. lipgloss v1 has no native
// border-title support.
func (p *Panel) renderRich(labelW int) string {
	pal := theme.Default().Palette()
	mutedC, _ := pal.ColorByRole(theme.RoleMuted)
	headingC, _ := pal.ColorByRole(theme.RoleHeading)

	borderStyle := lipgloss.NewStyle()
	labelStyle := lipgloss.NewStyle()
	titleStyle := lipgloss.NewStyle()
	if output.IsColorEnabled() {
		borderStyle = borderStyle.Foreground(mutedC)
		labelStyle = labelStyle.Foreground(mutedC)
		titleStyle = titleStyle.Bold(true).Foreground(headingC)
	}

	rows := make([]string, 0, len(p.fields))
	contentW := 0
	for _, f := range p.fields {
		row := labelStyle.Render(pad(f.label, labelW)) + strings.Repeat(" ", gutter) + f.value
		rows = append(rows, row)
		if w := lipgloss.Width(row); w > contentW {
			contentW = w
		}
	}

	// The top border must fit "─ title " plus trailing dashes even when
	// the title is wider than every row.
	titleW := lipgloss.Width(p.title)
	if p.title != "" && titleW+5 > contentW+2*hpad {
		contentW = titleW + 5 - 2*hpad
	}
	inner := contentW + 2*hpad

	var b strings.Builder
	b.WriteString(topBorder(p.title, inner, borderStyle, titleStyle))
	if len(rows) > 0 {
		blank := borderStyle.Render("│") + strings.Repeat(" ", inner) + borderStyle.Render("│")
		b.WriteString("\n" + blank)
		for _, row := range rows {
			b.WriteString("\n" + borderStyle.Render("│") + strings.Repeat(" ", hpad) +
				row + strings.Repeat(" ", inner-hpad-lipgloss.Width(row)) + borderStyle.Render("│"))
		}
		b.WriteString("\n" + blank)
	}
	b.WriteString("\n" + borderStyle.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return b.String()
}

func topBorder(title string, inner int, borderStyle, titleStyle lipgloss.Style) string {
	if title == "" {
		return borderStyle.Render("╭" + strings.Repeat("─", inner) + "╮")
	}
	fill := inner - lipgloss.Width(title) - 3
	return borderStyle.Render("╭─ ") + titleStyle.Render(title) +
		borderStyle.Render(" "+strings.Repeat("─", fill)+"╮")
}

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
