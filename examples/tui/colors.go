package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/theme"
)

func demoColors() {
	tui.Heading("Colors — semantic text helpers")
	tui.Info("Info message")
	tui.Success("Success message")
	tui.Warning("Warning message")
	tui.Error("Error message")
	tui.Faint("Faint — only shown when --verbose")

	fmt.Println()
	tui.Heading("Badges — severity + semantic roles")

	rows := []struct {
		role theme.Role
		text string
	}{
		{theme.RoleCritical, "Critical"},
		{theme.RoleHigh, "High"},
		{theme.RoleMedium, "Medium"},
		{theme.RoleLow, "Low"},
		{theme.RoleInfo, "Info"},
		{theme.RoleSuccess, "Success"},
	}
	for _, r := range rows {
		fmt.Printf("  %s  ", tui.Badge(r.role, r.text))
	}
	fmt.Println()
}
