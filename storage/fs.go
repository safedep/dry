package storage

import (
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

func NewFilesystemStorageDriver(config FilesystemStorageDriverConfig) (Storage, error) {
	_, err := os.Stat(config.Root)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(config.Root, 0755)
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
