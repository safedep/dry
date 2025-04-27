package utils

import "strings"

type Version string

// IsGreaterThenOrEqualTo compares its version to be greater than the provided input
func (v Version) IsGreaterThenOrEqualTo(version string) bool {
	selfVersionComponents := strings.Split(string(v), ".")
	inputVersionComponents := strings.Split(version, ".")

	selfVCLen := len(selfVersionComponents)
	inputVCLen := len(inputVersionComponents)

	maxCommonComponentsLen := max(selfVCLen, inputVCLen)

	// If a version is 2.23 and 2.12.3 i.e., different components length
	// [2, 23]
	// [2, 12, 3]
	// Then we add 0 to make then equal, resulting in
	// [2, 23, 0]
	// [2, 12, 3]
	// Then we compare each component with its counterpart
	for i := range maxCommonComponentsLen {
		componentSelf := string('0')
		componentInput := string('0')

		if i < selfVCLen {
			componentSelf = selfVersionComponents[i]
		}

		if i < inputVCLen {
			componentInput = inputVersionComponents[i]
		}

		if componentSelf != componentInput {
			return componentSelf > componentInput
		}
	}

	return true
}
