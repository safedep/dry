#!/usr/bin/env bash
# scripts/check-tui-discipline.sh
#
# CI guardrails for the dry/tui library. Enforces three rules:
#   1. Hex color literals ONLY in tui/theme/palette.go.
#   2. Direct os.Stdout / os.Stderr access ONLY in tui/output/* (where the
#      writer abstraction lives).
#   3. No hand-constructed ANSI escapes (\x1b / \033) outside explicitly-
#      allowlisted sites marked with an "ok-raw-ansi" comment.
#
# Also runs `go test -race ./tui/...` to catch concurrency regressions.
#
# Exit 0 = clean; non-zero = one or more rules violated.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail=0

say() { printf '==> %s\n' "$*"; }
bad() { printf 'ERROR: %s\n' "$*" >&2; fail=1; }

# ---------------------------------------------------------------------------
# 1. Hex colors restricted to tui/theme/palette.go.
# ---------------------------------------------------------------------------
say "Checking hex color literals confined to tui/theme/palette.go..."
hits=$(grep -rnE '#[0-9a-fA-F]{6}([^0-9a-fA-F]|$)' tui/ --include='*.go' 2>/dev/null \
       | grep -vE '^tui/theme/palette\.go:' || true)
if [ -n "$hits" ]; then
  bad "hex colors found outside tui/theme/palette.go:"
  printf '%s\n' "$hits" >&2
fi

# ---------------------------------------------------------------------------
# 2. Direct os.Stdout/os.Stderr only in tui/output/* non-test files.
# ---------------------------------------------------------------------------
say "Checking direct os.Stdout / os.Stderr usage..."
hits=$(grep -rnE 'os\.Std(out|err)' tui/ --include='*.go' 2>/dev/null \
       | grep -vE '_test\.go:' \
       | grep -vE '^tui/output/' || true)
if [ -n "$hits" ]; then
  bad "direct os.Stdout/os.Stderr outside tui/output/ (must route through output.Writer()):"
  printf '%s\n' "$hits" >&2
fi

# ---------------------------------------------------------------------------
# 3. No hand-constructed ANSI escapes outside allowlisted sites.
# A site is allowlisted by a nearby `ok-raw-ansi` comment on the same or
# preceding line — enforced by filtering with awk, not just grep.
# ---------------------------------------------------------------------------
say "Checking for hand-constructed ANSI escapes..."
# Collect all files containing \033 or \x1b; then per-file check whether each
# hit has an `ok-raw-ansi` token within 3 lines before it.
offenders=$(
  grep -rln --include='*.go' -E '\\033\[|\\x1b\[' tui/ 2>/dev/null \
    | grep -vE '_test\.go$' || true
)
for f in $offenders; do
  awk '
    /ok-raw-ansi/ { allowed_until = NR + 3 }
    /\\033\[|\\x1b\[/ {
      if (NR > allowed_until) {
        printf "%s:%d: %s\n", FILENAME, NR, $0
      }
    }
  ' "$f"
done | {
  read -r first || exit 0
  bad "hand-constructed ANSI escape not allowlisted (add '// ok-raw-ansi: <reason>' within 3 lines above):"
  echo "$first" >&2
  cat >&2
  fail=1
}

# ---------------------------------------------------------------------------
# 4. Race tests.
# ---------------------------------------------------------------------------
say "Running go test -race ./tui/..."
if ! go test -race ./tui/... >/tmp/tui-discipline-race.log 2>&1; then
  bad "go test -race failed; details:"
  sed 's/^/  /' /tmp/tui-discipline-race.log >&2
fi

if [ "$fail" -ne 0 ]; then
  echo
  echo "==> FAILED" >&2
  exit 1
fi
echo
echo "==> OK"
