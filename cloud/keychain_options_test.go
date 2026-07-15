package cloud

import (
	"os"
	"testing"

	"github.com/safedep/dry/keychain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildKeychainConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg := buildKeychainConfig(nil)
		assert.Equal(t, DefaultAppName, cfg.appName)
		assert.Equal(t, DefaultProfile, cfg.profile)
		assert.Nil(t, cfg.keychain)
		assert.False(t, cfg.insecureFileFallback)
		assert.Empty(t, cfg.insecureFilePath)
	})
}

func TestBuildKeychainConfigWithOptions(t *testing.T) {
	t.Run("with app name", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{WithAppName("custom-app")})
		assert.Equal(t, "custom-app", cfg.appName)
		assert.Equal(t, DefaultProfile, cfg.profile)
	})

	t.Run("with profile", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{WithProfile("staging")})
		assert.Equal(t, DefaultAppName, cfg.appName)
		assert.Equal(t, "staging", cfg.profile)
	})

	t.Run("empty profile normalizes to default", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{WithProfile("")})
		assert.Equal(t, DefaultProfile, cfg.profile)
	})

	t.Run("whitespace profile normalizes to default", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{WithProfile("  ")})
		assert.Equal(t, DefaultProfile, cfg.profile)
	})

	t.Run("with keychain handle", func(t *testing.T) {
		kc, err := keychain.New(keychain.Config{
			AppName:              "test-app",
			InsecureFileFallback: true,
			FilePath:             t.TempDir() + "/creds.json",
		})
		require.NoError(t, err)
		defer func() { require.NoError(t, kc.Close()) }()

		cfg := buildKeychainConfig([]KeychainOption{WithKeychainHandle(kc)})
		assert.NotNil(t, cfg.keychain)
	})

	t.Run("with insecure file fallback", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{WithInsecureFileFallback()})
		assert.True(t, cfg.insecureFileFallback)
		assert.Empty(t, cfg.insecureFilePath)
	})

	t.Run("with insecure file fallback path implies fallback", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{
			WithInsecureFileFallbackPath("/tmp/creds.json"),
		})
		assert.True(t, cfg.insecureFileFallback)
		assert.Equal(t, "/tmp/creds.json", cfg.insecureFilePath)
	})

	t.Run("with insecure file store bypasses keychain", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{
			WithInsecureFileStore("/tmp/creds.json", 0o644),
		})
		assert.True(t, cfg.insecureFileOnly)
		assert.Equal(t, "/tmp/creds.json", cfg.insecureFilePath)
		assert.Equal(t, os.FileMode(0o644), cfg.insecureFileMode)
	})
}

func TestKeyForField(t *testing.T) {
	t.Run("default profile", func(t *testing.T) {
		cfg := buildKeychainConfig(nil)
		assert.Equal(t, "default/api_key", cfg.keyForField("api_key"))
		assert.Equal(t, "default/token", cfg.keyForField("token"))
	})

	t.Run("custom profile", func(t *testing.T) {
		cfg := buildKeychainConfig([]KeychainOption{WithProfile("staging")})
		assert.Equal(t, "staging/api_key", cfg.keyForField("api_key"))
	})
}
