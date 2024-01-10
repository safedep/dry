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
		{"1.1", false},
		{"1-1-1", false},
	}

	for _, test := range cases {
		t.Run(test.input, func(t *testing.T) {
			assert.Equal(t, IsSemver(test.input), test.output)
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
			assert.Equal(t, IsVersionInRange(test.version, test.inRange), test.output)
		})
	}
}
