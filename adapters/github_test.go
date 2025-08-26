package adapters

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestRSAKey generates a test RSA private key in PEM format
func generateTestRSAKey(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return privateKeyPEM
}

func TestGitHubAppClient_CreateAppAuthenticationJWT(t *testing.T) {
	validKey := generateTestRSAKey(t)
	invalidPEM := []byte("invalid pem data")

	tests := []struct {
		name          string
		privateKey    []byte
		appID         string
		expectedError string
		validateToken bool
	}{
		{
			name:          "valid key and app ID",
			privateKey:    validKey,
			appID:         "12345",
			validateToken: true,
		},
		{
			name:          "empty private key",
			privateKey:    []byte{},
			appID:         "12345",
			expectedError: "app authentication private key is not configured",
		},
		{
			name:          "nil private key",
			privateKey:    nil,
			appID:         "12345",
			expectedError: "app authentication private key is not configured",
		},
		{
			name:          "invalid PEM format",
			privateKey:    invalidPEM,
			appID:         "12345",
			expectedError: "failed to parse app authentication private key",
		},
		{
			name:          "empty app ID",
			privateKey:    validKey,
			appID:         "",
			expectedError: "app ID is not configured",
		},
		{
			name:          "numeric app ID",
			privateKey:    validKey,
			appID:         "67890",
			validateToken: true,
		},
		{
			name:          "app ID with special characters",
			privateKey:    validKey,
			appID:         "app-123_test",
			validateToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &GitHubAppClient{
				Config: GitHubAppClientConfig{
					AppAuthenticationPrivateKey: tt.privateKey,
					AppID:                       tt.appID,
					EnableJWTTokenCache:         false, // Disable caching for table-driven tests to ensure consistent behavior
				},
			}

			token, err := client.CreateAppAuthenticationJWT()

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, token)

				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, token)

			if tt.validateToken {
				// Parse and validate the JWT structure without time validation first
				parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
					// Verify signing method
					if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
						t.Errorf("Unexpected signing method: %v", token.Header["alg"])
					}

					// Parse the public key from the private key for verification
					key, err := jwt.ParseRSAPrivateKeyFromPEM(tt.privateKey)
					require.NoError(t, err)
					return &key.PublicKey, nil
				}, jwt.WithoutClaimsValidation())

				assert.NoError(t, err)
				assert.True(t, parsedToken.Valid)

				// Verify claims
				claims, ok := parsedToken.Claims.(jwt.MapClaims)
				assert.True(t, ok)

				// Check issuer claim
				iss, ok := claims["iss"]
				assert.True(t, ok)
				assert.Equal(t, tt.appID, iss)

				// Check issued at time (should be ~1 minute ago)
				iat, ok := claims["iat"]
				assert.True(t, ok)

				iatTime := time.Unix(int64(iat.(float64)), 0)
				now := time.Now()

				// Should be approximately 1 minute in the past
				timeDiff := now.Sub(iatTime)
				assert.True(t, timeDiff >= 30*time.Second) // Should be at least 30 seconds ago
				assert.True(t, timeDiff <= 90*time.Second) // Should be at most 90 seconds ago

				// Check expiration time (should be ~10 minutes from now)
				exp, ok := claims["exp"]
				assert.True(t, ok)
				expTime := time.Unix(int64(exp.(float64)), 0)

				// Should be approximately 10 minutes in the future
				expDiff := expTime.Sub(now)
				assert.True(t, expDiff >= 9*time.Minute)  // Should be at least 9 minutes in the future
				assert.True(t, expDiff <= 11*time.Minute) // Should be at most 11 minutes in the future
			}
		})
	}
}

func TestGitHubAppClient_CreateAppAuthenticationJWT_WithEnvironment(t *testing.T) {
	validKey := generateTestRSAKey(t)

	// Test that environment variables don't interfere with the method
	// since it uses the config directly, not environment variables
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "invalid-key-from-env")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_FILE", "/nonexistent/path")
	t.Setenv("GITHUB_TOKEN", "token-from-env")
	t.Setenv("GITHUB_CLIENT_ID", "client-id-from-env")

	client := &GitHubAppClient{
		Config: GitHubAppClientConfig{
			AppAuthenticationPrivateKey: validKey,
			AppID:                       "test-app",
			EnableJWTTokenCache:         false, // Disable caching to test core functionality
		},
	}

	token, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify the JWT can be parsed and validated
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			t.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		key, err := jwt.ParseRSAPrivateKeyFromPEM(validKey)
		require.NoError(t, err)
		return &key.PublicKey, nil
	}, jwt.WithoutClaimsValidation())

	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	// Verify issuer claim is correct
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	iss, ok := claims["iss"]
	assert.True(t, ok)
	assert.Equal(t, "test-app", iss)
}

