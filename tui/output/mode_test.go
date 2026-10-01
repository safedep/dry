// tui/output/mode_test.go
package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAutoDetectMode(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Mode
	}{
		{"default rich when tty", map[string]string{}, Rich},
		{"CI forces plain", map[string]string{"CI": "true"}, Plain},
		{"TERM=dumb forces plain", map[string]string{"TERM": "dumb"}, Plain},
		{"SAFEDEP_OUTPUT=agent", map[string]string{"SAFEDEP_OUTPUT": "agent"}, Agent},
		{"SAFEDEP_OUTPUT=plain", map[string]string{"SAFEDEP_OUTPUT": "plain"}, Plain},
		{"CLAUDE_CODE marker", map[string]string{"CLAUDE_CODE": "1"}, Agent},
		{"ANTHROPIC_AGENT marker", map[string]string{"ANTHROPIC_AGENT": "1"}, Agent},
		{"CLAUDECODE marker set by Claude Code", map[string]string{"CLAUDECODE": "1"}, Agent},
		{"AI_AGENT marker", map[string]string{"AI_AGENT": "codex"}, Agent},
		{"SAFEDEP_OUTPUT wins over an agent marker", map[string]string{"SAFEDEP_OUTPUT": "plain", "CLAUDECODE": "1"}, Plain},
		{"agent marker wins over CI", map[string]string{"CI": "true", "AI_AGENT": "1"}, Agent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearModeEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			// Force isatty=true so the default-to-Rich case works in CI test env.
			got := autoDetectMode(func() bool { return true })
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("non-tty forces plain", func(t *testing.T) {
		clearModeEnv(t)
		got := autoDetectMode(func() bool { return false })
		assert.Equal(t, Plain, got)
	})
}

// clearModeEnv unsets every variable that mode detection reads, so the host
// environment (for example a test run inside Claude Code) does not leak in.
func clearModeEnv(t *testing.T) {
	t.Helper()
	for _, k := range append([]string{"CI", "TERM", "SAFEDEP_OUTPUT"}, agentEnvVars...) {
		t.Setenv(k, "")
	}
}

func TestSetModeOverride(t *testing.T) {
	ResetModeForTest()
	SetMode(Agent)
	assert.Equal(t, Agent, CurrentMode())
	SetMode(Rich)
	assert.Equal(t, Rich, CurrentMode())
}
