package keychain

import "os"

// legacyFallbackDir is where releases before the state directory kept the
// plaintext file. On Windows it is the same as platformStateDir.
func legacyFallbackDir() (string, error) {
	return platformStateDir()
}

// platformStateDir returns %LOCALAPPDATA% on Windows.
// Unlike %APPDATA% (returned by os.UserConfigDir), LOCALAPPDATA is
// machine-local and does not roam across domain-joined machines.
// Credentials should stay tied to the machine they were created on.
func platformStateDir() (string, error) {
	dir := os.Getenv("LOCALAPPDATA")
	if dir != "" {
		return dir, nil
	}

	// Fall back to os.UserConfigDir if LOCALAPPDATA is not set
	return os.UserConfigDir()
}
