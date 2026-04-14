// tui/errors/errors_test.go
package errors

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func setupErrorTest(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	output.SetWriters(buf, buf)
	output.SetMode(output.Plain)
	output.SetVerbosity(output.Normal)
	t.Cleanup(func() {
		output.SetWriters(os.Stdout, os.Stderr)
		output.SetMode(output.Rich)
		output.SetVerbosity(output.Normal)
	})
	return buf
}

func TestErrorExitPrintsAndExits(t *testing.T) {
	buf := setupErrorTest(t)

	exited := -1
	exitFn = func(code int) { exited = code; panic("exit") }
	defer func() { exitFn = defaultExit }()

	defer func() { _ = recover() }()
	ErrorExit(errors.New("boom"))

	assert.Equal(t, 1, exited)
	assert.Contains(t, buf.String(), "boom")
}

func TestErrorExitWithCode(t *testing.T) {
	setupErrorTest(t)

	exited := -1
	exitFn = func(code int) { exited = code; panic("exit") }
	defer func() { exitFn = defaultExit }()

	defer func() { _ = recover() }()
	ErrorExitWithCode(fmt.Errorf("disk"), 42)

	assert.Equal(t, 42, exited)
}

func TestErrorExitVerboseShowsStack(t *testing.T) {
	buf := setupErrorTest(t)
	output.SetVerbosity(output.Verbose)

	exitFn = func(code int) { panic("exit") }
	defer func() { exitFn = defaultExit }()

	defer func() { _ = recover() }()
	ErrorExit(fmt.Errorf("wrapped: %w", errors.New("inner")))

	assert.Contains(t, buf.String(), "wrapped")
	assert.Contains(t, buf.String(), "inner")
}
