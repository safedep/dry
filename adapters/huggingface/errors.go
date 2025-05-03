package huggingface

import (
	"errors"
)

// Error constants for the HuggingFace adapter
var (
	ErrInvalidRequest  = errors.New("invalid request")
	ErrNetworkError    = errors.New("network error")
	ErrIOError         = errors.New("io error")
	ErrInvalidResponse = errors.New("invalid response")
	ErrAPIError        = errors.New("api error")
)

// Wrap wraps an error with additional context
func wrap(err error, wrapErr error, msg string) error {
	return errors.New(msg + ": " + wrapErr.Error() + ": " + err.Error())
}
