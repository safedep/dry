//go:build !unix

package keychain

import "os"

// ownedByCurrentUser reports true where the platform has no Unix owner.
// Windows keeps the old path by default, so the move does not run there.
func ownedByCurrentUser(os.FileInfo) bool {
	return true
}
