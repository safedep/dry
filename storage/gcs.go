package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GoogleCloudStorageDriverConfig struct {
	BucketName     string
	CredentialFile string
}

type googleCloudStorageDriver struct {
	client          *storage.Client
	config          GoogleCloudStorageDriverConfig
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

func NewGoogleCloudStorageDriver(config GoogleCloudStorageDriverConfig,
	opts ...googleCloudStorageDriverOpts,
) (*googleCloudStorageDriver, error) {
	clientOpts := []option.ClientOption{}

	if config.CredentialFile != "" {
		clientOpts = append(clientOpts, option.WithCredentialsFile(config.CredentialFile))
	}

	client, err := storage.NewClient(context.Background(), clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create google cloud storage client: %w", err)
	}

	d := &googleCloudStorageDriver{
		config:          config,
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

	object := d.client.Bucket(d.config.BucketName).Object(keyName)
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

	object := d.client.Bucket(d.config.BucketName).Object(keyName)
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

// Exists checks if a key exists in storage
func (d *googleCloudStorageDriver) Exists(ctx context.Context, key string) (bool, error) {
	keyName, err := d.prefix(key)
	if err != nil {
		return false, err
	}

	object := d.client.Bucket(d.config.BucketName).Object(keyName)
	_, err = object.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}
	return true, nil
}

// GetMetadata retrieves metadata for a stored object
func (d *googleCloudStorageDriver) GetMetadata(ctx context.Context, key string) (*ObjectMetadata, error) {
	keyName, err := d.prefix(key)
	if err != nil {
		return nil, err
	}

	object := d.client.Bucket(d.config.BucketName).Object(keyName)
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object attributes: %w", err)
	}

	checksum := ""
	if attrs.MD5 != nil {
		checksum = string(attrs.MD5)
	}

	return &ObjectMetadata{
		Key:         key,
		Size:        attrs.Size,
		ContentType: attrs.ContentType,
		Checksum:    checksum,
		CreatedAt:   attrs.Created,
		UpdatedAt:   attrs.Updated,
	}, nil
}

// List returns keys matching a prefix
func (d *googleCloudStorageDriver) List(ctx context.Context, prefix string) ([]string, error) {
	keyPrefix, err := d.prefix(prefix)
	if err != nil {
		return nil, err
	}

	var keys []string
	it := d.client.Bucket(d.config.BucketName).Objects(ctx, &storage.Query{
		Prefix: keyPrefix,
	})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Remove prefix to get relative key
		relKey := strings.TrimPrefix(attrs.Name, keyPrefix)
		keys = append(keys, relKey)
	}

	return keys, nil
}

// Delete removes an object from storage
func (d *googleCloudStorageDriver) Delete(ctx context.Context, key string) error {
	keyName, err := d.prefix(key)
	if err != nil {
		return err
	}

	object := d.client.Bucket(d.config.BucketName).Object(keyName)
	err = object.Delete(ctx)
	if err != nil && err != storage.ErrObjectNotExist {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}
