package apiguard

import (
	"fmt"
	"time"

	"github.com/safedep/dry/crypto"
)

const (
	defaultKeyPrefix = "sfd"
	defaultKeyLength = 56
)

type KeyArgs struct {
	Info      KeyInfo
	Tags      []string
	Alias     string
	PolicyId  string
	Policies  []string
	ExpiresAt time.Time
}

type ApiKey struct {
	Key       string
	KeyId     string // API Guard specific key ID
	ExpiresAt time.Time
}

type KeyGen func() (string, error)

func defaultKeyGen() KeyGen {
	return func() (string, error) {
		str, err := crypto.RandomUrlSafeString(defaultKeyLength)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%s_%s", defaultKeyPrefix, str), nil
	}
}
