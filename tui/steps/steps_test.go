package steps

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
)

func withMode(t *testing.T, m output.Mode) *bytes.Buffer {
	t.Helper()
	prev := output.CurrentMode()
	buf := &bytes.Buffer{}
	output.SetMode(m)
	output.SetWriters(os.Stdout, buf)
	t.Cleanup(func() {
		output.SetMode(prev)
		output.SetWriters(os.Stdout, os.Stderr)
	})
	return buf
}

// Plain and Agent step lines must be byte-identical to tui.Info so agents
// and scripts see no change when a command adopts steps.
func TestStepMatchesInfoOutsideRich(t *testing.T) {
	for _, m := range []output.Mode{output.Plain, output.Agent} {
		buf := withMode(t, m)

		f := New(2)
		f.Step("Verifying API key")

		assert.Equal(t, style.Info("Verifying API key")+"\n", buf.String(), "mode %s", m)
	}
}

func TestDoneMatchesSuccess(t *testing.T) {
	for _, m := range []output.Mode{output.Rich, output.Plain, output.Agent} {
		buf := withMode(t, m)

		New(1).Done("saved for %q", "default")

		assert.Equal(t, style.Success(`saved for "default"`)+"\n", buf.String(), "mode %s", m)
	}
}

func TestFailMatchesError(t *testing.T) {
	buf := withMode(t, output.Plain)

	New(1).Fail("boom")

	assert.Equal(t, style.Error("boom")+"\n", buf.String())
}

func TestRichStepCarriesCounter(t *testing.T) {
	buf := withMode(t, output.Rich)

	f := New(3)
	f.Step("first")
	f.Step("second")

	got := buf.String()
	assert.Contains(t, got, "[1/3] first")
	assert.Contains(t, got, "[2/3] second")
}

func TestStepSilentSuppressed(t *testing.T) {
	buf := withMode(t, output.Plain)
	output.SetVerbosity(output.Silent)
	t.Cleanup(func() { output.SetVerbosity(output.Normal) })

	f := New(1)
	f.Step("hidden")
	f.Done("hidden too")
	f.Fail("always shown")

	assert.NotContains(t, buf.String(), "hidden")
	assert.Contains(t, buf.String(), "always shown")
}
