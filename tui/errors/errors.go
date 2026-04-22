// tui/errors/errors.go
//
// Package errors provides verbosity-aware error printing with process exit.
// Replaces the former dry/tui/error.go.
package errors

import (
	"fmt"
	"os"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
)

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
	line := style.Error(err.Error())
	_, _ = fmt.Fprintln(output.Stderr(), line)
	if output.CurrentVerbosity() >= output.Verbose {
		walkCauses(err, func(cause error) {
			_, _ = fmt.Fprintln(output.Stderr(), style.Faint("  caused by: "+cause.Error()))
		})
	}
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
