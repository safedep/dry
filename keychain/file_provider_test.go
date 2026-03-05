package keychain

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFileProvider(t *testing.T) *fileProvider {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-app", "creds.json")
	fp, err := newFileProvider("test-app", filePath)
	require.NoError(t, err)
	return fp
}

func TestFileProviderSetAndGet(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.set(ctx, "api-token", &Secret{Value: "sk-abc123"})
	require.NoError(t, err)

	secret, err := fp.get(ctx, "api-token")
	require.NoError(t, err)
	assert.Equal(t, "sk-abc123", secret.Value)
}

func TestFileProviderGetNotFound(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	_, err := fp.get(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileProviderGetFromEmptyStore(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	// No file exists yet
	_, err := fp.get(ctx, "anything")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileProviderSetOverwrite(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.set(ctx, "token", &Secret{Value: "v1"})
	require.NoError(t, err)

	err = fp.set(ctx, "token", &Secret{Value: "v2"})
	require.NoError(t, err)

	secret, err := fp.get(ctx, "token")
	require.NoError(t, err)
	assert.Equal(t, "v2", secret.Value)
}

func TestFileProviderDelete(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.set(ctx, "token", &Secret{Value: "v1"})
	require.NoError(t, err)

	err = fp.delete(ctx, "token")
	require.NoError(t, err)

	_, err = fp.get(ctx, "token")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileProviderDeleteNotFound(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileProviderDeleteFromNoFile(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.delete(ctx, "anything")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFileProviderMultipleKeys(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.set(ctx, "key1", &Secret{Value: "val1"})
	require.NoError(t, err)
	err = fp.set(ctx, "key2", &Secret{Value: "val2"})
	require.NoError(t, err)

	s1, err := fp.get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "val1", s1.Value)

	s2, err := fp.get(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, "val2", s2.Value)

	// Delete one, other still exists
	err = fp.delete(ctx, "key1")
	require.NoError(t, err)

	_, err = fp.get(ctx, "key1")
	assert.ErrorIs(t, err, ErrNotFound)

	s2, err = fp.get(ctx, "key2")
	require.NoError(t, err)
	assert.Equal(t, "val2", s2.Value)
}

func TestFileProviderFilePermissions(t *testing.T) {
	fp := newTestFileProvider(t)
	ctx := context.Background()

	err := fp.set(ctx, "token", &Secret{Value: "secret"})
	require.NoError(t, err)

	info, err := os.Stat(fp.filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(filePermissions), info.Mode().Perm())

	dirInfo, err := os.Stat(filepath.Dir(fp.filePath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(dirPermissions), dirInfo.Mode().Perm())
}

func TestFileProviderDefaultPath(t *testing.T) {
	fp, err := newFileProvider("myapp", "")
	require.NoError(t, err)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expected := filepath.Join(homeDir, ".config", "myapp", "creds.json")
	assert.Equal(t, expected, fp.filePath)
}

func TestFileProviderClose(t *testing.T) {
	fp := newTestFileProvider(t)
	assert.NoError(t, fp.close())
}
