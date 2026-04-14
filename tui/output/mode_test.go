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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			// Force isatty=true so the default-to-Rich case works in CI test env.
			got := autoDetectMode(func() bool { return true })
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("non-tty forces plain", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("TERM", "")
		t.Setenv("SAFEDEP_OUTPUT", "")
		t.Setenv("CLAUDE_CODE", "")
		t.Setenv("ANTHROPIC_AGENT", "")
		got := autoDetectMode(func() bool { return false })
		assert.Equal(t, Plain, got)
	})
}

func TestSetModeOverride(t *testing.T) {
	ResetModeForTest()
	SetMode(Agent)
	assert.Equal(t, Agent, CurrentMode())
	SetMode(Rich)
	assert.Equal(t, Rich, CurrentMode())
}
