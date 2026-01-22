package usefulerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsefulErrorBuilder_Error(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *usefulErrorBuilder
		expected string
	}{
		{
			name: "with original error",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().Wrap(errors.New("original error"))
			},
			expected: "original error",
		},
		{
			name: "with WithMsg only",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithMsg("test message")
			},
			expected: "test message",
		},
		{
			name: "with code and WithMsg",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithCode("TEST001").WithMsg("test message")
			},
			expected: "TEST001: test message",
		},
		{
			name: "with code only",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithCode("TEST001")
			},
			expected: "TEST001: unknown error",
		},
		{
			name: "empty builder",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError()
			},
			expected: "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestUsefulErrorBuilder_HumanError(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *usefulErrorBuilder
		expected string
	}{
		{
			name: "with human error set",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithHumanError("Something went wrong")
			},
			expected: "Something went wrong",
		},
		{
			name: "empty human error",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError()
			},
			expected: "An error occurred, but no human-readable message is available.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			assert.Equal(t, tt.expected, err.HumanError())
		})
	}
}

func TestUsefulErrorBuilder_Help(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *usefulErrorBuilder
		expected string
	}{
		{
			name: "with help set",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithHelp("Try running with --verbose flag")
			},
			expected: "Try running with --verbose flag",
		},
		{
			name: "empty help",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError()
			},
			expected: "No additional help is available for this error.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			assert.Equal(t, tt.expected, err.Help())
		})
	}
}

func TestUsefulErrorBuilder_AdditionalHelp(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *usefulErrorBuilder
		expected string
	}{
		{
			name: "with additional help set",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithAdditionalHelp("Use --force to override")
			},
			expected: "Use --force to override",
		},
		{
			name: "empty additional help",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError()
			},
			expected: "No additional help is available for this error.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			assert.Equal(t, tt.expected, err.AdditionalHelp())
		})
	}
}

func TestUsefulErrorBuilder_Code(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *usefulErrorBuilder
		expected string
	}{
		{
			name: "with code set",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithCode("ERR001")
			},
			expected: "ERR001",
		},
		{
			name: "empty code",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError()
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			assert.Equal(t, tt.expected, err.Code())
		})
	}
}

func TestUsefulErrorBuilder_ReferenceURL(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *usefulErrorBuilder
		expected string
	}{
		{
			name: "with reference URL set",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError().WithReferenceURL("https://app.safedep.io/community/malysis/01KF17TN5XE9135Z28EEF33D2E")
			},
			expected: "https://app.safedep.io/community/malysis/01KF17TN5XE9135Z28EEF33D2E",
		},
		{
			name: "empty reference URL",
			builder: func() *usefulErrorBuilder {
				return NewUsefulError()
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.builder()
			assert.Equal(t, tt.expected, err.ReferenceURL())
		})
	}
}

func TestUsefulErrorBuilder_ChainedMethods(t *testing.T) {
	err := NewUsefulError().
		WithCode("TEST001").
		WithMsg("test message").
		WithHumanError("User friendly error").
		WithHelp("Try this fix").
		WithAdditionalHelp("Or try this")

	assert.Equal(t, "TEST001: test message", err.Error())
	assert.Equal(t, "User friendly error", err.HumanError())
	assert.Equal(t, "Try this fix", err.Help())
	assert.Equal(t, "Or try this", err.AdditionalHelp())
	assert.Equal(t, "TEST001", err.Code())
}

func TestUsefulErrorBuilder_ChainedMethodsWithReferenceURL(t *testing.T) {
	err := NewUsefulError().
		WithCode("TEST001").
		WithMsg("test message").
		WithHumanError("User friendly error").
		WithHelp("Try this fix").
		WithAdditionalHelp("Or try this").
		WithReferenceURL("https://app.safedep.io/community/malysis/01KF17TN5XE9135Z28EEF33D2E")

	assert.Equal(t, "TEST001: test message", err.Error())
	assert.Equal(t, "User friendly error", err.HumanError())
	assert.Equal(t, "Try this fix", err.Help())
	assert.Equal(t, "Or try this", err.AdditionalHelp())
	assert.Equal(t, "TEST001", err.Code())
	assert.Equal(t, "https://app.safedep.io/community/malysis/01KF17TN5XE9135Z28EEF33D2E", err.ReferenceURL())
}

func TestAsUsefulError(t *testing.T) {
	tests := []struct {
		name        string
		input       error
		expectOk    bool
		expectError bool
	}{
		{
			name:        "nil error",
			input:       nil,
			expectOk:    false,
			expectError: false,
		},
		{
			name:        "useful error builder",
			input:       NewUsefulError().WithMsg("test"),
			expectOk:    true,
			expectError: false,
		},
		{
			name:        "regular error",
			input:       errors.New("regular error"),
			expectOk:    false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := AsUsefulError(tt.input)
			assert.Equal(t, tt.expectOk, ok)
			if tt.expectOk {
				assert.NotNil(t, result)
				assert.Implements(t, (*UsefulError)(nil), result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestUsefulErrorBuilder_ImplementsUsefulError(t *testing.T) {
	var _ UsefulError = (*usefulErrorBuilder)(nil)

	builder := NewUsefulError()
	assert.Implements(t, (*UsefulError)(nil), builder)
}

func TestUsefulErrorBuilder_Wrap(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := NewUsefulError().Wrap(originalErr)

	assert.Equal(t, "original error", wrappedErr.Error())
	assert.Equal(t, "An error occurred, but no human-readable message is available.", wrappedErr.HumanError())
}

func TestUsefulErrorBuilder_ComplexScenario(t *testing.T) {
	originalErr := errors.New("file not found")

	err := NewUsefulError().
		Wrap(originalErr).
		WithCode("FILE001").
		WithHumanError("The configuration file could not be found").
		WithHelp("Make sure the config file exists in the current directory").
		WithAdditionalHelp("Use --config flag to specify a different location")

	assert.Equal(t, "file not found", err.Error())
	assert.Equal(t, "The configuration file could not be found", err.HumanError())
	assert.Equal(t, "Make sure the config file exists in the current directory", err.Help())
	assert.Equal(t, "Use --config flag to specify a different location", err.AdditionalHelp())
	assert.Equal(t, "FILE001", err.Code())

	usefulErr, ok := AsUsefulError(err)
	assert.True(t, ok)
	assert.Equal(t, err, usefulErr)
}
