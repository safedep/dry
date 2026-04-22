// tui/spinner/spinner.go
//
// Package spinner provides a mode-aware, signal-safe spinner.
package spinner

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
)

// Spinner is a running-task indicator.
type Spinner struct {
	mu      sync.Mutex
	label   string
	stopCh  chan struct{}
	doneCh  chan struct{}
	sigCh   chan os.Signal
	running bool
	mode    output.Mode
	startTS time.Time
}

// New returns a Spinner with the given initial label. Call Start to begin.
func New(label string) *Spinner {
	return &Spinner{label: label}
}

// Start begins animation (Rich) or prints the start line (Plain/Agent) and
// installs a SIGINT handler that restores terminal state on Ctrl-C.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.mode = output.CurrentMode()
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = nil
	s.startTS = time.Now()
	s.mu.Unlock()

	s.installSignalHandler()

	switch s.mode {
	case output.Rich:
		s.mu.Lock()
		s.doneCh = make(chan struct{})
		s.mu.Unlock()
		go s.animate()
	case output.Plain:
		_, _ = fmt.Fprintf(output.Stderr(), "%s...\n", s.label)
	case output.Agent:
		_, _ = fmt.Fprintf(output.Stderr(), "[%s] start\n", s.label)
	}
}

// Status updates the spinner label without restarting. Plain and Agent modes
// print a new status line; Rich mode picks up the new label on the next frame.
func (s *Spinner) Status(label string) {
	s.mu.Lock()
	s.label = label
	mode := s.mode
	s.mu.Unlock()

	switch mode {
	case output.Plain:
		_, _ = fmt.Fprintf(output.Stderr(), "  %s...\n", label)
	case output.Agent:
		_, _ = fmt.Fprintf(output.Stderr(), "[%s] status\n", label)
	}
}

// Stop ends animation with a success line.
func (s *Spinner) Stop(final string) {
	s.stop(
		func() string { return style.Success(final) },
		func() string { return style.Success(final) }, // Plain also uses styled form
		"done",
	)
}

// Fail ends animation with an error line.
func (s *Spinner) Fail(final string) {
	s.stop(
		func() string { return style.Error(final) },
		func() string { return style.Error(final) }, // Plain uses Error() so [ERR] prefix appears
		"fail",
	)
}

func (s *Spinner) stop(richLine func() string, plainLine func() string, agentStatus string) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	stopCh := s.stopCh
	doneCh := s.doneCh
	mode := s.mode
	label := s.label
	s.mu.Unlock()

	// Uninstall signal handler BEFORE closing stopCh to avoid a race where the
	// signal goroutine tries to read from a closed channel.
	s.uninstallSignalHandler()
	close(stopCh)
	if doneCh != nil {
		<-doneCh
	}

	switch mode {
	case output.Rich:
		clearCurrentLine()
		_, _ = fmt.Fprintln(output.Stderr(), richLine())
	case output.Plain:
		_, _ = fmt.Fprintln(output.Stderr(), plainLine())
	case output.Agent:
		_, _ = fmt.Fprintf(output.Stderr(), "[%s] %s\n", label, agentStatus)
	}
}

func (s *Spinner) animate() {
	t := time.NewTicker(frameInterval * time.Millisecond)
	defer t.Stop()
	defer close(s.doneCh)

	pal := theme.Default().Palette()
	c, _ := pal.ColorByRole(theme.RoleBrandPrimary)
	st := lipgloss.NewStyle().Foreground(c)

	idx := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-t.C:
			s.mu.Lock()
			label := s.label
			s.mu.Unlock()
			frame := brailleFrames[idx%len(brailleFrames)]
			writeCurrentFrame(st.Render(string(frame)), label)
			idx++
		}
	}
}

func writeCurrentFrame(frame, label string) {
	// ok-raw-ansi: CR + ED (erase-to-end-of-line) are required for Rich-mode
	// spinner redraws so a shorter Status() label doesn't leave trailing bytes
	// from the previous frame on screen.
	_, _ = fmt.Fprintf(output.Stderr(), "\r\033[K%s %s", frame, label)
}

func clearCurrentLine() {
	// ok-raw-ansi: CR + ED (erase-to-end-of-line). Cursor control is the
	// spinner's job; no lipgloss equivalent exists. Justified exemption from
	// the no-raw-ANSI discipline check.
	_, _ = fmt.Fprint(output.Stderr(), "\r\033[K")
}

func (s *Spinner) installSignalHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	s.mu.Lock()
	s.sigCh = ch
	s.mu.Unlock()
	// Capture ch locally so the goroutine never touches s.sigCh.
	go func(ch chan os.Signal) {
		sig, ok := <-ch
		if !ok {
			// Channel was closed by uninstallSignalHandler — exit cleanly.
			return
		}
		clearCurrentLine()
		signal.Stop(ch)
		// Re-raise so the caller's handler runs.
		p, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = p.Signal(sig)
		}
	}(ch)
}

func (s *Spinner) uninstallSignalHandler() {
	s.mu.Lock()
	sigCh := s.sigCh
	s.sigCh = nil
	s.mu.Unlock()

	if sigCh != nil {
		signal.Stop(sigCh)
		close(sigCh)
	}
}
