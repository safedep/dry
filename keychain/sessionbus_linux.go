//go:build linux

package keychain

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// sessionBusReachable reports whether a D-Bus session bus already runs for
// this user. go-keyring connects with dbus.SessionBus, which runs dbus-launch
// when it finds no bus. dbus-launch starts a new daemon that outlives the
// process, so each call on a host without a session leaked one daemon
// (safedep/pmg#579). This check uses the bus discovery of godbus and never
// runs dbus-launch.
func sessionBusReachable() error {
	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return fmt.Errorf("keychain: no D-Bus session bus: %w", err)
	}
	return conn.Close()
}
