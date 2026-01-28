package tui

import (
	"fmt"
	"os"

	"github.com/safedep/dry/usefulerror"
)

// ErrorExit prints a minimal, clean error message and exits with a non-zero status code.
func ErrorExit(err error, isVerbose bool) {
	usefulErr, ok := usefulerror.AsUsefulError(err)
	if !ok {
		return
	}

	// Use help as hint, but for unknown errors show bug report link
	hint := usefulErr.Help()

	if isVerbose {
		printVerboseError(usefulErr.Code(), usefulErr.HumanError(), hint,
			usefulErr.AdditionalHelp(), usefulErr.Error())
	} else {
		printMinimalError(usefulErr.Code(), usefulErr.HumanError(), hint)
	}

	os.Exit(1)
}

// printMinimalError prints error in minimal two-line format:
func printMinimalError(code, message, hint string) {
	fmt.Printf("%s  %s\n", colors.ErrorCode(" %s ", code), colors.Red(message))

	if hint != "" && hint != "No additional help is available for this error." {
		fmt.Printf(" %s %s\n", colors.Dim("→"), colors.Dim(hint))
	}
}

// printVerboseError prints detailed error for debugging (--verbose mode)
// Includes additional help and original error chain for troubleshooting
func printVerboseError(code, message, hint, additionalHelp, originalError string) {
	fmt.Printf("%s  %s\n", colors.ErrorCode(" %s ", code), colors.Red(message))

	if hint != "" && hint != "No additional help is available for this error." {
		fmt.Printf(" %s %s\n", colors.Dim("→"), colors.Dim(hint))
	}

	if additionalHelp != "" && additionalHelp != "No additional help is available for this error." {
		fmt.Printf(" %s %s\n", colors.Dim("→"), colors.Dim(additionalHelp))
	}

	if originalError != "" && originalError != message {
		fmt.Printf(" %s %s\n", colors.Dim("┄"), colors.Dim(originalError))
	}
}
