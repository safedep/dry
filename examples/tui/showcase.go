package main

import "fmt"

// demoAll runs every implemented demo in order. Intended for a quick
// pre-ship visual sanity check.
func demoAll() {
	section("Banner", demoBanner)
	section("Colors", demoColors)
	section("Icons", demoIcons)
	section("Table", demoTable)
	section("Diff", demoDiff)
	section("Spinner", demoSpinner)
	section("Progress", demoProgress)
	section("Console", demoConsole)
	section("Renderable", demoRenderable)
	// Prompt is interactive; skips gracefully when stdin isn't a TTY.
	section("Prompt", demoPrompt)
}

func section(name string, fn func()) {
	fmt.Printf("\n── %s ──\n\n", name)
	fn()
}
