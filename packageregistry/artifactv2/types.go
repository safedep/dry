// Package artifactv2 contains the next generation artifact adapter system
// with improved storage abstraction, caching, and content-addressable design.
package artifactv2

import (
	"context"
	"io"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// ArtifactAdapterV2 is the main interface for artifact operations
type ArtifactAdapterV2 interface {
	// Fetch downloads an artifact from the registry and stores it
	// Returns a reader for accessing the artifact contents
	Fetch(ctx context.Context, info ArtifactInfo) (ArtifactReaderV2, error)

	// Load creates a reader from an existing artifact in storage
	// Uses the artifact ID to locate it in the storage backend
	Load(ctx context.Context, artifactID string) (ArtifactReaderV2, error)

	// LoadFromSource loads an artifact from a provided source
	// (for backward compatibility and local files)
	LoadFromSource(ctx context.Context, source ArtifactSource) (ArtifactReaderV2, error)

	// GetMetadata retrieves metadata for an artifact without loading it
	GetMetadata(ctx context.Context, artifactID string) (*ArtifactMetadata, error)

	// Exists checks if an artifact is already in storage
	// Returns (exists, artifactID, error)
	Exists(ctx context.Context, info ArtifactInfo) (bool, string, error)
}

// ArtifactInfo describes an artifact to fetch
type ArtifactInfo struct {
	// Name of the package (e.g., "express", "@angular/core")
	Name string

	// Version of the package (e.g., "4.17.1", "v1.2.3")
	Version string

	// Ecosystem the package belongs to
	Ecosystem packagev1.Ecosystem

	// Optional explicit URL (overrides registry conventions)
	URL string

	// Optional checksum for verification (SHA256)
	Checksum string
}

// ArtifactSource describes a local or provided artifact source
type ArtifactSource struct {
	// Origin URL or path from where the artifact was fetched
	Origin string

	// LocalPath to the artifact file (optional)
	LocalPath string

	// Reader to the artifact content
	Reader io.ReadCloser
}

// ArtifactReaderV2 provides read access to artifact contents
type ArtifactReaderV2 interface {
	// ID returns the unique content-addressable ID for this artifact
	ID() string

	// Metadata returns the artifact metadata
	Metadata() ArtifactMetadata

	// Reader returns a new reader for the raw artifact bytes
	Reader(ctx context.Context) (io.ReadCloser, error)

	// EnumFiles enumerates files within the artifact
	// The callback is called for each file and can return an error to stop enumeration
	EnumFiles(ctx context.Context, fn func(FileInfo) error) error

	// ReadFile reads a specific file from the artifact by path
	ReadFile(ctx context.Context, path string) (io.ReadCloser, error)

	// GetFileMetadata returns metadata for a specific file
	GetFileMetadata(ctx context.Context, path string) (*FileMetadata, error)

	// ListFiles returns a list of all file paths in the artifact
	ListFiles(ctx context.Context) ([]string, error)

	// Close releases resources (does NOT delete from storage)
	Close() error
}

// ArtifactMetadata contains metadata about an artifact
type ArtifactMetadata struct {
	// ID is the unique content-addressable identifier (ecosystem:sha256_prefix)
	ID string

	// Name of the package
	Name string

	// Version of the package
	Version string

	// Ecosystem the package belongs to
	Ecosystem packagev1.Ecosystem

	// Origin URL or path from where the artifact was fetched
	Origin string

	// SHA256 checksum of the artifact
	SHA256 string

	// Size of the artifact in bytes
	Size int64

	// FetchedAt is the timestamp when the artifact was fetched
	FetchedAt time.Time

	// StorageKey is the key used to store the artifact in storage
	StorageKey string

	// ContentType of the artifact (e.g., "application/gzip")
	ContentType string
}

// FileInfo describes a file within an artifact
type FileInfo struct {
	// Path of the file within the artifact
	Path string

	// Size of the file in bytes
	Size int64

	// ModTime is the modification time of the file
	ModTime time.Time

	// IsDir indicates if this is a directory
	IsDir bool

	// ContentID is a unique identifier for this file's content
	ContentID string

	// Reader provides access to file contents
	// Note: This reader is only valid during the EnumFiles callback
	Reader io.Reader
}

// FileMetadata contains metadata about a file within an artifact
type FileMetadata struct {
	// Path of the file within the artifact
	Path string

	// Size of the file in bytes
	Size int64

	// ModTime is the modification time of the file
	ModTime time.Time

	// SHA256 checksum of the file
	SHA256 string

	// ContentID is a unique identifier for this file's content
	ContentID string
}

// StorageManager manages artifact storage and caching
type StorageManager interface {
	// Store saves an artifact to storage and returns its ID
	// The reader should be positioned at the beginning of the artifact
	Store(ctx context.Context, info ArtifactInfo, reader io.Reader) (string, error)

	// Get retrieves an artifact by ID
	Get(ctx context.Context, artifactID string) (io.ReadCloser, error)

	// Exists checks if an artifact exists in storage
	Exists(ctx context.Context, artifactID string) (bool, error)

	// StoreMetadata saves artifact metadata
	StoreMetadata(ctx context.Context, metadata ArtifactMetadata) error

	// GetMetadata retrieves artifact metadata
	GetMetadata(ctx context.Context, artifactID string) (*ArtifactMetadata, error)

	// Delete removes an artifact and its metadata
	Delete(ctx context.Context, artifactID string) error

	// GetStorageKey returns the storage key for an artifact ID
	GetStorageKey(artifactID string) string
}

// ArtifactIDStrategy determines how artifact IDs are generated
type ArtifactIDStrategy int

const (
	// ArtifactIDStrategyConvention uses ecosystem:name:version (default)
	// This is the most efficient as it doesn't require reading content
	// Example: npm:express:4.17.1
	ArtifactIDStrategyConvention ArtifactIDStrategy = iota

	// ArtifactIDStrategyContentHash uses ecosystem:content_hash
	// This provides content-addressable storage but requires reading the artifact
	// Example: npm:a1b2c3d4e5f6g7h8
	ArtifactIDStrategyContentHash

	// ArtifactIDStrategyHybrid uses ecosystem:name:version:content_hash
	// This combines both approaches for registries that may have version mutations
	// Example: npm:express:4.17.1:a1b2c3d4
	ArtifactIDStrategyHybrid
)

// StorageConfig configures the storage manager
type StorageConfig struct {
	// PersistArtifacts controls whether artifacts are kept after use
	PersistArtifacts bool

	// CacheEnabled enables cache lookup before fetching
	CacheEnabled bool

	// MetadataEnabled enables metadata storage
	MetadataEnabled bool

	// KeyPrefix is prepended to all storage keys
	KeyPrefix string

	// ArtifactIDStrategy determines how artifact IDs are computed
	ArtifactIDStrategy ArtifactIDStrategy

	// IncludeContentHash adds content hash verification even with Convention strategy
	IncludeContentHash bool
}

// MetadataStore handles artifact metadata persistence
type MetadataStore interface {
	// Put stores metadata for an artifact
	Put(ctx context.Context, metadata ArtifactMetadata) error

	// Get retrieves metadata by artifact ID
	Get(ctx context.Context, artifactID string) (*ArtifactMetadata, error)

	// GetByPackage retrieves metadata by package name and version
	GetByPackage(ctx context.Context, ecosystem packagev1.Ecosystem, name, version string) (*ArtifactMetadata, error)

	// Delete removes metadata
	Delete(ctx context.Context, artifactID string) error

	// List returns metadata matching a query
	List(ctx context.Context, query MetadataQuery) ([]ArtifactMetadata, error)
}

// MetadataQuery for searching artifacts
type MetadataQuery struct {
	// Ecosystem to filter by (0 = all)
	Ecosystem packagev1.Ecosystem

	// Name of the package (empty = all)
	Name string

	// Version of the package (empty = all)
	Version string

	// Limit maximum number of results (0 = unlimited)
	Limit int

	// Offset for pagination
	Offset int
}
