package tui

import (
	"fmt"
	"os"

	"github.com/safedep/dry/usefulerror"
)

// FormatError formats and prints an error message, returning the exit code.
// If the error is a UsefulError, it prints a formatted message with code, message,
// and help text. Otherwise, it prints a generic error message to stderr.
// When isVerbose is true, additional details are included in the output.
// Returns 1 as the exit code (for use with os.Exit if desired).
func FormatError(err error, isVerbose bool) int {
	usefulErr, ok := usefulerror.AsUsefulError(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Use the error's help text as the hint
	hint := usefulErr.Help()

	if isVerbose {
		printVerboseError(usefulErr.Code(), usefulErr.HumanError(), hint,
			usefulErr.AdditionalHelp(), usefulErr.Error())
	} else {
		printMinimalError(usefulErr.Code(), usefulErr.HumanError(), hint)
	}

	return 1
}

// ErrorExit prints an error message and exits with a non-zero status code.
// This is a convenience wrapper around FormatError that calls os.Exit.
func ErrorExit(err error, isVerbose bool) {
	os.Exit(FormatError(err, isVerbose))
}
