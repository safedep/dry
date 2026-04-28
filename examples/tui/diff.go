package main

import (
	"fmt"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/diff"
)

func demoDiff() {
	tui.Heading("Diff — naive unified diff with colored +/-")

	old := `line one
line two
line three
line four`
	neu := `line one
line TWO
line three
line four updated`

	fmt.Println(diff.Render(old, neu))
}
