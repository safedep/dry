// tui/style/style_test.go
package style

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestStyleStripsColorInPlainMode(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	got := Success("done")
	// Plain mode: icon prefix + space + text, no ANSI escapes.
	assert.Equal(t, "[OK] done", got)
	assert.NotContains(t, got, "\x1b[")
}

func TestStyleAgentPrefixInAgentMode(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	got := Error("boom")
	assert.True(t, strings.HasPrefix(got, "ERR: "))
	assert.NotContains(t, got, "\x1b[")
}

func TestStyleRichEmitsAnsiUnlessNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	output.SetMode(output.Rich)
	defer output.SetMode(output.Rich)

	got := Success("done")
	// In Rich we expect the unicode icon at least.
	assert.True(t, strings.Contains(got, "✓"))
}
