// tui/theme/registry.go
package theme

import "sync"

var (
	defaultMu sync.RWMutex
	def       Theme = SafeDep()
)

// Default returns the globally installed Theme. Safe to call from any
// goroutine; components call this at every render.
func Default() Theme {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return def
}

// SetDefault replaces the global Theme. Typical usage: called once at program
// startup before any output is emitted. Goroutine-safe.
func SetDefault(t Theme) {
	if t == nil {
		panic("theme.SetDefault: nil Theme")
	}
	defaultMu.Lock()
	def = t
	defaultMu.Unlock()
}

// resetDefaultForTest restores the SafeDep theme; test-only.
func resetDefaultForTest() {
	defaultMu.Lock()
	def = SafeDep()
	defaultMu.Unlock()
}
