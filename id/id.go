// Package id makes short, prefixed, random ids, such as the rule id
// "r_9c1x4kq7dm".
//
// An id is a prefix, "_" and random characters from crypto/rand. The
// characters come from lowercase Crockford base32. It has no "i", "l", "o"
// or "u", so a person can read an id aloud and type it. The default length
// of 10 characters gives 50 bits.
//
// The package does not check for a collision. The caller checks the id
// against its own store.
package id

import (
	"fmt"
	"strings"

	"github.com/safedep/dry/crypto"
)

const (
	// Alphabet is lowercase Crockford base32.
	Alphabet = "0123456789abcdefghjkmnpqrstvwxyz"

	// DefaultLength is the number of random characters that New makes.
	DefaultLength = 10

	// MinLength is the smallest number of random characters. Fewer than 8
	// characters (40 bits) make an id that a caller can guess.
	MinLength = 8

	// MaxLength is the largest number of random characters.
	MaxLength = 64

	// MaxPrefixLength is the longest prefix. A short prefix keeps the id
	// short. The prefix only names the kind of the id.
	MaxPrefixLength = 16

	separator = "_"
)

// New returns the prefix, "_" and DefaultLength random characters.
func New(prefix string) (string, error) {
	return NewN(prefix, DefaultLength)
}

// NewN returns the prefix, "_" and n random characters. The prefix must be
// valid (see ValidPrefix) and n must be from MinLength to MaxLength.
func NewN(prefix string, n int) (string, error) {
	if err := checkPrefix(prefix); err != nil {
		return "", err
	}

	if n < MinLength || n > MaxLength {
		return "", fmt.Errorf("id: length %d is not from %d to %d", n, MinLength, MaxLength)
	}

	body, err := crypto.RandomString(n, Alphabet)
	if err != nil {
		return "", fmt.Errorf("id: read random characters: %w", err)
	}

	return prefix + separator + body, nil
}

// ValidPrefix reports whether a prefix is 1 to MaxPrefixLength lowercase
// ASCII letters. The prefix has no digit and no "_", so the first "_" of an
// id always ends the prefix.
func ValidPrefix(prefix string) bool {
	return checkPrefix(prefix) == nil
}

// Valid reports whether s is an id with the given prefix: the prefix, "_"
// and MinLength to MaxLength characters of the Alphabet. It returns false for
// an invalid prefix.
func Valid(prefix, s string) bool {
	if !ValidPrefix(prefix) {
		return false
	}

	body, ok := strings.CutPrefix(s, prefix+separator)
	if !ok || len(body) < MinLength || len(body) > MaxLength {
		return false
	}

	for i := 0; i < len(body); i++ {
		if strings.IndexByte(Alphabet, body[i]) < 0 {
			return false
		}
	}

	return true
}

func checkPrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("id: prefix is empty")
	}

	if len(prefix) > MaxPrefixLength {
		return fmt.Errorf("id: prefix %q is longer than %d characters", prefix, MaxPrefixLength)
	}

	for i := 0; i < len(prefix); i++ {
		if prefix[i] < 'a' || prefix[i] > 'z' {
			return fmt.Errorf("id: prefix %q has a character that is not a lowercase letter", prefix)
		}
	}

	return nil
}
