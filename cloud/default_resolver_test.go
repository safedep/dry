package cloud

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/safedep/dry/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type memKeychain struct {
	mu      sync.Mutex
	secrets map[string]string
}

func newMemKeychain(secrets map[string]string) *memKeychain {
	if secrets == nil {
		secrets = map[string]string{}
	}
	return &memKeychain{secrets: secrets}
}

func (m *memKeychain) Get(_ context.Context, key string) (*keychain.Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.secrets[key]
	if !ok {
		return nil, keychain.ErrNotFound
	}
	return &keychain.Secret{Value: v}, nil
}

func (m *memKeychain) Set(_ context.Context, key string, s *keychain.Secret) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets[key] = s.Value
	return nil
}

func (m *memKeychain) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.secrets, key)
	return nil
}

func (m *memKeychain) Close() error { return nil }

func TestResolveProfile(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		env      string
		want     string
	}{
		{"explicit wins", "prod", "staging", "prod"},
		{"env when no explicit", "", "staging", "staging"},
		{"default when neither", "", "", DefaultProfile},
		{"whitespace is empty", "  ", " ", DefaultProfile},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(ProfileEnvVar, tc.env)
			assert.Equal(t, tc.want, ResolveProfile(tc.explicit))
		})
	}
}

func TestDefaultCredentialResolver(t *testing.T) {
	cases := []struct {
		name       string
		credType   CredentialType
		env        map[string]string
		secrets    map[string]string
		opts       []KeychainOption
		wantErr    error
		wantErrMsg []string
		wantSecret string
		wantTenant string
		wantSource CredentialSource
	}{
		{
			name:       "environment wins over keychain",
			credType:   CredentialTypeAPIKey,
			env:        map[string]string{apiKeyEnvVar: "sk-env", tenantEnvVar: "env.safedep.io"},
			secrets:    map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": "kc.safedep.io"},
			wantSecret: "sk-env",
			wantTenant: "env.safedep.io",
			wantSource: CredentialSourceEnvironment,
		},
		{
			name:     "API key in environment without tenant is incomplete",
			credType: CredentialTypeAPIKey,
			env:      map[string]string{apiKeyEnvVar: "sk-env"},
			secrets:  map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": "kc.safedep.io"},
			wantErr:  ErrIncompleteCredentials,
		},
		{
			name:     "tenant in environment without API key or workload identity is incomplete",
			credType: CredentialTypeAPIKey,
			env:      map[string]string{tenantEnvVar: "env.safedep.io"},
			secrets:  map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": "kc.safedep.io"},
			wantErr:  ErrIncompleteCredentials,
			wantErrMsg: []string{
				"SAFEDEP_TENANT_ID is set but SAFEDEP_API_KEY is not",
				"permissions: id-token: write",
				"set SAFEDEP_API_KEY",
			},
		},
		{
			name:     "only one ID token variable keeps the detail in the hint",
			credType: CredentialTypeAPIKey,
			env: map[string]string{
				tenantEnvVar:                         "env.safedep.io",
				githubActionsIDTokenRequestURLEnvVar: "https://example.test/token",
			},
			wantErr: ErrIncompleteCredentials,
			wantErrMsg: []string{
				"permissions: id-token: write",
				"ACTIONS_ID_TOKEN_REQUEST_URL is set but ACTIONS_ID_TOKEN_REQUEST_TOKEN is not",
			},
		},
		{
			name:     "API key set to empty with workload identity does not exchange",
			credType: CredentialTypeAPIKey,
			env: map[string]string{
				apiKeyEnvVar:                           "",
				tenantEnvVar:                           "env.safedep.io",
				githubActionsIDTokenRequestURLEnvVar:   "https://example.test/token",
				githubActionsIDTokenRequestTokenEnvVar: "request-token",
			},
			wantErr:    ErrIncompleteCredentials,
			wantErrMsg: []string{"SAFEDEP_API_KEY is set to an empty value"},
		},
		{
			name:     "API key in environment wins over workload identity",
			credType: CredentialTypeAPIKey,
			env: map[string]string{
				apiKeyEnvVar:                           "sk-env",
				tenantEnvVar:                           "env.safedep.io",
				githubActionsIDTokenRequestURLEnvVar:   "https://example.test/token",
				githubActionsIDTokenRequestTokenEnvVar: "request-token",
			},
			wantSecret: "sk-env",
			wantTenant: "env.safedep.io",
			wantSource: CredentialSourceEnvironment,
		},
		{
			name:     "tenant in environment with workload identity selects the exchange",
			credType: CredentialTypeAPIKey,
			env: map[string]string{
				tenantEnvVar:                           "env.safedep.io",
				githubActionsIDTokenRequestURLEnvVar:   "https://example.test/token",
				githubActionsIDTokenRequestTokenEnvVar: "request-token",
			},
			secrets:    map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": "kc.safedep.io"},
			wantTenant: "env.safedep.io",
			wantSource: CredentialSourceWorkloadIdentity,
		},
		{
			name:     "workload identity without tenant reads the keychain",
			credType: CredentialTypeAPIKey,
			env: map[string]string{
				githubActionsIDTokenRequestURLEnvVar:   "https://example.test/token",
				githubActionsIDTokenRequestTokenEnvVar: "request-token",
			},
			secrets:    map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": "kc.safedep.io"},
			wantSecret: "sk-kc",
			wantTenant: "kc.safedep.io",
			wantSource: CredentialSourceKeychain,
		},
		{
			name:       "keychain when environment is empty",
			credType:   CredentialTypeAPIKey,
			secrets:    map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": "kc.safedep.io"},
			wantSecret: "sk-kc",
			wantTenant: "kc.safedep.io",
			wantSource: CredentialSourceKeychain,
		},
		{
			name:     "API key in keychain without tenant is incomplete",
			credType: CredentialTypeAPIKey,
			secrets:  map[string]string{"default/api_key": "sk-kc"},
			wantErr:  ErrIncompleteCredentials,
		},
		{
			name:     "nothing stored is missing",
			credType: CredentialTypeAPIKey,
			wantErr:  ErrMissingCredentials,
		},
		{
			name:       "SAFEDEP_PROFILE selects the keychain profile",
			credType:   CredentialTypeAPIKey,
			env:        map[string]string{ProfileEnvVar: "staging"},
			secrets:    map[string]string{"staging/api_key": "sk-staging", "staging/tenant_domain": "staging.safedep.io"},
			wantSecret: "sk-staging",
			wantTenant: "staging.safedep.io",
			wantSource: CredentialSourceKeychain,
		},
		{
			name:       "WithProfile wins over SAFEDEP_PROFILE",
			credType:   CredentialTypeAPIKey,
			env:        map[string]string{ProfileEnvVar: "staging"},
			secrets:    map[string]string{"prod/api_key": "sk-prod", "prod/tenant_domain": "prod.safedep.io"},
			opts:       []KeychainOption{WithProfile("prod")},
			wantSecret: "sk-prod",
			wantTenant: "prod.safedep.io",
			wantSource: CredentialSourceKeychain,
		},
		{
			name:       "token ignores API key environment",
			credType:   CredentialTypeToken,
			env:        map[string]string{apiKeyEnvVar: "sk-env", tenantEnvVar: "env.safedep.io"},
			secrets:    map[string]string{"default/token": "tok", "default/refresh_token": "ref", "default/tenant_domain": "kc.safedep.io"},
			wantSecret: "tok",
			wantTenant: "kc.safedep.io",
			wantSource: CredentialSourceKeychain,
		},
		{
			name:     "API key in keychain with an empty tenant is incomplete",
			credType: CredentialTypeAPIKey,
			secrets:  map[string]string{"default/api_key": "sk-kc", "default/tenant_domain": ""},
			wantErr:  ErrIncompleteCredentials,
		},
		{
			name:     "token in keychain with an empty tenant is incomplete",
			credType: CredentialTypeToken,
			secrets:  map[string]string{"default/token": "tok", "default/tenant_domain": ""},
			wantErr:  ErrIncompleteCredentials,
		},
		{
			name:     "token in keychain without tenant is incomplete",
			credType: CredentialTypeToken,
			secrets:  map[string]string{"default/token": "tok"},
			wantErr:  ErrIncompleteCredentials,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A key absent from tc.env is unset. A key with "" is set to empty.
			for _, k := range []string{
				apiKeyEnvVar, tenantEnvVar, ProfileEnvVar,
				githubActionsIDTokenRequestURLEnvVar, githubActionsIDTokenRequestTokenEnvVar,
			} {
				setOrUnsetEnv(t, k, tc.env)
			}

			opts := append([]KeychainOption{WithKeychainHandle(newMemKeychain(tc.secrets))}, tc.opts...)
			r, err := NewDefaultCredentialResolver(tc.credType, opts...)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, r.Close()) })

			creds, err := r.Resolve()
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				if !errors.Is(tc.wantErr, ErrIncompleteCredentials) {
					assert.NotErrorIs(t, err, ErrIncompleteCredentials)
				}
				for _, msg := range tc.wantErrMsg {
					assert.ErrorContains(t, err, msg)
				}
				return
			}
			require.NoError(t, err)

			tenant, err := creds.GetTenantDomain()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTenant, tenant)
			assert.Equal(t, tc.wantSource, creds.Source())

			_, isWorkload := creds.Workload()
			assert.Equal(t, tc.wantSource == CredentialSourceWorkloadIdentity, isWorkload)
			if isWorkload {
				// TestDefaultCredentialResolver_WorkloadIdentity runs the exchange.
				return
			}

			var secret string
			if tc.credType == CredentialTypeToken {
				secret, err = creds.GetToken()
			} else {
				secret, err = creds.GetAPIKey()
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSecret, secret)
		})
	}
}

