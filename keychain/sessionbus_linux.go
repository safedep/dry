//go:build linux

package keychain

import (
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

// sessionBusReachable reports whether a D-Bus session bus already runs for
// this user. go-keyring connects with dbus.SessionBus, which runs dbus-launch
// when it finds no bus. dbus-launch starts a new daemon that outlives the
// process, so each call on a host without a session leaked one daemon
// (safedep/pmg#579). This check uses the bus discovery of godbus and never
// runs dbus-launch.
//
// An address in DBUS_SESSION_BUS_ADDRESS never starts dbus-launch, so the
// check leaves it to go-keyring. A second dial to an unreachable tcp: address
// would double the wait.
func sessionBusReachable() error {
	if address := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); address != "" && address != "autolaunch:" {
		return nil
	}

	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return fmt.Errorf("no D-Bus session bus: %w", err)
	}
	return conn.Close()
}
