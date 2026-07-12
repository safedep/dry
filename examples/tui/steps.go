package main

import (
	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/steps"
)

func demoSteps() {
	tui.Heading("Steps — numbered multi-stage flow")

	f := steps.New(3)
	f.Step("Requesting device code")
	f.Step("Waiting for browser approval")
	f.Step("Saving credentials to keychain")
	f.Done("Authenticated as acme.safedep.io")
}
