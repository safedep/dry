package crypto

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func init() {
	if err := assertPRNG(); err != nil {
		panic(err)
	}
}

func assertPRNG() error {
	buf := make([]byte, 1)
	_, err := rand.Read(buf)
	if err != nil {
		return fmt.Errorf("crypto/rand: failed to read random data: %v", err)
	}

	return nil
}

func RandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// Ref: https://gist.github.com/dopey/c69559607800d2f2f90b1b1ed4e550fb?permalink_comment_id=3398811#gistcomment-3398811
func RandomString(length int, alphabet string) (string, error) {
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}

		bytes[i] = alphabet[n.Int64()]
	}

	return string(bytes), nil
}

func RandomUrlSafeString(length int) (string, error) {
	return RandomString(length,
		"0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_")
}
