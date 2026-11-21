package endpointidentity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMachineIDIsFrozen(t *testing.T) {
	cases := []struct {
		endpointID string
		want       string
	}{
		{endpointID: "gh:acme/web", want: "a9de23f27f00956189c20ff431787fba55be4c81178bf084afcd4a529fab9cdc"},
		{endpointID: "", want: "9658e06eab5c47722a63b4037eec38a0dded969057586d22ed82aa8e28673503"},
		{endpointID: "gh:acme/w\u00e9b", want: "f27880c7fa3fc7825e17f67c1ff757040d9222a062ea728b9efee307675e3ad6"},
	}

	for _, tc := range cases {
		t.Run(tc.endpointID, func(t *testing.T) {
			assert.Equal(t, tc.want, MachineID(tc.endpointID))
		})
	}
}