func TestDefaultCredentialResolver_WorkloadIdentity(t *testing.T) {
	tokens := newFakeGitHubTokenEndpoint(t)
	tokens.setGitHubActionsEnv(t)

	clock := &fakeClock{now: time.Unix(1_900_000_000, 0)}
	fake := &fakeWorkloadIdentityServer{clock: clock, life: time.Hour}
	t.Setenv(envDataPlaneAddr, startFakeWorkloadIdentityServer(t, fake))
	t.Setenv(insecureTransportEnvVar, "true")
	setOrUnsetEnv(t, apiKeyEnvVar, nil)
	t.Setenv(tenantEnvVar, "acme.safedep.io")

	r, err := NewDefaultCredentialResolver(CredentialTypeAPIKey, WithKeychainHandle(newMemKeychain(nil)))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, r.Close()) })

	creds, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, CredentialSourceWorkloadIdentity, creds.Source())

	again, err := r.Resolve()
	require.NoError(t, err)
	assert.Same(t, creds, again, "each Resolve returns the same workload credentials")

	_, err = creds.GetAPIKey()
	require.ErrorIs(t, err, ErrWorkloadCredential)
	assert.ErrorIs(t, err, ErrInvalidCredentialType)

	w, ok := creds.Workload()
	require.True(t, ok)
	w.now = clock.Now

	client, err := NewDataPlaneClient("test", creds)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, client.Close()) })
	assert.Equal(t, 0, tokens.servedCount(), "the first exchange runs on the first RPC")

	call := func() {
		err := client.Connection().Invoke(context.Background(), "/test.v1.TestService/Call", &emptypb.Empty{}, &emptypb.Empty{})
		require.Equal(t, codes.Unimplemented, status.Code(err))
	}

	call()

	// The connection outlives the first credential.
	clock.Advance(time.Hour)
	call()

	require.Len(t, fake.calls, 2)
	assert.Equal(t, []string{"wc-1"}, fake.calls[0].Get("authorization"))
	assert.Equal(t, []string{"wc-2"}, fake.calls[1].Get("authorization"))
	for _, md := range fake.calls {
		assert.Equal(t, []string{"acme.safedep.io"}, md.Get(tenantIDHeader))
	}
	assert.Equal(t, []string{"id-token-1", "id-token-2"}, fake.subjectTokens())

	require.NoError(t, w.Revoke(context.Background()))
	fresh, err := r.Resolve()
	require.NoError(t, err)
	assert.NotSame(t, creds, fresh, "a Resolve after Revoke returns new workload credentials")
}

