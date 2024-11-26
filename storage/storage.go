// Package storage contains the contract for implementing a
// general purpose storage system.
package storage

import "io"

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
