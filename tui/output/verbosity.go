// tui/output/verbosity.go
package output

import "sync"

// Verbosity controls how much output is emitted.
//
//   - Silent  — suppress Info/Success lines; Warning and Error still shown.
//   - Normal  — default; Info/Success/Warning/Error all shown.
//   - Verbose — additionally show Faint/debug lines.
type Verbosity int

const (
	Silent Verbosity = iota
	Normal
	Verbose
)

var (
	verbosityMu      sync.RWMutex
	verbosityCurrent = Normal
)

func SetVerbosity(v Verbosity)    { verbosityMu.Lock(); verbosityCurrent = v; verbosityMu.Unlock() }
func CurrentVerbosity() Verbosity { verbosityMu.RLock(); defer verbosityMu.RUnlock(); return verbosityCurrent }
