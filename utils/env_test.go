package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvBool(t *testing.T) {
	cases := []struct {
		name          string
		envSetName    string
		envSetValue   string
		envLookupName string
		envDefaultVal bool
		envRet        bool
	}{
		{
			"Value is true",
			"EB_A",
			"true",
			"EB_A",
			false,
			true,
		},
		{
			"Value is false",
			"EB_A",
			"false",
			"EB_A",
			true,
			false,
		},
		{
			"Value is not set",
			"EB_A",
			"true",
			"EB_B",
			false,
			false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.envSetName, test.envSetValue)
			val := EnvBool(test.envLookupName, test.envDefaultVal)
			assert.Equal(t, test.envRet, val)
		})
	}
}
