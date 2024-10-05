package crypto

import (
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAesEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, key)
	assert.NoError(t, err)

	encryptor, err := NewAesEncryptor(key)
	assert.NoError(t, err)

	data := []byte("hello world")
	encrypted, err := encryptor.Encrypt(data)
	assert.NoError(t, err)

	assert.NotEqual(t, data, encrypted)

	decrypted, err := encryptor.Decrypt(encrypted)
	assert.NoError(t, err)

	assert.Equal(t, data, decrypted)
}
