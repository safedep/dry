package async

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeSubjects(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		desired  []string
		want     []string
	}{
		{
			name:     "empty existing returns desired",
			existing: nil,
			desired:  []string{"a", "b"},
			want:     []string{"a", "b"},
		},
		{
			name:     "empty desired returns existing",
			existing: []string{"a", "b"},
			desired:  nil,
			want:     []string{"a", "b"},
		},
		{
			name:     "union deduplicates",
			existing: []string{"a", "b"},
			desired:  []string{"b", "c"},
			want:     []string{"a", "b", "c"},
		},
		{
			name:     "all present is no-op",
			existing: []string{"a", "b", "c"},
			desired:  []string{"a", "b"},
			want:     []string{"a", "b", "c"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, mergeSubjects(tc.existing, tc.desired))
		})
	}
}

func TestSameSubjects(t *testing.T) {
	assert.True(t, sameSubjects([]string{"a", "b"}, []string{"b", "a"}))
	assert.True(t, sameSubjects(nil, nil))
	assert.False(t, sameSubjects([]string{"a"}, []string{"a", "b"}))
	assert.False(t, sameSubjects([]string{"a", "b"}, []string{"a", "c"}))
}

