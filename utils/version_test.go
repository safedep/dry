package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVersion(t *testing.T) {
	cases := []struct {
		name               string
		versionSelf        Version
		versionToCompare   string
		isGreaterThenEqual bool
	}{
		{
			name:               "equal versions",
			versionSelf:        Version("1.0.0"),
			versionToCompare:   "1.0.0",
			isGreaterThenEqual: true,
		},
		{
			name:               "greater with v prefix",
			versionSelf:        Version("v1.10.2"),
			versionToCompare:   "v1.9.9",
			isGreaterThenEqual: true,
		},
		{
			name:               "greater versions 1",
			versionSelf:        Version("1.5.0"),
			versionToCompare:   "1.0.4",
			isGreaterThenEqual: true,
		},
		{
			name:               "greater versions 2",
			versionSelf:        Version("1.5.5"),
			versionToCompare:   "1.5.4",
			isGreaterThenEqual: true,
		},
		{
			name:               "greater versions 3",
			versionSelf:        Version("1.10.2"),
			versionToCompare:   "1.9.2",
			isGreaterThenEqual: true,
		},
		{
			name:               "lower versions",
			versionSelf:        Version("1.10.2"),
			versionToCompare:   "2.9.2",
			isGreaterThenEqual: false,
		},
		{
			name:               "lower with different component length",
			versionSelf:        Version("2.13"),
			versionToCompare:   "2.13.2",
			isGreaterThenEqual: false,
		},
		{
			name:               "higher with different component length",
			versionSelf:        Version("2.33"),
			versionToCompare:   "2.13.2",
			isGreaterThenEqual: true,
		},
		{
			name:               "higher with different component length",
			versionSelf:        Version("4"),
			versionToCompare:   "3.9.2",
			isGreaterThenEqual: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.isGreaterThenEqual, test.versionSelf.IsGreaterThenOrEqualTo(test.versionToCompare))
		})
	}
}
