// tui/output/writer.go
package output

import (
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
)

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	n, err := lw.w.Write(p)
	if err != nil && (errors.Is(err, syscall.EPIPE) || isEPIPE(err)) {
		// Piped consumer closed early (e.g., `pmg scan | head`).
		// Swallow the error — we pretend the write succeeded.
		return len(p), nil
	}
	return n, err
}

func isEPIPE(err error) bool {
	var sc syscall.Errno
	if errors.As(err, &sc) {
		return sc == syscall.EPIPE
	}
	return false
}

var (
	writersMu sync.RWMutex
	stdoutLW  = &lockedWriter{w: os.Stdout}
	stderrLW  = &lockedWriter{w: os.Stderr}
)

// writerRef is a stable indirection that always delegates to the current
// stdoutLW/stderrLW pointer at Write time. Callers who cache the Writer
// returned by Stdout()/Stderr() still see a live reference after SetWriters,
// eliminating the race where a cached *lockedWriter points at the pre-swap
// instance.
type writerRef struct{ which int }

const (
	refStdout = iota
	refStderr
)

func (r writerRef) Write(p []byte) (int, error) {
	writersMu.RLock()
	lw := stdoutLW
	if r.which == refStderr {
		lw = stderrLW
	}
	writersMu.RUnlock()
	return lw.Write(p)
}

// Stdout returns the mutex-serialized writer for data output.
// The returned Writer is stable across SetWriters calls.
func Stdout() io.Writer { return writerRef{which: refStdout} }

// Stderr returns the mutex-serialized writer for human chatter, spinners,
// progress, banners, and errors. Stable across SetWriters calls.
func Stderr() io.Writer { return writerRef{which: refStderr} }

// SetWriters replaces the underlying stdout and stderr sinks. Typical uses:
// tests capturing output into buffers; tools redirecting to a log file.
func SetWriters(stdout, stderr io.Writer) {
	writersMu.Lock()
	stdoutLW = &lockedWriter{w: stdout}
	stderrLW = &lockedWriter{w: stderr}
	writersMu.Unlock()
}
