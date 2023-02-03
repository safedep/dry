package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiff(t *testing.T) {
	cases := []struct {
		name  string
		base  string
		head  string
		drift SemverDrift
		delta uint64
	}{
		{
			"Both versions are same",
			"1.2.2", "1.2.2",
			NoDrift,
			0,
		},
		{
			"Patch version drift",
			"1.2.2", "1.2.3",
			PatchDrift,
			1,
		},
		{
			"Minor version drift",
			"1.2.2", "1.3.5",
			MinorDrift,
			1,
		},
		{
			"Major version drift",
			"1.2.3", "2.0.0",
			MajorDrift,
			1,
		},
		{
			"Both version are same except pre-release tag",
			"1.1.1-rc1", "1.1.1-rc2",
			NoDrift, 0,
		},
		{
			"All components are different",
			"1.2.3-rc1", "4.5.6-rc2",
			MajorDrift, 3,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dt, dd := Diff(test.base, test.head)
			assert.Equal(t, test.drift, dt)
			assert.Equal(t, test.delta, dd)
		})
	}
}

func TestDriftHelper(t *testing.T) {
	d, _ := Diff("1.2.3", "1.2.4")
	assert.True(t, d.IsPatch())
	assert.False(t, d.IsMajor())
	assert.False(t, d.IsMinor())

	d, _ = Diff("1.2.3", "1.3.5")
	assert.True(t, d.IsMinor())

	d, _ = Diff("1.2.3", "2.3.5")
	assert.True(t, d.IsMajor())
}
