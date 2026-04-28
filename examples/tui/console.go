package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/output"
)

// demoConsole shows the NewConsole value-typed seam — a Console instance can
// route output to a custom writer and force a specific mode independently of
// the global package-level state.
func demoConsole() {
	tui.Heading("Console — value-typed escape hatch")

	fmt.Println()
	fmt.Println("Global tui.Info (default routing):")
	tui.Info("this goes to the global writer in the global mode")

	fmt.Println()
	fmt.Println("Console with forced Agent mode + captured buffer:")
	buf := &bytes.Buffer{}
	c := tui.NewConsole(
		tui.WithWriter(buf),
		tui.WithMode(output.Agent),
	)
	c.Info("captured; agent mode regardless of global")
	c.Success("also captured")
	fmt.Fprint(os.Stdout, buf.String())

	fmt.Println()
	fmt.Println("(Global state was not touched — Info/Success above wrote to buf, not stderr.)")
}
