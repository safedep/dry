//go:build windows

package configsync

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holdOpen opens path as a reader in another process does: with no
// FILE_SHARE_DELETE, so Windows refuses a rename over the file until the
// handle closes. The returned func closes it, and a second call does nothing.
func holdOpen(t *testing.T, path string) func() {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	require.NoError(t, err)
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)

	var once sync.Once
	release := func() { once.Do(func() { _ = syscall.CloseHandle(h) }) }
	t.Cleanup(release)
	return release
}

func TestStore_CommitWhileAReaderHoldsTheStateOnWindows(t *testing.T) {
	cases := []struct {
		name       string
		holdFor    time.Duration // 0 holds the file for the whole fetch
		wantPolicy []byte
	}{
		{name: "the retry outlasts a short read", holdFor: 50 * time.Millisecond, wantPolicy: policyB},
		{name: "a long hold fails the fetch and keeps the cache", wantPolicy: policyA},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(policyA, 1)
			statePath := filepath.Join(f.dir, stateFileName)

			// The retry runs only when a held file refuses a plain rename.
			release := holdOpen(t, statePath)
			other := filepath.Join(f.dir, "other")
			require.NoError(t, os.WriteFile(other, policyA, 0o600))
			renameErr := os.Rename(other, statePath)
			release()
			if renameErr == nil {
				t.Skip("this Windows renames over a held file, so the retry never runs")
			}
			require.NoError(t, os.Remove(other))

			release = holdOpen(t, statePath)
			if tc.holdFor > 0 {
				go func() {
					time.Sleep(tc.holdFor)
					release()
				}()
			}

			f.tick()
			f.reply(modified(policyB, 2), nil)
			_, err := f.fetcher.Fetch(context.Background(), request())
			if tc.holdFor > 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			release()

			loaded, err := f.store.Load(acceptAll)
			require.NoError(t, err)
			assert.Equal(t, tc.wantPolicy, loaded.Policy)
		})
	}
}
