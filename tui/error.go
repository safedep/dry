package tui

import (
	"fmt"
	"os"

	"github.com/safedep/dry/usefulerror"
)

// exitFunc is the function used to exit the program. It can be overridden for testing.
var exitFunc = os.Exit

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

// ErrorExit prints an error message and exits with a non-zero status code.
// If the error is a UsefulError, it prints a formatted message with code, message,
// and help text. Otherwise, it prints a generic error message to stderr.
// When isVerbose is true, additional details are included in the output.
func ErrorExit(err error, isVerbose bool) {
	usefulErr, ok := usefulerror.AsUsefulError(err)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}

	// Use the error's help text as the hint
	hint := usefulErr.Help()

	if isVerbose {
		printVerboseError(usefulErr.Code(), usefulErr.HumanError(), hint,
			usefulErr.AdditionalHelp(), usefulErr.Error())
	} else {
		printMinimalError(usefulErr.Code(), usefulErr.HumanError(), hint)
	}

	exitFunc(1)
}
