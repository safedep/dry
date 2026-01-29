package tui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/safedep/dry/usefulerror"
	"github.com/stretchr/testify/assert"
)

func TestPrintMinimalError(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		message         string
		hint            string
		expectedContent []string
	}{
		{
			name:    "with hint",
			code:    "authentication_failed",
			message: "Login failed",
			hint:    "Re-authenticate with a valid token",
			expectedContent: []string{
				"authentication_failed",
				"Login failed",
				"Re-authenticate with a valid token",
			},
		},
		{
			name:    "without hint",
			code:    "internal_error",
			message: "Something went wrong",
			hint:    "",
			expectedContent: []string{
				"internal_error",
				"Something went wrong",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStderrForTest(t, func() {
				printMinimalError(tt.code, tt.message, tt.hint)
			})

			for _, content := range tt.expectedContent {
				assert.Contains(t, out, content, "expected content in output")
			}
		})
	}
}

func TestPrintVerboseError(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		message         string
		hint            string
		additionalHelp  string
		originalError   string
		expectedContent []string
	}{
		{
			name:           "all fields present",
			code:           "rate_limit_exceeded",
			message:        "Too many requests",
			hint:           "Wait a few seconds and try again",
			additionalHelp: "Use --retry 3 to automatically retry",
			originalError:  "HTTP 429 from api.example.com",
			expectedContent: []string{
				"rate_limit_exceeded",
				"Too many requests",
				"Wait a few seconds and try again",
				"Use --retry 3 to automatically retry",
				"HTTP 429 from api.example.com",
			},
		},
		{
			name:           "missing hint",
			code:           "quota_exceeded",
			message:        "Monthly quota exhausted",
			hint:           "",
			additionalHelp: "See docs: https://example.com/quota",
			originalError:  "usage=100%, limit=100%",
			expectedContent: []string{
				"quota_exceeded",
				"Monthly quota exhausted",
				"See docs: https://example.com/quota",
				"usage=100%, limit=100%",
			},
		},
		{
			name:           "only code and message",
			code:           "internal_server_error",
			message:        "Unexpected failure",
			hint:           "",
			additionalHelp: "",
			originalError:  "",
			expectedContent: []string{
				"internal_server_error",
				"Unexpected failure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStderrForTest(t, func() {
				printVerboseError(tt.code, tt.message, tt.hint, tt.additionalHelp, tt.originalError)
			})

			for _, content := range tt.expectedContent {
				assert.Contains(t, out, content, "expected content in output")
			}
		})
	}
}

// TestErrorExit_WithUsefulError tests ErrorExit with a UsefulError input
func TestErrorExit_WithUsefulError(t *testing.T) {
	var exitCode int
	SetExitFunc(func(code int) {
		exitCode = code
	})
	SetVerbosityLevel(VerbosityLevelNormal)
	t.Cleanup(func() {
		SetExitFunc(nil)
		SetVerbosityLevel(VerbosityLevelNormal)
	})

	usefulErr := usefulerror.NewUsefulError().
		WithCode("test_error_code").
		WithHumanError("Something went wrong").
		WithHelp("Try again later")

	out := captureStderrForTest(t, func() {
		ErrorExit(usefulErr)
	})

	assert.Equal(t, 1, exitCode, "expected exit code 1")
	assert.Contains(t, out, "test_error_code", "expected error code in output")
	assert.Contains(t, out, "Something went wrong", "expected human error in output")
	assert.Contains(t, out, "Try again later", "expected hint in output")
}

// TestErrorExit_WithUsefulError_Verbose tests ErrorExit with verbose mode enabled
func TestErrorExit_WithUsefulError_Verbose(t *testing.T) {
	var exitCode int
	SetExitFunc(func(code int) {
		exitCode = code
	})
	SetVerbosityLevel(VerbosityLevelVerbose)
	t.Cleanup(func() {
		SetExitFunc(nil)
		SetVerbosityLevel(VerbosityLevelNormal)
	})

	usefulErr := usefulerror.NewUsefulError().
		WithCode("verbose_error").
		WithHumanError("Detailed failure").
		WithHelp("Check logs").
		WithAdditionalHelp("Run with --debug for more info")

	out := captureStderrForTest(t, func() {
		ErrorExit(usefulErr)
	})

	assert.Equal(t, 1, exitCode, "expected exit code 1")
	assert.Contains(t, out, "verbose_error", "expected error code in output")
	assert.Contains(t, out, "Detailed failure", "expected human error in output")
	assert.Contains(t, out, "Check logs", "expected hint in output")
	assert.Contains(t, out, "Run with --debug for more info", "expected additional help in output")
}

// TestErrorExit_WithNonUsefulError tests ErrorExit with a regular error
func TestErrorExit_WithNonUsefulError(t *testing.T) {
	var exitCode int
	SetExitFunc(func(code int) {
		exitCode = code
	})
	t.Cleanup(func() {
		SetExitFunc(nil)
	})

	regularErr := errors.New("simple error message")

	out := captureStderrForTest(t, func() {
		ErrorExit(regularErr)
	})

	assert.Equal(t, 1, exitCode, "expected exit code 1")
	assert.Contains(t, out, "Error: simple error message", "expected error message in stderr")
}

// TestVerbosityLevel tests the verbosity level getter and setter
func TestVerbosityLevel(t *testing.T) {
	original := GetVerbosityLevel()
	t.Cleanup(func() {
		SetVerbosityLevel(original)
	})

	// Test setting and getting different levels
	SetVerbosityLevel(VerbosityLevelVerbose)
	assert.Equal(t, VerbosityLevelVerbose, GetVerbosityLevel())

	SetVerbosityLevel(VerbosityLevelNormal)
	assert.Equal(t, VerbosityLevelNormal, GetVerbosityLevel())
}

// TestSetExitFunc tests the exit function setter
func TestSetExitFunc(t *testing.T) {
	// Test that setting nil restores default
	SetExitFunc(nil)
	// We can't easily test that os.Exit is restored, but we can test custom func works

	called := false
	SetExitFunc(func(code int) {
		called = true
	})
	t.Cleanup(func() {
		SetExitFunc(nil)
	})

	exitFunc(0)
	assert.True(t, called, "custom exit func should be called")
}

// Test helper

func captureStderrForTest(t *testing.T, f func()) string {
	t.Helper()

	orig := os.Stderr
	r, w, err := os.Pipe()
	assert.NoError(t, err, "failed to create pipe")

	os.Stderr = w

	f()

	_ = w.Close()
	os.Stderr = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String()
}
