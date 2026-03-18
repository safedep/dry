package storage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoogleCloudStorageDriverPrefix(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		expected string
		err      error
	}{
		{
			name:     "simple key",
			key:      "test",
			expected: "test",
		},
		{
			name:     "key has path",
			key:      "/a/b/c/test",
			expected: "a/b/c/test",
		},
		{
			name: "empty key",
			key:  "",
			err:  fmt.Errorf("key cannot be empty"),
		},
		{
			name: "key has only slashes",
			key:  "/////",
			err:  fmt.Errorf("key cannot be empty"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &googleCloudStorageDriver{}

			pf, err := d.prefix(tc.key)
			if tc.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, pf)
			}
		})
	}
}
