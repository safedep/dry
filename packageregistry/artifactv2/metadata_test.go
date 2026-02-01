package artifactv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryMetadataStore_Put(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	metadata := ArtifactMetadata{
		ID:        "npm:abc123def456",
		Name:      "express",
		Version:   "4.17.1",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		Origin:    "https://registry.npmjs.org/express",
		SHA256:    "abcdef1234567890",
		Size:      12345,
		FetchedAt: time.Now(),
	}

	err := store.Put(ctx, metadata)
	require.NoError(t, err)
}

func TestInMemoryMetadataStore_PutRequiresID(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	metadata := ArtifactMetadata{
		Name:    "express",
		Version: "4.17.1",
		// Missing ID
	}

	err := store.Put(ctx, metadata)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID is required")
}

func TestInMemoryMetadataStore_Get(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	metadata := ArtifactMetadata{
		ID:        "npm:abc123def456",
		Name:      "express",
		Version:   "4.17.1",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		SHA256:    "abcdef1234567890",
	}

	err := store.Put(ctx, metadata)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := store.Get(ctx, metadata.ID)
	require.NoError(t, err)
	assert.Equal(t, metadata.ID, retrieved.ID)
	assert.Equal(t, metadata.Name, retrieved.Name)
	assert.Equal(t, metadata.Version, retrieved.Version)
	assert.Equal(t, metadata.SHA256, retrieved.SHA256)
}

func TestInMemoryMetadataStore_GetNotFound(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "npm:nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryMetadataStore_GetByArtifact(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	metadata := ArtifactMetadata{
		ID:        "npm:express:4.17.1",
		Name:      "express",
		Version:   "4.17.1",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	err := store.Put(ctx, metadata)
	require.NoError(t, err)

	// Get by ArtifactInfo
	info := ArtifactInfo{
		Name:      "express",
		Version:   "4.17.1",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}
	retrieved, err := store.GetByArtifact(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, metadata.ID, retrieved.ID)
	assert.Equal(t, metadata.Name, retrieved.Name)
	assert.Equal(t, metadata.Version, retrieved.Version)
}

func TestInMemoryMetadataStore_GetByArtifactNotFound(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	info := ArtifactInfo{
		Name:      "nonexistent",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}
	_, err := store.GetByArtifact(ctx, info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryMetadataStore_GetByArtifactScopedPackage(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	metadata := ArtifactMetadata{
		ID:        "npm:angular-core:12.0.0",
		Name:      "@angular/core",
		Version:   "12.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	err := store.Put(ctx, metadata)
	require.NoError(t, err)

	// Get by ArtifactInfo with scoped package name
	info := ArtifactInfo{
		Name:      "@angular/core",
		Version:   "12.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}
	retrieved, err := store.GetByArtifact(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, metadata.ID, retrieved.ID)
	assert.Equal(t, "@angular/core", retrieved.Name)
}

func TestInMemoryMetadataStore_Delete(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	metadata := ArtifactMetadata{
		ID:        "npm:abc123def456",
		Name:      "express",
		Version:   "4.17.1",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	err := store.Put(ctx, metadata)
	require.NoError(t, err)

	// Delete
	err = store.Delete(ctx, metadata.ID)
	require.NoError(t, err)

	// Should not be found anymore
	_, err = store.Get(ctx, metadata.ID)
	assert.Error(t, err)

	// Should not be found by artifact info either
	info := ArtifactInfo{
		Name:      "express",
		Version:   "4.17.1",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}
	_, err = store.GetByArtifact(ctx, info)
	assert.Error(t, err)
}

func TestInMemoryMetadataStore_List(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	// Store multiple artifacts
	artifacts := []ArtifactMetadata{
		{
			ID:        "npm:abc123",
			Name:      "express",
			Version:   "4.17.1",
			Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		},
		{
			ID:        "npm:def456",
			Name:      "lodash",
			Version:   "4.17.21",
			Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		},
		{
			ID:        "pypi:ghi789",
			Name:      "django",
			Version:   "3.2.0",
			Ecosystem: packagev1.Ecosystem_ECOSYSTEM_PYPI,
		},
	}

	for _, artifact := range artifacts {
		err := store.Put(ctx, artifact)
		require.NoError(t, err)
	}

	// List all
	results, err := store.List(ctx, MetadataQuery{})
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// List by ecosystem
	results, err = store.List(ctx, MetadataQuery{
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, packagev1.Ecosystem_ECOSYSTEM_NPM, r.Ecosystem)
	}

	// List by name
	results, err = store.List(ctx, MetadataQuery{
		Name: "express",
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "express", results[0].Name)

	// List by name and version
	results, err = store.List(ctx, MetadataQuery{
		Name:    "lodash",
		Version: "4.17.21",
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "lodash", results[0].Name)
	assert.Equal(t, "4.17.21", results[0].Version)
}

func TestInMemoryMetadataStore_ListPagination(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	// Store multiple artifacts
	for i := 0; i < 10; i++ {
		metadata := ArtifactMetadata{
			ID:        fmt.Sprintf("npm:test%d", i),
			Name:      fmt.Sprintf("package-%d", i),
			Version:   "1.0.0",
			Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		}
		err := store.Put(ctx, metadata)
		require.NoError(t, err)
	}

	// Test limit
	results, err := store.List(ctx, MetadataQuery{
		Limit: 5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 5)

	// Test offset
	results, err = store.List(ctx, MetadataQuery{
		Offset: 5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 5)

	// Test limit + offset
	results, err = store.List(ctx, MetadataQuery{
		Limit:  3,
		Offset: 7,
	})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestInMemoryMetadataStore_Concurrent(t *testing.T) {
	store := NewInMemoryMetadataStore()
	ctx := context.Background()

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			metadata := ArtifactMetadata{
				ID:        fmt.Sprintf("npm:test%d", idx),
				Name:      fmt.Sprintf("package-%d", idx),
				Version:   "1.0.0",
				Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
			}
			err := store.Put(ctx, metadata)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all were stored
	results, err := store.List(ctx, MetadataQuery{})
	require.NoError(t, err)
	assert.Len(t, results, 10)
}
