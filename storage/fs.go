package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FilesystemStorageDriverConfig struct {
	Root string
}

type filesystemStorageDriver struct {
	config FilesystemStorageDriverConfig
}

func NewFilesystemStorageDriver(config FilesystemStorageDriverConfig) (StorageWriter, error) {
	_, err := os.Stat(config.Root)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(config.Root, 0o755)
			if err != nil {
				return nil, fmt.Errorf("fs storage adapter: failed to create directory: %w", err)
			}
		} else {
			return nil, fmt.Errorf("fs storage adapter: failed to stat directory: %w", err)
		}
	}

	return &filesystemStorageDriver{config: config}, nil
}

func (d *filesystemStorageDriver) Put(key string, reader io.Reader) error {
	path := filepath.Join(d.config.Root, key)
	err := d.createParentDirs(path)
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("fs storage adapter: failed to create file: %w", err)
	}

	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("fs storage adapter: failed to write file: %w", err)
	}

	return nil
}

func (d *filesystemStorageDriver) Get(key string) (io.ReadCloser, error) {
	path := filepath.Join(d.config.Root, key)
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("fs storage adapter: failed to open file: %w", err)
	}

	return file, nil
}

func (d *filesystemStorageDriver) Writer(key string) (io.WriteCloser, error) {
	path := filepath.Join(d.config.Root, key)
	err := d.createParentDirs(path)
	if err != nil {
		return nil, err
	}

	return os.Create(path)
}

func (d *filesystemStorageDriver) createParentDirs(path string) error {
	parent := filepath.Dir(path)
	err := os.MkdirAll(parent, 0o755)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}

		return fmt.Errorf("fs storage adapter: failed to create parent directories: %w", err)
	}

	return nil
}

// Exists checks if a key exists in storage
func (d *filesystemStorageDriver) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(d.config.Root, key)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("fs storage adapter: failed to check existence: %w", err)
	}
	return true, nil
}

// GetMetadata retrieves metadata for a stored object
func (d *filesystemStorageDriver) GetMetadata(ctx context.Context, key string) (*ObjectMetadata, error) {
	path := filepath.Join(d.config.Root, key)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("fs storage adapter: failed to stat file: %w", err)
	}

	// Compute checksum
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("fs storage adapter: failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("fs storage adapter: failed to compute checksum: %w", err)
	}

	return &ObjectMetadata{
		Key:         key,
		Size:        info.Size(),
		Checksum:    hex.EncodeToString(hash.Sum(nil)),
		CreatedAt:   info.ModTime(),
		UpdatedAt:   info.ModTime(),
		ContentType: detectContentType(path),
	}, nil
}

// List returns keys matching a prefix
func (d *filesystemStorageDriver) List(ctx context.Context, prefix string) ([]string, error) {
	rootPath := filepath.Join(d.config.Root, prefix)
	var keys []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// If the root path doesn't exist, return empty list
			if os.IsNotExist(err) && path == rootPath {
				return filepath.SkipDir
			}
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Convert absolute path to relative key
		relPath, err := filepath.Rel(d.config.Root, path)
		if err != nil {
			return err
		}

		keys = append(keys, relPath)
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("fs storage adapter: failed to list files: %w", err)
	}

	return keys, nil
}

// Delete removes an object from storage
func (d *filesystemStorageDriver) Delete(ctx context.Context, key string) error {
	path := filepath.Join(d.config.Root, key)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("fs storage adapter: failed to delete file: %w", err)
	}
	return nil
}

// detectContentType attempts to detect the content type from the file extension
func detectContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".tgz", ".gz":
		return "application/gzip"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
