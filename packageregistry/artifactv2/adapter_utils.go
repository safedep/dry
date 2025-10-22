package artifactv2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	RetryAttempts int

	// RetryDelay is the base delay between retries (multiplied by attempt number)
	RetryDelay time.Duration

	// MaxRetryDelay is the maximum delay between retries (prevents unbounded exponential backoff)
	MaxRetryDelay time.Duration

	// UserAgent to use for HTTP requests
	UserAgent string

	// MaxRedirects is the maximum number of redirects to follow
	MaxRedirects int
}

// Default values for fetchConfig
const (
	defaultRetryAttempts = 3
	defaultRetryDelay    = 1 * time.Second
	defaultMaxRetryDelay = 15 * time.Second
	defaultUserAgent     = "safedep-dry/1.0"
	defaultMaxRedirects  = 10
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
	if config.MaxRedirects == 0 {
		config.MaxRedirects = defaultMaxRedirects
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= config.MaxRedirects {
					return fmt.Errorf("stopped after %d redirects", config.MaxRedirects)
				}
				return nil
			},
		}
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
}

// isRetryableStatusCode determines if an HTTP status code warrants a retry
func isRetryableStatusCode(statusCode int) bool {
	switch {
	case statusCode == http.StatusTooManyRequests:
		return true
	case statusCode >= 500 && statusCode < 600:
		return true
	case statusCode >= 400 && statusCode < 500:
		return false
	case statusCode >= 300 && statusCode < 400:
		return false
	default:
		return false
	}
}

// isRetryableError determines if an error is worth retrying based on its type
func isRetryableError(err error, statusCode int) bool {
	if err == nil {
		return false
	}

	if statusCode > 0 {
		return isRetryableStatusCode(statusCode)
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Temporary() {
			return true
		}
		if urlErr.Timeout() {
			return true
		}
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Temporary() {
			return true
		}
		if netErr.Timeout() {
			return true
		}
		return false
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Temporary() {
			return true
		}
		if opErr.Timeout() {
			return true
		}
		return false
	}

	return false
}

// parseRetryAfter extracts retry delay from Retry-After header.
// Returns 0 if header is missing or invalid.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}

	if t, err := http.ParseTime(header); err == nil {
		delay := time.Until(t)
		if delay > 0 {
			return delay
		}
	}

	return 0
}

// fetchHTTPWithRetry performs an HTTP GET request with retry logic.
// Returns the response body content on success.
func fetchHTTPWithRetry(ctx context.Context, url string, config fetchConfig) ([]byte, error) {
	applyFetchConfigDefaults(&config)

	var content []byte
	var fetchErr error

	for attempt := 0; attempt <= config.RetryAttempts; attempt++ {
		if attempt > 0 {
			delay := config.RetryDelay * time.Duration(attempt)
			if delay > config.MaxRetryDelay {
				delay = config.MaxRetryDelay
			}
			log.Debugf("Retry attempt %d/%d for %s (waiting %v)",
				attempt, config.RetryAttempts, url, delay)
			time.Sleep(delay)
		}

		reqCtx, cancel := context.WithTimeout(ctx, config.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		if err != nil {
			fetchErr = fmt.Errorf("failed to create request: %w", err)
			if !isRetryableError(err, 0) {
				log.Debugf("Non-retryable request creation error: %v", err)
				break
			}
			continue
		}

		req.Header.Set("User-Agent", config.UserAgent)

		resp, err := config.HTTPClient.Do(req)
		if err != nil {
			fetchErr = fmt.Errorf("failed to fetch: %w", err)
			if !isRetryableError(err, 0) {
				log.Debugf("Non-retryable network error for %s: %v", url, err)
				break
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()

			fetchErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)

			if !isRetryableStatusCode(resp.StatusCode) {
				log.Debugf("Non-retryable HTTP status %d for %s", resp.StatusCode, url)
				break
			}

			if resp.StatusCode == http.StatusTooManyRequests && retryAfter != "" {
				if retryDelay := parseRetryAfter(retryAfter); retryDelay > 0 {
					if retryDelay > config.MaxRetryDelay {
						retryDelay = config.MaxRetryDelay
					}
					log.Debugf("Rate limited, respecting Retry-After: %v", retryDelay)
					time.Sleep(retryDelay)
				}
			}

			continue
		}

		content, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fetchErr = fmt.Errorf("failed to read response: %w", err)
			if !isRetryableError(err, 0) {
				log.Debugf("Non-retryable body read error for %s: %v", url, err)
				break
			}
			continue
		}

		fetchErr = nil
		break
	}

	if fetchErr != nil {
		return nil, fmt.Errorf("failed after %d attempts: %w",
			config.RetryAttempts+1, fetchErr)
	}

	return content, nil
}

// verifyChecksum verifies that content matches the expected checksum.
// Returns nil if checksum matches or if expectedChecksum is empty.
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
	ArtifactID string
	SHA256     string
	Size       int64
}

// storeArtifactWithMetadata stores an artifact and its metadata.
// This is a common pattern used by most adapters.
func storeArtifactWithMetadata(
	ctx context.Context,
	storage StorageManager,
	info ArtifactInfo,
	content []byte,
	contentType string,
	originURL string,
) (*storeArtifactResult, error) {
	artifactID, err := storage.Store(ctx, info, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to store artifact: %w", err)
	}

	sha256Hash, err := computeSHA256(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to compute SHA256: %w", err)
	}

	metadata := ArtifactMetadata{
		ID:          artifactID,
		Name:        info.Name,
		Version:     info.Version,
		Ecosystem:   info.Ecosystem,
		Origin:      originURL,
		SHA256:      sha256Hash,
		Size:        int64(len(content)),
		FetchedAt:   time.Now(),
		StorageKey:  computeStorageKeyFromID(artifactID, ""),
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

// checkCacheAndFetch checks if an artifact exists in cache before fetching.
// Returns (exists, artifactID, error). If exists is true, artifactID will contain the cached artifact ID.
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
		log.Debugf("Cache check failed for %s@%s: %v", info.Name, info.Version, err)
		return false, "", nil
	}

	if exists {
		log.Debugf("Found %s@%s in cache: %s", info.Name, info.Version, artifactID)
	}

	return exists, artifactID, nil
}

// loadMetadataOrDefault loads metadata from storage or returns a minimal default.
// This is used when loading artifacts to ensure we always have some metadata.
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

// standardFetchFlow implements the standard artifact fetch workflow.
// This encapsulates the common pattern used by most adapters:
// 1. Check cache (if enabled)
// 2. Fetch from URL with retry
// 3. Verify checksum (if provided)
// 4. Store artifact and metadata
// 5. Return artifact ID
func standardFetchFlow(
	ctx context.Context,
	storage StorageManager,
	opts fetchOptions,
) (artifactID string, err error) {
	log.Debugf("Fetching artifact from: %s", opts.URL)

	content, err := fetchHTTPWithRetry(ctx, opts.URL, fetchConfig{
		HTTPClient:    opts.Config.httpClient,
		Timeout:       opts.Config.fetchTimeout,
		RetryAttempts: opts.Config.retryAttempts,
		RetryDelay:    opts.Config.retryDelay,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch artifact: %w", err)
	}

	if err := verifyChecksum(content, opts.Info.Checksum); err != nil {
		return "", err
	}

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
