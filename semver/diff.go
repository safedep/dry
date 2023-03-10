package semver

import (
	mver "github.com/Masterminds/semver/v3"
)

type SemverDrift int

const (
	NoDrift      = 0
	MajorDrift   = 1
	MinorDrift   = 2
	PatchDrift   = 3
	UnknownDrift = 100
)

// Diff calculates the differnce between two semver
// string and returns the drift type and delta
// Major > Minor > Patch precedence is followed
func Diff(base, head string) (SemverDrift, uint64) {
	v1, err := mver.NewVersion(base)
	if err != nil {
		return UnknownDrift, 0
	}

	v2, err := mver.NewVersion(head)
	if err != nil {
		return UnknownDrift, 0
	}

	if n := v2.Major() - v1.Major(); n != 0 {
		return MajorDrift, n
	}

	if n := v2.Minor() - v1.Minor(); n != 0 {
		return MinorDrift, n
	}

	if n := v2.Patch() - v1.Patch(); n != 0 {
		return PatchDrift, n
	}

	return NoDrift, 0
}

func (d SemverDrift) IsNone() bool {
	return d == NoDrift
}

func (d SemverDrift) IsMajor() bool {
	return d == MajorDrift
}

func (d SemverDrift) IsMinor() bool {
	return d == MinorDrift
}

func (d SemverDrift) IsPatch() bool {
	return d == PatchDrift
}
