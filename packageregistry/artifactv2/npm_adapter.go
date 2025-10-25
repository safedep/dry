package artifactv2

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/safedep/dry/log"
)

const (
	// npmRegistryURL is the base URL for the NPM registry
	npmRegistryURL = "https://registry.npmjs.org"
)

// npmAdapterV2 implements ArtifactAdapterV2 for NPM packages
type npmAdapterV2 struct {
	config  *adapterConfig
	storage StorageManager
}

// NewNpmAdapterV2 creates a new NPM adapter with the given options
func NewNpmAdapterV2(opts ...Option) (ArtifactAdapterV2, error) {
	config, err := applyOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	if err := config.ensureDefaults(); err != nil {
		return nil, fmt.Errorf("failed to initialize defaults: %w", err)
	}

	return &npmAdapterV2{
		config:  config,
		storage: config.storageManager,
	}, nil
}

// Fetch downloads an NPM package and stores it
func (a *npmAdapterV2) Fetch(ctx context.Context, info ArtifactInfo) (ArtifactReaderV2, error) {
	// Check if already cached
	if a.config.cacheEnabled {
		exists, artifactID, err := a.Exists(ctx, info)
		if err == nil && exists {
			log.Debugf("NPM package %s@%s found in cache: %s", info.Name, info.Version, artifactID)
			return a.Load(ctx, artifactID)
		}
	}

	// Determine URLs to try
	var urls []string
	var successURL string

	if info.URL != "" {
		// User provided explicit URL - bypass mirror logic
		urls = []string{info.URL}
	} else {
		// Use primary registry + mirrors
		urls = a.getRegistryURLs(info.Name, info.Version)
	}

	log.Debugf("Fetching NPM package from: %s (with %d total URLs)", urls[0], len(urls))

	// Fetch with retries and mirror support
	content, successURL, err := fetchHTTPWithMirrors(ctx, urls, fetchConfig{
		HTTPClient:    a.config.httpClient,
		Timeout:       a.config.fetchTimeout,
		RetryAttempts: a.config.retryAttempts,
		RetryDelay:    a.config.retryDelay,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NPM package: %w", err)
	}

	// Verify checksum if provided using common utility
	if err := verifyChecksum(content, info.Checksum); err != nil {
		return nil, err
	}

	// Store artifact and metadata using common utility
	result, err := storeArtifactWithMetadata(
		ctx,
		a.storage,
		info,
		content,
		"application/gzip",
		successURL,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf("Stored NPM package %s@%s as %s (from %s)", info.Name, info.Version, result.ArtifactID, successURL)

	// Return reader
	return a.Load(ctx, result.ArtifactID)
}

// Load creates a reader from an existing artifact in storage
func (a *npmAdapterV2) Load(ctx context.Context, artifactID string) (ArtifactReaderV2, error) {
	// Check if artifact exists
	exists, err := a.storage.Exists(ctx, artifactID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	// Load metadata using common utility
	metadata := loadMetadataOrDefault(
		ctx,
		a.storage,
		artifactID,
		a.config.metadataEnabled && a.config.metadataStore != nil,
	)

	return &npmReaderV2{
		artifactID:    artifactID,
		storage:       a.storage,
		metadata:      metadata,
		config:        a.config,
		archiveReader: newArchiveReader(artifactID, a.storage, archiveTypeTarGz),
	}, nil
}

// LoadFromSource loads an artifact from a provided source
func (a *npmAdapterV2) LoadFromSource(ctx context.Context, info ArtifactInfo, source ArtifactSource) (ArtifactReaderV2, error) {
	// Read content from source
	content, err := io.ReadAll(source.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read source: %w", err)
	}

	// Store artifact and metadata using common utility
	result, err := storeArtifactWithMetadata(
		ctx,
		a.storage,
		info,
		content,
		"application/gzip",
		source.Origin,
	)
	if err != nil {
		return nil, err
	}

	return a.Load(ctx, result.ArtifactID)
}

// GetMetadata retrieves metadata for an artifact
func (a *npmAdapterV2) GetMetadata(ctx context.Context, artifactID string) (*ArtifactMetadata, error) {
	if !a.config.metadataEnabled {
		return nil, fmt.Errorf("metadata not enabled")
	}

	return a.storage.GetMetadata(ctx, artifactID)
}

// Exists checks if an artifact is already in storage
func (a *npmAdapterV2) Exists(ctx context.Context, info ArtifactInfo) (bool, string, error) {
	// Try to find by metadata first (more efficient)
	if a.config.metadataEnabled && a.config.storageManager != nil {
		// For Convention strategy, we can predict the artifact ID using common function
		if a.config.artifactIDStrategy == ArtifactIDStrategyConvention {
			// Use common ID generation function (single source of truth)
			predictedID := generateArtifactID(info, ArtifactIDStrategyConvention, "")

			exists, err := a.storage.Exists(ctx, predictedID)
			if err == nil && exists {
				return true, predictedID, nil
			}
		}

		// For other strategies, we need to query metadata
		// This is not implemented in the current MetadataStore interface
		// but could be added via GetByPackage/GetByArtifact
	}

	return false, "", nil
}

// buildNpmUrl constructs the NPM registry URL for a package from a given registry base URL
func buildNpmUrl(registryBase, name, version string) string {
	base := name
	packageName := name

	// Handle scoped packages (@scope/package)
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) == 2 {
			base = name
			packageName = parts[1]
		}
	}

	return fmt.Sprintf("%s/%s/-/%s-%s.tgz",
		registryBase, base, packageName, version)
}

// getRegistryURLs returns the list of registry URLs to try (primary + mirrors)
func (a *npmAdapterV2) getRegistryURLs(name, version string) []string {
	urls := []string{buildNpmUrl(npmRegistryURL, name, version)}

	for _, mirror := range a.config.registryMirrors {
		urls = append(urls, buildNpmUrl(mirror, name, version))
	}

	return urls
}

// npmReaderV2 implements ArtifactReaderV2 for NPM packages
type npmReaderV2 struct {
	artifactID    string
	storage       StorageManager
	metadata      *ArtifactMetadata
	config        *adapterConfig
	archiveReader *archiveReader // Unified archive reader with caching
}

// ID returns the artifact ID
func (r *npmReaderV2) ID() string {
	return r.artifactID
}

// Metadata returns the artifact metadata
func (r *npmReaderV2) Metadata() ArtifactMetadata {
	if r.metadata != nil {
		return *r.metadata
	}
	return ArtifactMetadata{
		ID: r.artifactID,
	}
}

// Reader returns a new reader for the raw artifact bytes
func (r *npmReaderV2) Reader(ctx context.Context) (io.ReadCloser, error) {
	return r.storage.Get(ctx, r.artifactID)
}

// EnumFiles enumerates files within the NPM package (tar.gz)
func (r *npmReaderV2) EnumFiles(ctx context.Context, fn func(FileInfo) error) error {
	return r.archiveReader.enumFiles(ctx, fn)
}

// ReadFile reads a specific file from the NPM package
// This method uses the index cache for O(1) lookup validation
func (r *npmReaderV2) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	return r.archiveReader.readFile(ctx, path)
}

