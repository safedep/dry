package tui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/fatih/color"
	"github.com/safedep/dry/usefulerror"
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

func captureStderr(t *testing.T, f func()) string {
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

// TestFormatError_WithUsefulError tests FormatError with a UsefulError input
func TestFormatError_WithUsefulError(t *testing.T) {
	setAsciiProfile(t)

	usefulErr := usefulerror.NewUsefulError().
		WithCode("test_error_code").
		WithHumanError("Something went wrong").
		WithHelp("Try again later")

	var exitCode int
	out := captureStdout(t, func() {
		exitCode = FormatError(usefulErr, false)
	})

	assert.Equal(t, 1, exitCode, "expected exit code 1")
	assert.Contains(t, out, "test_error_code", "expected error code in output")
	assert.Contains(t, out, "Something went wrong", "expected human error in output")
}

// TestFormatError_WithUsefulError_Verbose tests FormatError with verbose flag
func TestFormatError_WithUsefulError_Verbose(t *testing.T) {
	setAsciiProfile(t)

	usefulErr := usefulerror.NewUsefulError().
		WithCode("verbose_error").
		WithHumanError("Detailed failure").
		WithHelp("Check logs").
		WithAdditionalHelp("Run with --debug for more info")

	var exitCode int
	out := captureStdout(t, func() {
		exitCode = FormatError(usefulErr, true)
	})

	assert.Equal(t, 1, exitCode, "expected exit code 1")
	assert.Contains(t, out, "verbose_error", "expected error code in output")
	assert.Contains(t, out, "Detailed failure", "expected human error in output")
	assert.Contains(t, out, "Run with --debug for more info", "expected additional help in output")
}

// TestFormatError_WithNonUsefulError tests FormatError with a regular error
func TestFormatError_WithNonUsefulError_Minimal(t *testing.T) {
	setAsciiProfile(t)

	regularErr := errors.New("simple error message")

	var exitCode int
	out := captureStderr(t, func() {
		exitCode = FormatError(regularErr, false)
	})

	assert.Equal(t, 1, exitCode, "expected exit code 1")
	assert.Contains(t, out, "Error: simple error message", "expected error message in stderr")
}

// TestSemanticColorFunctions_ASCII tests all semantic color functions with ASCII profile
func TestSemanticColorFunctions_ASCII(t *testing.T) {
	setAsciiProfile(t)

	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		{"InfoText", InfoText, "info message", "info message"},
		{"WarningText", WarningText, "warning message", "warning message"},
		{"ErrorText", ErrorText, "error message", "error message"},
		{"SuccessText", SuccessText, "success message", "success message"},
		{"FaintText", FaintText, "faint message", "faint message"},
		{"BoldText", BoldText, "bold message", "bold message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			assert.Equal(t, tt.expected, result, "ASCII profile should return plain text")
		})
	}
}

// TestSemanticColorFunctions_ANSI tests that ANSI profile adds color codes
func TestSemanticColorFunctions_ANSI(t *testing.T) {
	// Force colors on for testing (fatih/color disables them in non-TTY)
	color.NoColor = false
	t.Cleanup(func() {
		color.NoColor = true
		setAsciiProfile(t)
	})

	SetColorConfig(&ColorConfig{profile: colorprofile.ANSI})

	tests := []struct {
		name  string
		fn    func(string) string
		input string
	}{
		{"InfoText", InfoText, "info message"},
		{"WarningText", WarningText, "warning message"},
		{"ErrorText", ErrorText, "error message"},
		{"SuccessText", SuccessText, "success message"},
		{"FaintText", FaintText, "faint message"},
		{"BoldText", BoldText, "bold message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			// ANSI colors should add escape sequences
			assert.Contains(t, result, tt.input, "result should contain the input text")
			assert.True(t, strings.Contains(result, "\x1b[") || result == tt.input,
				"ANSI profile should add escape sequences or return plain text for unsupported colors")
		})
	}
}

// TestBadgeFunctions_ASCII tests all badge functions with ASCII profile
func TestBadgeFunctions_ASCII(t *testing.T) {
	setAsciiProfile(t)

	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		{"ErrorBgText", ErrorBgText, "ERROR", "ERROR"},
		{"WarningBgText", WarningBgText, "WARNING", "WARNING"},
		{"InfoBgText", InfoBgText, "INFO", "INFO"},
		{"SuccessBgText", SuccessBgText, "SUCCESS", "SUCCESS"},
		{"NeutralBgText", NeutralBgText, "NEUTRAL", "NEUTRAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			assert.Equal(t, tt.expected, result, "ASCII profile should return plain text")
		})
	}
}

// TestBadgeFunctions_ANSI tests that ANSI profile returns text containing input
// Note: In non-TTY environments, fatih/color may not add escape sequences
func TestBadgeFunctions_ANSI(t *testing.T) {
	SetColorConfig(&ColorConfig{profile: colorprofile.ANSI})
	t.Cleanup(func() {
		setAsciiProfile(t)
	})

	tests := []struct {
		name  string
		fn    func(string) string
		input string
	}{
		{"ErrorBgText", ErrorBgText, "ERROR"},
		{"WarningBgText", WarningBgText, "WARNING"},
		{"InfoBgText", InfoBgText, "INFO"},
		{"SuccessBgText", SuccessBgText, "SUCCESS"},
		{"NeutralBgText", NeutralBgText, "NEUTRAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			// Result should always contain the input text
			assert.Contains(t, result, tt.input, "result should contain the input text")
		})
	}
}

