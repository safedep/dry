package tui

import (
	"os"

	"github.com/safedep/dry/usefulerror"
)

// ErrorExit prints a minimal, clean error message and exits with a non-zero status code.
func ErrorExit(err error, isVerbose bool) {
	usefulErr, ok := usefulerror.AsUsefulError(err)
	if !ok {
		os.Exit(1)
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
