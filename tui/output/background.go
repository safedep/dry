// tui/output/background.go
package output

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ThemeEnv names the variable that sets the background of the terminal:
// dark or light. With it, the terminal gets no query.
const ThemeEnv = "SAFEDEP_THEME"

// lipgloss asks the terminal for its background colour the first time it
// renders an adaptive colour. It writes an OSC 11 query and a cursor
// position query, and waits for the answer. A terminal that does not
// answer shows the queries as text, and the wait slows the first line.
// When the environment already gives the background, vet sets it and
// lipgloss sends no query.
func init() { setKnownBackground(os.Getenv, lipgloss.SetHasDarkBackground) }

func setKnownBackground(getenv func(string) string, set func(bool)) {
	if dark, ok := knownBackground(getenv); ok {
		set(dark)
	}
}

// knownBackground returns whether the background is dark, when the
// environment gives it with no query:
//   - SAFEDEP_THEME is dark or light.
//   - COLORFGBG ends with the background colour code. The codes 0 to 6
//     and 8 are dark.
//   - TERM is linux. The Linux console has a black background and does
//     not answer the query.
func knownBackground(getenv func(string) string) (dark, ok bool) {
	switch strings.ToLower(strings.TrimSpace(getenv(ThemeEnv))) {
	case "dark":
		return true, true
	case "light":
		return false, true
	}
	if fgbg := getenv("COLORFGBG"); strings.Contains(fgbg, ";") {
		parts := strings.Split(fgbg, ";")
		if code, err := strconv.Atoi(parts[len(parts)-1]); err == nil && code >= 0 && code <= 15 {
			return code <= 6 || code == 8, true
		}
	}
	if getenv("TERM") == "linux" {
		return true, true
	}
	return false, false
}
