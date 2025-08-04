package errors

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSinkErrorBatcher_NewSinkErrorBatcher(t *testing.T) {
	t.Run("should create batcher with valid handler", func(t *testing.T) {
		config := DefaultSinkConfig()
		handler := func(*errorWithMeta) error { return nil }
		closer := func() error { return nil }

		batcher, err := newSinkErrorBatcher(config, handler, closer)
		require.NoError(t, err)
		require.NotNil(t, batcher)

		// Clean up
		err = batcher.Close()
		assert.NoError(t, err)
	})

	t.Run("should fail with nil handler", func(t *testing.T) {
		config := DefaultSinkConfig()

		batcher, err := newSinkErrorBatcher(config, nil, nil)
		assert.Error(t, err)
		assert.Nil(t, batcher)
		assert.Contains(t, err.Error(), "driverHandler cannot be nil")
	})
}

func TestSinkErrorBatcher_Handle(t *testing.T) {
	t.Run("should handle error successfully", func(t *testing.T) {
		config := DefaultSinkConfig()
		var handledError *errorWithMeta
		var mu sync.Mutex

		handler := func(em *errorWithMeta) error {
			mu.Lock()
			handledError = em
			mu.Unlock()
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)
		defer batcher.Close()

		testErr := errors.New("test error")
		batcher.Handle(testErr, WithErrorCode("TEST001"))

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		assert.NotNil(t, handledError)
		assert.Equal(t, testErr, handledError.err)
		assert.Equal(t, "TEST001", handledError.meta.errorCode)
		mu.Unlock()
	})

	t.Run("should ignore nil errors", func(t *testing.T) {
		config := DefaultSinkConfig()
		var handledCount int32

		handler := func(*errorWithMeta) error {
			atomic.AddInt32(&handledCount, 1)
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)
		defer batcher.Close()

		batcher.Handle(nil)
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, int32(0), atomic.LoadInt32(&handledCount))
	})

	t.Run("should handle multiple errors", func(t *testing.T) {
		config := DefaultSinkConfig()
		var handledCount int32

		handler := func(*errorWithMeta) error {
			atomic.AddInt32(&handledCount, 1)
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)
		defer batcher.Close()

		for i := 0; i < 10; i++ {
			batcher.Handle(errors.New("test error"))
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		assert.Equal(t, int32(10), atomic.LoadInt32(&handledCount))
	})

	t.Run("should drop errors when overloaded", func(t *testing.T) {
		config := SinkConfig{MaxBatchSize: 2}
		var handledCount int32

		// Slow handler to cause overload
		handler := func(*errorWithMeta) error {
			atomic.AddInt32(&handledCount, 1)
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)
		defer batcher.Close()

		// Send more errors than buffer can handle
		for i := 0; i < 10; i++ {
			batcher.Handle(errors.New("test error"))
		}

		// Wait for processing
		time.Sleep(500 * time.Millisecond)

		// Should handle fewer than 10 due to dropping
		handled := atomic.LoadInt32(&handledCount)
		assert.True(t, handled < 10, "Expected some errors to be dropped, but handled %d", handled)
	})

	t.Run("should not handle errors after close", func(t *testing.T) {
		config := DefaultSinkConfig()
		var handledCount int32

		handler := func(*errorWithMeta) error {
			atomic.AddInt32(&handledCount, 1)
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)

		err = batcher.Close()
		assert.NoError(t, err)

		batcher.Handle(errors.New("test error"))
		time.Sleep(100 * time.Millisecond)

		assert.Equal(t, int32(0), atomic.LoadInt32(&handledCount))
	})

	t.Run("should never block when buffer is full", func(t *testing.T) {
		config := SinkConfig{MaxBatchSize: 1} // Very small buffer

		// Handler that blocks to fill the buffer
		var handlerStarted sync.Once
		handlerStartedCh := make(chan struct{})
		handlerBlock := make(chan struct{})

		handler := func(*errorWithMeta) error {
			handlerStarted.Do(func() {
				close(handlerStartedCh)
			})
			<-handlerBlock // Block until we say continue
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)
		defer func() {
			close(handlerBlock) // Unblock handler
			batcher.Close()
		}()

		// Fill the buffer with one error (handler will block)
		batcher.Handle(errors.New("blocking error"))
		<-handlerStartedCh // Wait for handler to start

		// Now try to send many more errors - these should not block
		done := make(chan bool, 1)
		go func() {
			for i := 0; i < 100; i++ {
				batcher.Handle(errors.New("test error"))
			}
			done <- true
		}()

		// If Handle blocks, this will timeout
		select {
		case <-done:
			// Success - all Handle calls completed without blocking
		case <-time.After(1 * time.Second):
			t.Fatal("Handle method blocked when buffer was full")
		}
	})
}

