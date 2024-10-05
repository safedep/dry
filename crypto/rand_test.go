package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkRandomBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = RandomBytes(128)
	}
}

func TestRandomBytesSize(t *testing.T) {
	bytes, err := RandomBytes(128)
	assert.NoError(t, err)

	assert.Equal(t, 128, len(bytes))
}

func TestRandomBytesAreNotEqual(t *testing.T) {
	bytes1, err := RandomBytes(128)
	assert.NoError(t, err)

	bytes2, err := RandomBytes(128)
	assert.NoError(t, err)

	assert.NotEqual(t, bytes1, bytes2)
}

func TestUrlSafeStringSize(t *testing.T) {
	str, err := RandomUrlSafeString(128)
	assert.NoError(t, err)

	assert.Equal(t, 128, len(str))
}
