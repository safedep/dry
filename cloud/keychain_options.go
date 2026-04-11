package cloud

import "github.com/safedep/dry/keychain"

const (
	// DefaultAppName is the shared keychain application name used by all SafeDep tools.
	DefaultAppName = "safedep"

	// DefaultProfile is the default credential profile name.
	DefaultProfile = "default"
)

// KeychainOption configures keychain-based credential store and resolver.
type KeychainOption func(*keychainConfig)

type keychainConfig struct {
	appName                  string
	profile                  string
	keychain                 keychain.Keychain
	insecureFileFallback     bool
	insecureFileFallbackPath string
}

// WithAppName overrides the default application name for the keychain.
func WithAppName(name string) KeychainOption {
	return func(c *keychainConfig) {
		c.appName = name
	}
}

// WithProfile selects a named credential profile. Defaults to "default".
func WithProfile(profile string) KeychainOption {
	return func(c *keychainConfig) {
		c.profile = profile
	}
}

// WithKeychainHandle injects an existing keychain instance.
// The caller owns the lifecycle (Close) when this option is used.
func WithKeychainHandle(kc keychain.Keychain) KeychainOption {
	return func(c *keychainConfig) {
		c.keychain = kc
	}
}

// WithInsecureFileFallback enables plaintext file storage
// when the OS keychain is unavailable.
func WithInsecureFileFallback() KeychainOption {
	return func(c *keychainConfig) {
		c.insecureFileFallback = true
	}
}

// WithInsecureFileFallbackPath sets a custom file path for the insecure
// file fallback. Implies WithInsecureFileFallback.
func WithInsecureFileFallbackPath(path string) KeychainOption {
	return func(c *keychainConfig) {
		c.insecureFileFallback = true
		c.insecureFileFallbackPath = path
	}
}

func buildKeychainConfig(opts []KeychainOption) keychainConfig {
	cfg := keychainConfig{
		appName: DefaultAppName,
		profile: DefaultProfile,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (c *keychainConfig) keyForField(field string) string {
	return c.profile + "/" + field
}
