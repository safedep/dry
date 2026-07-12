package panel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/dry/tui/output"
)

func withMode(t *testing.T, m output.Mode) {
	t.Helper()
	prev := output.CurrentMode()
	output.SetMode(m)
	t.Cleanup(func() { output.SetMode(prev) })
}

func TestPanelPlainAlignsLabels(t *testing.T) {
	withMode(t, output.Plain)

	got := New("Profile").
		Field("Tenant", "acme.safedep.io").
		Field("API key", "configured").
		Render()

	lines := strings.Split(got, "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "Profile", lines[0])
	assert.Equal(t, "Tenant:   acme.safedep.io", lines[1])
	assert.Equal(t, "API key:  configured", lines[2])
}

func TestPanelPlainWithoutTitle(t *testing.T) {
	withMode(t, output.Plain)

	got := New("").Field("ID", "01ABC").Render()
	assert.Equal(t, "ID:  01ABC", got)
}

func TestPanelFieldIf(t *testing.T) {
	withMode(t, output.Plain)

	got := New("").
		Field("Always", "yes").
		FieldIf(false, "Skipped", "no").
		FieldIf(true, "Kept", "yes").
		Render()

	assert.Contains(t, got, "Always")
	assert.NotContains(t, got, "Skipped")
	assert.Contains(t, got, "Kept")
}

func TestPanelRichEmbedsTitleInTopBorder(t *testing.T) {
	withMode(t, output.Rich)

	got := New("Endpoint").Field("Hostname", "dev-box").Render()
	lines := strings.Split(got, "\n")
	assert.Contains(t, lines[0], "╭─ Endpoint ─")
	assert.Contains(t, got, "dev-box")
	assert.True(t, strings.HasPrefix(lines[len(lines)-1], "╰"))
}

func TestPanelRichHasVerticalPadding(t *testing.T) {
	withMode(t, output.Rich)

	got := New("T").Field("A", "1").Render()
	lines := strings.Split(got, "\n")
	// top border, blank, row, blank, bottom border.
	require.Len(t, lines, 5)
	assert.Empty(t, strings.TrimSpace(strings.Trim(lines[1], "│")), "second line is a blank padding row")
	assert.Empty(t, strings.TrimSpace(strings.Trim(lines[3], "│")), "fourth line is a blank padding row")
	assert.Contains(t, lines[2], "A")
}

func TestPanelRichAllLinesSameWidth(t *testing.T) {
	withMode(t, output.Rich)

	got := New("Authentication").
		Field("Status", "partially authenticated").
		Field("Profile", "default").
		Render()
	lines := strings.Split(got, "\n")
	w := lipgloss.Width(lines[0])
	for i, l := range lines {
		assert.Equal(t, w, lipgloss.Width(l), "line %d width", i)
	}
}

func TestPanelRichTitleWiderThanRows(t *testing.T) {
	withMode(t, output.Rich)

	got := New("A Very Long Panel Title Indeed").Field("A", "1").Render()
	lines := strings.Split(got, "\n")
	w := lipgloss.Width(lines[0])
	for i, l := range lines {
		assert.Equal(t, w, lipgloss.Width(l), "line %d width", i)
	}
}

func TestPanelRichTitleOnlyIsCompact(t *testing.T) {
	withMode(t, output.Rich)

	got := New("Only Title").Render()
	assert.Contains(t, got, "Only Title")
	// Top border with title, bottom border.
	assert.Len(t, strings.Split(got, "\n"), 2)
}
