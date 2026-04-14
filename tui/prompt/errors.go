// tui/prompt/errors.go
//
// Package prompt provides interactive input prompts. All prompts return typed
// sentinel errors for cancellation and environment-driven refusal; callers
// check with errors.Is.
package prompt

import "errors"

// ErrCancelled is returned when the user presses Ctrl-C or stdin reaches EOF
// during a prompt. The terminal mode is restored before this error returns.
var ErrCancelled = errors.New("prompt cancelled")

// ErrAgentMode is returned when output.Mode is Agent and no pre-answer was
// supplied. Callers should branch on a tool-level --yes (or similar) flag
// before calling a prompt, and only call through when interactive input is
// actually safe.
var ErrAgentMode = errors.New("prompt refused in agent mode")

// ErrNoTTY is returned when stdin is not a terminal and the prompt cannot be
// satisfied.
var ErrNoTTY = errors.New("stdin is not a terminal")
