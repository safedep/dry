//go:build unix

package keychain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileProviderSkipsMoveOfAnotherUsersFile(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to give the file another owner")
	}

	stateDir, legacyDir := isolateDirs(t)
	legacy := filepath.Join(legacyDir, "myapp", credsFileName)
	writeFile(t, legacy, `{"version":1,"secrets":{}}`)
	require.NoError(t, os.Chown(legacy, 12345, 12345))

	fp, err := newFileProvider("myapp", "")
	require.NoError(t, err)
	assert.Equal(t, legacy, fp.filePath)
	assert.FileExists(t, legacy)
	assert.NoDirExists(t, filepath.Join(stateDir, "myapp"))
}

func TestOwnedByCurrentUser(t *testing.T) {
	info, err := os.Stat(t.TempDir())
	require.NoError(t, err)
	assert.True(t, ownedByCurrentUser(info))
}
