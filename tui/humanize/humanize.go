// tui/humanize/humanize.go
//
// Package humanize formats machine values as short human-friendly strings
// for data presentation. Functions are pure and not mode-aware: callers
// decide where human formatting is appropriate (typically table-mode render
// paths, keeping plain and JSON representations exact).
package humanize

import (
	"fmt"
	"time"
)

// Time returns a compact relative description of t against now ("5m ago",
// "in 3d"), falling back to an absolute UTC date beyond 30 days. Zero time
// renders as "-".
func Time(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	switch {
	case d < time.Minute:
		if future {
			return "in <1m"
		}
		return "just now"
	case d < time.Hour:
		return rel(int(d.Minutes()), "m", future)
	case d < 24*time.Hour:
		return rel(int(d.Hours()), "h", future)
	case d < 30*24*time.Hour:
		return rel(int(d.Hours()/24), "d", future)
	}
	return t.UTC().Format("2006-01-02")
}

func rel(n int, unit string, future bool) string {
	if future {
		return fmt.Sprintf("in %d%s", n, unit)
	}
	return fmt.Sprintf("%d%s ago", n, unit)
}
