// tui/meter/meter.go
//
// Package meter renders static usage meters: a labelled fill/track bar with an
// already-formatted value, for point-in-time "how much of X is used" output
// (quota consumption, overage against a cap). Unlike tui/progress, which
// animates in-flight work on stderr, a meter is a stable snapshot rendered into
// normal command output.
//
// Rich mode draws an aligned fill (█) over a track (░) per line; Plain and Agent
// degrade to one "label: value/max (pct)" line each. The bar fill is monochrome
// unless Warn is set — color on the fill signals only trouble. Labels use the
// muted role like the sibling panel and stat components.
package meter

import (
	"fmt"
	"math"
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

	// Uniform label column so bars line up across rows. Width is cell-aware
	// (lipgloss), and lipgloss pads to that width the same way, so labels with
	// double-width runes still align — fmt's code-point padding would not.
	labelW := 0
	for _, bar := range bars {
		if w := lipgloss.Width(bar.Label); w > labelW {
			labelW = w
		}
	}
	labelStyle = labelStyle.Width(labelW)

	var b strings.Builder
	for i, bar := range bars {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s  %s  %s", labelStyle.Render(bar.Label), renderTrack(bar), bar.ValueText)
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

// fillCells is the number of filled cells for value out of max over width. A
// non-positive max is an empty track; a full bar renders only at value >= max,
// so a sub-max value is capped at width-1 even when rounding would reach width.
func fillCells(value, max, width int64) int {
	if max <= 0 || value <= 0 {
		return 0
	}
	if value >= max {
		return int(width)
	}

	// Nearest-cell rounding, guarding value*width against int64 overflow for very
	// large values by falling back to float division.
	var cells int64
	if value <= (math.MaxInt64-max/2)/width {
		cells = (value*width + max/2) / max
	} else {
		cells = int64(float64(value) / float64(max) * float64(width))
	}

	// value < max here, so a full bar would be misleading.
	if cells >= width {
		cells = width - 1
	}
	if cells < 0 {
		cells = 0
	}
	return int(cells)
}

// percent is value/max as an integer percentage, clamped to [0, 100]. value*100
// is guarded against int64 overflow the same way as fillCells.
func percent(value, max int64) int {
	if max <= 0 || value <= 0 {
		return 0
	}
	if value >= max {
		return 100
	}
	if value <= math.MaxInt64/100 {
		return int((value * 100) / max)
	}
	return int(float64(value) / float64(max) * 100)
}
