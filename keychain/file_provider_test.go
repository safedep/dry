package keychain

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		t.Skip("skipping: Unix file permissions are not supported on Windows")
	}

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

// isolateDirs points every directory the file provider reads at a temp dir.
func isolateDirs(t *testing.T) (stateDir, legacyDir string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "local"))
	stateDir = filepath.Join(root, "state")
	t.Setenv("XDG_STATE_HOME", stateDir)

	legacyDir, err := legacyFallbackDir()
	require.NoError(t, err)
	return stateDir, legacyDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), dirPermissions))
	require.NoError(t, os.WriteFile(path, []byte(content), filePermissions))
}

func TestFileProviderDefaultPath(t *testing.T) {
	t.Run("absolute XDG_STATE_HOME", func(t *testing.T) {
		stateDir, _ := isolateDirs(t)

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(stateDir, "myapp", credsFileName), fp.filePath)
	})

	t.Run("relative XDG_STATE_HOME is ignored", func(t *testing.T) {
		isolateDirs(t)
		t.Setenv("XDG_STATE_HOME", "relative/state")

		platformDir, err := platformStateDir()
		require.NoError(t, err)

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(platformDir, "myapp", credsFileName), fp.filePath)
	})
}

func TestFileProviderDefaultPathWithoutHome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("only Linux resolves the config directory from XDG_CONFIG_HOME alone")
	}

	configDir := t.TempDir()
	t.Setenv("HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", configDir)

	fp, err := newFileProvider("myapp", "")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, "myapp", credsFileName), fp.filePath)
}

func TestFileProviderMovesLegacyFile(t *testing.T) {
	const store = `{"version":1,"secrets":{"default/api_key":{"Value":"sk-legacy"}}}`

	t.Run("legacy file only moves to the state directory", func(t *testing.T) {
		stateDir, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		writeFile(t, legacy, store)

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)

		want := filepath.Join(stateDir, "myapp", credsFileName)
		assert.Equal(t, want, fp.filePath)
		assert.NoFileExists(t, legacy)

		secret, err := fp.get(context.Background(), "default/api_key")
		require.NoError(t, err)
		assert.Equal(t, "sk-legacy", secret.Value)

		if runtime.GOOS != "windows" {
			info, err := os.Stat(want)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(filePermissions), info.Mode().Perm())
		}
	})

	t.Run("new file wins and the legacy file stays", func(t *testing.T) {
		stateDir, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		current := filepath.Join(stateDir, "myapp", credsFileName)
		writeFile(t, legacy, store)
		writeFile(t, current, `{"version":1,"secrets":{"default/api_key":{"Value":"sk-current"}}}`)

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, current, fp.filePath)
		assert.FileExists(t, legacy)

		secret, err := fp.get(context.Background(), "default/api_key")
		require.NoError(t, err)
		assert.Equal(t, "sk-current", secret.Value)
	})

	t.Run("a file another process writes first is not replaced", func(t *testing.T) {
		stateDir, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		current := filepath.Join(stateDir, "myapp", credsFileName)
		writeFile(t, legacy, store)

		const other = `{"version":1,"secrets":{"default/api_key":{"Value":"sk-other"}}}`
		linkFile = func(oldPath, newPath string) error {
			writeFile(t, newPath, other)
			return os.Link(oldPath, newPath)
		}
		t.Cleanup(func() { linkFile = os.Link })

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, current, fp.filePath)

		secret, err := fp.get(context.Background(), "default/api_key")
		require.NoError(t, err)
		assert.Equal(t, "sk-other", secret.Value)
		assert.FileExists(t, legacy)
	})

	t.Run("an old file replaced during the move is kept", func(t *testing.T) {
		stateDir, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		writeFile(t, legacy, store)

		const newer = `{"version":1,"secrets":{"default/api_key":{"Value":"sk-newer"}}}`
		linkFile = func(oldPath, newPath string) error {
			replacement := legacy + ".tmp"
			writeFile(t, replacement, newer)
			require.NoError(t, os.Rename(replacement, legacy))
			return os.Link(oldPath, newPath)
		}
		t.Cleanup(func() { linkFile = os.Link })

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(stateDir, "myapp", credsFileName), fp.filePath)

		data, err := os.ReadFile(legacy)
		require.NoError(t, err)
		assert.Equal(t, newer, string(data))
	})

	t.Run("a file of another user stays in place", func(t *testing.T) {
		stateDir, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		writeFile(t, legacy, store)

		fileOwnedByCurrentUser = func(os.FileInfo) bool { return false }
		t.Cleanup(func() { fileOwnedByCurrentUser = ownedByCurrentUser })

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, legacy, fp.filePath)
		assert.FileExists(t, legacy)
		assert.NoDirExists(t, filepath.Join(stateDir, "myapp"))
	})

	t.Run("failed move keeps the old path", func(t *testing.T) {
		stateDir, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		writeFile(t, legacy, store)

		linkFile = func(oldPath, newPath string) error {
			return &os.LinkError{Op: "link", Old: oldPath, New: newPath, Err: os.ErrPermission}
		}
		t.Cleanup(func() { linkFile = os.Link })

		fp, err := newFileProvider("myapp", "")
		require.NoError(t, err)
		assert.Equal(t, legacy, fp.filePath)
		assert.FileExists(t, legacy)

		entries, err := os.ReadDir(filepath.Join(stateDir, "myapp"))
		require.NoError(t, err)
		assert.Empty(t, entries, "the temp file must be removed")
	})

	t.Run("explicit path skips the move", func(t *testing.T) {
		_, legacyDir := isolateDirs(t)
		legacy := filepath.Join(legacyDir, "myapp", credsFileName)
		writeFile(t, legacy, store)

		explicit := filepath.Join(t.TempDir(), "creds.json")
		fp, err := newFileProvider("myapp", explicit)
		require.NoError(t, err)
		assert.Equal(t, explicit, fp.filePath)
		assert.FileExists(t, legacy)
	})
}

func TestFileProviderClose(t *testing.T) {
	fp := newTestFileProvider(t)
	assert.NoError(t, fp.close())
}
