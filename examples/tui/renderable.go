package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/output"
)

// finding is a sample domain type that implements tui.Renderable. In a real
// tool (e.g., vet) this would be a vulnerability finding; here it's just an
// illustration that any type implementing Render(Theme, Mode) string can flow
// through tui.Print.
type finding struct {
	Package  string
	Version  string
	Severity string
	Advice   string
}

func (f finding) Render(t tui.Theme, m output.Mode) string {
	return fmt.Sprintf("%s@%s — %s: %s", f.Package, f.Version, f.Severity, f.Advice)
}

func demoRenderable() {
	tui.Heading("Renderable — custom types dispatched through tui.Print")

	tui.Print(finding{
		Package:  "lodash",
		Version:  "4.17.0",
		Severity: "High",
		Advice:   "upgrade to 4.17.21",
	})
	tui.Print(finding{
		Package:  "left-pad",
		Version:  "1.0.0",
		Severity: "Low",
		Advice:   "no action required",
	})
}
