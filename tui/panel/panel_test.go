package panel

import (
	"strings"
	"testing"

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

func TestPanelRichDrawsBorderAndTitle(t *testing.T) {
	withMode(t, output.Rich)

	got := New("Endpoint").Field("Hostname", "dev-box").Render()
	assert.Contains(t, got, "╭")
	assert.Contains(t, got, "╰")
	assert.Contains(t, got, "Endpoint")
	assert.Contains(t, got, "dev-box")
}

func TestPanelRichSkipsTitleGapWithoutFields(t *testing.T) {
	withMode(t, output.Rich)

	got := New("Only Title").Render()
	assert.Contains(t, got, "Only Title")
	// One content line inside the border: top border, title, bottom border.
	assert.Len(t, strings.Split(got, "\n"), 3)
}
