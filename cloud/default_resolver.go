package cloud

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

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
// credential, with the source CredentialSourceWorkloadIdentity. An explicit
// SAFEDEP_API_KEY always takes precedence.
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
		creds, err := envAPIKeyCredential()
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
func envAPIKeyCredential() (*Credentials, error) {
	apiKey, tenant := os.Getenv(apiKeyEnvVar), os.Getenv(tenantEnvVar)

	switch {
	case apiKey == "" && tenant == "":
		return nil, errNoEnvCredentials
	case tenant == "":
		return nil, fmt.Errorf("%w: %s is set but %s is not", ErrIncompleteCredentials, apiKeyEnvVar, tenantEnvVar)
	case apiKey == "":
		return envWorkloadCredential(tenant)
	}

	creds, err := NewAPIKeyCredential(apiKey, tenant)
	if err != nil {
		return nil, err
	}
	creds.source = CredentialSourceEnvironment
	return creds, nil
}

// envWorkloadCredential returns credentials that exchange the workload
// identity token of the environment for tenant. It makes no network call: the
// first exchange runs on the first RPC.
func envWorkloadCredential(tenant string) (*Credentials, error) {
	source, err := DetectWorkloadIdentitySource()
	if errors.Is(err, ErrNoWorkloadIdentity) {
		return nil, fmt.Errorf("%w: %s is set but %s is not. In GitHub Actions, give the job "+
			"\"permissions: id-token: write\" to use workload identity, or set %s",
			ErrIncompleteCredentials, tenantEnvVar, apiKeyEnvVar, apiKeyEnvVar)
	}
	if err != nil {
		return nil, err
	}

	conn, err := newWorkloadExchangeConn()
	if err != nil {
		return nil, err
	}

	w := NewRefreshingWorkloadCredential(NewWorkloadExchangeClient(conn), source,
		WorkloadExchangeRequest{TenantDomain: tenant})

	creds, err := NewWorkloadIdentityCredential(w)
	if err != nil {
		return nil, err
	}
	creds.source = CredentialSourceWorkloadIdentity
	return creds, nil
}

// newWorkloadExchangeConn returns a keyless connection to the data plane for
// the WorkloadIdentityService. It connects on the first call and stays open
// for the life of the process, idle between exchanges.
func newWorkloadExchangeConn() (*grpc.ClientConn, error) {
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
