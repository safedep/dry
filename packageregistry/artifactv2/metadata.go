package artifactv2

import (
	"context"
	"fmt"
	"sync"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// inMemoryMetadataStore implements MetadataStore using in-memory storage
type inMemoryMetadataStore struct {
	mu        sync.RWMutex
	byID      map[string]ArtifactMetadata
	byPackage map[string]string // "ecosystem:name:version" -> artifactID
}

// NewInMemoryMetadataStore creates a new in-memory metadata store
func NewInMemoryMetadataStore() MetadataStore {
	return &inMemoryMetadataStore{
		byID:      make(map[string]ArtifactMetadata),
		byPackage: make(map[string]string),
	}
}

// Put stores metadata for an artifact
func (m *inMemoryMetadataStore) Put(ctx context.Context, metadata ArtifactMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate required fields
	if metadata.ID == "" {
		return fmt.Errorf("artifact ID is required")
	}

	// Store by ID
	m.byID[metadata.ID] = metadata

	// Store by package lookup key
	if metadata.Name != "" && metadata.Version != "" {
		key := makePackageKey(metadata.Ecosystem, metadata.Name, metadata.Version)
		m.byPackage[key] = metadata.ID
	}

	return nil
}

// Get retrieves metadata by artifact ID
func (m *inMemoryMetadataStore) Get(ctx context.Context, artifactID string) (*ArtifactMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metadata, ok := m.byID[artifactID]
	if !ok {
		return nil, fmt.Errorf("metadata not found for artifact: %s", artifactID)
	}

	return &metadata, nil
}

// GetByArtifact retrieves metadata using ArtifactInfo
// This method uses the package lookup index to find metadata
func (m *inMemoryMetadataStore) GetByArtifact(ctx context.Context, info ArtifactInfo) (*ArtifactMetadata, error) {
	key := makePackageKey(info.Ecosystem, info.Name, info.Version)

	// First attempt: try with read lock
	m.mu.RLock()
	artifactID, ok := m.byPackage[key]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("artifact not found: %s", key)
	}

	metadata, ok := m.byID[artifactID]
	if ok {
		m.mu.RUnlock()
		return &metadata, nil
	}

	// Inconsistent state detected: artifactID exists in byPackage but not in byID
	// Release read lock and acquire write lock to clean up
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check the state under write lock (another goroutine may have fixed it)
	artifactID, ok = m.byPackage[key]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %s", key)
	}

	metadata, ok = m.byID[artifactID]
	if ok {
		return &metadata, nil
	}

	// Still inconsistent, remove the dangling reference
	delete(m.byPackage, key)
	return nil, fmt.Errorf("metadata not found for artifact: %s (removed dangling reference)", artifactID)
}

// Delete removes metadata
func (m *inMemoryMetadataStore) Delete(ctx context.Context, artifactID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get metadata to find package key
	metadata, ok := m.byID[artifactID]
	if ok {
		// Remove package lookup key
		if metadata.Name != "" && metadata.Version != "" {
			key := makePackageKey(metadata.Ecosystem, metadata.Name, metadata.Version)
			delete(m.byPackage, key)
		}
	}

	// Remove by ID
	delete(m.byID, artifactID)

	return nil
}

// List returns metadata matching a query
func (m *inMemoryMetadataStore) List(ctx context.Context, query MetadataQuery) ([]ArtifactMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []ArtifactMetadata

	for _, metadata := range m.byID {
		// Apply filters
		if query.Ecosystem != packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED && metadata.Ecosystem != query.Ecosystem {
			continue
		}
		if query.Name != "" && metadata.Name != query.Name {
			continue
		}
		if query.Version != "" && metadata.Version != query.Version {
			continue
		}

		results = append(results, metadata)
	}

	// Apply pagination
	if query.Offset > 0 {
		if query.Offset >= len(results) {
			return []ArtifactMetadata{}, nil
		}
		results = results[query.Offset:]
	}

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// makePackageKey creates a lookup key for package name and version
func makePackageKey(ecosystem packagev1.Ecosystem, name, version string) string {
	return fmt.Sprintf("%s:%s:%s", ecosystem.String(), name, version)
}
