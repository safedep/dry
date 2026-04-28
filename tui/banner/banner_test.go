// tui/banner/banner_test.go
package banner

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

const sampleArt = "█▀█ █▀▄▀█ █▀▀\n█▀▀ █ ▀ █ █▄█"

func TestBannerPlainFallback(t *testing.T) {
	prev := output.CurrentMode()
	output.SetMode(output.Plain)
	defer output.SetMode(prev)

	buf := &bytes.Buffer{}
	b := Banner{Art: sampleArt, Name: "pmg", Version: "1.2.3", Tagline: "pkg mgr"}
	b.PrintTo(buf)

	out := buf.String()
	assert.NotContains(t, out, "█") // art omitted in Plain
	assert.Contains(t, out, "pmg")
	assert.Contains(t, out, "1.2.3")
	assert.Contains(t, out, "pkg mgr")
}

func TestBannerAgentSingleLine(t *testing.T) {
	prev := output.CurrentMode()
	output.SetMode(output.Agent)
	defer output.SetMode(prev)

	buf := &bytes.Buffer{}
	Banner{Name: "pmg", Version: "1.2.3"}.PrintTo(buf)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], "tool=pmg")
	assert.Contains(t, lines[0], "version=1.2.3")
}

func TestCleanVersionReleaseVerbatim(t *testing.T) {
	assert.Equal(t, "v1.2.3", cleanVersion("v1.2.3"))
	assert.Equal(t, "1.2.3", cleanVersion("1.2.3"))
}

func TestCleanVersionPseudoTrimsToShortSHA(t *testing.T) {
	got := cleanVersion("v0.0.0-20260401123045-abcdef123456")
	assert.Equal(t, "dev (abcdef1)", got)
}

func TestCleanVersionEmpty(t *testing.T) {
	assert.Equal(t, "dev", cleanVersion(""))
}

func TestBannerAccentPointerNilDefaultsBrandPrimary(t *testing.T) {
	// Nil Accent should default to BrandPrimary — just ensure no panic
	b := Banner{Name: "test", Version: "1.0.0", Tagline: "test tagline", Accent: nil}
	prev := output.CurrentMode()
	output.SetMode(output.Plain)
	defer output.SetMode(prev)

	s := b.Render()
	assert.Contains(t, s, "test")
}
