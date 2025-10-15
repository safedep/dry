package artifactv2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/safedep/dry/log"
)

// fetchConfig contains configuration for HTTP fetch operations
type fetchConfig struct {
	// HTTPClient to use for requests
	HTTPClient *http.Client

	// Timeout for each fetch attempt
	Timeout time.Duration

	// RetryAttempts is the number of retry attempts (0 = no retries, just 1 attempt)
	// Default: 3
	RetryAttempts int

	// RetryDelay is the base delay between retries (multiplied by attempt number)
	// Default: 1 second
	RetryDelay time.Duration

	// MaxRetryDelay is the maximum delay between retries (prevents unbounded exponential backoff)
	// Default: 15 seconds
	MaxRetryDelay time.Duration

	// UserAgent to use for HTTP requests
	// Default: "safedep-dry/1.0"
	UserAgent string
}

// Default values for fetchConfig
const (
	defaultRetryAttempts = 3
	defaultRetryDelay    = 1 * time.Second
	defaultMaxRetryDelay = 15 * time.Second
	defaultUserAgent     = "safedep-dry/1.0"
)

// applyFetchConfigDefaults applies safe defaults to fetch config
func applyFetchConfigDefaults(config *fetchConfig) {
	if config.RetryAttempts == 0 {
		config.RetryAttempts = defaultRetryAttempts
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = defaultRetryDelay
	}
	if config.MaxRetryDelay == 0 {
		config.MaxRetryDelay = defaultMaxRetryDelay
	}
	if config.UserAgent == "" {
		config.UserAgent = defaultUserAgent
	}
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
}

// fetchHTTPWithRetry performs an HTTP GET request with retry logic
// Returns the response body content on success
func fetchHTTPWithRetry(ctx context.Context, url string, config fetchConfig) ([]byte, error) {
	// Apply safe defaults
	applyFetchConfigDefaults(&config)

	var content []byte
	var fetchErr error

	for attempt := 0; attempt <= config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: multiply delay by attempt number, capped at MaxRetryDelay
			delay := config.RetryDelay * time.Duration(attempt)
			if delay > config.MaxRetryDelay {
				delay = config.MaxRetryDelay
			}
			log.Debugf("Retry attempt %d/%d for %s (waiting %v)",
				attempt, config.RetryAttempts, url, delay)
			time.Sleep(delay)
		}

		// Create request with timeout
		reqCtx, cancel := context.WithTimeout(ctx, config.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		if err != nil {
			fetchErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		// Set User-Agent header
		req.Header.Set("User-Agent", config.UserAgent)

		// Perform request (HTTPClient will follow redirects by default)
		resp, err := config.HTTPClient.Do(req)
		if err != nil {
			fetchErr = fmt.Errorf("failed to fetch: %w", err)
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fetchErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			continue
		}

		// Read response body
		content, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fetchErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Success
		fetchErr = nil
		break
	}

	if fetchErr != nil {
		return nil, fmt.Errorf("failed after %d attempts: %w",
			config.RetryAttempts+1, fetchErr)
	}

	return content, nil
}

// verifyChecksum verifies that content matches the expected checksum
// Returns nil if checksum matches or if expectedChecksum is empty
func verifyChecksum(content []byte, expectedChecksum string) error {
	if expectedChecksum == "" {
		return nil
	}

	actualHash, err := computeSHA256(bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	if actualHash != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expectedChecksum, actualHash)
	}

	return nil
}

// storeArtifactResult contains the results of storing an artifact
type storeArtifactResult struct {
	// ArtifactID is the unique identifier for the stored artifact
	ArtifactID string

	// SHA256 is the computed checksum of the artifact
	SHA256 string

	// Size is the size of the artifact in bytes
	Size int64
}