// setOrUnsetEnv sets key to env[key], or unsets it when env has no key.
// t.Setenv first, so that the test restores the variable.
func setOrUnsetEnv(t *testing.T, key string, env map[string]string) {
	v, ok := env[key]
	t.Setenv(key, v)
	if !ok {
		require.NoError(t, os.Unsetenv(key))
	}
}

func TestDefaultCredentialResolver_UnsupportedType(t *testing.T) {
	for _, ct := range []CredentialType{CredentialTypeUnspecified, CredentialType(99)} {
		_, err := NewDefaultCredentialResolver(ct, WithKeychainHandle(newMemKeychain(nil)))
		assert.ErrorIs(t, err, ErrInvalidCredentialType)
	}
}

func TestDefaultCredentialResolver_NoKeychain(t *testing.T) {
	keychainErr := errors.New("no secret service")
	r := &defaultCredentialResolver{credType: CredentialTypeAPIKey, keychainErr: keychainErr}

	t.Run("environment still works", func(t *testing.T) {
		t.Setenv(apiKeyEnvVar, "sk-env")
		t.Setenv(tenantEnvVar, "env.safedep.io")

		creds, err := r.Resolve()
		require.NoError(t, err)
		assert.Equal(t, CredentialSourceEnvironment, creds.Source())
	})

	t.Run("empty environment reports the keychain error", func(t *testing.T) {
		t.Setenv(apiKeyEnvVar, "")
		t.Setenv(tenantEnvVar, "")

		_, err := r.Resolve()
		require.ErrorIs(t, err, ErrMissingCredentials)
		assert.ErrorIs(t, err, keychainErr)
	})

	assert.NoError(t, r.Close())
}
