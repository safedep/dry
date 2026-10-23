//go:build linux

package keychain

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDBusLaunch puts a dbus-launch first on PATH that records each run and
// fails, and returns the path of its record.
func fakeDBusLaunch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "dbus-launch-ran")
	t.Setenv("DBUS_LAUNCH_MARKER", marker)
	script := "#!/bin/sh\ntouch \"$DBUS_LAUNCH_MARKER\"\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dbus-launch"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

// skipWithSessionBus skips a test on a host where godbus finds a session bus
// without dbus-launch, because the keychain never runs dbus-launch there.
func skipWithSessionBus(t *testing.T) {
	t.Helper()
	u, err := user.Current()
	require.NoError(t, err)
	for _, name := range []string{"bus", "dbus-session"} {
		if _, err := os.Stat(filepath.Join("/run/user", u.Uid, name)); err == nil {
			t.Skip("this host has a session bus, so the keychain never needs dbus-launch")
		}
	}
}

func TestNewDoesNotStartASessionBus(t *testing.T) {
	skipWithSessionBus(t)
	marker := fakeDBusLaunch(t)

	for _, address := range []string{"", "autolaunch:"} {
		t.Run("DBUS_SESSION_BUS_ADDRESS="+address, func(t *testing.T) {
			t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)

			kc, err := New(Config{
				AppName:              "test-app",
				InsecureFileFallback: true,
				FilePath:             filepath.Join(t.TempDir(), "creds.json"),
			})
			require.NoError(t, err)
			assert.NoError(t, kc.Close())

			assert.NoFileExists(t, marker, "keychain.New ran dbus-launch")
		})
	}
}

func TestSessionBusReachableLeavesAnExplicitAddressToGoKeyring(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	assert.NoError(t, sessionBusReachable())
}
