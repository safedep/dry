//go:build !unix

package configsync

import (
	"fmt"
	"os"
)

// openNoFollow opens path only when Lstat reports a regular file. On Windows
// Lstat reports a symlink, a junction and each other reparse point as not
// regular. The check of the open file against the Lstat result refuses a file
// that changed between the two calls.
func openNoFollow(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrUnsafeFile, path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !os.SameFile(before, after) {
		_ = f.Close()
		return nil, fmt.Errorf("%w: %s changed while it was opened", ErrUnsafeFile, path)
	}
	return f, nil
}
