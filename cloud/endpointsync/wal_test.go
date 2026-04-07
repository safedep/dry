package endpointsync

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWAL(t *testing.T) {
	t.Run("open and close", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		require.NoError(t, w.close())
	})

	t.Run("insert and read pending", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		require.NoError(t, w.insert("evt-1", []byte("payload-1")))
		require.NoError(t, w.insert("evt-2", []byte("payload-2")))

		events, err := w.readPending(10)
		require.NoError(t, err)
		assert.Len(t, events, 2)
		assert.Equal(t, "evt-1", events[0].eventID)
		assert.Equal(t, []byte("payload-1"), events[0].payload)
		assert.Equal(t, "evt-2", events[1].eventID)
	})

	t.Run("mark delivered and purge", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		require.NoError(t, w.insert("evt-1", []byte("p1")))
		require.NoError(t, w.insert("evt-2", []byte("p2")))
		require.NoError(t, w.insert("evt-3", []byte("p3")))

		require.NoError(t, w.markDelivered([]string{"evt-1", "evt-3"}))
		require.NoError(t, w.purge())

		events, err := w.readPending(10)
		require.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, "evt-2", events[0].eventID)
	})

	t.Run("duplicate event ID is rejected", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		require.NoError(t, w.insert("evt-1", []byte("p1")))
		err = w.insert("evt-1", []byte("p1-dup"))
		assert.Error(t, err)
	})

	t.Run("read pending respects limit", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		for i := 0; i < 10; i++ {
			require.NoError(t, w.insert(fmt.Sprintf("evt-%d", i), []byte("p")))
		}

		events, err := w.readPending(3)
		require.NoError(t, err)
		assert.Len(t, events, 3)
	})
}

func TestWALBounds(t *testing.T) {
	t.Run("max pending enforced", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		w.maxPending = 3

		require.NoError(t, w.insert("evt-1", []byte("p")))
		require.NoError(t, w.insert("evt-2", []byte("p")))
		require.NoError(t, w.insert("evt-3", []byte("p")))

		err = w.insert("evt-4", []byte("p"))
		assert.ErrorIs(t, err, ErrWALFull)
	})

	t.Run("max pending recovers after delivery", func(t *testing.T) {
		w, err := openWAL(filepath.Join(t.TempDir(), "test.db"))
		require.NoError(t, err)
		defer func() { _ = w.close() }()

		w.maxPending = 2

		require.NoError(t, w.insert("evt-1", []byte("p")))
		require.NoError(t, w.insert("evt-2", []byte("p")))

		err = w.insert("evt-3", []byte("p"))
		assert.ErrorIs(t, err, ErrWALFull)

		require.NoError(t, w.markDelivered([]string{"evt-1"}))

		require.NoError(t, w.insert("evt-3", []byte("p")))
	})

	t.Run("pending count accurate after open with existing data", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")

		w, err := openWAL(dbPath)
		require.NoError(t, err)
		require.NoError(t, w.insert("evt-1", []byte("p")))
		require.NoError(t, w.insert("evt-2", []byte("p")))
		require.NoError(t, w.close())

		w2, err := openWAL(dbPath)
		require.NoError(t, err)
		defer func() { _ = w2.close() }()

		w2.maxPending = 3
		require.NoError(t, w2.insert("evt-3", []byte("p")))

		err = w2.insert("evt-4", []byte("p"))
		assert.ErrorIs(t, err, ErrWALFull)
	})
}
