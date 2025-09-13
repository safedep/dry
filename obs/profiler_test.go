package obs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsProfilerEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"NotSet", "", false},
		{"SetToTrue", "true", true},
		{"SetToFalse", "false", false},
		{"SetToYes", "yes", false},
		{"SetTo1", "1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envKeyProfilerEnabled, tt.envValue)
			result := isProfilerEnabled()
			assert.Equal(t, tt.expected, result)
		})
	}
}
