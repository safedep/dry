// tui/progress/progress.go
//
// Package progress provides mode-aware progress tracking. Rich mode uses
// go-pretty/progress for animated bars; Plain prints periodic status lines;
// Agent prints append-only, parseable lines.
//
// The go-pretty dependency is isolated behind this package's interface; a
// future replacement requires only rewiring this file.
package progress

import (
	"fmt"
	"sync"

	gp "github.com/jedib0t/go-pretty/v6/progress"

	"github.com/safedep/dry/tui/output"
)

// Progress is a manager for one or more concurrent trackers.
type Progress struct {
	mode output.Mode
	pw   gp.Writer
	done chan struct{}
}

// Tracker is a single progress item within a Progress manager.
type Tracker struct {
	progress *Progress
	label    string
	total    int64
	current  int64
	gpTr     *gp.Tracker
	mu       sync.Mutex
}

// New returns a fresh Progress manager. Its mode is captured at construction
// and used for the lifetime of this instance.
//
// In Rich mode, New spawns a background goroutine that drives the animation;
// the caller MUST call Wait() to stop it before the program exits or on any
// early-return path, else the goroutine leaks. Using defer at the call site
// is the idiomatic pattern:
//
//	p := progress.New()
//	defer p.Wait()
//	t := p.Track("downloading", total)
//	...
//
// In Plain and Agent modes, Wait is a no-op and no goroutine is created.
func New() *Progress {
	p := &Progress{mode: output.CurrentMode(), done: make(chan struct{})}
	if p.mode == output.Rich {
		p.pw = gp.NewWriter()
		p.pw.SetOutputWriter(output.Stderr())
		p.pw.SetAutoStop(false)
		go func() {
			p.pw.Render()
			close(p.done)
		}()
	}
	return p
}

// Track registers a new tracker with the given label and total.
func (p *Progress) Track(label string, total int64) *Tracker {
	tr := &Tracker{progress: p, label: label, total: total}
	switch p.mode {
	case output.Rich:
		tr.gpTr = &gp.Tracker{Message: label, Total: total, Units: gp.UnitsDefault}
		p.pw.AppendTracker(tr.gpTr)
	case output.Plain:
		_, _ = fmt.Fprintf(output.Stderr(), "%s: 0/%d (0%%)\n", label, total)
	case output.Agent:
		_, _ = fmt.Fprintf(output.Stderr(), "progress: %s 0/%d (0%%)\n", label, total)
	}
	return tr
}

// Increment advances the tracker by n units.
func (tr *Tracker) Increment(n int64) {
	tr.mu.Lock()
	tr.current += n
	cur, total := tr.current, tr.total
	tr.mu.Unlock()

	if tr.gpTr != nil {
		tr.gpTr.Increment(n)
		return
	}
	pct := 0
	if total > 0 {
		pct = int((cur * 100) / total)
	}
	switch tr.progress.mode {
	case output.Plain:
		_, _ = fmt.Fprintf(output.Stderr(), "%s: %d/%d (%d%%)\n", tr.label, cur, total, pct)
	case output.Agent:
		_, _ = fmt.Fprintf(output.Stderr(), "progress: %s %d/%d (%d%%)\n", tr.label, cur, total, pct)
	}
}

// Done marks the tracker complete.
func (tr *Tracker) Done() {
	if tr.gpTr != nil {
		tr.gpTr.MarkAsDone()
		return
	}
	switch tr.progress.mode {
	case output.Plain:
		_, _ = fmt.Fprintf(output.Stderr(), "%s: done\n", tr.label)
	case output.Agent:
		_, _ = fmt.Fprintf(output.Stderr(), "progress: %s done\n", tr.label)
	}
}

// Wait blocks until all trackers have completed (Rich) or returns immediately
// (Plain/Agent).
func (p *Progress) Wait() {
	if p.pw == nil {
		return
	}
	p.pw.Stop()
	<-p.done
}
