// tui/tui_test.go
package tui

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func setup(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	output.SetMode(output.Plain)
	output.SetVerbosity(output.Normal)
	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	return buf, func() {
		output.SetWriters(os.Stdout, os.Stderr)
		output.SetMode(output.Rich)
		output.SetVerbosity(output.Normal)
	}
}

func TestInfoPrintsToStderr(t *testing.T) {
	buf, done := setup(t)
	defer done()

	Info("hello %s", "world")
	assert.Contains(t, buf.String(), "hello world")
	assert.Contains(t, buf.String(), "[INFO]")
}

func TestSuccessPrintsToStderr(t *testing.T) {
	buf, done := setup(t)
	defer done()

	Success("done")
	assert.Contains(t, buf.String(), "[OK]")
	assert.Contains(t, buf.String(), "done")
}

func TestFaintSuppressedWhenVerbosityNormal(t *testing.T) {
	buf, done := setup(t)
	defer done()

	output.SetVerbosity(output.Normal)
	defer output.SetVerbosity(output.Normal)

	Faint("debug info")
	assert.Empty(t, buf.String())
}

func TestFaintShownInVerbose(t *testing.T) {
	buf, done := setup(t)
	defer done()

	output.SetVerbosity(output.Verbose)
	defer output.SetVerbosity(output.Normal)

	Faint("debug info")
	assert.Contains(t, buf.String(), "debug info")
}

func TestInfoSuppressedInSilent(t *testing.T) {
	buf, done := setup(t)
	defer done()

	output.SetVerbosity(output.Silent)
	defer output.SetVerbosity(output.Normal)

	Info("hidden")
	assert.Empty(t, buf.String())
}

func TestErrorShownInSilent(t *testing.T) {
	buf, done := setup(t)
	defer done()

	output.SetVerbosity(output.Silent)
	defer output.SetVerbosity(output.Normal)

	Error("boom")
	assert.Contains(t, buf.String(), "boom")
}

type renderableStub struct{ text string }

func (r renderableStub) Render(_ Theme, _ output.Mode) string { return r.text }

func TestPrintRenderable(t *testing.T) {
	buf, done := setup(t)
	defer done()

	Print(renderableStub{text: "rendered!"})
	assert.Contains(t, buf.String(), "rendered!")
}
