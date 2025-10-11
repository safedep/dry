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

// Store saves an artifact to storage and returns its ID
func (sm *storageManager) Store(ctx context.Context, info ArtifactInfo, reader io.Reader) (string, error) {
	// Step 1: Read content and compute hash
	hash := sha256.New()
	var buf bytes.Buffer
	tee := io.TeeReader(reader, &buf)

	if _, err := io.Copy(hash, tee); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	// Step 2: Generate artifact ID from ecosystem and hash
	hashBytes := hash.Sum(nil)
	artifactID := fmt.Sprintf("%s:%s",
		strings.ToLower(info.Ecosystem.String()),
		hex.EncodeToString(hashBytes[:8])) // Use first 8 bytes (16 hex chars)

	// Step 3: Check if already exists (deduplication)
	if sm.config.CacheEnabled {
		exists, err := sm.Exists(ctx, artifactID)
		if err == nil && exists {
			// Already exists, return the ID without re-storing
			return artifactID, nil
		}
	}

	// Step 4: Store to backend
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
func (sm *storageManager) GetStorageKey(artifactID string) string {
	parts := strings.Split(artifactID, ":")
	if len(parts) != 2 {
		// Invalid artifact ID format, return as-is
		return artifactID
	}

	ecosystem := parts[0]
	hash := parts[1]

	// Build hierarchical key: artifacts/{ecosystem}/{hash}/artifact
	key := filepath.Join("artifacts", ecosystem, hash, "artifact")

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
