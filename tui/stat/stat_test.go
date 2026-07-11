package stat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

func withMode(t *testing.T, m output.Mode) {
	t.Helper()
	prev := output.CurrentMode()
	output.SetMode(m)
	t.Cleanup(func() { output.SetMode(prev) })
}

func TestRenderPlainOneLinePerCard(t *testing.T) {
	withMode(t, output.Plain)

	got := Render(
		Card{Label: "Total endpoints", Value: "42"},
		Card{Label: "Blocked", Value: "3"},
	)
	assert.Equal(t, "Total endpoints: 42\nBlocked: 3", got)
}

func TestRenderRichDrawsCards(t *testing.T) {
	withMode(t, output.Rich)

	warn := theme.RoleWarning
	got := Render(
		Card{Label: "Active", Value: "12"},
		Card{Label: "Silent", Value: "5", Accent: &warn},
	)
	assert.Contains(t, got, "Active")
	assert.Contains(t, got, "Silent")
	assert.Contains(t, got, "╭")
	// Both cards on one row when they fit.
	top := strings.Split(got, "\n")[0]
	assert.Equal(t, 2, strings.Count(top, "╭"))
}

func TestRenderRichWrapsToWidth(t *testing.T) {
	withMode(t, output.Rich)
	output.SetWidthOverride(20)
	t.Cleanup(func() { output.SetWidthOverride(0) })

	got := Render(
		Card{Label: "First metric", Value: "100"},
		Card{Label: "Second metric", Value: "200"},
	)
	top := strings.Split(got, "\n")[0]
	require.Equal(t, 1, strings.Count(top, "╭"))
}
