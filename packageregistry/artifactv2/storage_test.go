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

func TestStorageManager_StoreConvention(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
		ArtifactIDStrategy: ArtifactIDStrategyConvention, // Default strategy
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

	// Verify Convention ID format: ecosystem:name:version
	parts := strings.Split(artifactID, ":")
	assert.Len(t, parts, 3)
	assert.Equal(t, "ecosystem_npm", parts[0])
	assert.Equal(t, "test-package", parts[1])
	assert.Equal(t, "1.0.0", parts[2])

	// Verify storage key
	expectedKey := "artifacts/ecosystem_npm/test-package/1.0.0/artifact"
	assert.Equal(t, expectedKey, sm.GetStorageKey(artifactID))
}

func TestStorageManager_StoreContentHash(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
		ArtifactIDStrategy: ArtifactIDStrategyContentHash,
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

	// Verify ContentHash ID format: ecosystem:hash
	parts := strings.Split(artifactID, ":")
	assert.Len(t, parts, 2)
	assert.Equal(t, "ecosystem_npm", parts[0])
	assert.Len(t, parts[1], 16) // 8 bytes = 16 hex chars
}

func TestStorageManager_StoreHybrid(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
		ArtifactIDStrategy: ArtifactIDStrategyHybrid,
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

	// Verify Hybrid ID format: ecosystem:name:version:hash
	parts := strings.Split(artifactID, ":")
	assert.Len(t, parts, 4)
	assert.Equal(t, "ecosystem_npm", parts[0])
	assert.Equal(t, "test-package", parts[1])
	assert.Equal(t, "1.0.0", parts[2])
	assert.Len(t, parts[3], 8) // Shorter hash for hybrid (4 bytes = 8 hex chars)
}

func TestStorageManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
		ArtifactIDStrategy: ArtifactIDStrategyConvention,
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
		ArtifactIDStrategy: ArtifactIDStrategyConvention,
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
		ArtifactIDStrategy: ArtifactIDStrategyConvention,
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
		ArtifactIDStrategy: ArtifactIDStrategyConvention,
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

func TestStorageManager_DeduplicationContentHash(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		ArtifactIDStrategy: ArtifactIDStrategyContentHash, // Use ContentHash for deduplication
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
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

	// Should be the same ID because content is identical (content-addressable)
	assert.Equal(t, id1, id2)
}

func TestStorageManager_ConventionNoDuplication(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		ArtifactIDStrategy: ArtifactIDStrategyConvention, // Convention uses name:version
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
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

	// Should be different IDs because name/version are different (convention-based)
	assert.NotEqual(t, id1, id2)
	assert.Equal(t, "ecosystem_npm:package-a:1.0.0", id1)
	assert.Equal(t, "ecosystem_npm:package-b:2.0.0", id2)
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

func TestEncodeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "express", "express"},
		{"scoped npm", "@angular/core", "angular-core"},
		{"babel preset", "@babel/preset-env", "babel-preset-env"},
		{"go module", "github.com/gin-gonic/gin", "github.com-gin-gonic-gin"},
		{"go versioned", "github.com/user/repo/v2", "github.com-user-repo-v2"},
		{"multiple slashes", "a/b/c/d", "a-b-c-d"},
		{"already hyphenated", "my-package-name", "my-package-name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStorageManager_StoreScopedPackages(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tmpDir,
	})
	require.NoError(t, err)

	metadata := NewInMemoryMetadataStore()
	sm := NewStorageManager(store, metadata, StorageConfig{
		PersistArtifacts:   true,
		CacheEnabled:       true,
		MetadataEnabled:    true,
		ArtifactIDStrategy: ArtifactIDStrategyConvention,
	})

	tests := []struct {
		name           string
		packageName    string
		version        string
		expectedID     string
		expectedKey    string
	}{
		{
			name:        "scoped npm package",
			packageName: "@angular/core",
			version:     "12.0.0",
			expectedID:  "ecosystem_npm:angular-core:12.0.0",
			expectedKey: "artifacts/ecosystem_npm/angular-core/12.0.0/artifact",
		},
		{
			name:        "go module",
			packageName: "github.com/gin-gonic/gin",
			version:     "v1.7.0",
			expectedID:  "ecosystem_go:github.com-gin-gonic-gin:v1.7.0",
			expectedKey: "artifacts/ecosystem_go/github.com-gin-gonic-gin/v1.7.0/artifact",
		},
		{
			name:        "go versioned module",
			packageName: "github.com/user/repo/v2",
			version:     "v2.1.0",
			expectedID:  "ecosystem_go:github.com-user-repo-v2:v2.1.0",
			expectedKey: "artifacts/ecosystem_go/github.com-user-repo-v2/v2.1.0/artifact",
		},
	}

	ctx := context.Background()
	content := []byte("test artifact content")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(content)
			info := ArtifactInfo{
				Name:      tt.packageName,
				Version:   tt.version,
				Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
			}

			if strings.Contains(tt.packageName, "github.com") {
				info.Ecosystem = packagev1.Ecosystem_ECOSYSTEM_GO
			}

			artifactID, err := sm.Store(ctx, info, reader)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedID, artifactID)
			assert.Equal(t, tt.expectedKey, sm.GetStorageKey(artifactID))
		})
	}
}

func TestGetStorageKeyFormats(t *testing.T) {
	sm := &storageManager{
		config: StorageConfig{},
	}

	tests := []struct {
		name        string
		artifactID  string
		expectedKey string
	}{
		{
			name:        "convention format",
			artifactID:  "ecosystem_npm:express:4.17.1",
			expectedKey: "artifacts/ecosystem_npm/express/4.17.1/artifact",
		},
		{
			name:        "content hash format",
			artifactID:  "ecosystem_npm:a1b2c3d4e5f6g7h8",
			expectedKey: "artifacts/ecosystem_npm/a1b2c3d4e5f6g7h8/artifact",
		},
		{
			name:        "hybrid format",
			artifactID:  "ecosystem_npm:express:4.17.1:a1b2c3d4",
			expectedKey: "artifacts/ecosystem_npm/express/4.17.1-a1b2c3d4/artifact",
		},
		{
			name:        "scoped package",
			artifactID:  "ecosystem_npm:angular-core:12.0.0",
			expectedKey: "artifacts/ecosystem_npm/angular-core/12.0.0/artifact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := sm.GetStorageKey(tt.artifactID)
			assert.Equal(t, tt.expectedKey, key)
		})
	}
}
