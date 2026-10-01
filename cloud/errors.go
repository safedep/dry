package cloud

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidCredentialType is returned when credentials don't match the
	// expected type for the client (e.g., API key for control plane).
	ErrInvalidCredentialType = errors.New("cloud: invalid credential type for this client")

	// ErrMissingCredentials is returned when required credential fields are empty.
	ErrMissingCredentials = errors.New("cloud: missing required credentials")

	// ErrIncompleteCredentials is returned when a source holds one half of a
	// credential pair, for example an API key with no tenant. It wraps
	// ErrMissingCredentials, so existing errors.Is checks still match.
	ErrIncompleteCredentials = fmt.Errorf("cloud: incomplete credentials: %w", ErrMissingCredentials)
)
