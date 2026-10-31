package id

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	got, err := New("r")
	require.NoError(t, err)

	assert.Len(t, got, len("r_")+DefaultLength)
	assert.True(t, strings.HasPrefix(got, "r_"))
	assert.True(t, Valid("r", got))
}

func TestNewN(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		n       int
		wantErr string
	}{
		{name: "minimum length", prefix: "r", n: MinLength},
		{name: "maximum length", prefix: "scan", n: MaxLength},
		{name: "longest prefix", prefix: strings.Repeat("p", MaxPrefixLength), n: DefaultLength},
		{name: "too short", prefix: "r", n: MinLength - 1, wantErr: "length"},
		{name: "too long", prefix: "r", n: MaxLength + 1, wantErr: "length"},
		{name: "zero length", prefix: "r", n: 0, wantErr: "length"},
		{name: "empty prefix", prefix: "", n: DefaultLength, wantErr: "empty"},
		{name: "prefix too long", prefix: strings.Repeat("p", MaxPrefixLength+1), n: DefaultLength, wantErr: "longer"},
		{name: "upper case prefix", prefix: "R", n: DefaultLength, wantErr: "lowercase"},
		{name: "digit in prefix", prefix: "r2", n: DefaultLength, wantErr: "lowercase"},
		{name: "underscore in prefix", prefix: "r_x", n: DefaultLength, wantErr: "lowercase"},
		{name: "non-ascii prefix", prefix: "é", n: DefaultLength, wantErr: "lowercase"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewN(tc.prefix, tc.n)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Len(t, got, len(tc.prefix)+1+tc.n)
			assert.True(t, Valid(tc.prefix, got))
		})
	}
}

func TestNewUsesOnlyTheAlphabet(t *testing.T) {
	seen := map[rune]bool{}
	for range 200 {
		got, err := NewN("x", MaxLength)
		require.NoError(t, err)

		for _, r := range strings.TrimPrefix(got, "x_") {
			seen[r] = true
		}
	}

	for r := range seen {
		assert.Contains(t, Alphabet, string(r))
	}
	for _, r := range "ilou" {
		assert.False(t, seen[r], "character %q is not in the alphabet", r)
	}
}

func TestNewIsRandom(t *testing.T) {
	ids := map[string]bool{}
	for range 1000 {
		got, err := New("r")
		require.NoError(t, err)
		require.False(t, ids[got], "duplicate id %s", got)
		ids[got] = true
	}
}

func TestValid(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		s      string
		want   bool
	}{
		{name: "valid", prefix: "r", s: "r_9c1x4kq7dm", want: true},
		{name: "valid long prefix", prefix: "scan", s: "scan_0123456789abcdefghjk", want: true},
		{name: "minimum length", prefix: "r", s: "r_" + strings.Repeat("a", MinLength), want: true},
		{name: "maximum length", prefix: "r", s: "r_" + strings.Repeat("a", MaxLength), want: true},
		{name: "other prefix", prefix: "r", s: "s_9c1x4kq7dm"},
		{name: "prefix is a part of another prefix", prefix: "r", s: "rr_9c1x4kq7dm"},
		{name: "no separator", prefix: "r", s: "r9c1x4kq7dm"},
		{name: "other separator", prefix: "r", s: "r-9c1x4kq7dm"},
		{name: "too short", prefix: "r", s: "r_" + strings.Repeat("a", MinLength-1)},
		{name: "too long", prefix: "r", s: "r_" + strings.Repeat("a", MaxLength+1)},
		{name: "letter i", prefix: "r", s: "r_9c1x4kq7di"},
		{name: "letter l", prefix: "r", s: "r_9c1x4kq7dl"},
		{name: "letter o", prefix: "r", s: "r_9c1x4kq7do"},
		{name: "letter u", prefix: "r", s: "r_9c1x4kq7du"},
		{name: "upper case", prefix: "r", s: "r_9C1X4KQ7DM"},
		{name: "second separator", prefix: "r", s: "r_9c1x4_q7dm"},
		{name: "base64 characters", prefix: "r", s: "r_9c1x4kq7d-"},
		{name: "empty", prefix: "r", s: ""},
		{name: "invalid prefix", prefix: "R", s: "R_9c1x4kq7dm"},
		{name: "empty prefix", prefix: "", s: "_9c1x4kq7dm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Valid(tc.prefix, tc.s))
		})
	}
}

func TestValidPrefix(t *testing.T) {
	cases := []struct {
		prefix string
		want   bool
	}{
		{prefix: "r", want: true},
		{prefix: "scan", want: true},
		{prefix: strings.Repeat("a", MaxPrefixLength), want: true},
		{prefix: ""},
		{prefix: strings.Repeat("a", MaxPrefixLength+1)},
		{prefix: "Rule"},
		{prefix: "r1"},
		{prefix: "r_"},
		{prefix: "r-x"},
		{prefix: " r"},
	}

	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			assert.Equal(t, tc.want, ValidPrefix(tc.prefix))
		})
	}
}
