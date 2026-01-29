package tui

import (
	"fmt"
	"os"

	"github.com/safedep/dry/usefulerror"
)

// VerbosityLevel represents the verbosity level for tui output.
type VerbosityLevel int

const (
	// VerbosityLevelNormal shows minimal status updates and standard error messages.
	VerbosityLevelNormal VerbosityLevel = iota

	// VerbosityLevelVerbose shows detailed information including additional help and debug info.
	VerbosityLevelVerbose
)

// exitFunc is the function used to exit the program. It can be overridden for testing.
var exitFunc = os.Exit

// verbosityLevel controls the global verbosity level for the tui package.
var verbosityLevel = VerbosityLevelNormal

// SetExitFunc sets the exit function used by ErrorExit.
// This is primarily intended for testing purposes to avoid actually exiting the process.
// Pass nil to restore the default os.Exit behavior.
func SetExitFunc(f func(int)) {
	if f == nil {
		exitFunc = os.Exit
		return
	}
	exitFunc = f
}

// SetVerbosityLevel sets the global verbosity level for the tui package.
// This affects how much detail is shown in error messages and other output.
func SetVerbosityLevel(level VerbosityLevel) {
	verbosityLevel = level
}

// GetVerbosityLevel returns the current global verbosity level.
func GetVerbosityLevel() VerbosityLevel {
	return verbosityLevel
}

// ErrorExit prints an error message and exits with a non-zero status code.
// If the error is a UsefulError, it prints a formatted message with code, message,
// and help text. Otherwise, it prints a generic error message to stderr.
// Uses the global verbosity level (set via SetVerbosityLevel) to determine output detail level.
func ErrorExit(err error) {
	usefulErr, ok := usefulerror.AsUsefulError(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}

	// Use the error's help text as the hint
	hint := usefulErr.Help()

	if verbosityLevel == VerbosityLevelVerbose {
		printVerboseError(usefulErr.Code(), usefulErr.HumanError(), hint,
			usefulErr.AdditionalHelp(), usefulErr.Error())
	} else {
		printMinimalError(usefulErr.Code(), usefulErr.HumanError(), hint)
	}

	exitFunc(1)
}
