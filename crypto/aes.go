package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

const (
	aesKDFSaltSize = 8
	aesKeySize     = 32
	aesNonce       = 12
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

// Based on: https://go.dev/src/crypto/cipher/example_test.go
// https://pkg.go.dev/golang.org/x/crypto/scrypt
func NewAesEncryptor(salt, key string) (SimpleEncryptor, error) {
	if len(salt) != aesKDFSaltSize {
		return nil, fmt.Errorf("salt size must be %d", aesKDFSaltSize)
	}

	sb := []byte(salt)
	kb, err := scrypt.Key([]byte(key), sb, 1<<15, 8, 1, aesKeySize)
	if err != nil {
		return nil, err
	}

	if len(kb) != aesKeySize {
		return nil, fmt.Errorf("key size must be %d", aesKeySize)
	}

	return &aesEncryptor{key: kb}, nil
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
