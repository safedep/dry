package artifactv2

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := defaultConfig()

	assert.True(t, config.cacheEnabled)
	assert.True(t, config.persistArtifacts)
	assert.True(t, config.metadataEnabled)
	assert.Equal(t, 5*time.Minute, config.fetchTimeout)
	assert.Equal(t, 3, config.retryAttempts)
	assert.Equal(t, time.Second, config.retryDelay)
	assert.NotNil(t, config.httpClient)
	assert.NotEmpty(t, config.tempDir)
}

func TestWithCacheEnabled(t *testing.T) {
	config, err := applyOptions(WithCacheEnabled(false))
	require.NoError(t, err)
	assert.False(t, config.cacheEnabled)

	config, err = applyOptions(WithCacheEnabled(true))
	require.NoError(t, err)
	assert.True(t, config.cacheEnabled)
}

func TestWithPersistArtifacts(t *testing.T) {
	config, err := applyOptions(WithPersistArtifacts(false))
	require.NoError(t, err)
	assert.False(t, config.persistArtifacts)

	config, err = applyOptions(WithPersistArtifacts(true))
	require.NoError(t, err)
	assert.True(t, config.persistArtifacts)
}

func TestWithMetadataEnabled(t *testing.T) {
	config, err := applyOptions(WithMetadataEnabled(false))
	require.NoError(t, err)
	assert.False(t, config.metadataEnabled)

	config, err = applyOptions(WithMetadataEnabled(true))
	require.NoError(t, err)
	assert.True(t, config.metadataEnabled)
}

func TestWithFetchTimeout(t *testing.T) {
	config, err := applyOptions(WithFetchTimeout(10 * time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 10*time.Minute, config.fetchTimeout)

	// Invalid timeout
	_, err = applyOptions(WithFetchTimeout(-1 * time.Second))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")
}

func TestWithRetry(t *testing.T) {
	config, err := applyOptions(WithRetry(5, 2*time.Second))
	require.NoError(t, err)
	assert.Equal(t, 5, config.retryAttempts)
	assert.Equal(t, 2*time.Second, config.retryDelay)

	// Invalid attempts
	_, err = applyOptions(WithRetry(-1, time.Second))
	assert.Error(t, err)

	// Invalid delay
	_, err = applyOptions(WithRetry(3, -1*time.Second))
	assert.Error(t, err)
}

func TestWithHTTPClient(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	config, err := applyOptions(WithHTTPClient(client))
	require.NoError(t, err)
	assert.Equal(t, client, config.httpClient)

	// Nil client
	_, err = applyOptions(WithHTTPClient(nil))
	assert.Error(t, err)
}

func TestWithStoragePrefix(t *testing.T) {
	config, err := applyOptions(WithStoragePrefix("prod/"))
	require.NoError(t, err)
	assert.Equal(t, "prod/", config.storagePrefix)

	// Empty prefix is valid
	config, err = applyOptions(WithStoragePrefix(""))
	require.NoError(t, err)
	assert.Equal(t, "", config.storagePrefix)
}

func TestWithTempDir(t *testing.T) {
	config, err := applyOptions(WithTempDir("/tmp/test"))
	require.NoError(t, err)
	assert.Equal(t, "/tmp/test", config.tempDir)

	// Empty temp dir is invalid
	_, err = applyOptions(WithTempDir(""))
	assert.Error(t, err)
}

func TestMultipleOptions(t *testing.T) {
	config, err := applyOptions(
		WithCacheEnabled(false),
		WithPersistArtifacts(false),
		WithFetchTimeout(10*time.Minute),
		WithRetry(5, 2*time.Second),
		WithStoragePrefix("test/"),
	)
	require.NoError(t, err)

	assert.False(t, config.cacheEnabled)
	assert.False(t, config.persistArtifacts)
	assert.Equal(t, 10*time.Minute, config.fetchTimeout)
	assert.Equal(t, 5, config.retryAttempts)
	assert.Equal(t, 2*time.Second, config.retryDelay)
	assert.Equal(t, "test/", config.storagePrefix)
}

func TestEnsureDefaults(t *testing.T) {
	config := defaultConfig()

	err := config.ensureDefaults()
	require.NoError(t, err)

	// Should have created default storage
	assert.NotNil(t, config.storage)

	// Should have created metadata store (since metadataEnabled is true)
	assert.NotNil(t, config.metadataStore)

	// Should have created storage manager
	assert.NotNil(t, config.storageManager)
}

func TestEnsureDefaultsWithMetadataDisabled(t *testing.T) {
	config := defaultConfig()
	config.metadataEnabled = false

	err := config.ensureDefaults()
	require.NoError(t, err)

	// Should have created storage
	assert.NotNil(t, config.storage)

	// Should NOT have created metadata store
	assert.Nil(t, config.metadataStore)

	// Should still have storage manager
	assert.NotNil(t, config.storageManager)
}

func TestEnsureDefaultsPreservesCustom(t *testing.T) {
	config := defaultConfig()

	// Set custom components
	customStore := NewInMemoryMetadataStore()
	config.metadataStore = customStore

	err := config.ensureDefaults()
	require.NoError(t, err)

	// Should preserve custom metadata store
	assert.Equal(t, customStore, config.metadataStore)
}

func TestOptionErrors(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		wantErr string
	}{
		{
			name:    "nil storage",
			options: []Option{WithStorage(nil)},
			wantErr: "storage cannot be nil",
		},
		{
			name:    "nil storage manager",
			options: []Option{WithStorageManager(nil)},
			wantErr: "storage manager cannot be nil",
		},
		{
			name:    "nil metadata store",
			options: []Option{WithMetadataStore(nil)},
			wantErr: "metadata store cannot be nil",
		},
		{
			name:    "nil HTTP client",
			options: []Option{WithHTTPClient(nil)},
			wantErr: "HTTP client cannot be nil",
		},
		{
			name:    "invalid timeout",
			options: []Option{WithFetchTimeout(0)},
			wantErr: "must be positive",
		},
		{
			name:    "empty temp dir",
			options: []Option{WithTempDir("")},
			wantErr: "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := applyOptions(tt.options...)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestWithArtifactIDStrategy(t *testing.T) {
	config, err := applyOptions(WithArtifactIDStrategy(ArtifactIDStrategyConvention))
	require.NoError(t, err)
	assert.Equal(t, ArtifactIDStrategyConvention, config.artifactIDStrategy)

	config, err = applyOptions(WithArtifactIDStrategy(ArtifactIDStrategyContentHash))
	require.NoError(t, err)
	assert.Equal(t, ArtifactIDStrategyContentHash, config.artifactIDStrategy)

	config, err = applyOptions(WithArtifactIDStrategy(ArtifactIDStrategyHybrid))
	require.NoError(t, err)
	assert.Equal(t, ArtifactIDStrategyHybrid, config.artifactIDStrategy)

	// Invalid strategy
	_, err = applyOptions(WithArtifactIDStrategy(ArtifactIDStrategy(999)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid artifact ID strategy")
}

func TestWithContentHashInID(t *testing.T) {
	config, err := applyOptions(WithContentHashInID(true))
	require.NoError(t, err)
	assert.True(t, config.includeContentHash)

	config, err = applyOptions(WithContentHashInID(false))
	require.NoError(t, err)
	assert.False(t, config.includeContentHash)
}

func TestDefaultArtifactIDStrategy(t *testing.T) {
	config := defaultConfig()
	assert.Equal(t, ArtifactIDStrategyConvention, config.artifactIDStrategy)
	assert.False(t, config.includeContentHash)
}
