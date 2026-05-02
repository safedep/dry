package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Environment variables consulted as a best-effort fallback when the
// corresponding fields on CloudflareR2StorageDriverConfig are empty.
// Explicit config always wins; env vars only fill gaps.
const (
	envCloudflareR2AccountID       = "CLOUDFLARE_R2_ACCOUNT_ID"
	envCloudflareR2AccessKeyID     = "CLOUDFLARE_R2_ACCESS_KEY_ID"
	envCloudflareR2SecretAccessKey = "CLOUDFLARE_R2_SECRET_ACCESS_KEY"
)

// CloudflareR2StorageDriverConfig is the config for the Cloudflare R2
// storage driver. R2 is S3-compatible, so the driver itself is the
// S3 driver pointed at R2's endpoint with static credentials.
//
// AccountID is the Cloudflare account ID; the R2 endpoint is derived as
// https://<AccountID>.r2.cloudflarestorage.com. AccessKeyID and
// SecretAccessKey come from an R2 API token (R2 -> Manage R2 API Tokens
// in the Cloudflare dashboard). See:
//
//	https://developers.cloudflare.com/r2/examples/aws/aws-sdk-go/
//
// Any of AccountID / AccessKeyID / SecretAccessKey left empty are
// resolved from CLOUDFLARE_R2_ACCOUNT_ID / CLOUDFLARE_R2_ACCESS_KEY_ID
// / CLOUDFLARE_R2_SECRET_ACCESS_KEY respectively. Explicit config
// always wins.
type CloudflareR2StorageDriverConfig struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
}

// NewCloudflareR2StorageDriver constructs a StorageWriter backed by
// Cloudflare R2. It builds an *s3.Client configured for R2 (BaseEndpoint
// set to the account-scoped R2 endpoint, region "auto", static
// credentials from the supplied API token) and delegates to
// NewS3StorageDriver via WithS3Client.
//
// R2 ignores the AWS region but the SDK requires one; "auto" is the
// value Cloudflare documents.
func NewCloudflareR2StorageDriver(config CloudflareR2StorageDriverConfig) (*s3StorageDriver, error) {
	if config.AccountID == "" {
		config.AccountID = os.Getenv(envCloudflareR2AccountID)
	}
	if config.AccessKeyID == "" {
		config.AccessKeyID = os.Getenv(envCloudflareR2AccessKeyID)
	}
	if config.SecretAccessKey == "" {
		config.SecretAccessKey = os.Getenv(envCloudflareR2SecretAccessKey)
	}

	if config.AccountID == "" {
		return nil, fmt.Errorf("cloudflare r2 storage adapter: account id is required (set %s or config.AccountID)", envCloudflareR2AccountID)
	}
	if config.AccessKeyID == "" {
		return nil, fmt.Errorf("cloudflare r2 storage adapter: access key id is required (set %s or config.AccessKeyID)", envCloudflareR2AccessKeyID)
	}
	if config.SecretAccessKey == "" {
		return nil, fmt.Errorf("cloudflare r2 storage adapter: secret access key is required (set %s or config.SecretAccessKey)", envCloudflareR2SecretAccessKey)
	}
	if config.BucketName == "" {
		return nil, fmt.Errorf("cloudflare r2 storage adapter: bucket name is required")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", config.AccountID)

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.AccessKeyID, config.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("cloudflare r2 storage adapter: failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return NewS3StorageDriver(
		S3StorageDriverConfig{BucketName: config.BucketName},
		WithS3Client(client),
	)
}
