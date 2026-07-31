// tui/errors/errors.go
//
// Package errors provides verbosity-aware error printing with process exit.
// Replaces the former dry/tui/error.go.
package errors

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/section"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
	"github.com/safedep/dry/usefulerror"
)

const unavailableHelpText = "No additional help is available for this error."

// ErrorExit prints the error (verbosity-aware) and exits with code 1.
func ErrorExit(err error) {
	ErrorExitWithCode(err, 1)
}

// ErrorExitWithCode prints the error and exits with the given code.
func ErrorExitWithCode(err error, code int) {
	if err == nil {
		exitFn(code)
		return
	}
	printError(err)
	exitFn(code)
}

func printError(err error) {
	if usefulErr, ok := usefulerror.AsUsefulError(err); ok {
		printUsefulError(usefulErr)
		return
	}

	line := style.Error(err.Error())
	_, _ = fmt.Fprintln(output.Stderr(), line)
	if output.CurrentVerbosity() >= output.Verbose {
		walkCauses(err, func(cause error) {
			_, _ = fmt.Fprintln(output.Stderr(), style.Faint("  caused by: "+cause.Error()))
		})
	}
}

func printUsefulError(err usefulerror.UsefulError) {
	summary := fmt.Sprintf("%s %s",
		style.Badge(theme.RoleError, err.Code()),
		errorText(err.HumanError()))
	_, _ = fmt.Fprintln(output.Stderr(), summary)

	if help := err.Help(); help != "" && help != unavailableHelpText {
		printUsefulErrorHint(help)
	}
	if referenceURL := err.ReferenceURL(); referenceURL != "" {
		printUsefulErrorHint("Learn more: " + referenceURL)
	}

	if output.CurrentVerbosity() < output.Verbose {
		return
	}
	if additionalHelp := err.AdditionalHelp(); additionalHelp != "" && additionalHelp != unavailableHelpText {
		printUsefulErrorHint(additionalHelp)
	}
	if originalError := err.Error(); originalError != "" && originalError != err.HumanError() {
		_, _ = fmt.Fprintln(output.Stderr(), style.Faint("  caused by: "+originalError))
	}
}

func printUsefulErrorHint(message string) {
	_, _ = fmt.Fprintln(output.Stderr(), section.Hint(message))
}

func errorText(message string) string {
	if output.CurrentMode() != output.Rich || !output.IsColorEnabled() {
		return message
	}
	color, _ := theme.Default().Palette().ColorByRole(theme.RoleError)
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(message)
}

// walkCauses traverses the error's wrap chain depth-first, invoking fn for
// each unwrapped cause. Supports both the single Unwrap() error form and the
// Go 1.20+ Unwrap() []error form (errors.Join, fmt.Errorf with multiple %w).
func walkCauses(err error, fn func(error)) {
	type single interface{ Unwrap() error }
	type multi interface{ Unwrap() []error }

	switch u := err.(type) {
	case multi:
		for _, sub := range u.Unwrap() {
			if sub == nil {
				continue
			}
			fn(sub)
			walkCauses(sub, fn)
		}
	case single:
		if sub := u.Unwrap(); sub != nil {
			fn(sub)
			walkCauses(sub, fn)
		}
	}
}

// exitFn is overridable for tests; production always calls os.Exit.
var exitFn = defaultExit

func defaultExit(code int) { os.Exit(code) }
