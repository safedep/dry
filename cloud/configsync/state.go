package configsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// FormatVersion is the format of state.json that this package writes. A
// reader that finds a higher version reports ErrNoCache, because it cannot
// know what the new fields mean.
const FormatVersion = 1

const digestPrefix = "sha256:"

var validDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// State is the content of state.json.
//
// fetched_at moves on each fetch that ends with the policy of the server in
// the cache. last_attempt_at and last_error record the last fetch that
// failed, and a commit. So the last fetch failed when last_attempt_at is
// after fetched_at (LastFetchFailed).
type State struct {
	FormatVersion int `json:"format_version"`

	// Tenant is the tenant domain of the policy files.
	Tenant string `json:"tenant"`

	// Digest and Revision name the current policy file.
	Digest   string `json:"digest"`
	Revision uint64 `json:"revision"`

	// PreviousDigest and PreviousRevision name the policy file that was
	// current before. Store.Load falls back to it.
	PreviousDigest   string `json:"previous_digest"`
	PreviousRevision uint64 `json:"previous_revision"`

	// HighestRevision is the highest revision of the tenant that a fetch
	// accepted. A change of tenant resets it.
	HighestRevision uint64 `json:"highest_revision"`

	// StoredAt is the time of the commit of the current policy file.
	StoredAt time.Time `json:"stored_at,omitzero"`

	// FetchedAt is the time of the last fetch that ended with the policy of
	// the server in the cache.
	FetchedAt time.Time `json:"fetched_at,omitzero"`

	// LastAttemptAt and LastError record the last failed fetch, or the last
	// commit, which clears LastError.
	LastAttemptAt time.Time `json:"last_attempt_at,omitzero"`
	LastError     string    `json:"last_error"`

	// Rejected records the last policy that the validator refused. A commit
	// clears it.
	Rejected *Rejection `json:"rejected,omitempty"`

	// CapabilitySources maps a capability name to the digest of the policy
	// file that the capability comes from, for each capability that does not
	// come from the current file. The store keeps each file that it names.
	CapabilitySources map[string]string `json:"capability_sources,omitempty"`
}

// Rejection is a policy that the validator of the tool refused.
type Rejection struct {
	Digest   string    `json:"digest"`
	Revision uint64    `json:"revision"`
	Reason   string    `json:"reason"`
	At       time.Time `json:"at"`
}

// HasPolicy reports whether the state names a current policy file.
func (s *State) HasPolicy() bool {
	return s != nil && s.Digest != ""
}

// LastFetchFailed reports whether the last fetch failed: a gRPC error, a
// failed check or a rejection.
func (s *State) LastFetchFailed() bool {
	return s != nil && s.LastAttemptAt.After(s.FetchedAt)
}

// LastFetchRejected reports whether the last fetch failed because the
// validator refused the policy.
func (s *State) LastFetchRejected() bool {
	return s.LastFetchFailed() && s.Rejected != nil && s.Rejected.At.Equal(s.LastAttemptAt)
}

// keepDigests returns the digests of the policy files that the state names.
// A digest that is not valid names no file, so a cache that a user edits
// cannot make the store touch a file outside its layout.
func (s *State) keepDigests() map[string]bool {
	keep := map[string]bool{}
	add := func(d string) {
		if validDigest.MatchString(d) {
			keep[d] = true
		}
	}
	add(s.Digest)
	add(s.PreviousDigest)
	for _, d := range s.CapabilitySources {
		add(d)
	}
	return keep
}

// Digest returns "sha256:" and the hex SHA-256 of raw, the form of the
// digest of a resolved policy.
func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return digestPrefix + hex.EncodeToString(sum[:])
}

func policyFileName(digest string) (string, error) {
	if !validDigest.MatchString(digest) {
		return "", fmt.Errorf("%w: %q", ErrInvalidDigest, digest)
	}
	return "policy-" + strings.TrimPrefix(digest, digestPrefix) + ".pb", nil
}
