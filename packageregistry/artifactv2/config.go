package artifactv2

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/safedep/dry/storage"
)

// Option is a functional option for configuring adapters
type Option func(*adapterConfig) error

// adapterConfig holds configuration for an adapter
type adapterConfig struct {
	storage          storage.Storage
	storageManager   StorageManager
	metadataStore    MetadataStore
	httpClient       *http.Client

	// Feature flags
	cacheEnabled     bool
	persistArtifacts bool
	metadataEnabled  bool

	// Behavior settings
	fetchTimeout  time.Duration
	retryAttempts int
	retryDelay    time.Duration

	// Storage settings
	storagePrefix      string
	tempDir            string
	artifactIDStrategy ArtifactIDStrategy
	includeContentHash bool
}

// WithStorage configures a custom storage backend
func WithStorage(s storage.Storage) Option {
	return func(c *adapterConfig) error {
		if s == nil {
			return fmt.Errorf("storage cannot be nil")
		}
		c.storage = s
		return nil
	}
}

// WithStorageManager configures a custom storage manager
func WithStorageManager(sm StorageManager) Option {
	return func(c *adapterConfig) error {
		if sm == nil {
			return fmt.Errorf("storage manager cannot be nil")
		}
		c.storageManager = sm
		return nil
	}
}

// WithMetadataStore configures a custom metadata store
func WithMetadataStore(ms MetadataStore) Option {
	return func(c *adapterConfig) error {
		if ms == nil {
			return fmt.Errorf("metadata store cannot be nil")
		}
		c.metadataStore = ms
		return nil
	}
}

// WithCacheEnabled enables or disables artifact caching
func WithCacheEnabled(enabled bool) Option {
	return func(c *adapterConfig) error {
		c.cacheEnabled = enabled
		return nil
	}
}

// WithPersistArtifacts controls whether artifacts are persisted after use
func WithPersistArtifacts(persist bool) Option {
	return func(c *adapterConfig) error {
		c.persistArtifacts = persist
		return nil
	}
}

// WithMetadataEnabled enables or disables metadata storage
func WithMetadataEnabled(enabled bool) Option {
	return func(c *adapterConfig) error {
		c.metadataEnabled = enabled
		return nil
	}
}

// WithFetchTimeout sets the download timeout
func WithFetchTimeout(timeout time.Duration) Option {
	return func(c *adapterConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		c.fetchTimeout = timeout
		return nil
	}
}

// WithRetry configures retry behavior for failed downloads
func WithRetry(attempts int, delay time.Duration) Option {
	return func(c *adapterConfig) error {
		if attempts < 0 {
			return fmt.Errorf("retry attempts must be non-negative")
		}
		if delay < 0 {
			return fmt.Errorf("retry delay must be non-negative")
		}
		c.retryAttempts = attempts
		c.retryDelay = delay
		return nil
	}
}

// WithHTTPClient provides a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *adapterConfig) error {
		if client == nil {
			return fmt.Errorf("HTTP client cannot be nil")
		}
		c.httpClient = client
		return nil
	}
}

// WithStoragePrefix sets a prefix for all storage keys
func WithStoragePrefix(prefix string) Option {
	return func(c *adapterConfig) error {
		c.storagePrefix = prefix
		return nil
	}
}

// WithTempDir sets the temporary directory for artifact operations
func WithTempDir(dir string) Option {
	return func(c *adapterConfig) error {
		if dir == "" {
			return fmt.Errorf("temp directory cannot be empty")
		}
		c.tempDir = dir
		return nil
	}
}

// WithArtifactIDStrategy sets the artifact ID generation strategy
func WithArtifactIDStrategy(strategy ArtifactIDStrategy) Option {
	return func(c *adapterConfig) error {
		if strategy < ArtifactIDStrategyConvention || strategy > ArtifactIDStrategyHybrid {
			return fmt.Errorf("invalid artifact ID strategy: %d", strategy)
		}
		c.artifactIDStrategy = strategy
		return nil
	}
}

// WithContentHashInID enables content hash inclusion in artifact ID
// This is useful for verifying artifact integrity even with Convention strategy
func WithContentHashInID(enabled bool) Option {
	return func(c *adapterConfig) error {
		c.includeContentHash = enabled
		return nil
	}
}

// defaultConfig returns the default configuration
func defaultConfig() *adapterConfig {
	return &adapterConfig{
		cacheEnabled:       true,
		persistArtifacts:   true,
		metadataEnabled:    true,
		fetchTimeout:       5 * time.Minute,
		retryAttempts:      3,
		retryDelay:         time.Second,
		artifactIDStrategy: ArtifactIDStrategyConvention, // Default to convention-based IDs
		includeContentHash: false,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		tempDir: os.TempDir(),
	}
}

// applyOptions applies options to a config and returns the updated config
func applyOptions(opts ...Option) (*adapterConfig, error) {
	config := defaultConfig()

	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return config, nil
}

// ensureDefaults ensures all required components are initialized
func (c *adapterConfig) ensureDefaults() error {
	// Create default storage if not provided
	if c.storage == nil {
		fs, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
			Root: c.tempDir,
		})
		if err != nil {
			return fmt.Errorf("failed to create default storage: %w", err)
		}
		c.storage = fs
	}

	// Create metadata store if not provided and metadata is enabled
	if c.metadataEnabled && c.metadataStore == nil {
		c.metadataStore = NewInMemoryMetadataStore()
	}

	// Create storage manager if not provided
	if c.storageManager == nil {
		c.storageManager = NewStorageManager(
			c.storage,
			c.metadataStore,
			StorageConfig{
				PersistArtifacts:   c.persistArtifacts,
				CacheEnabled:       c.cacheEnabled,
				MetadataEnabled:    c.metadataEnabled,
				KeyPrefix:          c.storagePrefix,
				ArtifactIDStrategy: c.artifactIDStrategy,
				IncludeContentHash: c.includeContentHash,
			},
		)
	}

	return nil
}
