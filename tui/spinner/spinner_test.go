// tui/spinner/spinner_test.go
package spinner

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestSpinnerPlainPrintsStartAndStop(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	t.Cleanup(func() { output.SetWriters(os.Stdout, os.Stderr) })

	s := New("scanning")
	s.Start()
	s.Stop("done")

	out := buf.String()
	assert.Contains(t, out, "scanning")
	assert.Contains(t, out, "done")
}

func TestSpinnerStatusUpdatesLabel(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	t.Cleanup(func() { output.SetWriters(os.Stdout, os.Stderr) })

	s := New("resolving")
	s.Start()
	s.Status("downloading lodash")
	s.Stop("done")

	out := buf.String()
	assert.Contains(t, out, "downloading lodash")
}

func TestSpinnerAgentModeSkipsAnimation(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	t.Cleanup(func() { output.SetWriters(os.Stdout, os.Stderr) })

	s := New("scanning")
	s.Start()
	time.Sleep(50 * time.Millisecond) // would let Rich mode draw frames; Agent should not
	s.Stop("done")

	// No braille frames.
	for _, r := range brailleFrames {
		assert.NotContains(t, buf.String(), string(r))
	}
	assert.Contains(t, buf.String(), "scanning")
	assert.Contains(t, buf.String(), "done")
}

func TestSpinnerFailUsesErrorPrefix(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	t.Cleanup(func() { output.SetWriters(os.Stdout, os.Stderr) })

	s := New("scanning")
	s.Start()
	s.Fail("network error")

	out := buf.String()
	lines := strings.Split(out, "\n")
	assert.True(t, len(lines) >= 2)
	assert.Contains(t, out, "[ERR]")
	assert.Contains(t, out, "network error")
}
