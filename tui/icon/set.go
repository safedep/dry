// tui/icon/set.go
package icon

// DefaultSet returns the canonical SafeDep icon set.
func DefaultSet() Set {
	return Set{
		KeySuccess: {Unicode: "✓", Ascii: "[OK]", Agent: "OK:"},
		KeyError:   {Unicode: "✗", Ascii: "[ERR]", Agent: "ERR:"},
		KeyWarning: {Unicode: "⚠", Ascii: "[WARN]", Agent: "WARN:"},
		KeyInfo:    {Unicode: "i", Ascii: "[INFO]", Agent: "INFO:"},
		KeyBullet:  {Unicode: "•", Ascii: "*", Agent: "-"},
		KeyArrow:   {Unicode: "›", Ascii: ">", Agent: ">"},
		// KeySpinnerFrames is intentionally not included: spinner frames live
		// in the spinner package (braille pattern), and Plain/Agent modes
		// don't animate — they print static lines.
	}
}
