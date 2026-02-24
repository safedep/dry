// Package storage contains the contract for implementing a
// general purpose storage system.
package storage

import (
	"context"
	"io"
	"time"
)

// Storage is a simple storage interface with read and write operations.
// This interface should be extended to support more capable contracts
type Storage interface {
	Put(key string, reader io.Reader) error
	Get(key string) (io.ReadCloser, error)

	// Exists checks if a key exists in storage
	Exists(ctx context.Context, key string) (bool, error)

	// GetMetadata retrieves metadata for a stored object
	GetMetadata(ctx context.Context, key string) (*ObjectMetadata, error)

	// List returns keys matching a prefix
	List(ctx context.Context, prefix string) ([]string, error)

	// Delete removes an object from storage
	Delete(ctx context.Context, key string) error
}

// StorageWriter is a storage interface that supports a special writer
// method that returns a writer for a given key.
type StorageWriter interface {
	Storage

	Writer(key string) (io.WriteCloser, error)
}

// ObjectMetadata describes a stored object
type ObjectMetadata struct {
	Key         string
	Size        int64
	ContentType string
	Checksum    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
