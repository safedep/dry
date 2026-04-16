// tui/prompt/prompt.go
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/safedep/dry/tui/output"
)

// Prompt reads a visible line of input. Returns ErrAgentMode in Agent mode
// and ErrNoTTY when stdin is not a terminal.
func Prompt(label string) (string, error) {
	if output.CurrentMode() == output.Agent {
		return "", ErrAgentMode
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", ErrNoTTY
	}
	return promptFromReader(os.Stdin, label)
}

// Confirm reads a y/n answer. defaultYes controls the default when the user
// just presses Enter.
func Confirm(label string, defaultYes bool) (bool, error) {
	if output.CurrentMode() == output.Agent {
		return false, ErrAgentMode
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, ErrNoTTY
	}
	return confirmFromReader(os.Stdin, label, defaultYes)
}

// Select presents a numbered list and reads the user's 1-based choice,
// returning the chosen string.
func Select(label string, choices []string) (string, error) {
	if output.CurrentMode() == output.Agent {
		return "", ErrAgentMode
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", ErrNoTTY
	}
	return selectFromReader(os.Stdin, label, choices)
}

// --- test-friendly core implementations ---

func promptFromReader(r io.Reader, label string) (string, error) {
	fmt.Fprintf(output.Stderr(), "%s: ", label)
	line, err := readLine(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func confirmFromReader(r io.Reader, label string, defaultYes bool) (bool, error) {
	br := bufio.NewReader(r)
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	for {
		fmt.Fprintf(output.Stderr(), "%s %s: ", label, suffix)

		line, err := readLineFromBuf(br)
		if err != nil {
			return false, err
		}
		line = strings.TrimSpace(strings.ToLower(line))
		switch line {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(output.Stderr(), "invalid choice; please answer y or n")
		}
	}
}

func selectFromReader(r io.Reader, label string, choices []string) (string, error) {
	br := bufio.NewReader(r)
	for {
		fmt.Fprintf(output.Stderr(), "%s\n", label)
		for i, c := range choices {
			fmt.Fprintf(output.Stderr(), "  %d) %s\n", i+1, c)
		}
		fmt.Fprint(output.Stderr(), "> ")

		line, err := readLineFromBuf(br)
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		idx, err := strconv.Atoi(line)
		if err != nil || idx < 1 || idx > len(choices) {
			fmt.Fprintln(output.Stderr(), "invalid choice; please enter a number from the list")
			continue
		}
		return choices[idx-1], nil
	}
}

func readLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	return readLineFromBuf(br)
}

func readLineFromBuf(br *bufio.Reader) (string, error) {
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && line == "" {
		return "", ErrCancelled
	}
	return line, nil
}
