package artifactv2

import (
	"archive/tar"
	"compress/gzip"
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

	// Build NPM registry URL
	url := info.URL
	if url == "" {
		url = a.buildNpmUrl(info.Name, info.Version)
	}

	log.Debugf("Fetching NPM package from: %s", url)

	// Fetch with retries using common utility
	content, err := fetchHTTPWithRetry(ctx, url, fetchConfig{
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
		url,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf("Stored NPM package %s@%s as %s", info.Name, info.Version, result.ArtifactID)

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
		artifactID: artifactID,
		storage:    a.storage,
		metadata:   metadata,
		config:     a.config,
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

// buildNpmUrl constructs the NPM registry URL for a package
func (a *npmAdapterV2) buildNpmUrl(name, version string) string {
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
		npmRegistryURL, base, packageName, version)
}

// npmReaderV2 implements ArtifactReaderV2 for NPM packages
type npmReaderV2 struct {
	artifactID string
	storage    StorageManager
	metadata   *ArtifactMetadata
	config     *adapterConfig
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
	reader, err := r.Reader(ctx)
	if err != nil {
		return fmt.Errorf("failed to get artifact reader: %w", err)
	}
	defer reader.Close()

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Skip directories
		if header.Typeflag != tar.TypeReg {
			continue
		}

		fileInfo := FileInfo{
			Path:    header.Name,
			Size:    header.Size,
			ModTime: header.ModTime,
			IsDir:   false,
			Reader:  tarReader,
		}

		if err := fn(fileInfo); err != nil {
			return err
		}
	}

	return nil
}

// ReadFile reads a specific file from the NPM package
func (r *npmReaderV2) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	reader, err := r.Reader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact reader: %w", err)
	}

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			gzipReader.Close()
			reader.Close()
			return nil, fmt.Errorf("file not found: %s", path)
		}
		if err != nil {
			gzipReader.Close()
			reader.Close()
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name == path {
			// Return a composite reader that closes all layers
			return &tarFileReader{
				tarReader:  tarReader,
				gzipReader: gzipReader,
				fileReader: reader,
			}, nil
		}
	}
}

// GetFileMetadata returns metadata for a specific file
func (r *npmReaderV2) GetFileMetadata(ctx context.Context, path string) (*FileMetadata, error) {
	reader, err := r.Reader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact reader: %w", err)
	}
	defer reader.Close()

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name == path {
			return &FileMetadata{
				Path:    header.Name,
				Size:    header.Size,
				ModTime: header.ModTime,
			}, nil
		}
	}
}

// ListFiles returns a list of all file paths in the artifact
func (r *npmReaderV2) ListFiles(ctx context.Context) ([]string, error) {
	var files []string

	err := r.EnumFiles(ctx, func(info FileInfo) error {
		files = append(files, info.Path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// Close releases resources
func (r *npmReaderV2) Close() error {
	// In v2, we don't delete artifacts on close if persistence is enabled
	// This is controlled by the StorageManager configuration
	return nil
}

// tarFileReader wraps tar reading with proper cleanup
type tarFileReader struct {
	tarReader  *tar.Reader
	gzipReader *gzip.Reader
	fileReader io.ReadCloser
}

func (r *tarFileReader) Read(p []byte) (n int, err error) {
	return r.tarReader.Read(p)
}

func (r *tarFileReader) Close() error {
	var errs []error

	if err := r.gzipReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("gzip close: %w", err))
	}

	if err := r.fileReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("file close: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}
