package utils

import (
	"strconv"
	"strings"
)

type Version string

// IsGreaterThenOrEqualTo compares its version to be greater than the provided input
func (v Version) IsGreaterThenOrEqualTo(version string) bool {
	selfVersionComponents := strings.Split(string(v), ".")
	inputVersionComponents := strings.Split(version, ".")

	selfVCLen := len(selfVersionComponents)
	inputVCLen := len(inputVersionComponents)

	maxCommonComponentsLen := max(selfVCLen, inputVCLen)

	var err error

	// If a version is 2.23 and 2.12.3 i.e., different components length
	// [2, 23]
	// [2, 12, 3]
	// Then we add 0 to make then equal, resulting in
	// [2, 23, 0]
	// [2, 12, 3]
	// Then we compare each component with its counterpart
	for i := range maxCommonComponentsLen {
		componentSelf := 0
		componentInput := 0

		if i < selfVCLen {
			token := strings.TrimPrefix(selfVersionComponents[i], "v")
			componentSelf, err = strconv.Atoi(token)
			if err != nil {
				return false
			}
		}

		if i < inputVCLen {
			token := strings.TrimPrefix(inputVersionComponents[i], "v")
			componentInput, err = strconv.Atoi(token)
			if err != nil {
				return false
			}
		}

		// convert it to

		if componentSelf != componentInput {
			return componentSelf > componentInput
		}
	}
	return true
}
