package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/banner"
	"github.com/safedep/dry/tui/diff"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/spinner"
	"github.com/safedep/dry/tui/table"
	"github.com/safedep/dry/tui/theme"
)

type finding struct {
	pkg      string
	version  string
	severity theme.Role
	advice   string
}

func main() {
	mode := flag.String("mode", "", "force output mode: rich|plain|agent")
	showDiff := flag.Bool("show-diff", true, "print a lockfile diff after the findings table")
	flag.Parse()

	if *mode != "" {
		switch *mode {
		case "rich":
			output.SetMode(output.Rich)
		case "plain":
			output.SetMode(output.Plain)
		case "agent":
			output.SetMode(output.Agent)
		default:
			fmt.Fprintf(os.Stderr, "unknown --mode=%q; must be rich|plain|agent\n", *mode)
			os.Exit(2)
		}
	}

	runScan()
	if *showDiff {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, diff.Render(oldLockfile, newLockfile))
	}
}

func runScan() {
	banner.Banner{
		Name:    "pmg",
		Version: "v0.0.0-20260416120000-123456789abc",
		Tagline: "playground scan flow for dry/tui",
	}.Print()

	tui.Info("loading dependency graph from package-lock.json")

	s := spinner.New("resolving advisories")
	s.Start()
	time.Sleep(80 * time.Millisecond)
	s.Status("matching vulnerable packages")
	time.Sleep(80 * time.Millisecond)
	s.Stop("analysis complete")

	findings := []finding{
		{pkg: "lodash", version: "4.17.15", severity: theme.RoleHigh, advice: "upgrade to 4.17.21"},
		{pkg: "minimist", version: "1.2.5", severity: theme.RoleCritical, advice: "pin to 1.2.8"},
		{pkg: "axios", version: "0.27.2", severity: theme.RoleMedium, advice: "review transitive use before bump"},
	}

	tui.Warning("%d findings require attention", len(findings))
	tui.Info("writing findings table to stdout")

	tbl := table.New().Headers("Package", "Version", "Severity", "Advice")
	for _, finding := range findings {
		tbl.Row(
			finding.pkg,
			finding.version,
			tui.Badge(finding.severity, roleLabel(finding.severity)),
			finding.advice,
		)
	}

	fmt.Fprintln(os.Stdout, tbl.Render())
	tui.Success("report complete")
}

func roleLabel(r theme.Role) string {
	switch r {
	case theme.RoleCritical:
		return "Critical"
	case theme.RoleHigh:
		return "High"
	case theme.RoleMedium:
		return "Medium"
	case theme.RoleLow:
		return "Low"
	default:
		return "Info"
	}
}

const oldLockfile = `lodash@4.17.15
minimist@1.2.5
axios@0.27.2`

const newLockfile = `lodash@4.17.21
minimist@1.2.8
axios@0.27.2`
