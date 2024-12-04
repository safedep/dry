package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

type googleCloudStorageDriver struct {
	client          *storage.Client
	bucket          string
	writeTimeout    time.Duration
	partitionByDate bool
}

type googleCloudStorageDriverOpts func(*googleCloudStorageDriver)

func WithGoogleStorageClient(client *storage.Client) googleCloudStorageDriverOpts {
	return func(d *googleCloudStorageDriver) {
		d.client = client
	}
}

func WithGoogleStorageWriteTimeout(timeout time.Duration) googleCloudStorageDriverOpts {
	return func(d *googleCloudStorageDriver) {
		d.writeTimeout = timeout
	}
}

func WithGoogleStoragePartitionByDate() googleCloudStorageDriverOpts {
	return func(d *googleCloudStorageDriver) {
		d.partitionByDate = true
	}
}

var _ StorageWriter = (*googleCloudStorageDriver)(nil)

func NewGoogleCloudStorageDriver(bucket string, opts ...googleCloudStorageDriverOpts) (*googleCloudStorageDriver, error) {
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create google cloud storage client: %w", err)
	}

	d := &googleCloudStorageDriver{
		bucket:          bucket,
		client:          client,
		writeTimeout:    10 * time.Second,
		partitionByDate: true,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}

func (d *googleCloudStorageDriver) Put(key string, reader io.Reader) error {
	writer, err := d.Writer(key)
	if err != nil {
		return fmt.Errorf("failed to create google cloud storage writer: %w", err)
	}

	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("failed to write to google cloud storage: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close google cloud storage writer: %w", err)
	}

	return nil
}

func (d *googleCloudStorageDriver) Get(key string) (io.ReadCloser, error) {
	keyName, err := d.prefix(key)
	if err != nil {
		return nil, fmt.Errorf("failed to prefix key: %w", err)
	}

	object := d.client.Bucket(d.bucket).Object(keyName)
	reader, err := object.NewReader(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create google cloud storage reader: %w", err)
	}

	return reader, nil
}

func (d *googleCloudStorageDriver) Writer(key string) (io.WriteCloser, error) {
	keyName, err := d.prefix(key)
	if err != nil {
		return nil, fmt.Errorf("failed to prefix key: %w", err)
	}

	object := d.client.Bucket(d.bucket).Object(keyName)
	writer := object.NewWriter(context.Background())

	return writer, nil
}

func (d *googleCloudStorageDriver) prefix(key string) (string, error) {
	key = strings.TrimLeft(key, "/")
	key = strings.TrimRight(key, "/")
	if len(key) == 0 {
		return "", fmt.Errorf("GCS Driver: key cannot be empty")
	}

	prefix := ""
	if d.partitionByDate {
		prefix = filepath.Join(prefix, time.Now().UTC().Format("2006/01/02"))
	}

	return filepath.Join(prefix, key), nil
}
