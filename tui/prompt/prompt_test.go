// tui/prompt/prompt_test.go
package prompt

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestPromptReadsLine(t *testing.T) {
	got, err := promptFromReader(strings.NewReader("Alice\n"), "Name")
	assert.NoError(t, err)
	assert.Equal(t, "Alice", got)
}

func TestPromptAgentModeRefuses(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	_, err := Prompt("Name")
	assert.True(t, errors.Is(err, ErrAgentMode))
}

func TestConfirmDefaults(t *testing.T) {
	// Empty answer → default.
	got, err := confirmFromReader(strings.NewReader("\n"), "Proceed?", true)
	assert.NoError(t, err)
	assert.True(t, got)

	got, err = confirmFromReader(strings.NewReader("\n"), "Proceed?", false)
	assert.NoError(t, err)
	assert.False(t, got)
}

func TestConfirmExplicitYes(t *testing.T) {
	got, err := confirmFromReader(strings.NewReader("y\n"), "Proceed?", false)
	assert.NoError(t, err)
	assert.True(t, got)
}

func TestConfirmExplicitNo(t *testing.T) {
	got, err := confirmFromReader(strings.NewReader("n\n"), "Proceed?", true)
	assert.NoError(t, err)
	assert.False(t, got)
}

func TestSelectByIndex(t *testing.T) {
	got, err := selectFromReader(strings.NewReader("2\n"), "Env", []string{"dev", "staging", "prod"})
	assert.NoError(t, err)
	assert.Equal(t, "staging", got)
}

func TestSelectInvalidThenValid(t *testing.T) {
	got, err := selectFromReader(strings.NewReader("hello\n1\n"), "Env", []string{"dev", "prod"})
	assert.NoError(t, err)
	assert.Equal(t, "dev", got)
}
