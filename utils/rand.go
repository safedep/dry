package utils

import (
	"crypto/rand"
	"log"
	"math/big"
)

// Int64 returns a cryptographically secure 64 bit random number
func Int64(max int64) int64 {
	return randSource(max).Int64()
}

func randSource(max int64) *big.Int {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		log.Fatalf("rand generator: %v", err)
	}

	return n
}
