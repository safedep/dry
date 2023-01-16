package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInt64(t *testing.T) {
	s := map[int64]bool{}
	count := 1000
	for i := 0; i < count; i++ {
		n := Int64(2 << 31)
		assert.False(t, s[n])
		s[n] = true
	}
}
