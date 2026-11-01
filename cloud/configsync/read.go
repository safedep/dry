package configsync

import (
	"fmt"
	"io"
	"path/filepath"
)

const (
	// maxPolicySize is the gRPC receive limit of the fetch. A resolved
	// policy is at most 2 MiB, so a larger file is not a policy.
	maxPolicySize = 4 << 20

	// maxStateSize bounds state.json, which holds only metadata.
	maxStateSize = 1 << 20
)

// readCacheFile reads one file of the cache folder. Each read of a cache file
// goes through it. The user owns the folder, and a tool that runs as root
// can read it (spec 3.2). Without these checks, a symlink makes root read a
// file that the user cannot read, a FIFO or a device blocks the tool, and a
// large file uses all its memory.
func readCacheFile(dir, name string, limit int64) ([]byte, error) {
	path := filepath.Join(dir, name)
	f, err := openNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrUnsafeFile, name)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%w: %s has more than %d bytes", ErrTooLarge, name, limit)
	}

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s has more than %d bytes", ErrTooLarge, name, limit)
	}
	return data, nil
}
