//go:build !windows

package keychain

import (
	"os"
	"path/filepath"
)

// legacyFallbackDir is where releases before the state directory kept the
// plaintext file. It delegates to os.UserConfigDir, which returns:
//   - macOS: ~/Library/Application Support
//   - Linux: $XDG_CONFIG_HOME (defaults to ~/.config)
func legacyFallbackDir() (string, error) {
	return os.UserConfigDir()
}

// platformStateDir returns the XDG default state directory, ~/.local/state.
// macOS uses the same path, as the SafeDep tools do.
func platformStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}
