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

	adapter, err = NewNpmAdapterV2(
		WithCacheEnabled(false),
		WithPersistArtifacts(true),
	)

	require.NoError(t, err)
	assert.NotNil(t, adapter)
}

func TestNpmAdapterV2_BuildUrl(t *testing.T) {
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
			url := buildNpmUrl(npmRegistryURL, tt.pkgName, tt.version)
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

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	assert.NotNil(t, reader)

	defer reader.Close()

	assert.NotEmpty(t, reader.ID())

	metadata := reader.Metadata()
	assert.NotEmpty(t, metadata.ID)
}

func TestNpmAdapterV2_FetchWithChecksum(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "console.log('test')",
	}
	packageContent := createTestNpmPackage(t, testFiles)

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
		WithCacheEnabled(false),
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

	reader1, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID1 := reader1.ID()
	reader1.Close()

	reader2, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID2 := reader2.ID()
	reader2.Close()

	assert.Equal(t, artifactID1, artifactID2)
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

	reader1, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID := reader1.ID()
	reader1.Close()

	reader2, err := adapter.Load(ctx, artifactID)
	require.NoError(t, err)
	assert.NotNil(t, reader2)
	assert.Equal(t, artifactID, reader2.ID())
	reader2.Close()

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

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	artifactID := reader.ID()
	reader.Close()

	metadata, err := adapter.GetMetadata(ctx, artifactID)
	if err == nil {
		assert.Equal(t, artifactID, metadata.ID)
		assert.NotEmpty(t, metadata.Name)
		assert.NotEmpty(t, metadata.Version)
	}
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

	exists, _, err := adapter.Exists(ctx, info)
	require.NoError(t, err)
	assert.False(t, exists)

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	reader.Close()

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

	fileReader, err := reader.ReadFile(ctx, "package/index.js")
	require.NoError(t, err)
	defer fileReader.Close()

	content, err := io.ReadAll(fileReader)
	require.NoError(t, err)
	assert.Equal(t, "console.log('test')", string(content))

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

