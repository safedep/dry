package main

import "github.com/safedep/dry/tui/banner"

var demoArt = `
 ____        __      ____
/ ___| __ _ / _| ___|  _ \  ___ _ __
\___ \/ _' | |_ / _ \ | | |/ _ \ '_ \
 ___) | (_| |  _|  __/ |_| |  __/ |_) |
|____/\__,_|_|  \___|____/ \___| .__/
                               |_|`

func demoBanner() {
	// Use a Go pseudo-version string to exercise the cleanVersion path:
	//   v0.0.0-<14-digit-ts>-<12-char-sha>  →  "dev (abcdef1)"
	banner.Banner{
		Art:     demoArt,
		Name:    "pmg",
		Version: "v0.0.0-20260401123045-abcdef123456",
		Tagline: "SafeDep package manager guard",
	}.Print()
}
