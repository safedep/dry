package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSemver(t *testing.T) {
	cases := []struct {
		input  string
		output bool
	}{
		{"1.2.3", true},
		{"1.2.3-alpha", true},
		{"1.2.3-alpha.1", true},
		{"1.2.3-0.3.7", true},
		{"1.2.3-x.7.z.92", true},
		{"1.2.3-x-y-z.-", true},
		{"1.2.3-x-y-z+metadata", true},
		{"1.2.3+metadata", true},
		{"<empty>", false},
		{"a.b", false},
		{"a.b.c", false},
	}

	for _, test := range cases {
		t.Run(test.input, func(t *testing.T) {
			assert.Equal(t, test.output, IsSemver(test.input))
		})
	}
}

func TestIsVersionInRange(t *testing.T) {
	cases := []struct {
		version string
		inRange string
		output  bool
	}{
		{
			"1.2.3",
			">=1.2.3",
			true,
		},
		{
			"1.2.3",
			">=1.2.3 <1.3.0",
			true,
		},
		{
			"1.2.3",
			"^1.2.0",
			true,
		},
		{
			"1.2.3",
			"^1.2.3",
			true,
		},
		{
			"1.2.3",
			"^1.2.4",
			false,
		},
		{
			"1.2.3",
			"^1.3.0",
			false,
		},
		{
			"1.2.3",
			"^1.0.0",
			true,
		},
	}

	for _, test := range cases {
		t.Run(test.version+" in "+test.inRange, func(t *testing.T) {
			assert.Equal(t, test.output, IsVersionInRange(test.version, test.inRange))
		})
	}
}

func TestIsAhead(t *testing.T) {
	cases := []struct {
		name   string
		base   string
		ahead  string
		output bool
	}{
		{
			"major version is ahead",
			"1.2.3",
			"2.0.0",
			true,
		},
		{
			"major version is behind",
			"2.0.0",
			"1.2.3",
			false,
		},
		{
			"minor version is ahead",
			"1.2.3",
			"1.3.0",
			true,
		},
		{
			"minor version is behind",
			"1.3.0",
			"1.2.3",
			false,
		},
		{
			"patch version is ahead",
			"1.2.3",
			"1.2.4",
			true,
		},
		{
			"patch version is behind",
			"1.2.4",
			"1.2.3",
			false,
		},
		{
			"versions are same",
			"1.2.3",
			"1.2.3",
			false,
		},
		{
			"versions have pre-release",
			"1.2.3-alpha",
			"1.2.3-beta",
			true,
		},
		{
			"alpha version is not ahead",
			"1.2.3",
			"1.2.3-alpha",
			false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.output, IsAhead(test.base, test.ahead))
		})
	}
}
