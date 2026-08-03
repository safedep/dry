// tui/meter/meter.go
//
// Package meter renders static usage meters: a labelled fill/track bar with an
// already-formatted value, for point-in-time "how much of X is used" output
// (quota consumption, overage against a cap). Unlike tui/progress, which
// animates in-flight work on stderr, a meter is a stable snapshot rendered into
// normal command output.
//
// Rich mode draws an aligned fill (█) over a track (░) per line; Plain and Agent
// degrade to one "label: value/max (pct)" line each. A bar is monochrome unless
// Warn is set, matching the house style where color signals only trouble.
package meter

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// richBarWidth is the fixed cell width of the Rich fill/track bar.
const richBarWidth = 26

const (
	fillRune  = "█"
	trackRune = "░"
)

// Bar is one usage meter. Value out of Max drives the fill; ValueText is the
// already-formatted reading shown after the bar (the caller owns formatting, so
// the same primitive serves a unit bar and a money bar). Warn colors the fill
// when the reading is at or near its ceiling; an unset Warn stays monochrome. A
// non-positive Max renders an empty track.
type Bar struct {
	Label     string
	Value     int64
	Max       int64
	ValueText string
	Warn      bool
}

// Render lays the bars out for the active output mode.
func Render(bars ...Bar) string {
	if output.CurrentMode() != output.Rich {
		return renderPlain(bars)
	}
	return renderRich(bars)
}

func renderPlain(bars []Bar) string {
	var b strings.Builder
	for i, bar := range bars {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s: %d/%d (%d%%)", bar.Label, bar.Value, bar.Max, percent(bar.Value, bar.Max))
	}
	return b.String()
}

func renderRich(bars []Bar) string {
	pal := theme.Default().Palette()
	mutedC, _ := pal.ColorByRole(theme.RoleMuted)
	labelStyle := lipgloss.NewStyle().Foreground(mutedC)

	// Uniform label column so bars line up across rows.
	labelW := 0
	for _, bar := range bars {
		if w := lipgloss.Width(bar.Label); w > labelW {
			labelW = w
		}
	}

	var b strings.Builder
	for i, bar := range bars {
		if i > 0 {
			b.WriteByte('\n')
		}
		label := labelStyle.Render(fmt.Sprintf("%-*s", labelW, bar.Label))
		fmt.Fprintf(&b, "%s  %s  %s", label, renderTrack(bar), bar.ValueText)
	}
	return b.String()
}

func renderTrack(bar Bar) string {
	filled := fillCells(bar.Value, bar.Max, richBarWidth)
	fill := strings.Repeat(fillRune, filled)
	if bar.Warn && output.IsColorEnabled() {
		if c, ok := theme.Default().Palette().ColorByRole(warnRole(bar)); ok {
			fill = lipgloss.NewStyle().Foreground(c).Render(fill)
		}
	}
	return fill + strings.Repeat(trackRune, richBarWidth-filled)
}

// warnRole escalates a warning bar to the error role once it is at or over its
// ceiling, so a full bar reads as red and a near-full one as amber.
func warnRole(bar Bar) theme.Role {
	if bar.Max > 0 && bar.Value >= bar.Max {
		return theme.RoleError
	}
	return theme.RoleWarning
}

// fillCells is the number of filled cells for value out of max over width,
// clamped to [0, width]. A non-positive max is an empty track.
func fillCells(value, max, width int64) int {
	if max <= 0 || value <= 0 {
		return 0
	}
	if value >= max {
		return int(width)
	}
	// Round to the nearest cell so a bar reaches full only at value == max.
	cells := (value*width + max/2) / max
	if cells > width {
		cells = width
	}
	return int(cells)
}

// percent is value/max as an integer percentage, clamped to [0, 100].
func percent(value, max int64) int {
	if max <= 0 || value <= 0 {
		return 0
	}
	if value >= max {
		return 100
	}
	return int((value * 100) / max)
}
