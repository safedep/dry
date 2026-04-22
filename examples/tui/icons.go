package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/icon"
	"github.com/safedep/dry/tui/output"
	"github.com/safedep/dry/tui/theme"
)

func demoIcons() {
	tui.Heading("Icon set — Unicode / ASCII / Agent side-by-side")

	set := theme.Default().Icons()
	modes := []output.Mode{output.Rich, output.Plain, output.Agent}

	fmt.Printf("%-12s %-10s %-10s %-10s\n", "Key", "Rich", "Plain", "Agent")
	fmt.Printf("%-12s %-10s %-10s %-10s\n", "---", "----", "-----", "-----")

	entries := []struct {
		key  icon.IconKey
		name string
	}{
		{icon.KeySuccess, "Success"},
		{icon.KeyError, "Error"},
		{icon.KeyWarning, "Warning"},
		{icon.KeyInfo, "Info"},
		{icon.KeyBullet, "Bullet"},
		{icon.KeyArrow, "Arrow"},
	}

	for _, e := range entries {
		i, _ := set.Get(e.key)
		fmt.Printf("%-12s ", e.name)
		for _, m := range modes {
			fmt.Printf("%-10s ", i.Resolve(m))
		}
		fmt.Println()
	}
}
