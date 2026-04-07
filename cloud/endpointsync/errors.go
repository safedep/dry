package endpointsync

import "errors"

var (
	// ErrMissingTransport is returned when no transport is provided.
	ErrMissingTransport = errors.New("endpointsync: transport is required")

	// ErrMissingIdentity is returned when no identity resolver is provided.
	ErrMissingIdentity = errors.New("endpointsync: identity resolver is required")

	// ErrWALOpen is returned when the WAL database cannot be opened.
	ErrWALOpen = errors.New("endpointsync: failed to open WAL database")

	// ErrWALFull is returned by Emit() when pending events reach MaxPending.
	// Tools must handle this gracefully and continue operating without sync.
	ErrWALFull = errors.New("endpointsync: WAL is full, sync required before emitting more events")
)
