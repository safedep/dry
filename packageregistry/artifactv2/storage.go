package artifactv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/safedep/dry/storage"
)

// storageManager implements StorageManager interface
type storageManager struct {
	storage  storage.Storage
	metadata MetadataStore
	config   StorageConfig
}

// NewStorageManager creates a new storage manager
func NewStorageManager(storage storage.Storage, metadata MetadataStore, config StorageConfig) StorageManager {
	return &storageManager{
		storage:  storage,
		metadata: metadata,
		config:   config,
	}
}

// encodeName safely encodes package names for use in artifact IDs and paths
// Handles names with slashes and special characters
// Examples:
//   - express -> express
//   - @angular/core -> angular-core
//   - @babel/preset-env -> babel-preset-env
//   - github.com/gin-gonic/gin -> github.com-gin-gonic-gin
//   - github.com/user/repo/v2 -> github.com-user-repo-v2
func encodeName(name string) string {
	// Replace slashes with hyphens
	encoded := strings.ReplaceAll(name, "/", "-")
	// Remove @ prefix (common in npm scoped packages)
	encoded = strings.TrimPrefix(encoded, "@")
	return encoded
}

// Store saves an artifact to storage and returns its ID
func (sm *storageManager) Store(ctx context.Context, info ArtifactInfo, reader io.Reader) (string, error) {
	var artifactID string
	var buf bytes.Buffer
	var contentHash string

	// Determine if we need to read content based on strategy
	needsContentHash := sm.config.ArtifactIDStrategy == ArtifactIDStrategyContentHash ||
		sm.config.ArtifactIDStrategy == ArtifactIDStrategyHybrid ||
		sm.config.IncludeContentHash

	if needsContentHash {
		// Read content and compute hash
		hash := sha256.New()
		tee := io.TeeReader(reader, &buf)

		if _, err := io.Copy(hash, tee); err != nil {
			return "", fmt.Errorf("failed to compute hash: %w", err)
		}

		hashBytes := hash.Sum(nil)
		contentHash = hex.EncodeToString(hashBytes[:8]) // First 8 bytes (16 hex chars)
	} else {
		// Just buffer the content without hashing
		if _, err := io.Copy(&buf, reader); err != nil {
			return "", fmt.Errorf("failed to read content: %w", err)
		}
	}

	// Generate artifact ID based on strategy
	ecosystem := strings.ToLower(info.Ecosystem.String())
	encodedName := encodeName(info.Name)

	switch sm.config.ArtifactIDStrategy {
	case ArtifactIDStrategyConvention:
		// Format: ecosystem:name:version
		artifactID = fmt.Sprintf("%s:%s:%s", ecosystem, encodedName, info.Version)

	case ArtifactIDStrategyContentHash:
		// Format: ecosystem:hash
		artifactID = fmt.Sprintf("%s:%s", ecosystem, contentHash)

	case ArtifactIDStrategyHybrid:
		// Format: ecosystem:name:version:hash
		// Use shorter hash (4 bytes = 8 hex chars) for hybrid
		artifactID = fmt.Sprintf("%s:%s:%s:%s",
			ecosystem, encodedName, info.Version, contentHash[:8])

	default:
		return "", fmt.Errorf("unknown artifact ID strategy: %d", sm.config.ArtifactIDStrategy)
	}

	// Check if already exists (deduplication)
	if sm.config.CacheEnabled {
		exists, err := sm.Exists(ctx, artifactID)
		if err == nil && exists {
			// Already exists, return the ID without re-storing
			return artifactID, nil
		}
	}

	// Store to backend
	key := sm.GetStorageKey(artifactID)
	if err := sm.storage.Put(key, &buf); err != nil {
		return "", fmt.Errorf("failed to store artifact: %w", err)
	}

	return artifactID, nil
}

