// tui/steps/steps.go
//
// Package steps provides a numbered step announcer for multi-stage
// operational flows (login, install, sync). Rich mode decorates each step
// with a progress counter; Plain and Agent modes emit exactly the lines
// tui.Info/tui.Success/tui.Error would, so scripted and agent consumers
// see no difference when a command adopts steps.
package steps

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/icon"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
)

// Flow tracks progress through a fixed number of steps.
type Flow struct {
	mu    sync.Mutex
	total int
	done  int
}

// New returns a Flow of total steps.
func New(total int) *Flow { return &Flow{total: total} }

// Step announces the next step to stderr. Suppressed when verbosity is
// Silent, matching tui.Info.
func (f *Flow) Step(format string, a ...any) {
	f.mu.Lock()
	f.done++
	n := f.done
	f.mu.Unlock()

	if output.CurrentVerbosity() <= output.Silent {
		return
	}
	text := fmt.Sprintf(format, a...)
	if output.CurrentMode() != output.Rich {
		_, _ = fmt.Fprintln(output.Stderr(), style.Info(text))
		return
	}
	_, _ = fmt.Fprintln(output.Stderr(), richStep(n, f.total, text))
}

// Done announces successful completion. Renders identically to tui.Success
// in every mode.
func (f *Flow) Done(format string, a ...any) {
	if output.CurrentVerbosity() <= output.Silent {
		return
	}
	_, _ = fmt.Fprintln(output.Stderr(), style.Success(fmt.Sprintf(format, a...)))
}

// Fail announces flow failure. Renders identically to tui.Error in every
// mode and is never suppressed.
func (f *Flow) Fail(format string, a ...any) {
	_, _ = fmt.Fprintln(output.Stderr(), style.Error(fmt.Sprintf(format, a...)))
}

func richStep(n, total int, text string) string {
	ic, _ := theme.Default().Icons().Get(icon.KeyArrow)
	glyph := ic.Resolve(output.Rich)
	counter := fmt.Sprintf("[%d/%d]", n, total)
	if !output.IsColorEnabled() {
		return fmt.Sprintf("%s %s %s", glyph, counter, text)
	}
	pal := theme.Default().Palette()
	accentC, _ := pal.ColorByRole(theme.RoleBrandAccent)
	mutedC, _ := pal.ColorByRole(theme.RoleMuted)
	return lipgloss.NewStyle().Foreground(accentC).Render(glyph) + " " +
		lipgloss.NewStyle().Foreground(mutedC).Render(counter) + " " + text
}
