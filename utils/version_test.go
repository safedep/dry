package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVersion(t *testing.T) {
	cases := []struct {
		name             string
		versionSelf      Version
		versionToCompare string
		isGreater        bool
	}{
		{
			name:             "equal versions",
			versionSelf:      Version("1.0.0"),
			versionToCompare: "1.0.0",
			isGreater:        true,
		},
		{
			name:             "greater versions 1",
			versionSelf:      Version("1.5.0"),
			versionToCompare: "1.0.4",
			isGreater:        true,
		},
		{
			name:             "greater versions 2",
			versionSelf:      Version("1.5.5"),
			versionToCompare: "1.5.4",
			isGreater:        true,
		},
		{
			name:             "greater versions 3",
			versionSelf:      Version("1.10.2"),
			versionToCompare: "1.9.2",
			isGreater:        false,
		},
		{
			name:             "lower versions",
			versionSelf:      Version("1.10.2"),
			versionToCompare: "2.9.2",
			isGreater:        false,
		},
		{
			name:             "lower with different component length",
			versionSelf:      Version("2.13"),
			versionToCompare: "2.13.2",
			isGreater:        false,
		},
		{
			name:             "higher with different component length",
			versionSelf:      Version("2.33"),
			versionToCompare: "2.13.2",
			isGreater:        true,
		},
		{
			name:             "higher with different component length",
			versionSelf:      Version("4"),
			versionToCompare: "3.9.2",
			isGreater:        true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.isGreater, test.versionSelf.IsGreaterThenOrEqualTo(test.versionToCompare))
		})
	}
}
