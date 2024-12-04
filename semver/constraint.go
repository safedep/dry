package semver

import (
	"fmt"

	mver "github.com/Masterminds/semver/v3"
)

type constraintResolver struct {
	constraint *mver.Constraints
}

func NewConstraintResolver(constraint string) (*constraintResolver, error) {
	c, err := mver.NewConstraint(constraint)
	if err != nil {
		return nil, err
	}

	return &constraintResolver{
		constraint: c,
	}, nil
}

// Check if a version is in the constraint range
func (c *constraintResolver) Check(v string) bool {
	ver, err := mver.NewVersion(v)
	if err != nil {
		return false
	}

	return c.constraint.Check(ver)
}

// Get the lowest version in the constraint range
// This is inefficient. Need to find a better way to achieve this
func (c *constraintResolver) Lowest() (string, error) {
	var major, minor, patch int
	maxPerGroup := 20

	for major < maxPerGroup {
		for minor < maxPerGroup {
			for patch < maxPerGroup {
				version := fmt.Sprintf("%d.%d.%d", major, minor, patch)
				if c.Check(version) {
					return version, nil
				}

				patch++
			}

			minor++
		}

		major++
	}

	return "", fmt.Errorf("No version found in the constraint range")
}
