package cloud

import "fmt"

// CredentialType identifies the authentication plane.
type CredentialType int

const (
	CredentialTypeUnspecified CredentialType = iota
	CredentialTypeAPIKey                     // Data plane (api.safedep.io)
	CredentialTypeToken                      // Control plane (cloud.safedep.io)
)

// CredentialSource identifies where a resolver found credentials.
type CredentialSource int

const (
	CredentialSourceUnspecified CredentialSource = iota
	CredentialSourceEnvironment
	CredentialSourceKeychain

	// CredentialSourceWorkloadIdentity marks a workload credential that the
	// resolver gets from an exchange of the workload identity token, such as
	// a GitHub Actions OIDC token.
	CredentialSourceWorkloadIdentity
)

func (s CredentialSource) String() string {
	switch s {
	case CredentialSourceEnvironment:
		return "environment"
	case CredentialSourceKeychain:
		return "keychain"
	case CredentialSourceWorkloadIdentity:
		return "workload identity"
	default:
		return "unspecified"
	}
}

// Credentials holds SafeDep Cloud authentication details.
// Fields are private. Use constructors to create, getters to access.
type Credentials struct {
	credType     CredentialType
	apiKey       string
	token        string
	refreshToken string
	tenantDomain string
	source       CredentialSource
	workload     *RefreshingWorkloadCredential
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

// NewWorkloadIdentityCredential creates data plane credentials from a
// workload credential that refreshes itself. The tenant is the tenant of w.
func NewWorkloadIdentityCredential(w *RefreshingWorkloadCredential) (*Credentials, error) {
	if w == nil {
		return nil, fmt.Errorf("%w: workload credential is required", ErrMissingCredentials)
	}
	if w.TenantDomain() == "" {
		return nil, fmt.Errorf("%w: tenant domain is required", ErrMissingCredentials)
	}
	return &Credentials{
		credType:     CredentialTypeAPIKey,
		tenantDomain: w.TenantDomain(),
		workload:     w,
	}, nil
}

// Workload returns the workload credential of workload identity credentials.
// Pass it to grpc.WithPerRPCCredentials on a connection that the caller
// dials, and call its Revoke when the job ends. ok is false for other
// credentials.
func (c *Credentials) Workload() (w *RefreshingWorkloadCredential, ok bool) {
	return c.workload, c.workload != nil
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
//
// Workload identity credentials hold no fixed API key, so GetAPIKey returns
// ErrWorkloadCredential for them and makes no network call. Use Workload.
func (c *Credentials) GetAPIKey() (string, error) {
	if !c.IsDataPlane() {
		return "", fmt.Errorf("%w: not a data plane credential", ErrInvalidCredentialType)
	}
	if c.workload != nil {
		return "", ErrWorkloadCredential
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

// Source returns where a resolver found the credentials. Credentials built
// directly with a constructor return CredentialSourceUnspecified.
func (c *Credentials) Source() CredentialSource {
	return c.source
}
