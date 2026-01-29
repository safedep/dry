package tui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
)

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
			assert.NotContains(t, result, "\x1b[", "ASCII profile should not add escape sequences")
		})
	}
}

// TestSemanticColorFunctions_ANSI tests that ANSI profile enables colors
func TestSemanticColorFunctions_ANSI(t *testing.T) {
	SetColorConfig(NewColorConfig(colorprofile.ANSI))
	t.Cleanup(func() {
		setAsciiProfile(t)
	})

	// Verify our config reports colors as enabled
	assert.True(t, GetColorConfig().IsColorEnabled(), "ANSI profile should report colors enabled")

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
			// Result should always contain the input text
			assert.Contains(t, result, tt.input, "result should contain the input text")
			// Note: We can't assert ANSI escape codes here because fatih/color
			// disables colors at init time in non-TTY environments (like tests).
			// The IsColorEnabled assertion above verifies our logic is correct.
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
			assert.NotContains(t, result, "\x1b[", "ASCII profile should not add escape sequences")
		})
	}
}

// TestBadgeFunctions_ANSI tests that ANSI profile enables colors for badges
func TestBadgeFunctions_ANSI(t *testing.T) {
	SetColorConfig(NewColorConfig(colorprofile.ANSI))
	t.Cleanup(func() {
		setAsciiProfile(t)
	})

	// Verify our config reports colors as enabled
	assert.True(t, GetColorConfig().IsColorEnabled(), "ANSI profile should report colors enabled")

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
			// Note: We can't assert ANSI escape codes here because fatih/color
			// disables colors at init time in non-TTY environments (like tests).
			// The IsColorEnabled assertion above verifies our logic is correct.
		})
	}
}

// TestColorConfig_NoTTY tests that NoTTY profile returns plain text
func TestColorConfig_NoTTY(t *testing.T) {
	SetColorConfig(NewColorConfig(colorprofile.NoTTY))
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
			assert.Contains(t, result, tt.input, "result should contain the input text")
			assert.NotContains(t, result, "\x1b[", "NoTTY profile should not add escape sequences")
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

// TestIsColorEnabled tests the IsColorEnabled helper
func TestIsColorEnabled(t *testing.T) {
	tests := []struct {
		name     string
		profile  colorprofile.Profile
		expected bool
	}{
		{"NoTTY", colorprofile.NoTTY, false},
		{"Ascii", colorprofile.Ascii, false},
		{"ANSI", colorprofile.ANSI, true},
		{"ANSI256", colorprofile.ANSI256, true},
		{"TrueColor", colorprofile.TrueColor, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewColorConfig(tt.profile)
			assert.Equal(t, tt.expected, cfg.IsColorEnabled())
		})
	}
}

func setAsciiProfile(t *testing.T) {
	t.Helper()
	original := GetColorConfig()

	SetColorConfig(NewColorConfig(colorprofile.Ascii))

	t.Cleanup(func() {
		SetColorConfig(original)
	})
}