// Get retrieves an artifact by ID
func (sm *storageManager) Get(ctx context.Context, artifactID string) (io.ReadCloser, error) {
	key := sm.GetStorageKey(artifactID)
	reader, err := sm.storage.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get artifact %s: %w", artifactID, err)
	}
	return reader, nil
}

// Exists checks if an artifact exists in storage
func (sm *storageManager) Exists(ctx context.Context, artifactID string) (bool, error) {
	key := sm.GetStorageKey(artifactID)

	// Try to get metadata first (faster)
	if sm.config.MetadataEnabled && sm.metadata != nil {
		_, err := sm.metadata.Get(ctx, artifactID)
		if err == nil {
			return true, nil
		}
	}

	// Check storage directly
	return sm.storage.Exists(ctx, key)
}

// StoreMetadata saves artifact metadata
func (sm *storageManager) StoreMetadata(ctx context.Context, metadata ArtifactMetadata) error {
	if !sm.config.MetadataEnabled || sm.metadata == nil {
		return nil
	}

	return sm.metadata.Put(ctx, metadata)
}

// GetMetadata retrieves artifact metadata
func (sm *storageManager) GetMetadata(ctx context.Context, artifactID string) (*ArtifactMetadata, error) {
	if !sm.config.MetadataEnabled || sm.metadata == nil {
		return nil, fmt.Errorf("metadata not enabled")
	}

	return sm.metadata.Get(ctx, artifactID)
}

// Delete removes an artifact and its metadata
func (sm *storageManager) Delete(ctx context.Context, artifactID string) error {
	key := sm.GetStorageKey(artifactID)

	// Delete from storage
	if err := sm.storage.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete artifact from storage: %w", err)
	}

	// Delete metadata
	if sm.config.MetadataEnabled && sm.metadata != nil {
		if err := sm.metadata.Delete(ctx, artifactID); err != nil {
			// Log warning but don't fail if metadata deletion fails
			return fmt.Errorf("failed to delete metadata: %w", err)
		}
	}

	return nil
}

// GetStorageKey returns the storage key for an artifact ID
// Handles different ID formats based on strategy:
// - Convention (3 parts): ecosystem:name:version -> artifacts/{ecosystem}/{name}/{version}/artifact
// - ContentHash (2 parts): ecosystem:hash -> artifacts/{ecosystem}/{hash}/artifact
// - Hybrid (4 parts): ecosystem:name:version:hash -> artifacts/{ecosystem}/{name}/{version}-{hash}/artifact
func (sm *storageManager) GetStorageKey(artifactID string) string {
	parts := strings.Split(artifactID, ":")

	var key string
	switch len(parts) {
	case 2:
		// ContentHash format: ecosystem:hash
		ecosystem := parts[0]
		hash := parts[1]
		key = filepath.Join("artifacts", ecosystem, hash, "artifact")

	case 3:
		// Convention format: ecosystem:name:version
		ecosystem := parts[0]
		name := parts[1]
		version := parts[2]
		key = filepath.Join("artifacts", ecosystem, name, version, "artifact")

	case 4:
		// Hybrid format: ecosystem:name:version:hash
		ecosystem := parts[0]
		name := parts[1]
		version := parts[2]
		hash := parts[3]
		// Combine version and hash with hyphen for single directory level
		versionHash := fmt.Sprintf("%s-%s", version, hash)
		key = filepath.Join("artifacts", ecosystem, name, versionHash, "artifact")

	default:
		// Invalid format, return as-is
		return artifactID
	}

	// Add prefix if configured
	if sm.config.KeyPrefix != "" {
		key = filepath.Join(sm.config.KeyPrefix, key)
	}

	return key
}

// ComputeArtifactID computes the artifact ID from content
func ComputeArtifactID(ecosystem string, reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	hashBytes := hash.Sum(nil)
	return fmt.Sprintf("%s:%s",
		strings.ToLower(ecosystem),
		hex.EncodeToString(hashBytes[:8])), nil
}

// ComputeSHA256 computes the full SHA256 checksum of content
func ComputeSHA256(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
