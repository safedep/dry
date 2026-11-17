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

	// SessionLifetime is how long the gateway keeps the session of the key.
	// Zero leaves the lifetime to the API definition. Set it to the life of a
	// short-lived key, so that the gateway removes the session when the key
	// expires.
	SessionLifetime time.Duration
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
