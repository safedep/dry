// Package tui is SafeDep's unified terminal output library.
//
// Typical usage touches only the top-level helpers:
//
//	tui.Info("scanning %d files", n)
//	tui.Success("done")
//	tui.Error("failed: %v", err)
//
// All output honors three globals — theme, output mode, and verbosity — each
// managed in its own subpackage and each optional for callers:
//
//   - theme.Default() / theme.SetDefault(...) — design language (palette, icons)
//   - output.CurrentMode() / output.SetMode(...) — Rich/Plain/Agent
//   - output.CurrentVerbosity() / output.SetVerbosity(...) — Silent/Normal/Verbose
//
// Components live in subpackages: banner, diff, table, spinner, progress,
// prompt. Each is composable with the top-level helpers; none require touching
// the theme API for the default SafeDep look.
package tui
