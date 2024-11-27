package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilesystemStorageDriver(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fs-driver-test-*")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)

	config := FilesystemStorageDriverConfig{
		Root: tmpDir,
	}

	driver, err := NewFilesystemStorageDriver(config)
	assert.NoError(t, err)

	t.Run("Put", func(t *testing.T) {
		err := driver.Put("file.txt", strings.NewReader("Hello, World!"))
		assert.NoError(t, err)

		fileShouldExist := filepath.Join(tmpDir, "file.txt")
		_, err = os.Stat(fileShouldExist)
		assert.NoError(t, err)
	})

	t.Run("Put with parent dirs", func(t *testing.T) {
		err := driver.Put("dir1/dir2/file.txt", strings.NewReader("Hello, World!"))
		assert.NoError(t, err)

		fileShouldExist := filepath.Join(tmpDir, "dir1", "dir2", "file.txt")
		_, err = os.Stat(fileShouldExist)
		assert.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		reader, err := driver.Get("file.txt")
		assert.NoError(t, err)
		assert.NotNil(t, reader)

		content, err := io.ReadAll(reader)
		assert.NoError(t, err)

		assert.Equal(t, "Hello, World!", string(content))
		reader.Close()
	})

	t.Run("Writer", func(t *testing.T) {
		writer, err := driver.Writer("file2.txt")
		assert.NoError(t, err)
		assert.NotNil(t, writer)

		_, err = writer.Write([]byte("Hello, World!"))
		assert.NoError(t, err)
		writer.Close()

		fileShouldExist := filepath.Join(tmpDir, "file2.txt")
		_, err = os.Stat(fileShouldExist)
		assert.NoError(t, err)

		reader, err := driver.Get("file2.txt")
		assert.NoError(t, err)
		assert.NotNil(t, reader)

		content, err := io.ReadAll(reader)
		assert.NoError(t, err)

		assert.Equal(t, "Hello, World!", string(content))
		reader.Close()
	})
}
