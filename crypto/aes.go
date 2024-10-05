package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const (
	aesKeySize = 32
	aesNonce   = 12
)

// This is a simple encryptor interface that does not support crypto configuration
// keys, IVs, etc. It is meant to be used for simple encryption/decryption tasks for
// internal data security. This is not meant to be used for secure communication
type SimpleEncryptor interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

type aesEncryptor struct {
	key []byte
}

func NewAesEncryptor(key []byte) (SimpleEncryptor, error) {
	if len(key) != aesKeySize {
		return nil, fmt.Errorf("key size must be %d", aesKeySize)
	}

	return &aesEncryptor{key: key}, nil
}

func (a *aesEncryptor) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesNonce)
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)
	return append(nonce, ciphertext...), nil
}

func (a *aesEncryptor) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}

	if len(data) < aesNonce {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:aesNonce]
	ciphertext := data[aesNonce:]

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
