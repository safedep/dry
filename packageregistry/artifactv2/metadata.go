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

// GetByPackage retrieves metadata by package name and version
func (m *inMemoryMetadataStore) GetByPackage(ctx context.Context, ecosystem packagev1.Ecosystem, name, version string) (*ArtifactMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := makePackageKey(ecosystem, name, version)
	artifactID, ok := m.byPackage[key]
	if !ok {
		return nil, fmt.Errorf("package not found: %s", key)
	}

	metadata, ok := m.byID[artifactID]
	if !ok {
		// Inconsistent state, remove the dangling reference
		m.mu.RUnlock()
		m.mu.Lock()
		delete(m.byPackage, key)
		m.mu.Unlock()
		m.mu.RLock()
		return nil, fmt.Errorf("metadata not found for artifact: %s", artifactID)
	}

	return &metadata, nil
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
