// tui/output/mode.go
package output

import (
	"os"
	"strings"
	"sync"
)

// Mode controls how components render output.
//
//   - Rich  — default for interactive terminals: full Unicode, colors, live animations.
//   - Plain — non-TTY, CI, or TERM=dumb: no color, ASCII borders, static status lines.
//   - Agent — explicit opt-in via flag/env: terse, parseable, append-only, no decoration.
type Mode int

const (
	Rich Mode = iota
	Plain
	Agent
)

func (m Mode) String() string {
	switch m {
	case Agent:
		return "agent"
	case Plain:
		return "plain"
	default:
		return "rich"
	}
}

var (
	modeMu      sync.RWMutex
	modeCurrent = unsetMode
)

// unsetMode is used internally to indicate "not yet detected"; CurrentMode
// resolves it lazily on first call.
const unsetMode Mode = -1

// SetMode replaces the active mode. Typically called once at startup after
// parsing a --output flag.
func SetMode(m Mode) {
	modeMu.Lock()
	modeCurrent = m
	modeMu.Unlock()
}

// CurrentMode returns the active mode, auto-detecting on first call.
func CurrentMode() Mode {
	modeMu.RLock()
	m := modeCurrent
	modeMu.RUnlock()
	if m != unsetMode {
		return m
	}
	detected := autoDetectMode(defaultIsTTY)
	modeMu.Lock()
	if modeCurrent == unsetMode {
		modeCurrent = detected
	}
	m = modeCurrent
	modeMu.Unlock()
	return m
}

// ResetModeForTest is an internal test helper that clears the cached mode.
// Exposed via export_test.go — not part of the public API.
func resetMode() {
	modeMu.Lock()
	modeCurrent = unsetMode
	modeMu.Unlock()
}

// autoDetectMode resolves the Mode per the spec's ordered rules.
// The isTTY callback is injected so tests can force either branch without
// touching the real terminal.
func autoDetectMode(isTTY func() bool) Mode {
	if v := os.Getenv("SAFEDEP_OUTPUT"); v != "" {
		switch strings.ToLower(v) {
		case "agent":
			return Agent
		case "plain":
			return Plain
		case "rich":
			return Rich
		}
	}
	// Known agent-environment markers.
	for _, k := range []string{"CLAUDE_CODE", "ANTHROPIC_AGENT"} {
		if os.Getenv(k) != "" {
			return Agent
		}
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return Plain
	}
	if v := os.Getenv("CI"); v != "" && v != "false" && v != "0" {
		return Plain
	}
	if !isTTY() {
		return Plain
	}
	return Rich
}

// defaultIsTTY is overridden by profile.go once that file introduces the
// terminal-detection dependency. Declared here as a hook so mode.go compiles
// standalone.
var defaultIsTTY = func() bool { return true }
