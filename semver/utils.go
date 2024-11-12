package semver

import (
	mver "github.com/Masterminds/semver/v3"
)

func IsSemver(s string) bool {
	_, err := mver.NewVersion(s)
	return err == nil
}

func IsVersionInRange(v, r string) bool {
	v1, err := mver.NewVersion(v)
	if err != nil {
		return false
	}

	r1, err := mver.NewConstraint(r)
	if err != nil {
		return false
	}

	return r1.Check(v1)
}

// IsAhead checks if head version is ahead of
// base version in terms of semver
func IsAhead(base, head string) bool {
	v1, err := mver.NewVersion(base)
	if err != nil {
		return false
	}

	v2, err := mver.NewVersion(head)
	if err != nil {
		return false
	}

	return v2.GreaterThan(v1)
}
