package cloud

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/safedep/dry/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:     "tenant in environment without API key is incomplete",
			credType: CredentialTypeAPIKey,
			env:      map[string]string{tenantEnvVar: "env.safedep.io"},
			wantErr:  ErrIncompleteCredentials,
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
			name:     "token in keychain without tenant is incomplete",
			credType: CredentialTypeToken,
			secrets:  map[string]string{"default/token": "tok"},
			wantErr:  ErrIncompleteCredentials,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{apiKeyEnvVar, tenantEnvVar, ProfileEnvVar} {
				t.Setenv(k, tc.env[k])
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
				return
			}
			require.NoError(t, err)

			var secret string
			if tc.credType == CredentialTypeToken {
				secret, err = creds.GetToken()
			} else {
				secret, err = creds.GetAPIKey()
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSecret, secret)

			tenant, err := creds.GetTenantDomain()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTenant, tenant)
			assert.Equal(t, tc.wantSource, creds.Source())
		})
	}
}

func TestDefaultCredentialResolver_UnspecifiedType(t *testing.T) {
	_, err := NewDefaultCredentialResolver(CredentialTypeUnspecified)
	assert.ErrorIs(t, err, ErrInvalidCredentialType)
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
