package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/safedep/dry/log"
)

// fieldAPIKeyMetadata holds the tenant, the id and the expiry of the stored
// API key as one JSON value, with the fingerprint of the key that it
// describes. An older dry replaces api_key and leaves this field, so the
// resolver trusts it only while the fingerprint matches api_key.
//
// tenant_domain cannot serve as the tenant of the API key, because
// SaveTokenCredential rewrites it.
const fieldAPIKeyMetadata = "api_key_metadata"

// storedAPIKeyMetadata is the JSON value of fieldAPIKeyMetadata.
type storedAPIKeyMetadata struct {
	Fingerprint  string `json:"fingerprint"`
	TenantDomain string `json:"tenant_domain"`
	KeyID        string `json:"key_id,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

func newStoredAPIKeyMetadata(apiKey, tenantDomain string, meta APIKeyMetadata) storedAPIKeyMetadata {
	s := storedAPIKeyMetadata{
		Fingerprint:  apiKeyFingerprint(apiKey),
		TenantDomain: tenantDomain,
		KeyID:        meta.KeyID,
	}
	if !meta.ExpiresAt.IsZero() {
		s.ExpiresAt = meta.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return s
}

// apiKeyFingerprint returns the SHA-256 of the API key in hex. The keychain
// holds the key itself next to it, so the fingerprint reveals nothing that
// the keychain does not.
func apiKeyFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

// apiKeyMetadataFor returns the stored metadata when it describes apiKey. A
// value that does not parse is logged and counts as absent, so the key
// resolves as an older dry saved it.
func apiKeyMetadataFor(value, apiKey string) (storedAPIKeyMetadata, bool) {
	if value == "" {
		return storedAPIKeyMetadata{}, false
	}

	var s storedAPIKeyMetadata
	if err := json.Unmarshal([]byte(value), &s); err != nil {
		log.Warnf("cloud: ignoring the API key metadata in the keychain: %v", err)
		return storedAPIKeyMetadata{}, false
	}
	if s.Fingerprint != apiKeyFingerprint(apiKey) || s.TenantDomain == "" {
		return storedAPIKeyMetadata{}, false
	}
	return s, true
}

// metadata returns the id and the expiry. An expiry that does not parse is
// logged and treated as unknown: the key may still be valid, and only
// SafeDep Cloud can tell.
func (s storedAPIKeyMetadata) metadata() APIKeyMetadata {
	meta := APIKeyMetadata{KeyID: s.KeyID}
	if s.ExpiresAt == "" {
		return meta
	}

	parsed, err := time.Parse(time.RFC3339, s.ExpiresAt)
	if err != nil {
		log.Warnf("cloud: ignoring the API key expiry %q in the keychain: %v", s.ExpiresAt, err)
		return meta
	}
	meta.ExpiresAt = parsed
	return meta
}
