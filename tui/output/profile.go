// tui/output/profile.go
package output

import (
	"os"
	"sync"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

var (
	profileMu     sync.RWMutex
	profileCached colorprofile.Profile
	profileLoaded bool
)

// CurrentProfile returns the detected terminal color profile. Detection runs
// once; subsequent calls return the cached value.
func CurrentProfile() colorprofile.Profile {
	profileMu.RLock()
	if profileLoaded {
		p := profileCached
		profileMu.RUnlock()
		return p
	}
	profileMu.RUnlock()

	detected := colorprofile.Detect(os.Stderr, os.Environ())
	profileMu.Lock()
	if !profileLoaded {
		profileCached = detected
		profileLoaded = true
	}
	p := profileCached
	profileMu.Unlock()
	return p
}

// IsColorEnabled reports whether ANSI color escapes should be emitted.
// Returns false if NO_COLOR is set (per no-color.org) or the profile is
// Ascii/NoTTY.
func IsColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	switch CurrentProfile() {
	case colorprofile.NoTTY, colorprofile.Ascii:
		return false
	}
	return true
}

// defaultIsTTY is set here so mode.go's auto-detection uses a real isatty.
func init() {
	defaultIsTTY = func() bool {
		return term.IsTerminal(int(os.Stderr.Fd()))
	}
}

// test hooks (implemented in export_test.go)

func setProfileForTest(p colorprofile.Profile) {
	profileMu.Lock()
	profileCached = p
	profileLoaded = true
	profileMu.Unlock()
}

func resetProfileForTest() {
	profileMu.Lock()
	profileLoaded = false
	profileMu.Unlock()
}
