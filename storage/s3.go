package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3StorageDriverConfig is the config for the S3 storage driver.
//
// BucketName is the only field exposed. Region, endpoint, and credentials
// are resolved from the AWS SDK default configuration chain (env vars,
// shared config/credentials files, IMDS, IAM roles / IRSA). In particular,
// LoadDefaultConfig honors AWS_REGION, AWS_ENDPOINT_URL, and
// AWS_ENDPOINT_URL_S3 — see env_config.go in the SDK:
//
//	https://github.com/aws/aws-sdk-go-v2/blob/main/config/env_config.go
type S3StorageDriverConfig struct {
	BucketName string
}

type s3StorageDriver struct {
	client     *s3.Client
	transferer *transfermanager.Client
	config     S3StorageDriverConfig
	ctx        context.Context
}

type s3StorageDriverOpts func(*s3StorageDriver)

// WithS3Client injects a pre-built *s3.Client. Used for tests and for
// callers that need custom SDK config (path-style addressing for MinIO,
// custom retryer, tracing middleware, etc.).
func WithS3Client(client *s3.Client) s3StorageDriverOpts {
	return func(d *s3StorageDriver) {
		d.client = client
	}
}

var _ StorageWriter = (*s3StorageDriver)(nil)

// NewS3StorageDriver constructs an S3-backed StorageWriter.
//
// Uploads go through feature/s3/transfermanager (GA 2026-01-30), the
// successor to the deprecated feature/s3/manager. transfermanager's
// UploadObject takes an io.Reader body; since StorageWriter.Writer must
// expose io.WriteCloser, we bridge the two with io.Pipe in Writer() — see
// that method for details. GetObject still uses *s3.Client directly,
// because transfermanager's GetObject returns an io.Reader (not a
// ReadCloser) and we need the ReadCloser contract for consumers. See:
//
//	https://github.com/aws/aws-sdk-go-v2/discussions/3306
//	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager
func NewS3StorageDriver(config S3StorageDriverConfig,
	opts ...s3StorageDriverOpts) (*s3StorageDriver, error) {
	if config.BucketName == "" {
		return nil, fmt.Errorf("s3 storage adapter: bucket name is required")
	}

	d := &s3StorageDriver{
		config: config,
		ctx:    context.Background(),
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.client == nil {
		// LoadDefaultConfig walks the SDK's default credential provider
		// chain in this order (per the AWS SDK for Go V2 developer guide):
		//
		//   1. Static env vars (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY /
		//      AWS_SESSION_TOKEN)
		//   2. Web Identity Token file (AWS_WEB_IDENTITY_TOKEN_FILE) —
		//      this is how EKS IRSA / Pod Identity delivers credentials
		//   3. Shared credentials / config files (~/.aws/{credentials,config})
		//   4. ECS task role (AWS_CONTAINER_CREDENTIALS_RELATIVE_URI /
		//      AWS_CONTAINER_CREDENTIALS_FULL_URI)
		//   5. EC2 instance profile via IMDS (IMDSv2 preferred)
		//
		// Result: EC2 instance roles, ECS task roles, and EKS IRSA all
		// work out-of-the-box — just attach the right IAM role to the
		// instance / task / service account. Reference:
		//
		//   https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html#specifying-credentials
		//
		// Caveats:
		//   - Set AWS_EC2_METADATA_DISABLED=true off-EC2 to skip the
		//     IMDS probe and avoid a startup delay on non-EC2 hosts.
		//   - On EKS, prefer IRSA / Pod Identity over the node's IMDS
		//     role: IRSA scopes credentials to the pod; node IMDS grants
		//     whatever the underlying EC2 role has.
		cfg, err := awsconfig.LoadDefaultConfig(d.ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 storage adapter: failed to load AWS config: %w", err)
		}
		d.client = s3.NewFromConfig(cfg)
	}

	d.transferer = transfermanager.New(d.client)

	return d, nil
}

func (d *s3StorageDriver) Put(key string, reader io.Reader) error {
	keyName, err := d.prefix(key)
	if err != nil {
		return fmt.Errorf("s3 storage adapter: failed to prefix key: %w", err)
	}

	_, err = d.transferer.UploadObject(d.ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(d.config.BucketName),
		Key:    aws.String(keyName),
		Body:   reader,
	})
	if err != nil {
		return fmt.Errorf("s3 storage adapter: failed to upload object: %w", err)
	}

	return nil
}

func (d *s3StorageDriver) Get(key string) (io.ReadCloser, error) {
	keyName, err := d.prefix(key)
	if err != nil {
		return nil, fmt.Errorf("s3 storage adapter: failed to prefix key: %w", err)
	}

	out, err := d.client.GetObject(d.ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.config.BucketName),
		Key:    aws.String(keyName),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 storage adapter: failed to get object: %w", err)
	}

	return out.Body, nil
}

// Writer bridges io.WriteCloser semantics onto S3. Neither s3.PutObject
// nor transfermanager.UploadObject expose an io.Writer — both take an
// io.Reader body. We pipe writes from the returned WriteCloser into a
// goroutine that feeds UploadObject. Memory is bounded by the
// transfermanager's part size × concurrency, independent of object size.
// Reference for transfermanager config and behavior:
//
//	https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager
//
// The io.Pipe + goroutine pattern follows the same shape AWS uses for
// arbitrary-sized streams in aws-sdk-go-v2 examples.
func (d *s3StorageDriver) Writer(key string) (io.WriteCloser, error) {
	keyName, err := d.prefix(key)
	if err != nil {
		return nil, fmt.Errorf("s3 storage adapter: failed to prefix key: %w", err)
	}

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		_, err := d.transferer.UploadObject(d.ctx, &transfermanager.UploadObjectInput{
			Bucket: aws.String(d.config.BucketName),
			Key:    aws.String(keyName),
			Body:   pr,
		})
		// Unblock any pending Write on the pipe if the upload errored
		// mid-stream; the caller's next Write will observe err.
		pr.CloseWithError(err)
		errCh <- err
	}()

	return &s3Writer{pw: pw, errCh: errCh}, nil
}

type s3Writer struct {
	pw    *io.PipeWriter
	errCh chan error
}

func (w *s3Writer) Write(p []byte) (int, error) {
	return w.pw.Write(p)
}

// Close signals EOF to the uploader goroutine and blocks until the upload
// finishes. We MUST NOT return before the object is durable — otherwise
// callers would observe a successful Close() with a subsequent Get()
// returning NoSuchKey.
func (w *s3Writer) Close() error {
	if err := w.pw.Close(); err != nil {
		return fmt.Errorf("s3 storage adapter: failed to close writer: %w", err)
	}

	if err := <-w.errCh; err != nil {
		return fmt.Errorf("s3 storage adapter: upload failed: %w", err)
	}

	return nil
}

func (d *s3StorageDriver) prefix(key string) (string, error) {
	key = strings.TrimLeft(key, "/")
	key = strings.TrimRight(key, "/")
	if len(key) == 0 {
		return "", fmt.Errorf("S3 Driver: key cannot be empty")
	}

	return key, nil
}
