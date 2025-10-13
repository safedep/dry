package artifactv2

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchHTTPWithRetry_Success(t *testing.T) {
	content := []byte("test content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	ctx := context.Background()
	result, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
	})

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestFetchHTTPWithRetry_WithRetries(t *testing.T) {
	content := []byte("retry content")
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	ctx := context.Background()
	result, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 5,
		RetryDelay:    10 * time.Millisecond,
	})

	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 3, attempts)
}

func TestFetchHTTPWithRetry_FailureAfterRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 2,
		RetryDelay:    10 * time.Millisecond,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 attempts")
}

func TestFetchHTTPWithRetry_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       10 * time.Millisecond,
		RetryAttempts: 0,
		RetryDelay:    10 * time.Millisecond,
	})

	assert.Error(t, err)
}

func TestVerifyChecksum_Success(t *testing.T) {
	content := []byte("test content for checksum")

	// Compute actual checksum
	hash, err := ComputeSHA256(bytes.NewReader(content))
	require.NoError(t, err)

	// Verify with correct checksum
	err = verifyChecksum(content, hash)
	assert.NoError(t, err)
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	content := []byte("test content")
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	err := verifyChecksum(content, wrongChecksum)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestVerifyChecksum_EmptyChecksum(t *testing.T) {
	content := []byte("test content")

	// Empty checksum should pass (no verification)
	err := verifyChecksum(content, "")
	assert.NoError(t, err)
}

func TestStoreArtifactWithMetadata(t *testing.T) {
	// Create test storage
	tempDir := t.TempDir()
	storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tempDir,
	})
	require.NoError(t, err)

	metadataStore := NewInMemoryMetadataStore()
	storageMgr := NewStorageManager(storageBackend, metadataStore, StorageConfig{
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
		ArtifactIDStrategy: ArtifactIDStrategyConvention,
	})

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}
	content := []byte("test content")
	contentType := "application/gzip"
	originURL := "https://example.com/package.tgz"

	result, err := storeArtifactWithMetadata(ctx, storageMgr, info, content, contentType, originURL)
	require.NoError(t, err)

	// Verify result
	assert.NotEmpty(t, result.ArtifactID)
	assert.NotEmpty(t, result.SHA256)
	assert.Equal(t, int64(len(content)), result.Size)

	// Verify artifact can be retrieved
	reader, err := storageMgr.Get(ctx, result.ArtifactID)
	require.NoError(t, err)
	defer reader.Close()

	retrievedContent := make([]byte, len(content))
	_, err = reader.Read(retrievedContent)
	require.NoError(t, err)
	assert.Equal(t, content, retrievedContent)

	// Verify metadata was stored
	metadata, err := storageMgr.GetMetadata(ctx, result.ArtifactID)
	require.NoError(t, err)
	assert.Equal(t, result.ArtifactID, metadata.ID)
	assert.Equal(t, "test-package", metadata.Name)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, originURL, metadata.Origin)
	assert.Equal(t, contentType, metadata.ContentType)
}

func TestLoadMetadataOrDefault_WithMetadata(t *testing.T) {
	metadataStore := NewInMemoryMetadataStore()
	tempDir := t.TempDir()
	storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tempDir,
	})
	require.NoError(t, err)

	storageMgr := NewStorageManager(storageBackend, metadataStore, StorageConfig{
		MetadataEnabled: true,
	})

	ctx := context.Background()

	// Store metadata first
	originalMetadata := ArtifactMetadata{
		ID:      "npm:test:1.0.0",
		Name:    "test",
		Version: "1.0.0",
	}
	err = metadataStore.Put(ctx, originalMetadata)
	require.NoError(t, err)

	// Load it back
	loaded := loadMetadataOrDefault(ctx, storageMgr, "npm:test:1.0.0", true)
	assert.Equal(t, "npm:test:1.0.0", loaded.ID)
	assert.Equal(t, "test", loaded.Name)
	assert.Equal(t, "1.0.0", loaded.Version)
}

func TestLoadMetadataOrDefault_NotFound(t *testing.T) {
	metadataStore := NewInMemoryMetadataStore()
	tempDir := t.TempDir()
	storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tempDir,
	})
	require.NoError(t, err)

	storageMgr := NewStorageManager(storageBackend, metadataStore, StorageConfig{
		MetadataEnabled: true,
	})

	ctx := context.Background()

	// Load non-existent metadata (should return default)
	loaded := loadMetadataOrDefault(ctx, storageMgr, "npm:nonexistent:1.0.0", true)
	assert.Equal(t, "npm:nonexistent:1.0.0", loaded.ID)
	assert.Empty(t, loaded.Name) // Should be default/empty
}

func TestLoadMetadataOrDefault_MetadataDisabled(t *testing.T) {
	metadataStore := NewInMemoryMetadataStore()
	tempDir := t.TempDir()
	storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tempDir,
	})
	require.NoError(t, err)

	storageMgr := NewStorageManager(storageBackend, metadataStore, StorageConfig{
		MetadataEnabled: false,
	})

	ctx := context.Background()

	// Load with metadata disabled (should return minimal default)
	loaded := loadMetadataOrDefault(ctx, storageMgr, "npm:test:1.0.0", false)
	assert.Equal(t, "npm:test:1.0.0", loaded.ID)
	assert.Empty(t, loaded.Name)
}
