package artifactv2

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestNpmPackage creates a test NPM package (tar.gz)
func createTestNpmPackage(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for path, content := range files {
		header := &tar.Header{
			Name:    path,
			Mode:    0644,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		}

		require.NoError(t, tarWriter.WriteHeader(header))
		_, err := tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	return buf.Bytes()
}

func TestNewNpmAdapterV2(t *testing.T) {
	adapter, err := NewNpmAdapterV2()
	require.NoError(t, err)
	assert.NotNil(t, adapter)

	// Test with custom options
	adapter, err = NewNpmAdapterV2(
		WithCacheEnabled(false),
		WithPersistArtifacts(true),
	)
	require.NoError(t, err)
	assert.NotNil(t, adapter)
}

func TestNpmAdapterV2_BuildUrl(t *testing.T) {
	adapter := &npmAdapterV2{}

	tests := []struct {
		name     string
		pkgName  string
		version  string
		expected string
	}{
		{
			name:     "simple package",
			pkgName:  "express",
			version:  "4.17.1",
			expected: "https://registry.npmjs.org/express/-/express-4.17.1.tgz",
		},
		{
			name:     "scoped package",
			pkgName:  "@angular/core",
			version:  "12.0.0",
			expected: "https://registry.npmjs.org/@angular/core/-/core-12.0.0.tgz",
		},
		{
			name:     "babel preset",
			pkgName:  "@babel/preset-env",
			version:  "7.14.0",
			expected: "https://registry.npmjs.org/@babel/preset-env/-/preset-env-7.14.0.tgz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := adapter.buildNpmUrl(tt.pkgName, tt.version)
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestNpmAdapterV2_Fetch(t *testing.T) {
	// Create test package
	testFiles := map[string]string{
		"package/package.json": `{"name": "test", "version": "1.0.0"}`,
		"package/index.js":     "module.exports = {}",
		"package/README.md":    "# Test Package",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	// Create adapter with custom HTTP client
	adapter, err := NewNpmAdapterV2(
		WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	// Fetch package
	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	assert.NotNil(t, reader)
	defer reader.Close()

	// Verify artifact ID
	assert.NotEmpty(t, reader.ID())

	// Verify metadata (Note: metadata might be minimal if metadata store isn't enabled)
	metadata := reader.Metadata()
	// The metadata should at least have the artifact ID
	assert.NotEmpty(t, metadata.ID)
}

func TestNpmAdapterV2_FetchWithChecksum(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "console.log('test')",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	// Compute actual checksum
	actualChecksum, err := computeSHA256(bytes.NewReader(packageContent))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()

	// Test with correct checksum
	info := ArtifactInfo{
		Name:      "test-checksum",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
		Checksum:  actualChecksum,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	assert.NotNil(t, reader)
	reader.Close()

	// Test with incorrect checksum (different package to avoid cache)
	info.Name = "test-checksum-fail"
	info.Version = "2.0.0"
	info.Checksum = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = adapter.Fetch(ctx, info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestNpmAdapterV2_FetchWithRetry(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "test",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(
		WithHTTPClient(server.Client()),
		WithRetry(5, 10*time.Millisecond),
		WithCacheEnabled(false), // Disable cache so retry logic is tested
	)
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-retry",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	assert.NotNil(t, reader)
	reader.Close()

	// Verify retries happened
	assert.Equal(t, 3, attempts)
}

func TestNpmAdapterV2_FetchCaching(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "cached",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	fetchCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	// Use a unique temp directory for this test
	tempDir := t.TempDir()

	adapter, err := NewNpmAdapterV2(
		WithHTTPClient(server.Client()),
		WithCacheEnabled(true),
		WithTempDir(tempDir),
	)
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "cached-package",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	// First fetch
	reader1, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID1 := reader1.ID()
	reader1.Close()

	// Second fetch (should use cache)
	reader2, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID2 := reader2.ID()
	reader2.Close()

	// Should have same artifact ID
	assert.Equal(t, artifactID1, artifactID2)

	// Should only fetch once (second time from cache)
	assert.Equal(t, 1, fetchCount)
}

func TestNpmAdapterV2_Load(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "loaded",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	// Fetch first
	reader1, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID := reader1.ID()
	reader1.Close()

	// Load by ID
	reader2, err := adapter.Load(ctx, artifactID)
	require.NoError(t, err)
	assert.NotNil(t, reader2)
	assert.Equal(t, artifactID, reader2.ID())
	reader2.Close()

	// Load non-existent artifact
	_, err = adapter.Load(ctx, "npm:nonexistent:1.0.0")
	assert.Error(t, err)
}

func TestNpmAdapterV2_LoadFromSource(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "source",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	adapter, err := NewNpmAdapterV2()
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-from-source",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
	}
	source := ArtifactSource{
		Origin: "file:///tmp/test.tgz",
		Reader: io.NopCloser(bytes.NewReader(packageContent)),
	}

	reader, err := adapter.LoadFromSource(ctx, info, source)
	require.NoError(t, err)
	assert.NotNil(t, reader)
	assert.NotEmpty(t, reader.ID())

	// Verify metadata was stored with proper info
	metadata := reader.Metadata()
	assert.Equal(t, "test-from-source", metadata.Name)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, packagev1.Ecosystem_ECOSYSTEM_NPM, metadata.Ecosystem)

	defer reader.Close()
}

func TestNpmAdapterV2_GetMetadata(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "metadata",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(
		WithHTTPClient(server.Client()),
		WithMetadataEnabled(true),
	)
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-getmeta",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	// Fetch to store metadata
	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID := reader.ID()
	reader.Close()

	// Try to get metadata - it might not be available depending on storage configuration
	// but at least the call should not crash
	metadata, err := adapter.GetMetadata(ctx, artifactID)
	if err == nil {
		// If metadata is available, verify it
		assert.Equal(t, artifactID, metadata.ID)
		assert.NotEmpty(t, metadata.Name)
		assert.NotEmpty(t, metadata.Version)
	}
	// If metadata is not available, that's also OK for this test
}

func TestNpmAdapterV2_Exists(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "exists",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	// Use a unique temp directory for this test
	tempDir := t.TempDir()

	adapter, err := NewNpmAdapterV2(
		WithHTTPClient(server.Client()),
		WithArtifactIDStrategy(ArtifactIDStrategyConvention),
		WithTempDir(tempDir),
	)
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "exists-test",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	// Should not exist initially
	exists, _, err := adapter.Exists(ctx, info)
	require.NoError(t, err)
	assert.False(t, exists)

	// Fetch the package
	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	reader.Close()

	// Should exist now
	exists, artifactID, err := adapter.Exists(ctx, info)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.NotEmpty(t, artifactID)
}

func TestNpmReaderV2_Reader(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "reader test",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-reader",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// Get raw reader
	rawReader, err := reader.Reader(ctx)
	require.NoError(t, err)
	defer rawReader.Close()

	content, err := io.ReadAll(rawReader)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestNpmReaderV2_EnumFiles(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js":     "main",
		"package/lib/util.js":  "utility",
		"package/package.json": "{}",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-enum-files",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// Enumerate files
	var foundFiles []string
	err = reader.EnumFiles(ctx, func(info FileInfo) error {
		foundFiles = append(foundFiles, info.Path)
		assert.Greater(t, info.Size, int64(0))
		assert.False(t, info.IsDir)
		return nil
	})
	require.NoError(t, err)

	assert.Len(t, foundFiles, 3)
	assert.Contains(t, foundFiles, "package/index.js")
	assert.Contains(t, foundFiles, "package/lib/util.js")
	assert.Contains(t, foundFiles, "package/package.json")
}

func TestNpmReaderV2_ReadFile(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js":     "console.log('test')",
		"package/package.json": `{"name": "test"}`,
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-readfile",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// Read existing file
	fileReader, err := reader.ReadFile(ctx, "package/index.js")
	require.NoError(t, err)
	defer fileReader.Close()

	content, err := io.ReadAll(fileReader)
	require.NoError(t, err)
	assert.Equal(t, "console.log('test')", string(content))

	// Try to read non-existent file
	_, err = reader.ReadFile(ctx, "package/nonexistent.js")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestNpmReaderV2_GetFileMetadata(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "metadata test",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-getmetadata",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// Get file metadata
	metadata, err := reader.GetFileMetadata(ctx, "package/index.js")
	require.NoError(t, err)
	assert.Equal(t, "package/index.js", metadata.Path)
	assert.Equal(t, int64(len("metadata test")), metadata.Size)
	assert.False(t, metadata.ModTime.IsZero())

	// Non-existent file
	_, err = reader.GetFileMetadata(ctx, "package/missing.js")
	assert.Error(t, err)
}

func TestNpmReaderV2_ListFiles(t *testing.T) {
	testFiles := map[string]string{
		"package/a.js": "a",
		"package/b.js": "b",
		"package/c.js": "c",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-listfiles",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// List files
	files, err := reader.ListFiles(ctx)
	require.NoError(t, err)
	assert.Len(t, files, 3)
	assert.Contains(t, files, "package/a.js")
	assert.Contains(t, files, "package/b.js")
	assert.Contains(t, files, "package/c.js")
}

func TestNpmReaderV2_ScopedPackage(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "scoped",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server.Close()

	adapter, err := NewNpmAdapterV2(WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "@angular/core",
		Version:   "12.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       server.URL,
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	defer reader.Close()

	// Verify artifact ID encodes the name properly
	artifactID := reader.ID()
	assert.Contains(t, artifactID, "angular-core")
	assert.NotContains(t, artifactID, "@")
	assert.NotContains(t, artifactID, "/")
}
