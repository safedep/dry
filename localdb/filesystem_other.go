//go:build !linux && !darwin

package localdb

func detectNetworkFilesystem(string) (string, bool, error) {
	return "", false, nil
}
