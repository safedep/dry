package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKnownBackground(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		dark bool
		ok   bool
	}{
		{"nothing known", map[string]string{"TERM": "xterm-256color"}, false, false},
		{"theme dark", map[string]string{ThemeEnv: "dark"}, true, true},
		{"theme light wins over COLORFGBG", map[string]string{ThemeEnv: " Light ", "COLORFGBG": "15;0"}, false, true},
		{"COLORFGBG dark", map[string]string{"COLORFGBG": "15;0"}, true, true},
		{"COLORFGBG light", map[string]string{"COLORFGBG": "0;15"}, false, true},
		{"COLORFGBG with default", map[string]string{"COLORFGBG": "15;default;0"}, true, true},
		{"COLORFGBG grey is dark", map[string]string{"COLORFGBG": "7;8"}, true, true},
		{"COLORFGBG not a code", map[string]string{"COLORFGBG": "15;default", "TERM": "xterm"}, false, false},
		{"linux console", map[string]string{"TERM": "linux"}, true, true},
		{"unknown theme value", map[string]string{ThemeEnv: "blue"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dark, ok := knownBackground(func(k string) string { return tc.env[k] })
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.dark, dark)
		})
	}
}

func TestSetKnownBackgroundSetsOnlyAKnownValue(t *testing.T) {
	var got []bool
	set := func(b bool) { got = append(got, b) }
	setKnownBackground(func(string) string { return "" }, set)
	assert.Empty(t, got, "lipgloss keeps its own detection")
	setKnownBackground(func(k string) string { return map[string]string{ThemeEnv: "light"}[k] }, set)
	assert.Equal(t, []bool{false}, got)
}
