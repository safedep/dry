package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGoogleCloudStorageDriverPrefix(t *testing.T) {
	cases := []struct {
		name            string
		key             string
		partitionByDate bool
		expected        string
		err             error
	}{
		{
			name:            "no partition",
			key:             "test",
			partitionByDate: false,
			expected:        "test",
		},
		{
			name:            "partition by date",
			key:             "test",
			partitionByDate: true,
			expected:        fmt.Sprintf("%s/%s", time.Now().UTC().Format("2006/01/02"), "test"),
		},
		{
			name:            "key has path",
			key:             "/a/b/c/test",
			partitionByDate: false,
			expected:        "a/b/c/test",
		},
		{
			name:            "key has path and partitioned by date",
			key:             "/a/b/c/test",
			partitionByDate: true,
			expected:        fmt.Sprintf("%s/%s", time.Now().UTC().Format("2006/01/02"), "a/b/c/test"),
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
			d := &googleCloudStorageDriver{
				partitionByDate: tc.partitionByDate,
			}

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
