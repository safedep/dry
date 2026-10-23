//go:build darwin || windows

package keychain

// sessionBusReachable is a Linux check. The keychains of macOS and Windows
// do not use D-Bus.
func sessionBusReachable() error {
	return nil
}
