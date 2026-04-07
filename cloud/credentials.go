package cloud

import "fmt"

// CredentialType identifies the authentication plane.
type CredentialType int

const (
	CredentialTypeUnspecified CredentialType = iota
	CredentialTypeAPIKey                     // Data plane (api.safedep.io)
	CredentialTypeToken                      // Control plane (cloud.safedep.io)
)

// Credentials holds SafeDep Cloud authentication details.
// Fields are private. Use constructors to create, getters to access.
type Credentials struct {
	credType     CredentialType
	apiKey       string
	token        string
	refreshToken string
	tenantDomain string
}

// NewAPIKeyCredential creates data plane credentials.
// Returns error if apiKey is empty.
func NewAPIKeyCredential(apiKey, tenantDomain string) (*Credentials, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrMissingCredentials)
	}
	return &Credentials{
		credType:     CredentialTypeAPIKey,
		apiKey:       apiKey,
		tenantDomain: tenantDomain,
	}, nil
}

// NewTokenCredential creates control plane credentials.
// Returns error if token is empty.
func NewTokenCredential(token, refreshToken, tenantDomain string) (*Credentials, error) {
	if token == "" {
		return nil, fmt.Errorf("%w: token is required", ErrMissingCredentials)
	}
	return &Credentials{
		credType:     CredentialTypeToken,
		token:        token,
		refreshToken: refreshToken,
		tenantDomain: tenantDomain,
	}, nil
}

// IsDataPlane returns true if these are data plane credentials.
func (c *Credentials) IsDataPlane() bool {
	return c.credType == CredentialTypeAPIKey
}

// IsControlPlane returns true if these are control plane credentials.
func (c *Credentials) IsControlPlane() bool {
	return c.credType == CredentialTypeToken
}

// GetAPIKey returns the API key. Errors if not data plane credentials.
func (c *Credentials) GetAPIKey() (string, error) {
	if !c.IsDataPlane() {
		return "", fmt.Errorf("%w: not a data plane credential", ErrInvalidCredentialType)
	}
	return c.apiKey, nil
}

// GetToken returns the access token. Errors if not control plane credentials.
func (c *Credentials) GetToken() (string, error) {
	if !c.IsControlPlane() {
		return "", fmt.Errorf("%w: not a control plane credential", ErrInvalidCredentialType)
	}
	return c.token, nil
}

// GetRefreshToken returns the refresh token. Errors if not control plane credentials.
func (c *Credentials) GetRefreshToken() (string, error) {
	if !c.IsControlPlane() {
		return "", fmt.Errorf("%w: not a control plane credential", ErrInvalidCredentialType)
	}
	return c.refreshToken, nil
}

// GetTenantDomain returns the tenant domain. Returns error if empty.
func (c *Credentials) GetTenantDomain() (string, error) {
	if c.tenantDomain == "" {
		return "", fmt.Errorf("%w: tenant domain is required", ErrMissingCredentials)
	}
	return c.tenantDomain, nil
}
