// tui/console_test.go
package tui

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

func TestNewConsoleOverrides(t *testing.T) {
	buf := &bytes.Buffer{}
	c := NewConsole(
		WithWriter(buf),
		WithMode(output.Agent),
		WithTheme(theme.SafeDep()),
	)

	c.Info("hello")
	assert.Contains(t, buf.String(), "INFO:")
	assert.Contains(t, buf.String(), "hello")
}

func TestConsoleIsolatedFromGlobal(t *testing.T) {
	globalBuf := &bytes.Buffer{}
	output.SetWriters(globalBuf, globalBuf)
	defer output.SetWriters(os.Stdout, os.Stderr)
	output.SetMode(output.Rich)
	defer output.SetMode(output.Rich)

	consoleBuf := &bytes.Buffer{}
	c := NewConsole(WithWriter(consoleBuf), WithMode(output.Agent))

	c.Info("via console")

	// Console wrote to its own buffer.
	assert.Contains(t, consoleBuf.String(), "via console")
	// Global stderr was NOT touched by the Console.
	assert.Empty(t, globalBuf.String())
}
