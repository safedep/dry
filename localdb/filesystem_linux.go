package localdb

import "golang.org/x/sys/unix"

// linuxNetworkFSTypes maps statfs f_type magic numbers to names.
var linuxNetworkFSTypes = map[uint32]string{
	0x6969:     "nfs",
	0x517b:     "smb",
	0xff534d42: "cifs",
	0xfe534d42: "smb2",
	0x5346414f: "afs",
	0x01021997: "9p",
	0x00c36400: "ceph",
	0x0bd00bd0: "lustre",
	0x47504653: "gpfs",
}

func detectNetworkFilesystem(dir string) (string, bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return "", false, err
	}

	// f_type is a 32-bit magic number. Its Go type differs by architecture.
	name, ok := linuxNetworkFSTypes[uint32(st.Type)] //nolint:gosec // f_type magic numbers fit in 32 bits
	return name, ok, nil
}
