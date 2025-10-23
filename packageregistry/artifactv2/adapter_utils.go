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
		return netErr.Timeout()
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
	content, _, err := fetchHTTPWithMirrors(ctx, []string{url}, config)
	return content, err
}

// fetchHTTPWithMirrors performs an HTTP GET request with retry logic and mirror support.
// It cycles through the provided URLs in round-robin fashion during retries.
// On 404 errors, it immediately tries the next URL in the rotation.
// Returns the response body content, the successful URL, and any error.
func fetchHTTPWithMirrors(ctx context.Context, urls []string, config fetchConfig) ([]byte, string, error) {
	if len(urls) == 0 {
		return nil, "", fmt.Errorf("no URLs provided")
	}

	applyFetchConfigDefaults(&config)

	var content []byte
	var fetchErr error
	urlIndex := 0

	for attempt := 0; attempt <= config.RetryAttempts; attempt++ {
		currentURL := urls[urlIndex]

		if attempt > 0 {
			delay := config.RetryDelay * time.Duration(attempt)
			if delay > config.MaxRetryDelay {
				delay = config.MaxRetryDelay
			}
			log.Debugf("Retry attempt %d/%d for %s (waiting %v)",
				attempt, config.RetryAttempts, currentURL, delay)
			time.Sleep(delay)
		}

		reqCtx, cancel := context.WithTimeout(ctx, config.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, "GET", currentURL, nil)
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
				log.Debugf("Non-retryable network error for %s: %v", currentURL, err)
				break
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()

			fetchErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)

			// On 404, cycle to next URL in round-robin fashion
			if resp.StatusCode == http.StatusNotFound && len(urls) > 1 {
				log.Debugf("Got 404 from %s, trying next mirror", currentURL)
				urlIndex = (urlIndex + 1) % len(urls)
				continue
			}

			if !isRetryableStatusCode(resp.StatusCode) {
				log.Debugf("Non-retryable HTTP status %d for %s", resp.StatusCode, currentURL)
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
				log.Debugf("Non-retryable body read error for %s: %v", currentURL, err)
				break
			}
			continue
		}

		// Success!
		return content, currentURL, nil
	}

	if fetchErr != nil {
		return nil, "", fmt.Errorf("failed after %d attempts: %w",
			config.RetryAttempts+1, fetchErr)
	}

	return content, urls[urlIndex], nil
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
