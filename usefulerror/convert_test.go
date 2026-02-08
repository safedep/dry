package usefulerror

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterErrorConverter(t *testing.T) {
	tests := []struct {
		name          string
		identifier    string
		converterFunc ErrorConverterFunc
		expectPanic   bool
	}{
		{
			name:       "register error converter",
			identifier: "test",
			converterFunc: func(err error) (UsefulError, bool) {
				return nil, false
			},
		},
		{
			name:        "register error converter with empty identifier",
			identifier:  "",
			expectPanic: true,
		},
		{
			name:          "register error converter with nil converter function",
			identifier:    "test",
			converterFunc: nil,
			expectPanic:   true,
		},
		{
			name:       "register error converter with duplicate identifier",
			identifier: "test",
			converterFunc: func(err error) (UsefulError, bool) {
				return nil, false
			},
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					RegisterErrorConverter(tt.identifier, tt.converterFunc)
				})
			} else {
				assert.NotPanics(t, func() {
					RegisterErrorConverter(tt.identifier, tt.converterFunc)
				})

				// If it is not panicking, we should check if the converter is registered
				assert.True(t, registryIdentifierMap[fmt.Sprintf("%s/%s", registryIdentifierApplication, tt.identifier)])
			}
		})
	}
}
