package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3StorageDriverPrefix(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		expected string
		err      error
	}{
		{
			name:     "simple key",
			key:      "test",
			expected: "test",
		},
		{
			name:     "key has path",
			key:      "/a/b/c/test",
			expected: "a/b/c/test",
		},
		{
			name: "empty key",
			key:  "",
			err:  fmt.Errorf("key cannot be empty"),
		},
		{
			name: "key has only slashes",
			key:  "/////",
			err:  fmt.Errorf("key cannot be empty"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &s3StorageDriver{}

			pf, err := d.prefix(tc.key)
			if tc.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, pf)
			}
		})
	}
}

func TestNewS3StorageDriverRequiresBucket(t *testing.T) {
	_, err := NewS3StorageDriver(S3StorageDriverConfig{})
	assert.Error(t, err)
	assert.ErrorContains(t, err, "bucket name is required")
}

// TestS3StorageDriver_Integration exercises Put/Get/Writer against a real
// S3-compatible endpoint (MinIO, LocalStack, real S3). Skipped unless
// SAFEDEP_S3_INTEGRATION_TEST=1.
//
// Env used:
//   - SAFEDEP_S3_INTEGRATION_BUCKET: bucket name (required; must exist)
//   - AWS_REGION, AWS_ENDPOINT_URL, AWS credentials: resolved by the SDK
//     default chain. For MinIO, set AWS_ENDPOINT_URL to the MinIO URL.
//
// Test shape:
//   - Generates a ULID-scoped run prefix so repeated / parallel runs
//     against a shared bucket don't collide.
//   - Each case cleans up its own object in t.Cleanup.
//   - Subtests run with t.Parallel() to get reasonable wall time.
//
// Checkout docs/s3_test_minio.md for running integration tests with MinIO
func TestS3StorageDriver_Integration(t *testing.T) {
	if os.Getenv("SAFEDEP_S3_INTEGRATION_TEST") != "1" {
		t.Skip("integration test skipped; set SAFEDEP_S3_INTEGRATION_TEST=1 to run")
	}

	bucket := os.Getenv("SAFEDEP_S3_INTEGRATION_BUCKET")
	require.NotEmpty(t, bucket, "SAFEDEP_S3_INTEGRATION_BUCKET must be set")

	// Force path-style addressing for the integration test. The SDK
	// defaults to virtual-hosted-style (bucket.host), which requires the
	// S3-compatible server to recognize bucket subdomains — MinIO does
	// this only when MINIO_DOMAIN is set. Path-style (host/bucket) works
	// against MinIO, LocalStack, and real S3 uniformly, so we pin it
	// here via WithS3Client to keep the test portable across targets.
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	require.NoError(t, err)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	driver, err := NewS3StorageDriver(
		S3StorageDriverConfig{BucketName: bucket},
		WithS3Client(client),
	)
	require.NoError(t, err)

	runPrefix := fmt.Sprintf("safedep-dry-integration/%s", ulid.Make().String())
	t.Logf("integration run prefix: %s", runPrefix)

	roundTripCases := []struct {
		name   string
		keySuf string
		body   []byte
		upload func(t *testing.T, key string, body []byte)
	}{
		{
			name:   "Put: small ascii",
			keySuf: "put-small.txt",
			body:   []byte("Hello, S3!"),
			upload: putViaPut(driver),
		},
		{
			name:   "Put: utf-8 body",
			keySuf: "put-utf8.txt",
			body:   []byte("héllo 世界 🚀"),
			upload: putViaPut(driver),
		},
		{
			name:   "Put: empty body",
			keySuf: "put-empty.bin",
			body:   []byte{},
			upload: putViaPut(driver),
		},
		{
			name:   "Put: nested key path",
			keySuf: "put-nested/a/b/c.txt",
			body:   []byte("nested"),
			upload: putViaPut(driver),
		},
		{
			name:   "Writer: small body",
			keySuf: "writer-small.txt",
			body:   []byte("Hello from writer!"),
			upload: putViaWriter(driver),
		},
		{
			name:   "Writer: empty body",
			keySuf: "writer-empty.bin",
			body:   []byte{},
			upload: putViaWriter(driver),
		},
		{
			// 6 MiB exceeds the SDK's default 5 MiB part size, forcing
			// the transfermanager to exercise its multipart path.
			name:   "Writer: 6 MiB body exercises multipart",
			keySuf: "writer-large.bin",
			body:   randomBytes(t, 6<<20),
			upload: putViaWriter(driver),
		},
	}

	for _, tc := range roundTripCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := runPrefix + "/" + tc.keySuf
			t.Cleanup(func() { deleteKey(t, driver, key) })

			tc.upload(t, key, tc.body)

			reader, err := driver.Get(key)
			require.NoError(t, err)
			defer func() { _ = reader.Close() }()

			got, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, tc.body, got, "round-trip content mismatch")
		})
	}

	t.Run("Get returns error for missing key", func(t *testing.T) {
		t.Parallel()
		_, err := driver.Get(runPrefix + "/does-not-exist.txt")
		assert.Error(t, err)
	})

	t.Run("Put overwrites existing object", func(t *testing.T) {
		t.Parallel()

		key := runPrefix + "/overwrite.txt"
		t.Cleanup(func() { deleteKey(t, driver, key) })

		require.NoError(t, driver.Put(key, strings.NewReader("first")))
		require.NoError(t, driver.Put(key, strings.NewReader("second")))

		reader, err := driver.Get(key)
		require.NoError(t, err)
		defer func() { _ = reader.Close() }()

		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, "second", string(got))
	})

	t.Run("prefix-only keys are rejected", func(t *testing.T) {
		t.Parallel()
		assert.Error(t, driver.Put("///", strings.NewReader("x")))
		_, err := driver.Get("///")
		assert.Error(t, err)
		_, err = driver.Writer("///")
		assert.Error(t, err)
	})
}

func putViaPut(driver *s3StorageDriver) func(t *testing.T, key string, body []byte) {
	return func(t *testing.T, key string, body []byte) {
		t.Helper()
		require.NoError(t, driver.Put(key, bytes.NewReader(body)))
	}
}

func putViaWriter(driver *s3StorageDriver) func(t *testing.T, key string, body []byte) {
	return func(t *testing.T, key string, body []byte) {
		t.Helper()
		w, err := driver.Writer(key)
		require.NoError(t, err)
		_, err = io.Copy(w, bytes.NewReader(body))
		require.NoError(t, err)
		require.NoError(t, w.Close())
	}
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return buf
}

// deleteKey removes a test object. Uses driver internals because our
// Storage interface deliberately has no Delete — integration tests clean
// up their own objects to keep the bucket tidy across runs.
func deleteKey(t *testing.T, driver *s3StorageDriver, key string) {
	t.Helper()
	keyName, err := driver.prefix(key)
	if err != nil {
		t.Logf("cleanup: failed to prefix key %q: %v", key, err)
		return
	}
	_, err = driver.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(driver.config.BucketName),
		Key:    aws.String(keyName),
	})
	if err != nil {
		t.Logf("cleanup: failed to delete %q: %v", keyName, err)
	}
}
