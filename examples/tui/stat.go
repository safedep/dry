package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/stat"
	"github.com/safedep/dry/tui/theme"
)

func demoStat() {
	tui.Heading("Stat — metric summary cards")

	warn := theme.RoleWarning
	err := theme.RoleError
	ok := theme.RoleSuccess

	fmt.Println(stat.Render(
		stat.Card{Label: "Total endpoints", Value: "42"},
		stat.Card{Label: "Active", Value: "37", Accent: &ok},
		stat.Card{Label: "Silent", Value: "5", Accent: &warn},
		stat.Card{Label: "Blocked installs", Value: "3", Accent: &err},
	))
}
