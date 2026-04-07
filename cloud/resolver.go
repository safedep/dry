package cloud

import (
	"fmt"
	"os"
)

// CredentialResolver resolves SafeDep Cloud credentials.
type CredentialResolver interface {
	Resolve() (*Credentials, error)
}

// envCredentialResolver resolves credentials from environment variables.
type envCredentialResolver struct{}

// NewEnvCredentialResolver creates a resolver that reads from
// SAFEDEP_API_KEY and SAFEDEP_TENANT_ID environment variables.
func NewEnvCredentialResolver() (CredentialResolver, error) {
	return &envCredentialResolver{}, nil
}

func (r *envCredentialResolver) Resolve() (*Credentials, error) {
	return NewAPIKeyCredential(os.Getenv("SAFEDEP_API_KEY"), os.Getenv("SAFEDEP_TENANT_ID"))
}

// chainCredentialResolver tries resolvers in order, returning the first success.
type chainCredentialResolver struct {
	resolvers []CredentialResolver
}

// NewChainCredentialResolver tries resolvers in order, returning the
// first successful result.
func NewChainCredentialResolver(resolvers ...CredentialResolver) CredentialResolver {
	return &chainCredentialResolver{resolvers: resolvers}
}

func (r *chainCredentialResolver) Resolve() (*Credentials, error) {
	var lastErr error
	for _, resolver := range r.resolvers {
		creds, err := resolver.Resolve()
		if err == nil {
			return creds, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all credential resolvers failed: %w", lastErr)
}