func TestSinkErrorBatcher_Close(t *testing.T) {
	t.Run("should close gracefully", func(t *testing.T) {
		config := DefaultSinkConfig()
		var handledCount int32
		var closerCalled bool

		handler := func(*errorWithMeta) error {
			atomic.AddInt32(&handledCount, 1)
			return nil
		}

		closer := func() error {
			closerCalled = true
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, closer)
		require.NoError(t, err)

		// Add some errors
		for i := 0; i < 5; i++ {
			batcher.Handle(errors.New("test error"))
		}

		err = batcher.Close()
		assert.NoError(t, err)
		assert.True(t, closerCalled)
		assert.Equal(t, int32(5), atomic.LoadInt32(&handledCount))
	})

	t.Run("should handle closer error", func(t *testing.T) {
		config := DefaultSinkConfig()
		expectedErr := errors.New("closer error")

		handler := func(*errorWithMeta) error { return nil }
		closer := func() error { return expectedErr }

		batcher, err := newSinkErrorBatcher(config, handler, closer)
		require.NoError(t, err)

		err = batcher.Close()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to close sink driver")
	})

	t.Run("should be idempotent", func(t *testing.T) {
		config := DefaultSinkConfig()
		handler := func(*errorWithMeta) error { return nil }

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)

		err1 := batcher.Close()
		err2 := batcher.Close()

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})

	t.Run("should timeout on slow processing", func(t *testing.T) {
		config := DefaultSinkConfig()

		// Very slow handler
		handler := func(*errorWithMeta) error {
			time.Sleep(10 * time.Second)
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)

		// Add an error
		batcher.Handle(errors.New("slow error"))

		start := time.Now()
		err = batcher.Close()
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.True(t, duration < 6*time.Second, "Close should timeout after 5 seconds")
	})

	t.Run("should respect custom close timeout", func(t *testing.T) {
		config := SinkConfig{
			MaxBatchSize: 10,
			CloseTimeout: 1 * time.Second, // Short timeout
		}

		// Slow handler that takes longer than timeout
		handler := func(*errorWithMeta) error {
			time.Sleep(2 * time.Second)
			return nil
		}

		batcher, err := newSinkErrorBatcher(config, handler, nil)
		require.NoError(t, err)

		// Add an error
		batcher.Handle(errors.New("slow error"))

		start := time.Now()
		err = batcher.Close()
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.True(t, duration >= 1*time.Second, "Should wait at least the timeout duration")
		assert.True(t, duration < 1500*time.Millisecond, "Should timeout after custom timeout")
	})
}

func TestSinkMetaFunctions(t *testing.T) {
	t.Run("WithErrorCode should set error code", func(t *testing.T) {
		meta := sinkMeta{}
		fn := WithErrorCode("TEST001")
		fn(&meta)

		assert.Equal(t, "TEST001", meta.errorCode)
	})

	t.Run("WithValue should add single value", func(t *testing.T) {
		meta := sinkMeta{}
		fn := WithValue("key1", "value1")
		fn(&meta)

		assert.Equal(t, "value1", meta.values["key1"])
	})

	t.Run("WithValue should initialize values map", func(t *testing.T) {
		meta := sinkMeta{}
		fn := WithValue("key1", "value1")
		fn(&meta)

		assert.NotNil(t, meta.values)
		assert.Equal(t, 1, len(meta.values))
	})

	t.Run("WithValues should set multiple values", func(t *testing.T) {
		meta := sinkMeta{}
		values := map[string]any{
			"key1": "value1",
			"key2": 42,
		}
		fn := WithValues(values)
		fn(&meta)

		assert.Equal(t, values, meta.values)
	})

	t.Run("WithValues should override existing values", func(t *testing.T) {
		meta := sinkMeta{
			values: map[string]any{"old": "value"},
		}
		values := map[string]any{
			"key1": "value1",
			"key2": 42,
		}
		fn := WithValues(values)
		fn(&meta)

		assert.Equal(t, values, meta.values)
		assert.NotContains(t, meta.values, "old")
	})
}

func TestSinkConfig(t *testing.T) {
	t.Run("DefaultSinkConfig should return valid config", func(t *testing.T) {
		config := DefaultSinkConfig()
		assert.Equal(t, 100, config.MaxBatchSize)
		assert.Equal(t, 5*time.Second, config.CloseTimeout)
	})
}
