// tui/output/width.go
package output

import (
	"os"
	"sync"

	"golang.org/x/term"
)

// DefaultWidth is used when the terminal width cannot be detected.
const DefaultWidth = 80

var (
	widthMu   sync.RWMutex
	widthOver int // 0 means "no override; query terminal each draw"
)

// Width returns the current terminal width. It is re-queried on every call
// — never cached — so SIGWINCH / resize-during-render produces correct
// wrapping on the next draw without explicit signal handling.
func Width() int {
	widthMu.RLock()
	o := widthOver
	widthMu.RUnlock()
	if o > 0 {
		return o
	}
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 0 {
		return w
	}
	return DefaultWidth
}

// SetWidthOverride forces Width() to return n; 0 disables the override.
// Used by the example program's --width flag and by tests.
func SetWidthOverride(n int) {
	widthMu.Lock()
	widthOver = n
	widthMu.Unlock()
}
