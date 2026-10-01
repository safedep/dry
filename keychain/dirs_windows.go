package keychain

import "os"

// localConfigDir returns %LOCALAPPDATA% on Windows.
// Unlike %APPDATA% (returned by os.UserConfigDir), LOCALAPPDATA is
// machine-local and does not roam across domain-joined machines.
// Credentials should stay tied to the machine they were created on.
func localConfigDir() (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir != "" {
		return dir, nil
	}

	// Fall back to os.UserConfigDir if LOCALAPPDATA is not set
	return os.UserConfigDir()
}
