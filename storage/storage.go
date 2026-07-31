// Package storage contains the contract for implementing a
// general purpose storage system.
package storage

import (
	"io"
	"time"
)

// Storage is a simple storage interface with read and write operations.
// This interface should be extended to support more capable contracts
type Storage interface {
	Put(key string, reader io.Reader) error
	Get(key string) (io.ReadCloser, error)
}

// StorageReader is a storage interface that supports a special writer
// method that returns a writer for a given key.
type StorageWriter interface {
	Storage

	Writer(key string) (io.WriteCloser, error)
}

// PresignedStorage is a Storage that can mint short-TTL pre-signed download
// URLs for its objects, letting callers hand out time-limited read access
// without proxying bytes or exposing storage keys. Not every backend supports
// this (the local filesystem driver does not); S3 is the first implementation.
type PresignedStorage interface {
	Storage

	// PresignedGetURL returns a pre-signed URL that grants read access to the
	// object at key for ttl, together with the time the URL expires. ttl must
	// be positive.
	PresignedGetURL(key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
}
