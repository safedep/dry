// tui/stat/stat.go
//
// Package stat renders a row of metric summary cards for status and
// dashboard-style command output. Rich mode draws uniform bordered cards
// that wrap to the terminal width; Plain and Agent modes degrade to one
// label/value line per card.
package stat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// Card is one metric: a short label and an already-formatted value.
// Accent colors the value in Rich mode. Nil means the default text color.
type Card struct {
	Label  string
	Value  string
	Accent *theme.Role
}

// Render returns the cards laid out for the active output mode.
func Render(cards ...Card) string {
	if output.CurrentMode() != output.Rich {
		var b strings.Builder
		for _, c := range cards {
			fmt.Fprintf(&b, "%s: %s\n", c.Label, c.Value)
		}
		return strings.TrimRight(b.String(), "\n")
	}
	return renderRich(cards)
}

func renderRich(cards []Card) string {
	pal := theme.Default().Palette()
	mutedC, _ := pal.ColorByRole(theme.RoleMuted)
	labelStyle := lipgloss.NewStyle().Foreground(mutedC)

	// Uniform card width keeps the row visually even regardless of which
	// metric happens to hold the longest value.
	contentW := 0
	for _, c := range cards {
		if w := lipgloss.Width(c.Value); w > contentW {
			contentW = w
		}
		if w := lipgloss.Width(c.Label); w > contentW {
			contentW = w
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(mutedC).
		Padding(0, 2).
		Width(contentW + 4)

	rendered := make([]string, 0, len(cards))
	for _, c := range cards {
		valueStyle := lipgloss.NewStyle().Bold(true)
		if c.Accent != nil {
			if col, ok := pal.ColorByRole(*c.Accent); ok {
				valueStyle = valueStyle.Foreground(col)
			}
		}
		content := valueStyle.Render(c.Value) + "\n" + labelStyle.Render(c.Label)
		rendered = append(rendered, box.Render(content))
	}
	return joinWrapped(rendered, output.Width())
}

// joinWrapped joins card boxes horizontally, wrapping to a new row when the
// next card would overflow the terminal width.
func joinWrapped(cards []string, width int) string {
	var rows []string
	var row []string
	rowW := 0
	for _, c := range cards {
		w := lipgloss.Width(c)
		if len(row) > 0 && rowW+w > width {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
			row, rowW = nil, 0
		}
		row = append(row, c)
		rowW += w
	}
	if len(row) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}
