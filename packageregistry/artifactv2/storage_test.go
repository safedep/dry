package artifactv2

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageManager_Store(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts: true,
		CacheEnabled:     true,
		MetadataEnabled:  true,
	})

	ctx := context.Background()
	content := []byte("test artifact content")
	reader := bytes.NewReader(content)

	info := ArtifactInfo{
		Name:      "test-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	artifactID, err := sm.Store(ctx, info, reader)
	require.NoError(t, err)
	assert.NotEmpty(t, artifactID)

	// Verify ID format
	parts := strings.Split(artifactID, ":")
	assert.Len(t, parts, 2)
	assert.Equal(t, "ecosystem_npm", parts[0])
	assert.Len(t, parts[1], 16) // 8 bytes = 16 hex chars
}

func TestStorageManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts: true,
		CacheEnabled:     true,
		MetadataEnabled:  true,
	})

	ctx := context.Background()
	content := []byte("test artifact content for retrieval")
	reader := bytes.NewReader(content)

	info := ArtifactInfo{
		Name:      "test-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	artifactID, err := sm.Store(ctx, info, reader)
	require.NoError(t, err)

	// Get the artifact back
	retrieved, err := sm.Get(ctx, artifactID)
	require.NoError(t, err)
	defer retrieved.Close()

	retrievedContent, err := io.ReadAll(retrieved)
	require.NoError(t, err)
	assert.Equal(t, content, retrievedContent)
}

func TestStorageManager_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts: true,
		CacheEnabled:     true,
		MetadataEnabled:  true,
	})

	ctx := context.Background()

	// Non-existent artifact
	exists, err := sm.Exists(ctx, "npm:nonexistent123")
	require.NoError(t, err)
	assert.False(t, exists)

	// Store an artifact
	content := []byte("test artifact")
	reader := bytes.NewReader(content)
	info := ArtifactInfo{
		Name:      "test-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	artifactID, err := sm.Store(ctx, info, reader)
	require.NoError(t, err)

	// Should exist now
	exists, err = sm.Exists(ctx, artifactID)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestStorageManager_Metadata(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts: true,
		CacheEnabled:     true,
		MetadataEnabled:  true,
	})

	ctx := context.Background()

	testMetadata := ArtifactMetadata{
		ID:          "npm:abc123def456",
		Name:        "express",
		Version:     "4.17.1",
		Ecosystem:   packagev1.Ecosystem_ECOSYSTEM_NPM,
		Origin:      "https://registry.npmjs.org/express",
		SHA256:      "abcdef1234567890",
		Size:        12345,
		StorageKey:  "artifacts/npm/abc123def456/artifact",
		ContentType: "application/gzip",
	}

	// Store metadata
	err = sm.StoreMetadata(ctx, testMetadata)
	require.NoError(t, err)

	// Retrieve metadata
	retrieved, err := sm.GetMetadata(ctx, testMetadata.ID)
	require.NoError(t, err)
	assert.Equal(t, testMetadata.Name, retrieved.Name)
	assert.Equal(t, testMetadata.Version, retrieved.Version)
	assert.Equal(t, testMetadata.SHA256, retrieved.SHA256)
}

func TestStorageManager_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts: true,
		CacheEnabled:     true,
		MetadataEnabled:  true,
	})

	ctx := context.Background()
	content := []byte("test artifact to delete")
	reader := bytes.NewReader(content)

	info := ArtifactInfo{
		Name:      "test-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	artifactID, err := sm.Store(ctx, info, reader)
	require.NoError(t, err)

	// Store metadata
	testMetadata := ArtifactMetadata{
		ID:        artifactID,
		Name:      info.Name,
		Version:   info.Version,
		Ecosystem: info.Ecosystem,
	}
	err = sm.StoreMetadata(ctx, testMetadata)
	require.NoError(t, err)

	// Verify exists
	exists, err := sm.Exists(ctx, artifactID)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete
	err = sm.Delete(ctx, artifactID)
	require.NoError(t, err)

	// Should not exist anymore
	exists, err = sm.Exists(ctx, artifactID)
	require.NoError(t, err)
	assert.False(t, exists)

	// Metadata should be gone too
	_, err = sm.GetMetadata(ctx, artifactID)
	assert.Error(t, err)
}

func TestStorageManager_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts: true,
		CacheEnabled:     true,
		MetadataEnabled:  true,
	})

	ctx := context.Background()
	content := []byte("identical content")

	// Store same content twice with different names
	info1 := ArtifactInfo{
		Name:      "package-a",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	info2 := ArtifactInfo{
		Name:      "package-b",
		Version:   "2.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	id1, err := sm.Store(ctx, info1, bytes.NewReader(content))
	require.NoError(t, err)

	id2, err := sm.Store(ctx, info2, bytes.NewReader(content))
	require.NoError(t, err)

	// Should be the same ID (content-addressable)
	assert.Equal(t, id1, id2)
}

func TestComputeArtifactID(t *testing.T) {
	content := []byte("test content for hashing")
	reader := bytes.NewReader(content)

	id, err := ComputeArtifactID("npm", reader)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Verify format
	parts := strings.Split(id, ":")
	assert.Len(t, parts, 2)
	assert.Equal(t, "npm", parts[0])
	assert.Len(t, parts[1], 16)

	// Same content should produce same ID
	reader2 := bytes.NewReader(content)
	id2, err := ComputeArtifactID("npm", reader2)
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	// Different content should produce different ID
	reader3 := bytes.NewReader([]byte("different content"))
	id3, err := ComputeArtifactID("npm", reader3)
	require.NoError(t, err)
	assert.NotEqual(t, id, id3)
}

func TestComputeSHA256(t *testing.T) {
	content := []byte("test content")
	reader := bytes.NewReader(content)

	checksum, err := ComputeSHA256(reader)
	require.NoError(t, err)
	assert.NotEmpty(t, checksum)
	assert.Len(t, checksum, 64) // SHA256 = 32 bytes = 64 hex chars

	// Same content should produce same checksum
	reader2 := bytes.NewReader(content)
	checksum2, err := ComputeSHA256(reader2)
	require.NoError(t, err)
	assert.Equal(t, checksum, checksum2)
}
