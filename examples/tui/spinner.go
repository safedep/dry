package main

import (
	"time"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/spinner"
)

func demoSpinner() {
	tui.Heading("Spinner — Status updates, Stop, Fail")

	s := spinner.New("Scanning dependencies")
	s.Start()
	time.Sleep(350 * time.Millisecond)
	s.Status("Resolving lodash@4.17.0")
	time.Sleep(350 * time.Millisecond)
	s.Stop("scan complete")

	s = spinner.New("Uploading artifact")
	s.Start()
	time.Sleep(250 * time.Millisecond)
	s.Fail("network error")
}