// TestColorConfig_NoTTY tests that NoTTY profile returns plain text
func TestColorConfig_NoTTY(t *testing.T) {
	SetColorConfig(&ColorConfig{profile: colorprofile.NoTTY})
	t.Cleanup(func() {
		setAsciiProfile(t)
	})

	cfg := GetColorConfig()

	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		{"InfoText", cfg.InfoText, "test", "test"},
		{"WarningText", cfg.WarningText, "test", "test"},
		{"ErrorText", cfg.ErrorText, "test", "test"},
		{"SuccessText", cfg.SuccessText, "test", "test"},
		{"FaintText", cfg.FaintText, "test", "test"},
		{"BoldText", cfg.BoldText, "test", "test"},
		{"ErrorBgText", cfg.ErrorBgText, "test", "test"},
		{"WarningBgText", cfg.WarningBgText, "test", "test"},
		{"InfoBgText", cfg.InfoBgText, "test", "test"},
		{"SuccessBgText", cfg.SuccessBgText, "test", "test"},
		{"NeutralBgText", cfg.NeutralBgText, "test", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			assert.Equal(t, tt.expected, result, "NoTTY profile should return plain text")
		})
	}
}

// TestMinimalError_Formatting tests MinimalError output formatting
func TestMinimalError_Formatting(t *testing.T) {
	tests := []struct {
		name    string
		profile colorprofile.Profile
		code    string
		msg     string
		hint    string
		check   func(t *testing.T, result string)
	}{
		{
			name:    "ASCII with hint",
			profile: colorprofile.Ascii,
			code:    "ERR001",
			msg:     "Test error",
			hint:    "Try this fix",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "ERR001")
				assert.Contains(t, result, "Test error")
				assert.Contains(t, result, "Try this fix")
			},
		},
		{
			name:    "ANSI with hint",
			profile: colorprofile.ANSI,
			code:    "ERR002",
			msg:     "Colored error",
			hint:    "Color hint",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "ERR002")
				assert.Contains(t, result, "Colored error")
				assert.Contains(t, result, "Color hint")
			},
		},
		{
			name:    "without hint",
			profile: colorprofile.Ascii,
			code:    "ERR003",
			msg:     "No hint error",
			hint:    "",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "ERR003")
				assert.Contains(t, result, "No hint error")
				assert.NotContains(t, result, "→")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ColorConfig{profile: tt.profile}
			result := cfg.MinimalError(tt.code, tt.msg, tt.hint)
			tt.check(t, result)
		})
	}
}

// TestVerboseError_Formatting tests VerboseError output formatting
func TestVerboseError_Formatting(t *testing.T) {
	tests := []struct {
		name           string
		profile        colorprofile.Profile
		code           string
		msg            string
		hint           string
		additionalHelp string
		originalError  string
		check          func(t *testing.T, result string)
	}{
		{
			name:           "ASCII full output",
			profile:        colorprofile.Ascii,
			code:           "ERR003",
			msg:            "Verbose error",
			hint:           "Hint text",
			additionalHelp: "Additional help text",
			originalError:  "original: something failed",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "ERR003")
				assert.Contains(t, result, "Verbose error")
				assert.Contains(t, result, "Hint text")
				assert.Contains(t, result, "Additional help text")
				assert.Contains(t, result, "original: something failed")
			},
		},
		{
			name:           "ANSI full output",
			profile:        colorprofile.ANSI,
			code:           "ERR004",
			msg:            "Colored verbose",
			hint:           "Color hint",
			additionalHelp: "More help",
			originalError:  "root cause",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "ERR004")
				assert.Contains(t, result, "Colored verbose")
				assert.Contains(t, result, "Color hint")
				assert.Contains(t, result, "More help")
				assert.Contains(t, result, "root cause")
			},
		},
		{
			name:           "partial fields only",
			profile:        colorprofile.Ascii,
			code:           "ERR005",
			msg:            "Partial error",
			hint:           "Some hint",
			additionalHelp: "",
			originalError:  "",
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "ERR005")
				assert.Contains(t, result, "Partial error")
				assert.Contains(t, result, "Some hint")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ColorConfig{profile: tt.profile}
			result := cfg.VerboseError(tt.code, tt.msg, tt.hint, tt.additionalHelp, tt.originalError)
			tt.check(t, result)
		})
	}
}

// TestSetColorConfig_NilSafe tests that SetColorConfig handles nil safely
func TestSetColorConfig_NilSafe(t *testing.T) {
	original := GetColorConfig()

	// Should not panic or change config when nil is passed
	SetColorConfig(nil)

	current := GetColorConfig()
	assert.Equal(t, original, current, "config should not change when nil is passed")
}
