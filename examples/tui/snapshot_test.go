// Snapshot test for the tui example program. Runs every non-interactive,
// non-animated demo across Plain and Agent modes into a single buffer, and
// diffs against a committed golden file.
//
// Usage:
//
//	go test ./examples/tui              # fail if output drifts from golden
//	go test ./examples/tui -update      # rewrite golden; review diff in PR
//
// Deliberately excluded:
//   - Rich mode: terminal-profile detection under `go test` is non-deterministic
//     (lipgloss emits different output depending on whether stderr is a TTY).
//   - Spinner: animated; timing-dependent.
//   - Progress in Rich mode: animated.
//   - Prompt: interactive.
package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

var updateSnapshot = flag.Bool("update", false, "rewrite testdata/snapshot.txt")

func TestSnapshot(t *testing.T) {
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Stdout = oldStdout
		output.SetWriters(os.Stdout, os.Stderr)
		output.SetMode(output.Rich)
	})

	var got bytes.Buffer
	for _, m := range []output.Mode{output.Plain, output.Agent} {
		stderrBuf := &bytes.Buffer{}
		output.SetWriters(stderrBuf, stderrBuf)

		stdoutFile, err := os.CreateTemp("", "dry-tui-snapshot-stdout-*")
		if err != nil {
			t.Fatal(err)
		}
		stdoutPath := stdoutFile.Name()
		os.Stdout = stdoutFile

		output.SetMode(m)
		demoBanner()
		demoColors()
		demoIcons()
		demoTable()
		demoDiff()
		demoConsole()
		demoRenderable()

		if err := stdoutFile.Close(); err != nil {
			t.Fatal(err)
		}

		stdoutBytes, err := os.ReadFile(stdoutPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(stdoutPath); err != nil {
			t.Fatal(err)
		}

		got.WriteString("\n=== mode=" + m.String() + " stderr ===\n")
		got.Write(stderrBuf.Bytes())
		got.WriteString("\n=== mode=" + m.String() + " stdout ===\n")
		got.Write(stdoutBytes)
	}

	path := filepath.Join("testdata", "snapshot.txt")

	if *updateSnapshot {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("snapshot updated")
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file missing at %s; run `go test ./examples/tui -update` to create it: %v", path, err)
	}
	assert.Equal(t, string(want), got.String(), "snapshot drift — run `go test ./examples/tui -update` to accept and commit")
}
