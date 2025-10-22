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
	hash, err := computeSHA256(bytes.NewReader(content))
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

// Tests for intelligent retry logic

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		retryable  bool
	}{
		{"200 OK", http.StatusOK, false},
		{"201 Created", http.StatusCreated, false},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"429 Rate Limit", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableStatusCode(tt.statusCode)
			assert.Equal(t, tt.retryable, result,
				"Status %d should be retryable=%v", tt.statusCode, tt.retryable)
		})
	}
}

func TestFetchHTTPWithRetry_NoRetryOn404(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 5, // Set high retry count
		RetryDelay:    10 * time.Millisecond,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Equal(t, 1, attempts, "Should not retry on 404")
}

func TestFetchHTTPWithRetry_NoRetryOn403(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	ctx := context.Background()
	_, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 5,
		RetryDelay:    10 * time.Millisecond,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Equal(t, 1, attempts, "Should not retry on 403")
}

func TestFetchHTTPWithRetry_RetryOn503(t *testing.T) {
	content := []byte("success after retries")
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
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
	assert.Equal(t, 3, attempts, "Should retry on 503")
}

func TestFetchHTTPWithRetry_RateLimitWithRetryAfter(t *testing.T) {
	content := []byte("success after rate limit")
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "1") // 1 second
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	ctx := context.Background()
	start := time.Now()
	result, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
		MaxRetryDelay: 5 * time.Second,
	})

	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, content, result)
	assert.Equal(t, 2, attempts, "Should retry on 429")
	// Should have waited at least 1 second for Retry-After
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(1000))
}

func TestFetchHTTPWithRetry_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := fetchHTTPWithRetry(ctx, server.URL, fetchConfig{
		HTTPClient:    server.Client(),
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "5", 5 * time.Second},
		{"large seconds", "120", 120 * time.Second},
		{"invalid", "invalid", 0},
		{"negative", "-5", 0},
		{"zero", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRetryAfter(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFetchHTTPWithRetry_NoRetryOnInvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := fetchHTTPWithRetry(ctx, "://invalid-url", fetchConfig{
		HTTPClient:    http.DefaultClient,
		Timeout:       5 * time.Second,
		RetryAttempts: 5,
		RetryDelay:    10 * time.Millisecond,
	})

	assert.Error(t, err)
	// Should fail immediately without retries
	assert.Contains(t, err.Error(), "failed to create request")
}
