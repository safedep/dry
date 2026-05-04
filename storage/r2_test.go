package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCloudflareR2StorageDriverValidation(t *testing.T) {
	cases := []struct {
		name   string
		config CloudflareR2StorageDriverConfig
		errMsg string
	}{
		{
			name:   "missing account id",
			config: CloudflareR2StorageDriverConfig{AccessKeyID: "k", SecretAccessKey: "s", BucketName: "b"},
			errMsg: "account id is required",
		},
		{
			name:   "missing access key id",
			config: CloudflareR2StorageDriverConfig{AccountID: "a", SecretAccessKey: "s", BucketName: "b"},
			errMsg: "access key id is required",
		},
		{
			name:   "missing secret access key",
			config: CloudflareR2StorageDriverConfig{AccountID: "a", AccessKeyID: "k", BucketName: "b"},
			errMsg: "secret access key is required",
		},
		{
			name:   "missing bucket",
			config: CloudflareR2StorageDriverConfig{AccountID: "a", AccessKeyID: "k", SecretAccessKey: "s"},
			errMsg: "bucket name is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCloudflareR2StorageDriver(tc.config)
			assert.Error(t, err)
			assert.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestNewCloudflareR2StorageDriverSucceeds(t *testing.T) {
	d, err := NewCloudflareR2StorageDriver(CloudflareR2StorageDriverConfig{
		AccountID:       "test-account",
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secret",
		BucketName:      "my-bucket",
	})
	assert.NoError(t, err)
	assert.NotNil(t, d)
	assert.Equal(t, "my-bucket", d.config.BucketName)
}

func TestNewCloudflareR2StorageDriverResolvesFromEnv(t *testing.T) {
	t.Setenv(envCloudflareR2AccountID, "env-account")
	t.Setenv(envCloudflareR2AccessKeyID, "env-key")
	t.Setenv(envCloudflareR2SecretAccessKey, "env-secret")

	d, err := NewCloudflareR2StorageDriver(CloudflareR2StorageDriverConfig{
		BucketName: "my-bucket",
	})
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestNewCloudflareR2StorageDriverConfigOverridesEnv(t *testing.T) {
	t.Setenv(envCloudflareR2AccountID, "env-account")
	t.Setenv(envCloudflareR2AccessKeyID, "env-key")
	t.Setenv(envCloudflareR2SecretAccessKey, "env-secret")

	// Empty env vars should not be required if config supplies them.
	cfg := CloudflareR2StorageDriverConfig{
		AccountID:       "cfg-account",
		AccessKeyID:     "cfg-key",
		SecretAccessKey: "cfg-secret",
		BucketName:      "my-bucket",
	}
	d, err := NewCloudflareR2StorageDriver(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func TestNewCloudflareR2StorageDriverFailsFastWhenEnvEmpty(t *testing.T) {
	t.Setenv(envCloudflareR2AccountID, "")
	t.Setenv(envCloudflareR2AccessKeyID, "")
	t.Setenv(envCloudflareR2SecretAccessKey, "")

	_, err := NewCloudflareR2StorageDriver(CloudflareR2StorageDriverConfig{BucketName: "b"})
	assert.Error(t, err)
	assert.ErrorContains(t, err, "account id is required")
}
