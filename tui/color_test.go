package tui

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
)

func TestPrintMinimalError_ASCII(t *testing.T) {
	setAsciiProfile(t)

	tests := []struct {
		name     string
		code     string
		message  string
		hint     string
		expected string
	}{
		{
			name:     "with hint",
			code:     "authentication_failed",
			message:  "Login failed",
			hint:     "Re-authenticate with a valid token",
			expected: "authentication_failed  Login failed\n → Re-authenticate with a valid token\n",
		},
		{
			name:     "without hint",
			code:     "internal_error",
			message:  "Something went wrong",
			hint:     "",
			expected: "internal_error  Something went wrong\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				printMinimalError(tt.code, tt.message, tt.hint)
			})

			assert.Equal(t, tt.expected, out, "unexpected output")
		})
	}
}

func TestPrintVerboseError_ASCII(t *testing.T) {
	setAsciiProfile(t)

	tests := []struct {
		name           string
		code           string
		message        string
		hint           string
		additionalHelp string
		originalError  string
		expected       string
	}{
		{
			name:           "all fields present",
			code:           "rate_limit_exceeded",
			message:        "Too many requests",
			hint:           "Wait a few seconds and try again",
			additionalHelp: "Use --retry 3 to automatically retry",
			originalError:  "HTTP 429 from api.example.com",
			expected: "" +
				"rate_limit_exceeded  Too many requests\n" +
				" → Wait a few seconds and try again\n" +
				" → Use --retry 3 to automatically retry\n" +
				" ┄ HTTP 429 from api.example.com\n",
		},
		{
			name:           "missing hint",
			code:           "quota_exceeded",
			message:        "Monthly quota exhausted",
			hint:           "",
			additionalHelp: "See docs: https://example.com/quota",
			originalError:  "usage=100%, limit=100%",
			expected: "" +
				"quota_exceeded  Monthly quota exhausted\n" +
				" → See docs: https://example.com/quota\n" +
				" ┄ usage=100%, limit=100%\n",
		},
		{
			name:           "missing additional help",
			code:           "authorization_failed",
			message:        "Access denied",
			hint:           "Ensure your role has necessary permissions",
			additionalHelp: "",
			originalError:  "403 Forbidden",
			expected: "" +
				"authorization_failed  Access denied\n" +
				" → Ensure your role has necessary permissions\n" +
				" ┄ 403 Forbidden\n",
		},
		{
			name:           "missing original error",
			code:           "authentication_failed",
			message:        "Invalid credentials",
			hint:           "Re-enter username and password",
			additionalHelp: "Use --login to start an interactive login",
			originalError:  "",
			expected: "" +
				"authentication_failed  Invalid credentials\n" +
				" → Re-enter username and password\n" +
				" → Use --login to start an interactive login\n",
		},
		{
			name:           "only code and message",
			code:           "internal_server_error",
			message:        "Unexpected failure",
			hint:           "",
			additionalHelp: "",
			originalError:  "",
			expected:       "internal_server_error  Unexpected failure\n",
		},
		{
			name:           "only original error present besides code+message",
			code:           "unknown_error",
			message:        "An unknown error occurred",
			hint:           "",
			additionalHelp: "",
			originalError:  "stack: nil dereference",
			expected: "" +
				"unknown_error  An unknown error occurred\n" +
				" ┄ stack: nil dereference\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				printVerboseError(tt.code, tt.message, tt.hint, tt.additionalHelp, tt.originalError)
			})

			assert.Equal(t, tt.expected, out, "unexpected output")
		})
	}
}

func setAsciiProfile(t *testing.T) {
	t.Helper()
	SetColorConfig(&ColorConfig{profile: colorprofile.Ascii})
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	assert.NoError(t, err, "failed to create pipe")

	os.Stdout = w

	// Run the function that prints to stdout
	f()

	// Restore stdout before reading to avoid deadlocks in case f() also writes
	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String()
}
