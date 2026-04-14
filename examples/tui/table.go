package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/table"
	"github.com/safedep/dry/tui/theme"
)

func demoTable() {
	tui.Heading("Table — severity report with pre-styled Badge cells")

	t := table.New().
		Headers("Package", "Version", "Severity").
		Row("lodash", "4.17.0", tui.Badge(theme.RoleHigh, "High")).
		Row("minimist", "1.2.0", tui.Badge(theme.RoleCritical, "Critical")).
		Row("left-pad", "1.0.0", tui.Badge(theme.RoleLow, "Low"))

	fmt.Println(t.Render())
}
