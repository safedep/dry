package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/panel"
	"github.com/safedep/dry/tui/theme"
)

func demoPanel() {
	tui.Heading("Panel — titled key-value detail card")

	p := panel.New("Endpoint dev-box-01").
		Field("ID", "01JXAMPLE9K3F").
		Field("Hostname", "dev-box-01.local").
		Field("OS/Arch", "darwin/arm64").
		Field("Status", tui.Badge(theme.RoleSuccess, "active")).
		FieldIf(true, "Last sync", "5m ago").
		FieldIf(false, "Hidden", "never rendered")

	fmt.Println(p.Render())
}
