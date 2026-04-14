// tui/table/table_test.go
package table

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestTableRendersHeadersAndRows(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	out := New().
		Headers("Package", "Version").
		Row("lodash", "4.17.0").
		Row("minimist", "1.2.0").
		Render()

	assert.Contains(t, out, "Package")
	assert.Contains(t, out, "Version")
	assert.Contains(t, out, "lodash")
	assert.Contains(t, out, "minimist")
}

func TestTableAgentModeUsesNoBorders(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	out := New().
		Headers("A", "B").
		Row("1", "2").
		Render()

	// Agent mode: no box-drawing chars.
	for _, r := range []string{"│", "─", "┌", "┐", "└", "┘", "┬", "┴"} {
		assert.NotContains(t, out, r)
	}
	// Data still present.
	assert.Contains(t, out, "A")
	assert.Contains(t, out, "B")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "2")
}

func TestTableStyleFuncInvoked(t *testing.T) {
	output.SetMode(output.Rich)
	defer output.SetMode(output.Rich)

	called := false
	out := New().
		Headers("Name").
		Row("hello").
		StyleFunc(func(row, col int) lipgloss.Style {
			called = true
			return lipgloss.NewStyle()
		}).
		Render()

	// StyleFunc is invoked at least once while rendering.
	assert.True(t, called, "StyleFunc should have been invoked during Render")
	// Basic structural checks remain.
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "hello")

	// ANSI presence is only meaningful on a real TTY (IsColorEnabled == true);
	// under `go test` lipgloss renders to Ascii and emits no escapes. Pattern
	// matches TestDiffRichContainsColor.
	if output.IsColorEnabled() {
		assert.True(t, strings.Contains(out, "\x1b["), "Rich + color enabled should emit ANSI")
	}
}