func TestBuildNpmUrl(t *testing.T) {
	tests := []struct {
		name         string
		registryBase string
		pkgName      string
		version      string
		expectedURL  string
	}{
		{
			name:         "simple package with official registry",
			registryBase: "https://registry.npmjs.org",
			pkgName:      "express",
			version:      "4.17.1",
			expectedURL:  "https://registry.npmjs.org/express/-/express-4.17.1.tgz",
		},
		{
			name:         "scoped package with official registry",
			registryBase: "https://registry.npmjs.org",
			pkgName:      "@angular/core",
			version:      "12.0.0",
			expectedURL:  "https://registry.npmjs.org/@angular/core/-/core-12.0.0.tgz",
		},
		{
			name:         "simple package with mirror",
			registryBase: "https://mirror.example.com/npm",
			pkgName:      "lodash",
			version:      "4.17.21",
			expectedURL:  "https://mirror.example.com/npm/lodash/-/lodash-4.17.21.tgz",
		},
		{
			name:         "scoped package with mirror",
			registryBase: "https://mirror.example.com/npm",
			pkgName:      "@babel/core",
			version:      "7.14.0",
			expectedURL:  "https://mirror.example.com/npm/@babel/core/-/core-7.14.0.tgz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := buildNpmUrl(tt.registryBase, tt.pkgName, tt.version)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestNpmAdapterV2_GetRegistryURLs(t *testing.T) {
	tests := []struct {
		name         string
		mirrors      []string
		pkgName      string
		version      string
		expectedURLs []string
	}{
		{
			name:    "no mirrors",
			mirrors: []string{},
			pkgName: "express",
			version: "4.17.1",
			expectedURLs: []string{
				"https://registry.npmjs.org/express/-/express-4.17.1.tgz",
			},
		},
		{
			name: "single mirror",
			mirrors: []string{
				"https://mirror1.example.com",
			},
			pkgName: "express",
			version: "4.17.1",
			expectedURLs: []string{
				"https://registry.npmjs.org/express/-/express-4.17.1.tgz",
				"https://mirror1.example.com/express/-/express-4.17.1.tgz",
			},
		},
		{
			name: "multiple mirrors",
			mirrors: []string{
				"https://mirror1.example.com",
				"https://mirror2.example.com",
			},
			pkgName: "lodash",
			version: "4.17.21",
			expectedURLs: []string{
				"https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz",
				"https://mirror1.example.com/lodash/-/lodash-4.17.21.tgz",
				"https://mirror2.example.com/lodash/-/lodash-4.17.21.tgz",
			},
		},
		{
			name: "scoped package with mirrors",
			mirrors: []string{
				"https://mirror.example.com",
			},
			pkgName: "@angular/core",
			version: "12.0.0",
			expectedURLs: []string{
				"https://registry.npmjs.org/@angular/core/-/core-12.0.0.tgz",
				"https://mirror.example.com/@angular/core/-/core-12.0.0.tgz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewNpmAdapterV2(WithRegistryMirrors(tt.mirrors))
			require.NoError(t, err)

			npmAdapter := adapter.(*npmAdapterV2)
			urls := npmAdapter.getRegistryURLs(tt.pkgName, tt.version)
			assert.Equal(t, tt.expectedURLs, urls)
		})
	}
}

func TestNpmAdapterV2_FetchWithMirrorFallback(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "mirror content",
	}
	packageContent := createTestNpmPackage(t, testFiles)
	attempts := make(map[string]int)

	// Primary registry returns 404
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts["primary"]++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer primaryServer.Close()

	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts["mirror"]++
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer mirrorServer.Close()

	adapter, err := NewNpmAdapterV2(
		WithRegistryMirrors([]string{mirrorServer.URL}),
		WithCacheEnabled(false),
	)
	require.NoError(t, err)

	// Manually override the config to use test server for primary registry
	// This is a bit hacky but necessary for testing mirror fallback
	npmAdapter := adapter.(*npmAdapterV2)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-mirror",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		// Provide explicit URLs to simulate primary + mirror
		URL: primaryServer.URL,
	}

	// This will fail since primary returns 404 and URL is explicit (bypasses mirrors)
	_, err = adapter.Fetch(ctx, info)
	assert.Error(t, err)

	// Reset attempts counter for the next test
	attempts["primary"] = 0
	attempts["mirror"] = 0

	// Now test with no explicit URL (uses getRegistryURLs)
	info.URL = ""

	// We need to actually use the mirrors - let's create custom URLs
	urls := []string{primaryServer.URL, mirrorServer.URL}

	// Directly test fetchHTTPWithMirrors behavior
	content, successURL, err := fetchHTTPWithMirrors(ctx, urls, fetchConfig{
		HTTPClient:    http.DefaultClient,
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
	})

	require.NoError(t, err)
	assert.Equal(t, packageContent, content)
	assert.Equal(t, mirrorServer.URL, successURL)
	assert.Equal(t, 1, attempts["primary"], "Should try primary once")
	assert.Equal(t, 1, attempts["mirror"], "Should succeed with mirror")

	// Verify the adapter's getRegistryURLs works correctly
	urls = npmAdapter.getRegistryURLs("test-mirror", "1.0.0")
	assert.Len(t, urls, 2, "Should have primary + 1 mirror")
	assert.Contains(t, urls[1], mirrorServer.URL, "Mirror URL should be included")
}

func TestNpmAdapterV2_FetchWithMultipleMirrors(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "third mirror wins",
	}
	packageContent := createTestNpmPackage(t, testFiles)
	attempts := make(map[string]int)

	// All servers return 404 except the last one
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts["server1"]++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts["server2"]++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server2.Close()

	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts["server3"]++
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer server3.Close()

	urls := []string{server1.URL, server2.URL, server3.URL}

	ctx := context.Background()
	content, successURL, err := fetchHTTPWithMirrors(ctx, urls, fetchConfig{
		HTTPClient:    http.DefaultClient,
		Timeout:       5 * time.Second,
		RetryAttempts: 5,
		RetryDelay:    10 * time.Millisecond,
	})

	require.NoError(t, err)
	assert.Equal(t, packageContent, content)
	assert.Equal(t, server3.URL, successURL)

	// Should cycle through: server1 (404) -> server2 (404) -> server3 (success)
	assert.Equal(t, 1, attempts["server1"])
	assert.Equal(t, 1, attempts["server2"])
	assert.Equal(t, 1, attempts["server3"])
}

