//go:build linux || darwin

package configsync

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A daemon that runs with umask 077 asks for 0755 folders. Each folder that
// the store creates, also above the cache folder, must get 0755, or a tool
// cannot pass through it to read the cache. A folder that exists keeps its
// mode.
func TestStore_NewParentFoldersGetTheDirMode(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	root := t.TempDir()
	existing := filepath.Join(root, "var-lib")
	require.NoError(t, os.Mkdir(existing, 0o711))
	require.NoError(t, os.Chmod(existing, 0o711))

	dir := filepath.Join(existing, "safedep", "cloud")
	store, err := NewStore(dir, WithDirMode(0o755), WithFileMode(0o644))
	require.NoError(t, err)

	transport := NewMockTransport(t)
	fetcher, err := NewFetcher(store, transport)
	require.NoError(t, err)
	transport.EXPECT().GetResolvedPolicy(mock.Anything, mock.Anything).Return(modified(policyA, 1), nil).Once()
	_, err = fetcher.Fetch(context.Background(), request(tenantA))
	require.NoError(t, err)

	for _, d := range []string{filepath.Join(existing, "safedep"), dir} {
		info, err := os.Stat(d)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), d)
	}

	info, err := os.Stat(existing)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o711), info.Mode().Perm(), "a folder that exists keeps its mode")
}
