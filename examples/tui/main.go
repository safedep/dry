// examples/tui — runnable reference program for the dry/tui library.
//
// Double-use: a pre-ship smoke-test fixture for developers, and a snapshot-test
// backend for CI (via go test ./examples/tui).
//
// Components demoed as each phase lands:
//
//	Phase 3: colors, icons, showcase  (done)
//	Phase 4: banner, diff             (done)
//	Phase 5: table, spinner, progress (done)
//	Phase 6: prompt                   (done)
//
// Run `go run ./examples/tui -h` for usage.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/safedep/dry/tui/output"
)

func main() {
	var (
		mode     = flag.String("mode", "", "force output mode: rich|plain|agent (default: auto-detect)")
		compare  = flag.Bool("compare", false, "render chosen component in all three modes side-by-side")
		snapshot = flag.Bool("snapshot", false, "run every demo in every mode, deterministic output, then exit")
		width    = flag.Int("width", 0, "force terminal width (0 = detect)")
		verbose  = flag.Bool("verbose", false, "set output verbosity to Verbose (shows Faint lines)")
	)
	flag.Usage = usage
	flag.Parse()

	if *mode != "" {
		applyMode(*mode)
	}
	if *width > 0 {
		output.SetWidthOverride(*width)
	}
	if *verbose {
		output.SetVerbosity(output.Verbose)
	}

	if *snapshot {
		runSnapshot()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		usage()
		return
	}
	component := args[0]

	if *compare {
		runCompare(component)
		return
	}
	runOne(component)
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: go run ./examples/tui [flags] <component>

Components (implemented):
  all        — run every implemented demo in sequence
  colors     — Info/Success/Warning/Error/Faint/Heading + Badge palette
  icons      — icon set per mode (Rich/Plain/Agent, side by side)
  console    — NewConsole value-typed seam in isolation
  renderable — custom Renderable type flowing through tui.Print
  banner     — branded tool banner with version cleaning
  diff       — line-level unified diff with colored +/-
  table      — severity report with pre-styled Badge cells
  spinner    — braille animation + Status/Stop/Fail
  progress   — two-tracker progress bars
  prompt     — interactive Prompt/Secret/Confirm/Select (requires TTY)

Flags:`)
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, `
Environment:
  NO_COLOR=1          strip ANSI colors (layout preserved)
  SAFEDEP_OUTPUT=...  force rich|plain|agent
  TERM=dumb           forces plain
  CI=true             forces plain`)
}

func applyMode(m string) {
	switch m {
	case "rich":
		output.SetMode(output.Rich)
	case "plain":
		output.SetMode(output.Plain)
	case "agent":
		output.SetMode(output.Agent)
	default:
		fmt.Fprintf(os.Stderr, "unknown --mode=%q; must be rich|plain|agent\n", m)
		os.Exit(2)
	}
}

func runOne(name string) {
	switch name {
	case "all":
		demoAll()
	case "colors":
		demoColors()
	case "icons":
		demoIcons()
	case "console":
		demoConsole()
	case "renderable":
		demoRenderable()
	case "banner":
		demoBanner()
	case "diff":
		demoDiff()
	case "table":
		demoTable()
	case "spinner":
		demoSpinner()
	case "progress":
		demoProgress()
	case "prompt":
		demoPrompt()
	default:
		fmt.Fprintf(os.Stderr, "unknown component %q\n", name)
		os.Exit(2)
	}
}

func pending(name, phase string) {
	fmt.Printf("(%s is not yet implemented — scheduled for %s)\n", name, phase)
}

func runCompare(name string) {
	for _, m := range []output.Mode{output.Rich, output.Plain, output.Agent} {
		fmt.Printf("\n=== mode=%s ===\n", m)
		output.SetMode(m)
		runOne(name)
	}
}

func runSnapshot() {
	for _, m := range []output.Mode{output.Rich, output.Plain, output.Agent} {
		fmt.Printf("\n=== mode=%s ===\n", m)
		output.SetMode(m)
		demoAll()
	}
}
