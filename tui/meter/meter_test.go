package meter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func withMode(t *testing.T, m output.Mode) {
	t.Helper()
	prev := output.CurrentMode()
	output.SetMode(m)
	t.Cleanup(func() { output.SetMode(prev) })
}

func TestRenderPlainOneLinePerBar(t *testing.T) {
	withMode(t, output.Plain)

	got := Render(
		Bar{Label: "Included", Value: 72, Max: 100, ValueText: "72 / 100 scans"},
		Bar{Label: "Overage", Value: 0, Max: 100, ValueText: "$0.00 of $50.00 cap"},
	)
	assert.Equal(t, "Included: 72/100 (72%)\nOverage: 0/100 (0%)", got)
}

func TestRenderAgentDegradesLikePlain(t *testing.T) {
	withMode(t, output.Agent)

	got := Render(Bar{Label: "Included", Value: 50, Max: 100, ValueText: "50 / 100"})
	assert.Equal(t, "Included: 50/100 (50%)", got)
}

func TestRenderRichDrawsFillAndTrackAndValueText(t *testing.T) {
	withMode(t, output.Rich)

	got := Render(Bar{Label: "Included", Value: 50, Max: 100, ValueText: "50 / 100 scans (50%)"})
	assert.Contains(t, got, "Included")
	assert.Contains(t, got, "50 / 100 scans (50%)")
	// Half of a 26-cell bar rounds to 13 filled, 13 track.
	assert.Equal(t, 13, strings.Count(got, fillRune))
	assert.Equal(t, 13, strings.Count(got, trackRune))
}

func TestRenderRichFullAndEmpty(t *testing.T) {
	withMode(t, output.Rich)

	full := Render(Bar{Label: "Included", Value: 100, Max: 100, ValueText: "full"})
	assert.Equal(t, richBarWidth, strings.Count(full, fillRune))
	assert.Equal(t, 0, strings.Count(full, trackRune))

	empty := Render(Bar{Label: "Overage", Value: 0, Max: 100, ValueText: "none"})
	assert.Equal(t, 0, strings.Count(empty, fillRune))
	assert.Equal(t, richBarWidth, strings.Count(empty, trackRune))
}

func TestRenderRichAlignsLabels(t *testing.T) {
	withMode(t, output.Rich)

	got := Render(
		Bar{Label: "Included", Value: 1, Max: 10, ValueText: "a"},
		Bar{Label: "Overage", Value: 1, Max: 10, ValueText: "b"},
	)
	lines := strings.Split(got, "\n")
	// Labels are padded to a uniform width, so the bar column starts at the same
	// byte offset on both rows.
	assert.Equal(t, strings.Index(lines[0], fillRune), strings.Index(lines[1], fillRune))
	assert.Equal(t, strings.LastIndex(lines[0], fillRune), strings.LastIndex(lines[1], fillRune))
}

func TestFillCells(t *testing.T) {
	cases := []struct {
		value, max int64
		want       int
	}{
		{0, 100, 0},
		{100, 100, 26},
		{150, 100, 26}, // clamp over-max
		{50, 100, 13},
		{1, 100, 0},  // rounds down to empty
		{2, 100, 1},  // rounds up to one cell
		{10, 0, 0},   // non-positive max => empty
		{-5, 100, 0}, // non-positive value => empty
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, fillCells(c.value, c.max, richBarWidth),
			"fillCells(%d,%d)", c.value, c.max)
	}
}

func TestPercent(t *testing.T) {
	assert.Equal(t, 0, percent(0, 100))
	assert.Equal(t, 72, percent(72, 100))
	assert.Equal(t, 100, percent(100, 100))
	assert.Equal(t, 100, percent(150, 100)) // clamp
	assert.Equal(t, 0, percent(10, 0))      // non-positive max
}
