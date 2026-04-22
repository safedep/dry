// Package banner renders a SafeDep-styled tool banner. The ASCII art is
// provided by the caller; this package owns layout, coloring, version
// cleaning, and mode fallback.
package banner

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
	"github.com/safedep/dry/tui/theme"
)

// Banner is a tool-identification header: ASCII art + name + version + tagline.
type Banner struct {
	Art     string
	Name    string
	Version string
	Tagline string
	// Accent picks which palette role colors the art.
	// Nil means default (BrandPrimary); non-nil uses the pointed role.
	Accent *theme.Role
}

// Print writes the banner to stderr.
func (b Banner) Print() {
	b.PrintTo(output.Stderr())
}

// PrintTo writes the banner to the given writer.
func (b Banner) PrintTo(w io.Writer) {
	_, _ = fmt.Fprintln(w, b.Render())
}

// Render returns the banner as a string, respecting the active output.Mode.
func (b Banner) Render() string {
	mode := output.CurrentMode()
	version := cleanVersion(b.Version)

	switch mode {
	case output.Agent:
		return fmt.Sprintf("tool=%s version=%s tagline=%q", b.Name, version, b.Tagline)
	case output.Plain:
		return fmt.Sprintf("SafeDep %s %s — %s", b.Name, version, b.Tagline)
	}

	// Rich mode.
	// Resolve accent role: nil means use BrandPrimary as default.
	accentRole := theme.RoleBrandPrimary
	if b.Accent != nil {
		accentRole = *b.Accent
	}

	c, ok := theme.Default().Palette().ColorByRole(accentRole)
	if !ok {
		c, _ = theme.Default().Palette().ColorByRole(theme.RoleBrandPrimary)
	}

	var parts []string
	if b.Art != "" && output.IsColorEnabled() {
		parts = append(parts, lipgloss.NewStyle().Foreground(c).Render(b.Art))
	} else if b.Art != "" {
		parts = append(parts, b.Art)
	}
	header := style.Faint(fmt.Sprintf("%s %s", b.Name, version))
	parts = append(parts, header)
	if b.Tagline != "" {
		parts = append(parts, b.Tagline)
	}
	return strings.Join(parts, "\n")
}

// cleanVersion normalizes a version string for display:
//   - empty → "dev"
//   - Go pseudo-version (v0.0.0-<timestamp>-<sha>) → "dev (<short-sha>)"
//   - anything else → verbatim
var pseudoVersionRE = regexp.MustCompile(`^v\d+\.\d+\.\d+-\d{14}-([0-9a-f]{12})$`)

func cleanVersion(v string) string {
	if v == "" {
		return "dev"
	}
	if m := pseudoVersionRE.FindStringSubmatch(v); m != nil {
		return "dev (" + m[1][:7] + ")"
	}
	return v
}
