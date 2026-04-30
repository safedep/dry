// Package diff renders a line-oriented unified diff. Uses a naive line-by-line
// comparison — suitable for small (<1k line) diffs typical in CLI reports.
// For serious diffing, tools should use go-diff or similar and pass the
// resulting hunks through Render.
package diff

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

// Render returns a unified diff string comparing oldText to newText.
// Returns empty string if the inputs are equal.
func Render(oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	oldLines := strings.Split(strings.TrimRight(oldText, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(newText, "\n"), "\n")

	var b strings.Builder
	// Naive line-by-line diff: walk both slices in parallel.
	// Equal lines → " " prefix; mismatched lines → "-old" then "+new".
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}
	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		hasOld := i < len(oldLines)
		hasNew := i < len(newLines)
		if hasOld {
			oldLine = oldLines[i]
		}
		if hasNew {
			newLine = newLines[i]
		}
		if hasOld && hasNew && oldLine == newLine {
			b.WriteString(" ")
			b.WriteString(oldLine)
			b.WriteString("\n")
			continue
		}
		if hasOld {
			b.WriteString(styleLine("-"+oldLine, theme.RoleDiffRemove))
			b.WriteString("\n")
		}
		if hasNew {
			b.WriteString(styleLine("+"+newLine, theme.RoleDiffAdd))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func styleLine(line string, role theme.Role) string {
	if output.CurrentMode() != output.Rich || !output.IsColorEnabled() {
		return line
	}
	c, _ := theme.Default().Palette().ColorByRole(role)
	return lipgloss.NewStyle().Foreground(c).Render(line)
}
