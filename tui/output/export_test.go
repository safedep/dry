// tui/output/export_test.go
//
// Test-only helpers for the output package. Only compiled during `go test`
// and only visible to tests in this package (output_test).
package output

import "os"

// ResetModeForTest clears the cached Mode so tests can exercise the
// auto-detect path repeatedly.
func ResetModeForTest() { resetMode() }

// ResetVerbosityForTest restores verbosity to Normal.
func ResetVerbosityForTest() {
	verbosityMu.Lock()
	verbosityCurrent = Normal
	verbosityMu.Unlock()
}

// ResetWritersForTest restores stdout/stderr to os.Stdout/os.Stderr.
func ResetWritersForTest() { SetWriters(os.Stdout, os.Stderr) }
