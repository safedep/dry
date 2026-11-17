package cloud

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ProfileEnvVar names the environment variable that selects the credential
// profile when the caller has no explicit profile.
const ProfileEnvVar = "SAFEDEP_PROFILE"

var errNoEnvCredentials = errors.New("cloud: no credentials in the environment")

// ResolveProfile returns the credential profile that SafeDep tools use:
// explicit (usually a --profile flag value) when it is set, else
// SAFEDEP_PROFILE, else DefaultProfile.
func ResolveProfile(explicit string) string {
	for _, p := range []string{explicit, os.Getenv(ProfileEnvVar)} {
		if p = strings.TrimSpace(p); p != "" {
			return p
		}
	}
	return DefaultProfile
}

type defaultCredentialResolver struct {
	credType    CredentialType
	keychain    CloseableCredentialResolver
	keychainErr error

	// workload holds the workload identity credentials of the first Resolve
	// that selected them. A later Resolve for the same tenant returns them, so
	// that one process holds one workload credential, not one per Resolve.
	mu       sync.Mutex
	workload *Credentials
}

// NewDefaultCredentialResolver returns the resolver chain that SafeDep tools
// share, so that one sign-in serves every tool.
//
// For CredentialTypeAPIKey it reads SAFEDEP_API_KEY and SAFEDEP_TENANT_ID
// first. Then it reads the keychain profile from ResolveProfile. A WithProfile
// option replaces that profile.
//
// SAFEDEP_TENANT_ID with no SAFEDEP_API_KEY selects the workload identity of
// the environment, such as a GitHub Actions job with "id-token: write". The
// credentials then exchange the workload identity token for a workload
// credential, with the source CredentialSourceWorkloadIdentity. Each Resolve
// returns the same workload identity credentials. An explicit SAFEDEP_API_KEY
// always takes precedence, and a SAFEDEP_API_KEY that is set to an empty value
// is ErrIncompleteCredentials, so that a missing secret does not silently
// select the exchange.
//
// A source that holds half a credential stops the chain with
// ErrIncompleteCredentials, so a typo in one variable does not silently
// select another tenant from the keychain.
//
// A keychain that cannot open does not fail construction. The environment
// still works on a machine with no keychain, and Resolve reports the keychain
// error only when the environment has no credentials.
func NewDefaultCredentialResolver(credType CredentialType, opts ...KeychainOption) (CloseableCredentialResolver, error) {
	if credType != CredentialTypeAPIKey && credType != CredentialTypeToken {
		return nil, fmt.Errorf("%w: unsupported credential type %d", ErrInvalidCredentialType, credType)
	}

	opts = append([]KeychainOption{WithProfile(ResolveProfile(""))}, opts...)
	kc, err := NewKeychainCredentialResolver(credType, opts...)

	return &defaultCredentialResolver{
		credType:    credType,
		keychain:    kc,
		keychainErr: err,
	}, nil
}

func (r *defaultCredentialResolver) Resolve() (*Credentials, error) {
	if r.credType == CredentialTypeAPIKey {
		creds, err := r.envAPIKeyCredential()
		if !errors.Is(err, errNoEnvCredentials) {
			return creds, err
		}
	}

	if r.keychain == nil {
		return nil, fmt.Errorf("%w: none in the environment, and the keychain is unavailable: %w", ErrMissingCredentials, r.keychainErr)
	}

	return r.keychain.Resolve()
}

func (r *defaultCredentialResolver) Close() error {
	if r.keychain == nil {
		return nil
	}
	return r.keychain.Close()
}

// envAPIKeyCredential reads the data plane credentials from the environment.
// An API key needs a tenant. A tenant with no API key selects the workload
// identity of the environment, and with none the pair is incomplete.
func (r *defaultCredentialResolver) envAPIKeyCredential() (*Credentials, error) {
	apiKey, apiKeySet := os.LookupEnv(apiKeyEnvVar)
	tenant := os.Getenv(tenantEnvVar)

	switch {
	case apiKey == "" && tenant == "":
		return nil, errNoEnvCredentials
	case tenant == "":
		return nil, fmt.Errorf("%w: %s is set but %s is not", ErrIncompleteCredentials, apiKeyEnvVar, tenantEnvVar)
	case apiKey == "":
		return r.workloadCredential(tenant, apiKeySet)
	}

	creds, err := NewAPIKeyCredential(apiKey, tenant)
	if err != nil {
		return nil, err
	}
	creds.source = CredentialSourceEnvironment
	return creds, nil
}

// workloadCredential returns credentials that exchange the workload identity
// token of the environment for tenant. It makes no network call: the first
// exchange runs on the first RPC.
func (r *defaultCredentialResolver) workloadCredential(tenant string, apiKeySet bool) (*Credentials, error) {
	source, err := DetectWorkloadIdentitySource()
	if errors.Is(err, ErrNoWorkloadIdentity) {
		return nil, fmt.Errorf("%w: %s is set but %s is not. In GitHub Actions, give the job "+
			"\"permissions: id-token: write\" to use workload identity, or set %s: %w",
			ErrIncompleteCredentials, tenantEnvVar, apiKeyEnvVar, apiKeyEnvVar, err)
	}
	if err != nil {
		return nil, err
	}

	// A workflow that maps a missing secret to SAFEDEP_API_KEY sets it to an
	// empty value. Exchanging then would hide the missing secret.
	if apiKeySet {
		return nil, fmt.Errorf("%w: %s is set to an empty value. Unset it to use the workload identity "+
			"of the environment, or set it to an API key", ErrIncompleteCredentials, apiKeyEnvVar)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// A caller revokes the credential at the end of its work. A later Resolve
	// in the same process gets a new credential, not the revoked one.
	if r.workload != nil && r.workload.tenantDomain == tenant && !r.workload.workload.isRevoked() {
		return r.workload, nil
	}

	exchange := &WorkloadExchangeClient{dial: dialWorkloadExchange}
	w := NewRefreshingWorkloadCredential(exchange, source, WorkloadExchangeRequest{TenantDomain: tenant})

	creds, err := NewWorkloadIdentityCredential(w)
	if err != nil {
		return nil, err
	}
	creds.source = CredentialSourceWorkloadIdentity
	r.workload = creds
	return creds, nil
}

// dialWorkloadExchange returns a keyless connection to the data plane for
// the WorkloadIdentityService. The exchange client closes it after one call.
func dialWorkloadExchange() (*grpc.ClientConn, error) {
	host, port := parseAddr(envOrDefault(envDataPlaneAddr, defaultDataPlaneAddr))

	transport := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	if insecureTransport() {
		transport = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(net.JoinHostPort(host, port), grpc.WithTransportCredentials(transport))
	if err != nil {
		return nil, fmt.Errorf("cloud: failed to create workload identity connection: %w", err)
	}
	return conn, nil
}
