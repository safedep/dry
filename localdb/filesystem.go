package localdb

import "fmt"

// networkFilesystem reports the file system type of dir and whether it is a
// network file system. Tests replace it.
var networkFilesystem = detectNetworkFilesystem

// CheckLocalFilesystem returns an ErrCodeUnsafeFilesystem error when dir is on
// a network file system. WAL mode can corrupt the database there. An empty dir
// means the current directory. The check runs on Linux and macOS. On other
// systems it always passes.
func CheckLocalFilesystem(dir string) error {
	if dir == "" {
		dir = "."
	}

	fsType, network, err := networkFilesystem(dir)
	if err != nil {
		return newError(ErrCodeOpenFailure, fmt.Sprintf("inspect file system of %s", dir), err)
	}

	if network {
		return newError(ErrCodeUnsafeFilesystem,
			fmt.Sprintf("%s is on a network file system (%s)", dir, fsType), nil)
	}

	return nil
}
