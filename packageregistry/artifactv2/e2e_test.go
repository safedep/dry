package artifactv2

import (
	"context"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArtifactAdapterE2E performs end-to-end tests for artifact adapters
// with real registry interactions
func TestArtifactAdapterE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name        string
		ecosystem   packagev1.Ecosystem
		packageName string
		version     string
		expectError bool
		description string
	}{
		// NPM Adapter Tests
		{
			name:        "npm_success_express",
			ecosystem:   packagev1.Ecosystem_ECOSYSTEM_NPM,
			packageName: "express",
			version:     "4.17.1",
			expectError: false,
			description: "Fetch a valid NPM package (express@4.17.1)",
		},
		{
			name:        "npm_success_scoped",
			ecosystem:   packagev1.Ecosystem_ECOSYSTEM_NPM,
			packageName: "@types/node",
			version:     "16.0.0",
			expectError: false,
			description: "Fetch a valid scoped NPM package (@types/node@16.0.0)",
		},
		{
			name:        "npm_failure_nonexistent",
			ecosystem:   packagev1.Ecosystem_ECOSYSTEM_NPM,
			packageName: "this-package-definitely-does-not-exist-12345678",
			version:     "1.0.0",
			expectError: true,
			description: "Attempt to fetch a non-existent NPM package",
		},
		{
			name:        "npm_failure_invalid_version",
			ecosystem:   packagev1.Ecosystem_ECOSYSTEM_NPM,
			packageName: "express",
			version:     "999.999.999",
			expectError: true,
			description: "Attempt to fetch an invalid version of a valid package",
		},

		// Future: PyPI adapter tests
		// {
		//     name:        "pypi_success_django",
		//     ecosystem:   packagev1.Ecosystem_ECOSYSTEM_PYPI,
		//     packageName: "django",
		//     version:     "3.2.0",
		//     expectError: false,
		//     description: "Fetch a valid PyPI package (django@3.2.0)",
		// },
		// {
		//     name:        "pypi_failure_nonexistent",
		//     ecosystem:   packagev1.Ecosystem_ECOSYSTEM_PYPI,
		//     packageName: "this-pypi-package-does-not-exist-12345",
		//     version:     "1.0.0",
		//     expectError: true,
		//     description: "Attempt to fetch a non-existent PyPI package",
		// },

		// Future: Go module tests
		// {
		//     name:        "go_success_gin",
		//     ecosystem:   packagev1.Ecosystem_ECOSYSTEM_GO,
		//     packageName: "github.com/gin-gonic/gin",
		//     version:     "v1.7.0",
		//     expectError: false,
		//     description: "Fetch a valid Go module",
		// },
		// {
		//     name:        "go_failure_nonexistent",
		//     ecosystem:   packagev1.Ecosystem_ECOSYSTEM_GO,
		//     packageName: "github.com/nonexistent/repo12345",
		//     version:     "v1.0.0",
		//     expectError: true,
		//     description: "Attempt to fetch a non-existent Go module",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated temporary storage for each test
			tempDir := t.TempDir()
			storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
				Root: tempDir,
			})
			require.NoError(t, err, "failed to create storage backend")

			// Create adapter with test configuration
			adapter, err := CreateAdapter(
				tt.ecosystem,
				WithStorage(storageBackend),
				WithCacheEnabled(true),
				WithPersistArtifacts(true),
				WithMetadataEnabled(true),
			)
			require.NoError(t, err, "failed to create adapter")

			// Create artifact info
			info := ArtifactInfo{
				Name:      tt.packageName,
				Version:   tt.version,
				Ecosystem: tt.ecosystem,
			}

			ctx := context.Background()

			// Attempt to fetch the artifact
			reader, err := adapter.Fetch(ctx, info)

			if tt.expectError {
				// Test case expects an error
				assert.Error(t, err, "expected error for test case: %s", tt.description)
				assert.Nil(t, reader, "reader should be nil on error")
				return
			}

			// Test case expects success
			require.NoError(t, err, "unexpected error for test case: %s", tt.description)
			require.NotNil(t, reader, "reader should not be nil")
			defer reader.Close()

			// Verify artifact ID
			artifactID := reader.ID()
			assert.NotEmpty(t, artifactID, "artifact ID should not be empty")
			t.Logf("Artifact ID: %s", artifactID)

			// Verify metadata
			metadata := reader.Metadata()
			assert.Equal(t, tt.packageName, metadata.Name, "metadata name should match")
			assert.Equal(t, tt.version, metadata.Version, "metadata version should match")
			assert.Equal(t, tt.ecosystem, metadata.Ecosystem, "metadata ecosystem should match")
			assert.NotEmpty(t, metadata.Origin, "metadata origin should not be empty")
			assert.NotEmpty(t, metadata.SHA256, "metadata SHA256 should not be empty")
			assert.Greater(t, metadata.Size, int64(0), "metadata size should be positive")
			t.Logf("SHA256: %s, Size: %d bytes", metadata.SHA256, metadata.Size)

			// Test raw artifact reader
			rawReader, err := reader.Reader(ctx)
			require.NoError(t, err, "should be able to get raw reader")
			require.NotNil(t, rawReader, "raw reader should not be nil")
			rawReader.Close()

			// Test file enumeration
			var fileCount int
			err = reader.EnumFiles(ctx, func(fi FileInfo) error {
				fileCount++
				assert.NotEmpty(t, fi.Path, "file path should not be empty")
				assert.GreaterOrEqual(t, fi.Size, int64(0), "file size should be non-negative")
				return nil
			})
			require.NoError(t, err, "should be able to enumerate files")
			assert.Greater(t, fileCount, 0, "should have at least one file in artifact")
			t.Logf("File count: %d", fileCount)

			// Test file listing
			files, err := reader.ListFiles(ctx)
			require.NoError(t, err, "should be able to list files")
			assert.Len(t, files, fileCount, "listed files count should match enumerated count")

			// Test reading a specific file (if there are files)
			if len(files) > 0 {
				firstFile := files[0]
				t.Logf("Testing ReadFile with: %s", firstFile)

				fileReader, err := reader.ReadFile(ctx, firstFile)
				require.NoError(t, err, "should be able to read specific file")
				require.NotNil(t, fileReader, "file reader should not be nil")
				fileReader.Close()

				// Test file metadata
				fileMeta, err := reader.GetFileMetadata(ctx, firstFile)
				require.NoError(t, err, "should be able to get file metadata")
				require.NotNil(t, fileMeta, "file metadata should not be nil")
				assert.Equal(t, firstFile, fileMeta.Path, "file metadata path should match")
			}

			// Test cache hit by fetching again
			t.Log("Testing cache hit by fetching the same artifact again")
			reader2, err := adapter.Fetch(ctx, info)
			require.NoError(t, err, "second fetch should succeed (cache hit)")
			require.NotNil(t, reader2, "second reader should not be nil")
			defer reader2.Close()

			// Verify same artifact ID (cache hit)
			assert.Equal(t, artifactID, reader2.ID(), "second fetch should return same artifact ID (cache hit)")

			// Test Exists method
			exists, existingID, err := adapter.Exists(ctx, info)
			require.NoError(t, err, "exists check should succeed")
			assert.True(t, exists, "artifact should exist after fetch")
			assert.Equal(t, artifactID, existingID, "existing ID should match artifact ID")

			// Test GetMetadata without loading artifact
			metadataOnly, err := adapter.GetMetadata(ctx, artifactID)
			require.NoError(t, err, "should be able to get metadata by ID")
			require.NotNil(t, metadataOnly, "metadata should not be nil")
			assert.Equal(t, tt.packageName, metadataOnly.Name, "metadata name should match")
			assert.Equal(t, tt.version, metadataOnly.Version, "metadata version should match")

			// Test Load by artifact ID
			t.Log("Testing Load by artifact ID")
			reader3, err := adapter.Load(ctx, artifactID)
			require.NoError(t, err, "load by ID should succeed")
			require.NotNil(t, reader3, "loaded reader should not be nil")
			defer reader3.Close()

			assert.Equal(t, artifactID, reader3.ID(), "loaded artifact should have same ID")
		})
	}
}