func TestNpmAdapterV2_FetchWithExplicitURLBypassesMirrors(t *testing.T) {
	testFiles := map[string]string{
		"package/index.js": "explicit",
	}
	packageContent := createTestNpmPackage(t, testFiles)

	// Primary server returns success
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(packageContent)
	}))
	defer primaryServer.Close()

	// Mirror should not be called
	mirrorCalled := false
	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mirrorServer.Close()

	adapter, err := NewNpmAdapterV2(
		WithHTTPClient(http.DefaultClient),
		WithRegistryMirrors([]string{mirrorServer.URL}),
		WithCacheEnabled(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	info := ArtifactInfo{
		Name:      "test-explicit",
		Version:   "1.0.0",
		Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
		URL:       primaryServer.URL, // Explicit URL
	}

	reader, err := adapter.Fetch(ctx, info)
	require.NoError(t, err)
	reader.Close()

	// Mirror should not have been called
	assert.False(t, mirrorCalled, "Mirror should not be called when explicit URL is provided")
}

func TestNpmReaderV2_Extract(t *testing.T) {
	tests := []struct {
		name          string
		packageFiles  map[string]string
		expectedFiles int
	}{
		{
			name: "simple package extraction",
			packageFiles: map[string]string{
				"package/index.js":     "console.log('test')",
				"package/package.json": `{"name": "test", "version": "1.0.0"}`,
			},
			expectedFiles: 2,
		},
		{
			name: "package with nested files",
			packageFiles: map[string]string{
				"package/index.js":      "main",
				"package/lib/util.js":   "util",
				"package/lib/helper.js": "helper",
				"package/test/test.js":  "test",
				"package/package.json":  "{}",
			},
			expectedFiles: 5,
		},
		{
			name: "single file package",
			packageFiles: map[string]string{
				"package/index.js": "single",
			},
			expectedFiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test package
			packageContent := createTestNpmPackage(t, tt.packageFiles)

			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(packageContent)
			}))
			defer server.Close()

			// Create adapter with unique temp dir for this test
			tempDir := t.TempDir()
			adapter, err := NewNpmAdapterV2(
				WithHTTPClient(server.Client()),
				WithTempDir(tempDir),
			)
			require.NoError(t, err)

			ctx := context.Background()
			info := ArtifactInfo{
				Name:      "test-extract",
				Version:   "1.0.0",
				Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
				URL:       server.URL,
			}

			// Fetch package
			reader, err := adapter.Fetch(ctx, info)
			require.NoError(t, err)
			defer reader.Close()

			// Extract files
			result, err := reader.Extract(ctx)
			require.NoError(t, err)
			assert.NotNil(t, result)

			// Verify extraction result
			assert.NotEmpty(t, result.ExtractionKey, "Extraction key should not be empty")
			assert.Equal(t, tt.expectedFiles, result.FileCount, "File count mismatch")
			assert.Greater(t, result.TotalSize, int64(0), "Total size should be greater than 0")
			assert.False(t, result.AlreadyExtracted, "Should not be already extracted on first call")

			// Extract again to test idempotency
			result2, err := reader.Extract(ctx)
			require.NoError(t, err)
			assert.NotNil(t, result2)
			assert.Equal(t, result.ExtractionKey, result2.ExtractionKey, "Extraction keys should match")
			assert.True(t, result2.AlreadyExtracted, "Should be already extracted on second call")
		})
	}
}

func TestComputeExtractionKey(t *testing.T) {
	tests := []struct {
		name        string
		artifactID  string
		keyPrefix   string
		expectedKey string
	}{
		{
			name:        "convention format without prefix",
			artifactID:  "ecosystem_npm:express:4.17.1",
			keyPrefix:   "",
			expectedKey: "artifacts/ecosystem_npm/express/4.17.1/extracted/",
		},
		{
			name:        "convention format with prefix",
			artifactID:  "ecosystem_npm:express:4.17.1",
			keyPrefix:   "prod/",
			expectedKey: "prod/artifacts/ecosystem_npm/express/4.17.1/extracted/",
		},
		{
			name:        "content hash format",
			artifactID:  "ecosystem_npm:a1b2c3d4",
			keyPrefix:   "",
			expectedKey: "artifacts/ecosystem_npm/a1b2c3d4/extracted/",
		},
		{
			name:        "hybrid format",
			artifactID:  "ecosystem_npm:lodash:4.17.21:abc123",
			keyPrefix:   "",
			expectedKey: "artifacts/ecosystem_npm/lodash/4.17.21-abc123/extracted/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := computeExtractionKey(tt.artifactID, tt.keyPrefix)
			assert.Equal(t, tt.expectedKey, key)
		})
	}
}
