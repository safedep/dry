// tui/theme/registry_test.go
package theme

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReturnsSafeDep(t *testing.T) {
	resetDefaultForTest()
	d := Default()
	assert.Equal(t, "safedep", d.Name())
}

func TestSetDefault(t *testing.T) {
	resetDefaultForTest()

	custom := &basicTheme{name: "test"}
	SetDefault(custom)
	assert.Equal(t, "test", Default().Name())

	resetDefaultForTest()
	assert.Equal(t, "safedep", Default().Name())
}

func TestSetDefaultNilPanics(t *testing.T) {
	resetDefaultForTest()

	assert.PanicsWithValue(t, "theme.SetDefault: nil Theme", func() {
		SetDefault(nil)
	})
	assert.Equal(t, "safedep", Default().Name())
}

func TestDefaultConcurrentReadWrite(t *testing.T) {
	resetDefaultForTest()

	// Mix concurrent readers and writers so the race detector has actual work
	// to do. Without a concurrent writer, the RWMutex contract is unexercised.
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = Default().Name()
				}
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			t := &basicTheme{name: "rotating"}
			SetDefault(t)
		}(i)
	}

	// Let both sides run briefly, then stop readers.
	for i := 0; i < 100; i++ {
		_ = Default().Name()
	}
	close(stop)
	wg.Wait()
	resetDefaultForTest()
}
