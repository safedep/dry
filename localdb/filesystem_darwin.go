package localdb

import "golang.org/x/sys/unix"

func detectNetworkFilesystem(dir string) (string, bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return "", false, err
	}

	return unix.ByteSliceToString(st.Fstypename[:]), st.Flags&unix.MNT_LOCAL == 0, nil
}
