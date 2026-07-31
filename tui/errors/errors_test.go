// tui/errors/errors_test.go
package errors

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/safedep/dry/usefulerror"
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
	t.Cleanup(func() { exitFn = defaultExit })

	defer func() {
		assert.Equal(t, "exit", recover())
		assert.Equal(t, 1, exited)
		assert.Contains(t, buf.String(), "boom")
	}()
	ErrorExit(errors.New("boom"))
}

func TestErrorExitWithCode(t *testing.T) {
	setupErrorTest(t)

	exited := -1
	exitFn = func(code int) { exited = code; panic("exit") }
	t.Cleanup(func() { exitFn = defaultExit })

	defer func() {
		assert.Equal(t, "exit", recover())
		assert.Equal(t, 42, exited)
	}()
	ErrorExitWithCode(fmt.Errorf("disk"), 42)
}

func TestErrorExitVerboseShowsStack(t *testing.T) {
	buf := setupErrorTest(t)
	output.SetVerbosity(output.Verbose)

	exitFn = func(code int) { panic("exit") }
	t.Cleanup(func() { exitFn = defaultExit })

	defer func() {
		assert.Equal(t, "exit", recover())
		assert.Contains(t, buf.String(), "wrapped")
		assert.Contains(t, buf.String(), "inner")
	}()
	ErrorExit(fmt.Errorf("wrapped: %w", errors.New("inner")))
}

func TestPrintErrorRendersUsefulError(t *testing.T) {
	tests := []struct {
		name        string
		build       func() error
		verbosity   output.Verbosity
		want        string
		notContains []string
	}{
		{
			name: "normal output is structured and actionable",
			build: func() error {
				usefulErr := usefulerror.NewUsefulError().
					WithCode(usefulerror.ErrBadRequest).
					WithHumanError("Project not scannable").
					WithHelp("Grant the SafeDep GitHub App access to this repository, wait for project sync, then retry.").
					WithAdditionalHelp("rpc error: internal detail").
					WithReferenceURL("https://docs.safedep.io/governance/integrations/github").
					Wrap(errors.New("rpc error: internal detail"))
				return fmt.Errorf("project scan create: %w", usefulErr)
			},
			verbosity: output.Normal,
			want: "[bad_request] Project not scannable\n" +
				"> Grant the SafeDep GitHub App access to this repository, wait for project sync, then retry.\n" +
				"> Learn more: https://docs.safedep.io/governance/integrations/github\n",
			notContains: []string{"rpc error", "No additional help is available"},
		},
		{
			name: "missing optional fields do not render placeholders",
			build: func() error {
				return usefulerror.NewUsefulError().
					WithCode(usefulerror.ErrBadRequest).
					WithHumanError("Invalid request")
			},
			verbosity:   output.Normal,
			want:        "[bad_request] Invalid request\n",
			notContains: []string{"No additional help is available", "unknown"},
		},
		{
			name: "verbose output includes diagnostic details",
			build: func() error {
				return usefulerror.NewUsefulError().
					WithCode(usefulerror.ErrBadRequest).
					WithHumanError("Invalid request").
					WithHelp("Correct the request and retry.").
					WithAdditionalHelp("field project_id is empty").
					Wrap(errors.New("rpc error: invalid project_id"))
			},
			verbosity: output.Verbose,
			want: "[bad_request] Invalid request\n" +
				"> Correct the request and retry.\n" +
				"> field project_id is empty\n" +
				"  caused by: rpc error: invalid project_id\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := setupErrorTest(t)
			output.SetVerbosity(tt.verbosity)

			printError(tt.build())

			assert.Equal(t, tt.want, buf.String())
			for _, unwanted := range tt.notContains {
				assert.NotContains(t, buf.String(), unwanted)
			}
		})
	}
}
