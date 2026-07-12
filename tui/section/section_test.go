package section

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func withMode(t *testing.T, m output.Mode) {
	t.Helper()
	prev := output.CurrentMode()
	output.SetMode(m)
	t.Cleanup(func() { output.SetMode(prev) })
}

func TestTitled(t *testing.T) {
	withMode(t, output.Plain)
	assert.Equal(t, "Guard events\nbody", Titled("Guard events", "body"))
	assert.Equal(t, "Guard events", Titled("Guard events", ""))
}

func TestHintPlainUsesAsciiArrow(t *testing.T) {
	withMode(t, output.Plain)
	assert.Equal(t, "> use --since to widen", Hint("use --since to widen"))
}

func TestEmptyPlainIsUndecorated(t *testing.T) {
	withMode(t, output.Plain)
	assert.Equal(t, "no endpoints", Empty("no endpoints"))
}

func TestJoinSkipsBlanks(t *testing.T) {
	assert.Equal(t, "a\n\nb", Join("a", "", "  ", "b"))
	assert.Equal(t, "", Join("", "  "))
}
