// tui/table/table.go
//
// Package table wraps lipgloss/table with theme-aware defaults and mode-aware
// border degradation.
package table

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"

	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/style"
)

// Table is a fluent builder for styled tables.
type Table struct {
	headers  []string
	rows     [][]string
	styler   func(row, col int) lipgloss.Style
	title    string
	footer   string
	emptyMsg string
}

// New returns an empty Table.
func New() *Table { return &Table{} }

// Headers sets the header row.
func (t *Table) Headers(h ...string) *Table {
	t.headers = append([]string(nil), h...)
	return t
}

// Row appends one data row. All cells are strings; callers wanting styled
// cells pre-style them with tui.Badge or any lipgloss.Style.Render(text).
func (t *Table) Row(cells ...string) *Table {
	t.rows = append(t.rows, append([]string(nil), cells...))
	return t
}

// Rows appends multiple rows in one call.
func (t *Table) Rows(rows ...[]string) *Table {
	for _, r := range rows {
		t.rows = append(t.rows, append([]string(nil), r...))
	}
	return t
}

// Title sets a heading line rendered above the table. Styled in Rich mode,
// plain text otherwise.
func (t *Table) Title(s string) *Table {
	t.title = s
	return t
}

// Footer sets a summary line rendered below the table (row counts,
// next-page hints). Muted in Rich mode, plain text otherwise.
func (t *Table) Footer(s string) *Table {
	t.footer = s
	return t
}

// EmptyMessage sets the text rendered in place of the table when it has no
// data rows. Without it an empty table renders as a header-only grid.
func (t *Table) EmptyMessage(s string) *Table {
	t.emptyMsg = s
	return t
}

// StyleFunc registers a callback that receives (row, col) and returns a
// lipgloss.Style applied to that cell. Row lgtable.HeaderRow denotes headers;
// data rows are 0-indexed. Matches the underlying lipgloss/table contract —
// no string-to-Style bridging to silently discard caller intent.
func (t *Table) StyleFunc(fn func(row, col int) lipgloss.Style) *Table {
	t.styler = fn
	return t
}

// Render returns the table as a string, respecting the active output mode.
//
// Note: lipgloss's color emission is driven by its own renderer's profile
// detection at package init time — when running under `go test` (no TTY),
// that yields Ascii and styles emit no ANSI. We intentionally do NOT override
// lipgloss's global renderer state here (that would race with other goroutines
// doing lipgloss styling mid-render). Production callers on a real TTY get
// colors; tests asserting on ANSI should gate with output.IsColorEnabled().
func (t *Table) Render() string {
	mode := output.CurrentMode()

	if len(t.rows) == 0 && t.emptyMsg != "" {
		return t.decorate(style.Faint(t.emptyMsg))
	}

	lt := lgtable.New().Headers(t.headers...)
	lt.Rows(t.rows...)
	lt.Border(borderForMode(mode))
	lt.StyleFunc(t.composeStyleFunc(mode))
	return t.decorate(lt.Render())
}

// decorate wraps the rendered body with the optional title and footer.
func (t *Table) decorate(body string) string {
	parts := make([]string, 0, 3)
	if t.title != "" {
		parts = append(parts, style.Heading(t.title))
	}
	parts = append(parts, body)
	if t.footer != "" {
		parts = append(parts, style.Faint(t.footer))
	}
	return strings.Join(parts, "\n")
}

// composeStyleFunc returns the final per-cell styler: a horizontal padding of
// one column on each side so content isn't flush against borders, plus
// bold-headers in Rich (the default), plus whatever the caller registered via
// StyleFunc layered on top.
func (t *Table) composeStyleFunc(mode output.Mode) func(row, col int) lipgloss.Style {
	return func(row, col int) lipgloss.Style {
		base := lipgloss.NewStyle().Padding(0, 1)
		if mode == output.Rich && row == lgtable.HeaderRow {
			base = base.Bold(true)
		}
		if t.styler != nil {
			return t.styler(row, col).Inherit(base)
		}
		return base
	}
}

func borderForMode(m output.Mode) lipgloss.Border {
	switch m {
	case output.Agent:
		return lipgloss.HiddenBorder()
	case output.Plain:
		return lipgloss.NormalBorder() // ASCII-ish; lipgloss handles this
	default:
		return lipgloss.RoundedBorder()
	}
}
