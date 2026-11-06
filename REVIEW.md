# REVIEW.md

Review instructions for Claude Code Review. `AGENTS.md` holds the build, the test and the general
rules. This file says what to report and how severe it is.

dry is a library of Go packages. control-tower, malysis, vet, pmg, gryph and cli import it. A change
in dry reaches all of them at the next version bump, and the caller tests do not run here.

## What Important means here

Report a finding as Important only when the change can do one of these:

- Break a downstream caller: it does not compile, or it compiles and behaves differently.
- Change a stored or frozen value: a package name, a key, a purl, an id format, a cache file format.
- Leak a secret or weaken a security check: a token, a key, a file read, a sandbox rule.
- Lose an error, corrupt data, race, or panic on a valid input.

Style, naming, refactor ideas and doc wording are Nit at most. Report at most five Nits per review.
When a review has Important findings, skip the Nits.

## Do not report

- What CI already checks. `go test ./...` with `go mod tidy`, `go tool mockery` and `go generate`
  (`.github/workflows/go.yml`). `scripts/check-tui-discipline.sh`: hex colors outside
  `tui/theme/palette.go`, `os.Stdout` or `os.Stderr` outside `tui/output/`, raw ANSI escapes, and
  `go test -race ./tui/...`. The vet dependency policy (`.github/workflows/vet-ci.yml`).
- Format and import order. Lefthook runs `golangci-lint` and `gitleaks` on pre-commit. CI does not
  run `golangci-lint`, so a real bug that a linter can also find (an unchecked error, a nil
  dereference) is still a finding.
- Generated files: `api/api.gen.go`, `*_mock.go`, `go.sum`.
- Test data from upstream: `api/pb/testdata/version-order`.

## Always check

1. **Exported API compatibility (Important).** A removed or renamed exported identifier, a changed
   signature, or a changed behavior or output of an exported function that callers use. Watch the
   quiet breaks too. A function made variadic breaks a caller that passes it as a func value: pmg
   passes `(*endpointsync.SyncClient).CheckIn` as a `func(context.Context) error`. A method added
   to an exported interface breaks each implementation outside dry. A new default value changes
   behavior for callers that do not set it.
   The safe form is a new function, a new option or a new field.
2. **Frozen outputs (Important).** `NewPurlPackageVersion`, `Purl` and the other helpers that say
   "frozen" in their doc comment in `api/pb/purl.go`. A change to their output, or an edit of
   `TestNewPurlPackageVersionIsFrozen`, `TestPurlIsFrozen` or
   `TestPurlRoundTripThroughFrozenHelper` to match a new output, is Important unless the doc
   comment names the change as an exception.
3. **Package identity in `api/pb` (Important).** A change to a fold function without a version bump
   of that ecosystem in `identityRules` (`api/pb/identity_rules.go`). A comparison of two
   `PackageVersion` values with `==`, or a map keyed on the value. Use `Equal` and `Key`.
4. **Copied OSV code (Important).** A change to the logic of `api/pb/internal/semantic` that the
   package doc (`api/pb/internal/semantic/doc.go`) does not list as a local difference.
5. **Dependencies (Important).** A new module in `go.mod` that the standard library or a module
   already in `go.mod` can replace. It reaches every SafeDep tool. A raised Go version in `go.mod`
   forces every caller to that version: report it when the PR does not say why.
6. **Errors (Important).** An error that the code drops, logs and then ignores, or returns without
   `%w` when a caller must match it with `errors.Is` or `errors.As`. A change in `usefulerror` or
   the API error helpers that changes how an existing error maps to a code or a reason
   (`docs/api-error.md`).
7. **Logging (Important for a secret, else Nit).** Library code logs through
   `github.com/safedep/dry/log`, not the standard `log` package and not `fmt.Print` to stdout. A
   token, an API key, a password or a full credential URL in a log line or an error is Important.
   `BeginEvent` belongs at an entry point only (HTTP middleware, async wrapper, `RunCommand`). A
   helper that starts its own or a nested event is a Nit (`docs/logger.md`).
8. **Security-sensitive packages (Important).** In `crypto`, `id`, `keychain`, `sandbox` and
   `cloud/configsync`: random values for an id, a token or a key come from `crypto/rand`, not
   `math/rand` or `utils.NewUniqueId`. File reads in the policy cache keep the no-symlink open, the
   regular-file check and the size limit. A `sandbox` change does not widen what the sandboxed
   code can reach without a reason in the PR.
9. **Shared state (Important).** A package-level variable that two goroutines can read and write
   without a lock or an atomic. `log` sets its global logger atomically for this reason.
10. **Each OS (Important).** `keychain`, `cloud/configsync` and `sandbox` run on Linux, macOS and
    Windows. A path, a file mode, a rename or a build tag that works on one OS only.
11. **Tests (Nit).** A bug fix needs a test that fails without the fix. Tests use `testify`
    (`require`, `assert`) and tables, not `t.Errorf`.
12. **Package docs (Nit).** A change to a package with a page in `docs/` (for example `docs/tui.md`,
    `docs/logger.md`, `docs/policy.md`) updates the page in the same change.

## Evidence

A claim about behavior needs a file:line citation in the source, not an inference from a name. A
claim that a change breaks a caller names the exported identifier and the old and the new form.
