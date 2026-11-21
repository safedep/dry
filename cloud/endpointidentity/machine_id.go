// Package endpointidentity derives the MachineId of an endpoint from its
// endpoint ID. It imports only the standard library, so a server can check a
// client's MachineId without the client packages of cloud/endpointsync.
package endpointidentity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACKey is the key of every MachineId that SafeDep tools derive, from a
// configured endpoint ID or from the host machine ID.
const HMACKey = "safedep"

// MachineID returns the MachineId that a client sends for a configured
// endpoint ID: the hex HMAC-SHA256 of endpointID with HMACKey, over the raw
// bytes with no normalisation. A server calls it to check that a client's
// MachineId matches an endpoint ID that the server derived on its own. The
// output is frozen, because the backend stores it as the identity of the
// endpoint. A client never sends the value of an empty ID, so a server rejects
// an empty ID before it compares.
func MachineID(endpointID string) string {
	mac := hmac.New(sha256.New, []byte(HMACKey))
	mac.Write([]byte(endpointID))
	return hex.EncodeToString(mac.Sum(nil))
}