// storeArtifactWithMetadata stores an artifact and its metadata
// This is a common pattern used by most adapters
func storeArtifactWithMetadata(
	ctx context.Context,
	storage StorageManager,
	info ArtifactInfo,
	content []byte,
	contentType string,
	originURL string,
) (*storeArtifactResult, error) {
	// Store the artifact
	artifactID, err := storage.Store(ctx, info, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to store artifact: %w", err)
	}

	// Compute full SHA256 for metadata
	sha256Hash, err := computeSHA256(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to compute SHA256: %w", err)
	}

	// Create and store metadata
	metadata := ArtifactMetadata{
		ID:          artifactID,
		Name:        info.Name,
		Version:     info.Version,
		Ecosystem:   info.Ecosystem,
		Origin:      originURL,
		SHA256:      sha256Hash,
		Size:        int64(len(content)),
		FetchedAt:   time.Now(),
		StorageKey:  computeStorageKeyFromID(artifactID, ""), // Use package-local helper
		ContentType: contentType,
	}

	if err := storage.StoreMetadata(ctx, metadata); err != nil {
		log.Warnf("Failed to store metadata for %s: %v", artifactID, err)
	}

	return &storeArtifactResult{
		ArtifactID: artifactID,
		SHA256:     sha256Hash,
		Size:       int64(len(content)),
	}, nil
}

// checkCacheAndFetch checks if an artifact exists in cache before fetching
// Returns (exists, artifactID, error)
// If exists is true, artifactID will contain the cached artifact ID
func checkCacheAndFetch(
	ctx context.Context,
	adapter ArtifactAdapterV2,
	info ArtifactInfo,
	cacheEnabled bool,
) (exists bool, artifactID string, err error) {
	if !cacheEnabled {
		return false, "", nil
	}

	exists, artifactID, err = adapter.Exists(ctx, info)
	if err != nil {
		// Error checking cache, but we can continue with fetch
		log.Debugf("Cache check failed for %s@%s: %v", info.Name, info.Version, err)
		return false, "", nil
	}

	if exists {
		log.Debugf("Found %s@%s in cache: %s", info.Name, info.Version, artifactID)
	}

	return exists, artifactID, nil
}

// loadMetadataOrDefault loads metadata from storage or returns a minimal default
// This is used when loading artifacts to ensure we always have some metadata
func loadMetadataOrDefault(
	ctx context.Context,
	storage StorageManager,
	artifactID string,
	metadataEnabled bool,
) *ArtifactMetadata {
	if !metadataEnabled {
		return &ArtifactMetadata{ID: artifactID}
	}

	metadata, err := storage.GetMetadata(ctx, artifactID)
	if err != nil {
		log.Debugf("Metadata not found for %s: %v", artifactID, err)
		return &ArtifactMetadata{ID: artifactID}
	}

	return metadata
}

// fetchOptions contains common options for fetching artifacts
type fetchOptions struct {
	// Config contains adapter configuration
	Config *adapterConfig

	// Info describes the artifact to fetch
	Info ArtifactInfo

	// URL is the download URL (if empty, adapter must build it)
	URL string

	// ContentType is the MIME type of the artifact (e.g., "application/gzip")
	ContentType string
}

// standardFetchFlow implements the standard artifact fetch workflow:
// 1. Check cache (if enabled)
// 2. Fetch from URL with retry
// 3. Verify checksum (if provided)
// 4. Store artifact and metadata
// 5. Return artifact ID
//
// This encapsulates the common pattern used by most adapters
func standardFetchFlow(
	ctx context.Context,
	storage StorageManager,
	opts fetchOptions,
) (artifactID string, err error) {
	// Check if already cached
	// Note: Cache checking should typically be done by the adapter before calling
	// this function, as it requires adapter-specific logic to determine the artifact ID

	log.Debugf("Fetching artifact from: %s", opts.URL)

	// Fetch with retries
	content, err := fetchHTTPWithRetry(ctx, opts.URL, fetchConfig{
		HTTPClient:    opts.Config.httpClient,
		Timeout:       opts.Config.fetchTimeout,
		RetryAttempts: opts.Config.retryAttempts,
		RetryDelay:    opts.Config.retryDelay,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch artifact: %w", err)
	}

	// Verify checksum if provided
	if err := verifyChecksum(content, opts.Info.Checksum); err != nil {
		return "", err
	}

	// Store artifact and metadata
	result, err := storeArtifactWithMetadata(
		ctx,
		storage,
		opts.Info,
		content,
		opts.ContentType,
		opts.URL,
	)
	if err != nil {
		return "", err
	}

	log.Debugf("Stored artifact %s@%s as %s",
		opts.Info.Name, opts.Info.Version, result.ArtifactID)

	return result.ArtifactID, nil
}