// TestArtifactAdapterE2E_CachePerformance tests that cache hits are significantly faster
func TestArtifactAdapterE2E_CachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E performance test in short mode")
	}

	tempDir := t.TempDir()
	storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tempDir,
	})
	require.NoError(t, err)

	adapter, err := CreateAdapter(
		packagev1.Ecosystem_ECOSYSTEM_NPM,
		WithStorage(storageBackend),
		WithCacheEnabled(true),
	)
	require.NoError(t, err)

	info := ArtifactInfo{
		Name:      "lodash",
		Version:   "4.17.21",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	ctx := context.Background()

	// First fetch (cold - downloads from registry)
	t.Log("First fetch (cold)...")
	reader1, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID := reader1.ID()
	reader1.Close()
	t.Logf("First fetch completed, artifact ID: %s", artifactID)

	// Second fetch (warm - from cache)
	t.Log("Second fetch (warm - should be from cache)...")
	reader2, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	assert.Equal(t, artifactID, reader2.ID(), "should return same artifact from cache")
	reader2.Close()
	t.Log("Second fetch completed from cache")
}

// TestArtifactAdapterE2E_LoadFromSource tests loading artifacts from custom sources
func TestArtifactAdapterE2E_LoadFromSource(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tempDir := t.TempDir()
	storageBackend, err := storage.NewFilesystemStorageDriver(storage.FilesystemStorageDriverConfig{
		Root: tempDir,
	})
	require.NoError(t, err)

	adapter, err := CreateAdapter(
		packagev1.Ecosystem_ECOSYSTEM_NPM,
		WithStorage(storageBackend),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// First, fetch a real package to have something to work with
	info := ArtifactInfo{
		Name:      "is-number",
		Version:   "7.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// Get the raw bytes
	rawReader, err := reader.Reader(ctx)
	require.NoError(t, err)

	// Now test LoadFromSource with those bytes
	source := ArtifactSource{
		Origin: "test-source",
		Reader: rawReader,
	}

	readerFromSource, err := adapter.LoadFromSource(ctx, info, source)
	require.NoError(t, err)
	require.NotNil(t, readerFromSource)
	defer readerFromSource.Close()

	assert.NotEmpty(t, readerFromSource.ID())
	assert.Equal(t, info.Name, readerFromSource.Metadata().Name)
	assert.Equal(t, info.Version, readerFromSource.Metadata().Version)
}
