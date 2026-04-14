package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/safedep/dry/tui"
	"github.com/safedep/dry/tui/prompt"
)

func demoPrompt() {
	tui.Heading("Prompt — Prompt / Secret / Confirm / Select")

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println("(prompt demo skipped: stdin is not a terminal)")
		fmt.Println("run directly without piping stdin, e.g.: go run ./examples/tui prompt")
		return
	}

	name, err := prompt.Prompt("Your name")
	if handlePromptErr(err, "Prompt") {
		return
	}
	fmt.Printf("Hello, %s!\n", name)

	fmt.Println()
	pw, err := prompt.Secret("Password (masked with *)")
	if handlePromptErr(err, "Secret") {
		return
	}
	fmt.Printf("(read %d chars)\n", len(pw))

	fmt.Println()
	pw2, err := prompt.Secret("Password (no mask — silent)", prompt.WithNoMask())
	if handlePromptErr(err, "Secret WithNoMask") {
		return
	}
	fmt.Printf("(read %d chars)\n", len(pw2))

	fmt.Println()
	ok, err := prompt.Confirm("Continue?", false)
	if handlePromptErr(err, "Confirm") {
		return
	}
	fmt.Printf("Confirmed: %v\n", ok)

	fmt.Println()
	env, err := prompt.Select("Choose environment", []string{"dev", "staging", "prod"})
	if handlePromptErr(err, "Select") {
		return
	}
	fmt.Printf("Env: %s\n", env)
}

func handlePromptErr(err error, which string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, prompt.ErrCancelled):
		fmt.Printf("(%s cancelled)\n", which)
	case errors.Is(err, prompt.ErrAgentMode):
		fmt.Printf("(%s refused: agent mode requires pre-answered input via flags)\n", which)
	case errors.Is(err, prompt.ErrNoTTY):
		fmt.Printf("(%s skipped: stdin is not a terminal)\n", which)
	default:
		fmt.Printf("(%s error: %v)\n", which, err)
	}
	return true
}
