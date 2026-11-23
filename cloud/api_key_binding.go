package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/safedep/dry/log"
)

// fieldAPIKeyBindings holds the tenant and the metadata of API keys, each
// keyed by the fingerprint of its key, in one JSON value. One value means one
// keychain write, so a save can publish the binding of the new key next to
// the binding of the key that it replaces before it writes api_key. A reader
// then finds the binding of whichever key is stored at any point of the save.
//
// tenant_domain cannot serve as the tenant of the API key: SaveTokenCredential
// rewrites it, and an older dry writes it after api_key.
const fieldAPIKeyBindings = "api_key_bindings"

// apiKeyBinding is the tenant and the metadata of one API key.
type apiKeyBinding struct {
	Fingerprint  string `json:"fingerprint"`
	TenantDomain string `json:"tenant_domain"`
	KeyID        string `json:"key_id,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

func newAPIKeyBinding(apiKey, tenantDomain string, meta APIKeyMetadata) apiKeyBinding {
	b := apiKeyBinding{
		Fingerprint:  apiKeyFingerprint(apiKey),
		TenantDomain: tenantDomain,
		KeyID:        meta.KeyID,
	}
	if !meta.ExpiresAt.IsZero() {
		b.ExpiresAt = meta.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return b
}

// apiKeyFingerprint returns the SHA-256 of the API key in hex. It names the
// key that a binding belongs to. The keychain holds the key itself next to
// it, so the fingerprint reveals nothing that the keychain does not.
func apiKeyFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

// decodeAPIKeyBindings parses the stored bindings. A value that does not
// parse is logged and counts as no bindings: the key then resolves as an
// older dry saved it.
func decodeAPIKeyBindings(value string) []apiKeyBinding {
	if value == "" {
		return nil
	}

	var bindings []apiKeyBinding
	if err := json.Unmarshal([]byte(value), &bindings); err != nil {
		log.Warnf("cloud: ignoring the API key bindings in the keychain: %v", err)
		return nil
	}
	return bindings
}

// findAPIKeyBinding returns the binding of apiKey.
func findAPIKeyBinding(bindings []apiKeyBinding, apiKey string) (apiKeyBinding, bool) {
	fingerprint := apiKeyFingerprint(apiKey)
	for _, b := range bindings {
		if b.Fingerprint == fingerprint && b.TenantDomain != "" {
			return b, true
		}
	}
	return apiKeyBinding{}, false
}

// metadata returns the metadata of the binding. An expiry that does not parse
// is logged and treated as unknown: the key may still be valid, and only
// SafeDep Cloud can tell.
func (b apiKeyBinding) metadata() APIKeyMetadata {
	meta := APIKeyMetadata{KeyID: b.KeyID}
	if b.ExpiresAt == "" {
		return meta
	}

	parsed, err := time.Parse(time.RFC3339, b.ExpiresAt)
	if err != nil {
		log.Warnf("cloud: ignoring the API key expiry %q in the keychain: %v", b.ExpiresAt, err)
		return meta
	}
	meta.ExpiresAt = parsed
	return meta
}