func TestGitHubAppClient_CreateAppAuthenticationJWT_MissingAppID(t *testing.T) {
	validKey := generateTestRSAKey(t)

	client := &GitHubAppClient{
		Config: GitHubAppClientConfig{
			AppAuthenticationPrivateKey: validKey,
			EnableJWTTokenCache:         false,
			// AppID is intentionally not set
		},
	}

	token, err := client.CreateAppAuthenticationJWT()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "app ID is not configured")
	assert.Empty(t, token)
}

func TestGitHubAppClient_CreateAppAuthenticationJWT_CachingEnabled(t *testing.T) {
	validKey := generateTestRSAKey(t)

	client := &GitHubAppClient{
		Config: GitHubAppClientConfig{
			AppAuthenticationPrivateKey: validKey,
			AppID:                       "test-app",
			EnableJWTTokenCache:         true,
		},
	}

	// First call should generate a new token
	token1, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token1)

	// Second call should return the cached token (same token)
	token2, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token2)
	assert.Equal(t, token1, token2)

	// Verify the cache fields are set
	assert.NotEmpty(t, client.cachedToken)
	assert.True(t, client.cachedTokenExp.After(time.Now()))
	assert.True(t, client.cachedTokenExp.Before(time.Now().Add(10*time.Minute)))
}

func TestGitHubAppClient_CreateAppAuthenticationJWT_CachingDisabled(t *testing.T) {
	validKey := generateTestRSAKey(t)

	client := &GitHubAppClient{
		Config: GitHubAppClientConfig{
			AppAuthenticationPrivateKey: validKey,
			AppID:                       "test-app",
			EnableJWTTokenCache:         false,
		},
	}

	// First call should generate a token
	token1, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token1)

	// Second call should also generate a token successfully
	token2, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token2)

	// Most importantly, verify the cache fields are never set when caching is disabled
	assert.Empty(t, client.cachedToken)
	assert.True(t, client.cachedTokenExp.IsZero())
}

func TestGitHubAppClient_CreateAppAuthenticationJWT_CacheExpiration(t *testing.T) {
	validKey := generateTestRSAKey(t)

	client := &GitHubAppClient{
		Config: GitHubAppClientConfig{
			AppAuthenticationPrivateKey: validKey,
			AppID:                       "test-app",
			EnableJWTTokenCache:         true,
		},
	}

	// Generate first token
	token1, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token1)

	// Verify token is cached
	assert.Equal(t, token1, client.cachedToken)
	originalExpiry := client.cachedTokenExp

	// Manually expire the cached token by setting expiry to past
	client.m.Lock()
	client.cachedTokenExp = time.Now().Add(-1 * time.Minute)
	client.m.Unlock()

	// Next call should generate a new token since the cached one is expired
	token2, err := client.CreateAppAuthenticationJWT()
	assert.NoError(t, err)
	assert.NotEmpty(t, token2)

	// Verify cache was updated with new token and new expiry
	assert.Equal(t, token2, client.cachedToken)
	assert.True(t, client.cachedTokenExp.After(time.Now()))
	assert.True(t, client.cachedTokenExp.After(originalExpiry)) // New expiry should be later than original
}

func TestGitHubAppClient_CreateAppAuthenticationJWT_ConcurrentAccess(t *testing.T) {
	validKey := generateTestRSAKey(t)

	client := &GitHubAppClient{
		Config: GitHubAppClientConfig{
			AppAuthenticationPrivateKey: validKey,
			AppID:                       "test-app",
			EnableJWTTokenCache:         true,
		},
	}

	const numGoroutines = 10
	tokens := make([]string, numGoroutines)
	var wg sync.WaitGroup

	// Launch multiple goroutines that try to get JWT tokens concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			token, err := client.CreateAppAuthenticationJWT()
			assert.NoError(t, err)
			assert.NotEmpty(t, token)
			tokens[index] = token
		}(i)
	}

	wg.Wait()

	// All tokens should be identical due to caching
	firstToken := tokens[0]
	assert.NotEmpty(t, firstToken)

	for i, token := range tokens {
		assert.Equal(t, firstToken, token, "Token at index %d should match first token", i)
	}

	// Verify cache state is consistent
	assert.Equal(t, firstToken, client.cachedToken)
	assert.True(t, client.cachedTokenExp.After(time.Now()))
}
