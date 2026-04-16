// tui/progress/progress_test.go
package progress

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestProgressPlainEmitsLines(t *testing.T) {
	output.SetMode(output.Plain)
	defer output.SetMode(output.Rich)

	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	t.Cleanup(func() { output.SetWriters(os.Stdout, os.Stderr) })

	p := New()
	tr := p.Track("downloading", 100)
	tr.Increment(50)
	tr.Done()
	p.Wait()

	out := buf.String()
	assert.Contains(t, out, "downloading")
	assert.Contains(t, out, "50/100")
}

func TestProgressAgentAppendOnly(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	t.Cleanup(func() { output.SetWriters(os.Stdout, os.Stderr) })

	p := New()
	tr := p.Track("uploading", 4)
	tr.Increment(1)
	tr.Increment(1)
	tr.Done()
	p.Wait()

	out := buf.String()
	assert.NotContains(t, out, "\r")
	assert.Contains(t, out, "progress:")
	assert.Contains(t, out, "uploading")
}
