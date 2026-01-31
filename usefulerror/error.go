package usefulerror

import (
	"errors"
	"fmt"
	"strings"
)

// UsefulError is an interface that can be implemented for custom error types
// that are actually useful for the user. Think of this as a way out of showing
// weird internal errors to the user, which actually don't help them
type UsefulError interface {
	// Error returns a string that is useful for the user.
	// Maintains compatibility with the standard error interface.
	Error() string

	// HumanError returns a string that is more human-readable.
	HumanError() string

	// Help returns a string that provides help or guidance specific to the
	// business logic of the error.
	Help() string

	// AdditionalHelp returns a string that provides additional help or guidance
	// This is useful for providing specific tooling related instructions such
	// as command line flags to use to fix the error.
	AdditionalHelp() string

	// ReferenceURL returns a reference URL for an error
	ReferenceURL() string

	// Code returns a string that can be used to identify the error types
	// Meant for programmatic use, such as logging or categorization
	Code() string
}

type usefulErrorBuilder struct {
	originalError  error
	humanError     string
	help           string
	additionalHelp string
	referenceURL   string
	code           string
	msg            string
}

var _ UsefulError = (*usefulErrorBuilder)(nil)

var _ error = (*usefulErrorBuilder)(nil)

func NewUsefulError() *usefulErrorBuilder {
	return &usefulErrorBuilder{}
}

func (b *usefulErrorBuilder) Wrap(originalError error) *usefulErrorBuilder {
	b.originalError = originalError
	return b
}

// Unwrap returns the wrapped original error to support errors.Is and errors.As.
func (b *usefulErrorBuilder) Unwrap() error {
	return b.originalError
}

// WithHumanError sets a string that is more human-readable.
func (b *usefulErrorBuilder) WithHumanError(humanError string) *usefulErrorBuilder {
	b.humanError = humanError
	return b
}

// WithHelp sets a string that provides help or guidance.
func (b *usefulErrorBuilder) WithHelp(help string) *usefulErrorBuilder {
	b.help = help
	return b
}

// WithCode sets a code that can be used to identify the error types.
func (b *usefulErrorBuilder) WithCode(code string) *usefulErrorBuilder {
	b.code = code
	return b
}

// WithMsg sets a message that is useful for the user, but not necessarily human-readable.
func (b *usefulErrorBuilder) WithMsg(msg string) *usefulErrorBuilder {
	b.msg = msg
	return b
}

// WithAdditionalHelp sets a string that provides tooling-related instructions.
func (b *usefulErrorBuilder) WithAdditionalHelp(additionalHelp string) *usefulErrorBuilder {
	b.additionalHelp = additionalHelp
	return b
}

// WithReferenceURL sets a reference URL associated with the error.
func (b *usefulErrorBuilder) WithReferenceURL(url string) *usefulErrorBuilder {
	b.referenceURL = url
	return b
}

// Error implements the standard error interface. It returns the original error's message if present;
// otherwise, it returns a constructed message based on the code and msg fields, or "unknown error" if none are set.
func (b *usefulErrorBuilder) Error() string {
	if b.originalError != nil {
		return b.originalError.Error()
	}

	// If neither code nor message is set, fall back to a generic error string.
	if b.code == "" && b.msg == "" {
		return "unknown error"
	}

	// If only a code is set, return the code directly.
	if b.code != "" && b.msg == "" {
		return fmt.Sprintf("%s: unknown error", b.code)
	}

	msgParts := []string{}
	if b.code != "" {
		msgParts = append(msgParts, b.code)
	}

	msgParts = append(msgParts, b.msg)

	return strings.Join(msgParts, ": ")
}

// HumanError returns a string that is more human-readable.
func (b *usefulErrorBuilder) HumanError() string {
	if b.humanError == "" {
		return "An error occurred, but no human-readable message is available."
	}

	return b.humanError
}

// Help returns a string that provides help or guidance specific to the business logic of the error.
func (b *usefulErrorBuilder) Help() string {
	if b.help == "" {
		return "No additional help is available for this error."
	}

	return b.help
}

// Code returns a string that can be used to identify the error types.
func (b *usefulErrorBuilder) Code() string {
	if b.code == "" {
		return "unknown"
	}

	return b.code
}

// AdditionalHelp returns a string that provides tooling-related instructions.
func (b *usefulErrorBuilder) AdditionalHelp() string {
	if b.additionalHelp == "" {
		return "No additional help is available for this error."
	}

	return b.additionalHelp
}

func (b *usefulErrorBuilder) ReferenceURL() string {
	return b.referenceURL
}

// AsUsefulError attempts to convert a given error into a UsefulError. The following are the precedence rules:
//  1. If the error itself implements UsefulError, return it immediately.
//  2. Otherwise, if any error in the chain wrapped by err implements UsefulError, return that UsefulError
//     (the first one found when traversing the chain).
//  3. Otherwise, if there is a converter that can convert the error into a UsefulError, use it.
//  4. If no UsefulError can be found or constructed, return (nil, false).
//
// Error conversion is done on a best-effort basis using the registered converters. The first converter that can convert the error
// into a UsefulError will be used and the conversion will stop. Internally we maintain two registries for error converters:
//
// - Internal registry for standard error types.
// - Application registry for application-specific error types.
//
// Application registered error converters take precedence over internal error converters. This allows application code
// to register error converters for its own error types and override the default behavior for standard error types.
func AsUsefulError(err error) (UsefulError, bool) {
	if err == nil {
		return nil, false
	}

	// Check if err is already a UsefulError & return early
	if usefulErr, ok := err.(UsefulError); ok {
		return usefulErr, true
	}

	// Check if there is a wrapped error that is a UsefulError
	var usefulErr UsefulError
	if errors.As(err, &usefulErr) {
		return usefulErr, true
	}

	// Check if there is a converter that can convert the error into a UsefulError
	usefulErr, ok := convertToUsefulError(err)
	if ok {
		return usefulErr, true
	}

	return nil, false
}
