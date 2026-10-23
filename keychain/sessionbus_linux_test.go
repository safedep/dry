//go:build linux

package keychain

import (
	"fmt"
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
	script := fmt.Sprintf("#!/bin/sh\ntouch %q\nexit 1\n", marker)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dbus-launch"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func TestNewDoesNotStartASessionBus(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join("/run/user", u.Uid, "bus")); err == nil {
		t.Skip("this host has a session bus, so the keychain never needs dbus-launch")
	}

	marker := fakeDBusLaunch(t)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")

	kc, err := New(Config{
		AppName:              "test-app",
		InsecureFileFallback: true,
		FilePath:             filepath.Join(t.TempDir(), "creds.json"),
	})
	require.NoError(t, err)
	assert.NoError(t, kc.Close())

	assert.NoFileExists(t, marker, "keychain.New ran dbus-launch")
}

func TestSessionBusReachableFailsWithoutABus(t *testing.T) {
	marker := fakeDBusLaunch(t)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))

	err := sessionBusReachable()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no D-Bus session bus")
	assert.NoFileExists(t, marker)
}
