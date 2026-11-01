//go:build unix

package configsync

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// openNoFollow opens path and refuses a symlink in its last element.
// O_NONBLOCK keeps the open of a FIFO from a wait for a writer. readCacheFile
// then refuses the FIFO, because it is not a regular file.
func openNoFollow(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		// Linux and macOS return ELOOP for a symlink, FreeBSD returns EMLINK.
		if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.EMLINK) {
			return nil, fmt.Errorf("%w: %s is a symlink", ErrUnsafeFile, path)
		}
		return nil, err
	}
	return f, nil
}
