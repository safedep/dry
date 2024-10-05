package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAesEncryptDecrypt(t *testing.T) {
	salt := "saltsalt"
	key := "keykeykeykey"

	encryptor, err := NewAesEncryptor(salt, key)
	assert.NoError(t, err)

	data := []byte("hello world")
	encrypted, err := encryptor.Encrypt(data)
	assert.NoError(t, err)

	assert.NotEqual(t, data, encrypted)

	decrypted, err := encryptor.Decrypt(encrypted)
	assert.NoError(t, err)

	assert.Equal(t, data, decrypted)
}
