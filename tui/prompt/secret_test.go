// tui/prompt/secret_test.go
package prompt

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestSecretReadsMasked(t *testing.T) {
	input := bytes.NewBufferString("pass\n")
	outBuf := &bytes.Buffer{}
	got, err := readSecretMasked(input, outBuf, "Password", true)

	assert.NoError(t, err)
	assert.Equal(t, "pass", got)
	// Each char echoed as '*'.
	assert.Contains(t, outBuf.String(), "****")
}

func TestSecretNoMaskSuppressesEcho(t *testing.T) {
	input := bytes.NewBufferString("pass\n")
	outBuf := &bytes.Buffer{}
	got, err := readSecretMasked(input, outBuf, "Password", false)

	assert.NoError(t, err)
	assert.Equal(t, "pass", got)
	assert.NotContains(t, outBuf.String(), "****")
}

func TestSecretBackspaceErases(t *testing.T) {
	// "abc\x7f\x7fz\n" → "az"
	input := bytes.NewBufferString("abc\x7f\x7fz\n")
	outBuf := &bytes.Buffer{}
	got, err := readSecretMasked(input, outBuf, "Password", true)

	assert.NoError(t, err)
	assert.Equal(t, "az", got)
}

func TestSecretCtrlCReturnsCancelled(t *testing.T) {
	input := bytes.NewBufferString("ab\x03")
	outBuf := &bytes.Buffer{}
	_, err := readSecretMasked(input, outBuf, "Password", true)

	assert.True(t, errors.Is(err, ErrCancelled))
}

func TestSecretAgentModeRefuses(t *testing.T) {
	output.SetMode(output.Agent)
	defer output.SetMode(output.Rich)

	_, err := Secret("Password")
	assert.True(t, errors.Is(err, ErrAgentMode))
}
