package configsync

import "errors"

var (
	// ErrNoCache means that the store holds no usable policy: no state, an
	// unreadable state, a state of a newer format, or no policy file that
	// passes its checks. The tool then applies its baseline.
	ErrNoCache = errors.New("configsync: no usable cache")

	// ErrUnsupportedFormat means that state.json has a format_version that
	// this package does not know. Errors that wrap it also wrap ErrNoCache.
	ErrUnsupportedFormat = errors.New("configsync: unsupported state format")

	// ErrDigestMismatch means that bytes do not match their digest: a reply
	// of the server, or a policy file in the cache.
	ErrDigestMismatch = errors.New("configsync: digest mismatch")

	// ErrInvalidDigest means that a digest is not "sha256:" and 64 lower
	// case hex characters.
	ErrInvalidDigest = errors.New("configsync: invalid digest")

	// ErrLowerRevision means that the server sent a revision lower than the
	// highest revision that the cache saw for the tenant.
	ErrLowerRevision = errors.New("configsync: revision is lower than the highest revision")

	// ErrRejected means that the validator of the tool refused the policy.
	ErrRejected = errors.New("configsync: policy rejected by the validator")

	// ErrUnknownStatus means that the reply has a status that this package
	// does not know.
	ErrUnknownStatus = errors.New("configsync: unknown reply status")

	// ErrUnsafeFile means that a cache file is a symlink, a FIFO, a device,
	// a folder or another file that is not a regular file.
	ErrUnsafeFile = errors.New("configsync: cache file is not a regular file")

	// ErrTooLarge means that a cache file is larger than its limit: 4 MiB for
	// a policy file, 1 MiB for state.json.
	ErrTooLarge = errors.New("configsync: cache file is too large")

	// ErrInvalidRequest means that a FetchRequest misses a required field.
	ErrInvalidRequest = errors.New("configsync: invalid fetch request")
)
