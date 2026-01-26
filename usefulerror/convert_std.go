package usefulerror

import (
	"errors"
	"io/fs"
	"os"
)

// Here we register all internal error converters for standard error types. This is the place where we
// should register common error types for which we want to provide a useful error message. This does not
// mean we should register every single error type, but rather the most common ones that are likely to
// be encountered in practice.
func init() {
	registerInternalErrorConverters("os/err_not_exist", func(err error) (UsefulError, bool) {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			return NewUsefulError().
				WithCode(ErrNotFound).
				WithHumanError("File or directory not found").
				WithHelp("Check if the path exists").
				WithAdditionalHelp("Make sure the path exists and is accessible.").
				Wrap(err), true
		}

		return nil, false
	})
}
