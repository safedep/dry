//go:build !windows

package keychain

import "os"

// localConfigDir returns the platform-appropriate config directory.
// On non-Windows platforms, this delegates to os.UserConfigDir which returns:
//   - macOS: ~/Library/Application Support
//   - Linux: $XDG_CONFIG_HOME (defaults to ~/.config)
func localConfigDir() (string, error) {
	return os.UserConfigDir()
}
