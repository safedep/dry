package main

import (
	"fmt"
	"time"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/humanize"
	tuisection "github.com/safedep/dry/tui/section"
	"github.com/safedep/dry/tui/table"
)

func demoSection() {
	tui.Heading("Section — titled blocks, hints, empty states")

	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	events := table.New().
		Headers("Time", "Package", "Verdict").
		Row(humanize.Time(now.Add(-2*time.Hour), now), "left-pad@1.0.0", "blocked").
		Footer("1 event across 1 tool run").
		Render()

	inventory := table.New().
		Headers("Kind", "Name").
		EmptyMessage("No inventory in the last 7d. Try a wider window with --since.").
		Render()

	fmt.Println(tuisection.Join(
		tuisection.Titled("Guard events (last 7d)", events),
		tuisection.Titled("Inventory (last 7d)", inventory),
		tuisection.Hint("Drill in with `safedep endpoint activity list --invocation <id>`."),
	))
}
