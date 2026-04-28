// tui/prompt/secret.go
package prompt

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/safedep/dry/tui/output"
)

// SecretOption configures Secret behavior.
type SecretOption func(*secretConfig)

type secretConfig struct {
	mask bool
}

// WithNoMask disables the default echoed-asterisk behavior; input is fully silent.
// Use sparingly — users generally benefit from seeing that keystrokes registered.
func WithNoMask() SecretOption { return func(c *secretConfig) { c.mask = false } }

// Secret reads a password with masked echo by default. Each typed character
// prints '*' to stderr. Backspace (BS or DEL) erases one character in both
// buffer and echo. Ctrl-C (0x03) returns ErrCancelled. Enter commits.
//
// Non-TTY stdin → ErrNoTTY. Agent mode → ErrAgentMode.
func Secret(label string, opts ...SecretOption) (string, error) {
	if output.CurrentMode() == output.Agent {
		return "", ErrAgentMode
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", ErrNoTTY
	}

	cfg := secretConfig{mask: true}
	for _, o := range opts {
		o(&cfg)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("secret: enter raw mode: %w", err)
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	return readSecretMasked(os.Stdin, output.Stderr(), label, cfg.mask)
}

// readSecretMasked is the test-friendly core. r must be a byte-oriented reader;
// in production it is os.Stdin in raw mode.
func readSecretMasked(r io.Reader, w io.Writer, label string, mask bool) (string, error) {
	_, _ = fmt.Fprintf(w, "%s: ", label)

	var buf []byte
	one := make([]byte, 1)
	for {
		n, err := r.Read(one)
		if err != nil {
			if err == io.EOF {
				_, _ = fmt.Fprintln(w)
				return "", ErrCancelled
			}
			return "", err
		}
		if n == 0 {
			continue
		}
		b := one[0]
		switch b {
		case '\r', '\n':
			_, _ = fmt.Fprintln(w)
			return string(buf), nil
		case 0x03: // Ctrl-C
			_, _ = fmt.Fprintln(w)
			return "", ErrCancelled
		case '\b', 0x7f: // backspace / DEL
			if len(buf) == 0 {
				continue
			}
			buf = buf[:len(buf)-1]
			if mask {
				// erase one masked char visually: \b space \b
				_, _ = fmt.Fprint(w, "\b \b")
			}
		default:
			if b < 0x20 { // ignore other control chars
				continue
			}
			buf = append(buf, b)
			if mask {
				_, _ = fmt.Fprint(w, "*")
			}
		}
	}
}
