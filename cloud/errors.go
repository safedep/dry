package cloud

import "errors"

var (
	// ErrInvalidCredentialType is returned when credentials don't match the
	// expected type for the client (e.g., API key for control plane).
	ErrInvalidCredentialType = errors.New("cloud: invalid credential type for this client")

	// ErrMissingCredentials is returned when required credential fields are empty.
	ErrMissingCredentials = errors.New("cloud: missing required credentials")
)
