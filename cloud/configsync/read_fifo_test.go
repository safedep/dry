//go:build linux || darwin

package configsync

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A FIFO with no writer blocks a plain open for read. The read must return
// at once and refuse the file.
func TestStore_RefusesAFIFO(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	f.tick()
	f.commit(tenantA, policyB, 2)

	current := filepath.Join(f.dir, fileOf(t, policyB)[0])
	require.NoError(t, os.Remove(current))
	require.NoError(t, syscall.Mkfifo(current, 0o600))

	done := make(chan struct{})
	var loaded *Loaded
	var err error
	go func() {
		defer close(done)
		loaded, err = f.store.Load(nil)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Load waits on a FIFO")
	}

	require.NoError(t, err)
	assert.True(t, loaded.FromPrevious)
	assert.ErrorIs(t, loaded.CurrentErr, ErrUnsafeFile)

	state := filepath.Join(f.dir, stateFileName)
	require.NoError(t, os.Remove(state))
	require.NoError(t, syscall.Mkfifo(state, 0o600))
	_, err = f.store.State()
	assert.ErrorIs(t, err, ErrUnsafeFile)
	assert.ErrorIs(t, err, ErrNoCache)
}
