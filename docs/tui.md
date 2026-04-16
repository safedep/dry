# dry/tui — developer notes

Minimal guide for SafeDep developers using or extending the `dry/tui` library.

## Quick start

```go
import "github.com/safedep/dry/tui"

tui.Info("scanning %d files", n)    // cyan i
tui.Success("done")                 // green ✓
tui.Warning("some files skipped")   // yellow ⚠
tui.Error("failed: %v", err)        // red ✗
tui.Heading("Report")               // bold accent
tui.Faint("debug info")             // muted; only shown in verbose
```

All output honors three globals — each optional:

| Global       | Setter                        | Default   |
|--------------|-------------------------------|-----------|
| Theme        | `theme.SetDefault(...)`       | `SafeDep` |
| Output mode  | `output.SetMode(Rich\|Plain\|Agent)` | auto-detect |
| Verbosity    | `output.SetVerbosity(Silent\|Normal\|Verbose)` | `Normal` |

If you don't touch any of these, tools get the SafeDep design language on a
detected-terminal mode.

## Running the demo

```bash
go run ./examples/tui all                 # implemented demos
go run ./examples/tui colors              # one component
go run ./examples/tui all --mode=plain    # force mode
go run ./examples/tui all --mode=agent
NO_COLOR=1 go run ./examples/tui all      # layout preserved, color stripped
go run ./examples/tui -h                  # flags + env var reference
```

All implemented: `colors`, `icons`, `console`, `renderable`, `banner`,
`diff`, `table`, `spinner`, `progress`, `prompt`, `all`.

For a more realistic consumer flow that mixes banner/chatter on `stderr` with
report output on `stdout`:

```bash
go run ./examples/tui-playground --mode=plain
./scripts/explore-tui-playground.sh agent
```

## Output modes — what each tool should expect

The library picks one of three modes per call, based on environment:

- **Rich** (default TTY) — full Unicode, colors, braille spinner, box-drawing
  borders, live progress bars.
- **Plain** (pipe / CI / `TERM=dumb` / `CI=true`) — no color, ASCII borders,
  static progress lines, single-line banner.
- **Agent** (`SAFEDEP_OUTPUT=agent` or known agent env vars) — terse,
  parseable, append-only, no cursor control. Prompts return `ErrAgentMode`
  unless the tool pre-answers via flags.

Mode detection order (first match wins):

1. `output.SetMode(...)` / `--output=rich|plain|agent`
2. `SAFEDEP_OUTPUT`, `CLAUDE_CODE`, `ANTHROPIC_AGENT` env vars
3. `TERM=dumb`
4. `CI=true`
5. Not a TTY → Plain
6. Otherwise → Rich

Color stripping is separate: `NO_COLOR=1` strips colors without changing
layout. Respects [no-color.org](https://no-color.org/).

## Writers — stderr vs stdout

- Human chatter (Info/Success/Warning/Error, spinners, progress, banner) →
  **stderr** via `output.Stderr()`.
- Tool data output (tables, JSON the tool produces) → **stdout** via
  `output.Stdout()`.

This is unix convention: `pmg scan | jq` stays safe because chatter doesn't
pollute the pipe. Both writers are mutex-serialized; concurrent goroutines
can't interleave mid-line.

## Theme — customization (rare)

If a tool wants to tweak the design language, it can override the global
theme at startup:

```go
import (
    "github.com/charmbracelet/lipgloss"
    "github.com/safedep/dry/tui/theme"
)

custom := theme.From(theme.SafeDep(),
    theme.WithColor(theme.RoleBrandPrimary,
        lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}),
    theme.WithName("my-tool-purple"),
)
theme.SetDefault(custom)
```

- `Palette` is **closed** — no `Custom map[string]color` escape hatch. Runtime-
  keyed palettes (e.g., gryph's per-agent colors) stay in the consuming tool.
- Every hex color lives in `tui/theme/palette.go`. A CI grep check enforces
  this.

## Renderable — extension point

Any type can implement `tui.Renderable` to flow through `tui.Print`:

```go
type Finding struct { /* ... */ }

func (f Finding) Render(t tui.Theme, m output.Mode) string {
    return ... // pure; no I/O, no globals beyond the passed t and m.
}

tui.Print(finding) // writes to output.Stderr() with current theme/mode
```

Implementations must be pure — no I/O, no time-dependent output, no global
reads beyond the arguments. See `examples/tui/renderable.go` for a live
demo.

## Console — testing / exceptional callers

The `tui.Console` value-typed seam is for tests and edge cases that need
to isolate from global state:

```go
buf := &bytes.Buffer{}
c := tui.NewConsole(
    tui.WithWriter(buf),
    tui.WithMode(output.Agent),
)
c.Info("captured; doesn't touch the global writer or mode")
```

Most production code should use the package-level `tui.Info`/`Success`/etc.,
not `NewConsole`.

## What's here (by phase)

| Phase | Package(s)                                  | Status      |
|-------|---------------------------------------------|-------------|
| 1     | `tui/output` (mode, verbosity, writer, profile, width) | ✅ Done |
| 2     | `tui/icon`, `tui/theme`                     | ✅ Done     |
| 3     | `tui/style`, `tui` (Renderable, Console), `tui/errors` | ✅ Done |
| 4     | `tui/banner`, `tui/diff`                    | ✅ Done     |
| 5     | `tui/table`, `tui/spinner`, `tui/progress`  | ✅ Done     |
| 6     | `tui/prompt`                                | ✅ Done     |
| 7     | full `examples/tui`, snapshot test, CI discipline script | ✅ Done     |

## Implementation plan

Source of truth for the ongoing implementation is
`docs/superpowers/plans/2026-04-13-tui-unified-lib.md`. Review hooks: each
phase runs a spec-compliance reviewer + a code-quality reviewer before the
phase is marked done.

## Tests

```bash
go test -v -race ./tui/...               # all tui tests
go test -race ./tui/output/...           # one package
go test ./examples/tui                   # snapshot test (diffs against golden)
go test ./examples/tui -update           # accept output drift, rewrite golden
./scripts/check-tui-discipline.sh        # full CI guardrails
```

All mutable globals (theme, mode, verbosity, writers) are behind
`sync.RWMutex` (or `sync.Mutex` for writers). `go test -race` passes clean
across the tree.

## CI discipline

`scripts/check-tui-discipline.sh` enforces three invariants:

1. **No hex colors outside `tui/theme/palette.go`.** Forces callers through
   the Palette abstraction; themes remain swappable without grep-and-replace.
2. **No direct `os.Stdout` / `os.Stderr` outside `tui/output/`.** Every
   component writes through `output.Stderr()` or `output.Stdout()`, keeping
   the mutex-serialization and EPIPE-swallow invariants intact.
3. **No hand-constructed ANSI escapes** (`\x1b[` / `\033[`) outside sites
   explicitly allowlisted with an `// ok-raw-ansi: <reason>` comment within
   three lines above.

Plus `go test -race ./tui/...` runs on every invocation to catch
concurrency regressions.

Add this script to your pre-push hook or CI pipeline.
