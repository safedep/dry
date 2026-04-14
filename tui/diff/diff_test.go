// tui/diff/diff_test.go
package diff

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestDiffPlainHasAddAndRemovePrefixes(t *testing.T) {
	prev := output.CurrentMode()
	output.SetMode(output.Plain)
	defer output.SetMode(prev)

	old := "line A\nline B\n"
	neu := "line A\nline C\n"
	got := Render(old, neu)

	assert.Contains(t, got, "-line B")
	assert.Contains(t, got, "+line C")
	assert.NotContains(t, got, "\x1b[")
}

func TestDiffRichContainsColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	prev := output.CurrentMode()
	output.SetMode(output.Rich)
	defer output.SetMode(prev)

	got := Render("a\n", "b\n")
	// If color is enabled, we expect at least one ANSI escape.
	if output.IsColorEnabled() {
		assert.Contains(t, got, "\x1b[")
	}
}

func TestDiffEqualReturnsEmpty(t *testing.T) {
	got := Render("same\n", "same\n")
	assert.Empty(t, got)
}

func TestDiffContextLineHasSpacePrefix(t *testing.T) {
	prev := output.CurrentMode()
	output.SetMode(output.Plain)
	defer output.SetMode(prev)

	old := "line A\nline B\nline C\n"
	neu := "line A\nline X\nline C\n"
	got := Render(old, neu)

	// "line A" and "line C" are unchanged — they should appear with space prefix
	assert.Contains(t, got, " line A")
	assert.Contains(t, got, " line C")
	assert.Contains(t, got, "-line B")
	assert.Contains(t, got, "+line X")
}
