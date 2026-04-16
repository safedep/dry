#!/usr/bin/env bash
set -euo pipefail

mode="${1:-plain}"
shift || true

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

go run ./examples/tui-playground --mode="$mode" "$@" >"$tmpdir/stdout.txt" 2>"$tmpdir/stderr.txt"

printf 'stderr (%s)\n' "$tmpdir/stderr.txt"
sed -n '1,200p' "$tmpdir/stderr.txt"

printf '\nstdout (%s)\n' "$tmpdir/stdout.txt"
sed -n '1,200p' "$tmpdir/stdout.txt"