// GetFileMetadata returns metadata for a specific file
// This method uses the index cache for O(1) lookup without scanning the tar
func (r *npmReaderV2) GetFileMetadata(ctx context.Context, path string) (*FileMetadata, error) {
	entry, err := r.archiveReader.getEntry(ctx, path)
	if err != nil {
		return nil, err
	}

	return &FileMetadata{
		Path:    entry.path,
		Size:    entry.size,
		ModTime: entry.modTime,
	}, nil
}

// ListFiles returns a list of all file paths in the artifact
// This method uses the index cache for O(1) retrieval without scanning the tar
func (r *npmReaderV2) ListFiles(ctx context.Context) ([]string, error) {
	return r.archiveReader.listEntries(ctx, true)
}

// Extract extracts the archive contents to storage
// Files are extracted to a sibling directory alongside the archive
// Returns information about the extraction including the base storage key
func (r *npmReaderV2) Extract(ctx context.Context) (*ExtractResult, error) {
	// Compute extraction key based on artifact ID
	baseKey := computeExtractionKey(r.artifactID, r.config.storagePrefix)

	// Get underlying storage from storage manager
	storage := r.storage.GetStorage()

	// Perform extraction
	return extractToStorage(ctx, storage, r.archiveReader, baseKey)
}

// Close releases resources
func (r *npmReaderV2) Close() error {
	// In v2, we don't delete artifacts on close if persistence is enabled
	// This is controlled by the StorageManager configuration
	return nil
}
