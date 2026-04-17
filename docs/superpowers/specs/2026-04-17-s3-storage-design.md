# S3 Storage Driver for `dry/storage`

**Status:** Implemented
**Date:** 2026-04-17
**Author:** Kunal Singh (with Claude)

## Problem

`github.com/safedep/dry/storage` ships with two drivers today: filesystem and Google Cloud Storage. Malysis is migrating from GCP to AWS and needs an S3 driver with feature parity to the GCS driver so the migration can land without changes to caller code.

## Scope

- Add an S3 driver to `dry/storage` that satisfies the existing `StorageWriter` interface.
- Config surface: only `BucketName` is exposed. Region, endpoint, and credentials come from the AWS SDK's default configuration chain — no new env-reading code in `dry`.

## Public API

New file `storage/s3.go`.

```go
type S3StorageDriverConfig struct {
    BucketName string
}

type s3StorageDriverOpts func(*s3StorageDriver)

// WithS3Client injects a pre-built *s3.Client for tests and for callers
// that need custom SDK config (path-style for MinIO, tracing middleware,
// custom retryer).
func WithS3Client(client *s3.Client) s3StorageDriverOpts

var _ StorageWriter = (*s3StorageDriver)(nil)

func NewS3StorageDriver(config S3StorageDriverConfig,
    opts ...s3StorageDriverOpts) (*s3StorageDriver, error)
```

Shape mirrors `NewGoogleCloudStorageDriver` exactly — same functional-options pattern, same `StorageWriter` compile-time assertion, same exported config-struct / unexported driver-struct convention.

## Credentials & Env

Construction calls `awsconfig.LoadDefaultConfig(ctx)`. The SDK resolves, in order:

- **Region:** `AWS_REGION` → `AWS_DEFAULT_REGION` → shared config profile.
- **Endpoint:** `AWS_ENDPOINT_URL_S3` → `AWS_ENDPOINT_URL` → shared config → real S3.
- **Credentials:** env vars → shared credentials file → shared config → IMDS (EC2) → ECS / EKS IRSA / IAM role.

No `CredentialFile` field on our config — unlike GCS, the AWS chain handles IAM roles natively, and exposing a file path would invite misuse on ECS/EKS.

## Writer Semantics

The critical implementation choice. S3 has no native streaming writer — `PutObject` and `manager.Uploader.Upload` both take an `io.Reader`. We expose `io.WriteCloser` via an `io.Pipe` bridge:

```go
func (d *s3StorageDriver) Writer(key string) (io.WriteCloser, error) {
    // ... prefix key, validate ...

    pr, pw := io.Pipe()
    errCh := make(chan error, 1)

    go func() {
        _, err := d.transferer.UploadObject(d.ctx, &transfermanager.UploadObjectInput{
            Bucket: &d.config.BucketName,
            Key:    &prefixedKey,
            Body:   pr,
        })
        pr.CloseWithError(err) // unblock any pending Write on upload failure
        errCh <- err
    }()

    return &s3Writer{pw: pw, errCh: errCh}, nil
}

func (w *s3Writer) Close() error {
    // Signal EOF to uploader. MUST block until upload completes —
    // returning early would let callers observe Close() success with
    // a subsequent Get() returning NoSuchKey.
    if err := w.pw.Close(); err != nil {
        return err
    }
    return <-w.errCh
}
```

### Why `io.Pipe` + `transfermanager.UploadObject`

| Option | Memory for 10GB upload | Streaming | Multipart |
|---|---|---|---|
| `bytes.Buffer` + `PutObject` | ~10GB | No | No |
| `io.Pipe` + `transfermanager.UploadObject` **(chosen)** | bounded by part size × concurrency | Yes | Yes (automatic) |
| Manual multipart (`CreateMultipartUpload`/`UploadPart`) | ~`PartSize` | Yes | Yes (manual) |

Uploads go through `feature/s3/transfermanager` (GA 2026-01-30), the successor to the now-deprecated `feature/s3/manager`. `UploadObject` takes an `io.Reader` body and handles multipart internally. `StorageWriter.Writer` wraps this with `io.Pipe` + a goroutine so callers see a normal `io.WriteCloser`.

### Why `*s3.Client.GetObject` for reads (not `transfermanager.GetObject`)

`transfermanager.Client.GetObject` returns `GetObjectOutput.Body` as `io.Reader`, not `io.ReadCloser`. Our `Storage.Get` contract returns `io.ReadCloser`. Using the plain `*s3.Client.GetObject` gives us the right type for free and avoids wrapping the concurrent-reader in a custom `ReadCloser` adapter. Uploads get transfermanager's multipart benefits; downloads get the standard single-stream semantics callers already expect. Clean split.

### Context lifetime

The upload context is `context.Background()`, stored on the driver at construction — matching the existing GCS driver's behavior exactly (`gcs.go:91,106`). Callers can't cancel an in-flight `Writer` the way they could if `Writer(ctx, key)` took one. Deliberately keeping the interface stable rather than diverging just for S3. A future enhancement could add `WriterContext(ctx, key)` to both drivers in sync.

## Put / Get

```go
func (d *s3StorageDriver) Put(key string, reader io.Reader) error {
    _, err := d.transferer.UploadObject(d.ctx, &transfermanager.UploadObjectInput{
        Bucket: &d.config.BucketName,
        Key:    &prefixedKey,
        Body:   reader,
    })
    // ... wrap error ...
}

func (d *s3StorageDriver) Get(key string) (io.ReadCloser, error) {
    out, err := d.client.GetObject(d.ctx, &s3.GetObjectInput{
        Bucket: &d.config.BucketName,
        Key:    &prefixedKey,
    })
    // ... wrap error, return out.Body ...
}
```

`prefix()` matches `gcs.go` — trim leading/trailing slashes, reject empty keys.

## Uploader Memory Model (reference)

For `io.Reader` bodies (no `Seek` support), both the legacy `manager.Uploader` and the new `transfermanager.Client` buffer each in-flight part in memory before uploading. Peak memory ≈ part size × concurrency — bounded, independent of total object size. We rely on SDK defaults; expose knobs via functional options if a concrete caller needs larger parts for throughput.

## Error Handling

All errors wrapped with the `"s3 storage adapter: ..."` prefix, matching `fs.go`'s `"fs storage adapter: ..."` style. Errors from the SDK propagate through `%w` so callers can `errors.Is`/`errors.As` on AWS error types if needed.

## Tests

`storage/s3_test.go`:

- `TestS3StorageDriverPrefix` — table-driven, identical structure to `TestGoogleCloudStorageDriverPrefix` in `gcs_test.go`. Covers simple keys, path keys, empty keys, slash-only keys.
- `TestS3StorageDriver_Integration` — skipped unless `SAFEDEP_S3_INTEGRATION_TEST=1`. Reads bucket name from `SAFEDEP_S3_INTEGRATION_BUCKET`. Does a full Put/Get/Writer round-trip against whatever `AWS_ENDPOINT_URL` points at. Useful for local verification with MinIO or LocalStack; not run in standard `go test ./...`.

This matches the GCS driver's test scope exactly — intentional parity rather than building more machinery than the neighbors have.

## Code Comments & Source References

Per agreement with Kunal, inline comments in `s3.go` carry source references for the non-obvious bits:

- Why uploads go through `transfermanager` and reads through `*s3.Client` (return-type split) → `transfermanager` pkg.go.dev; discussion #3306.
- `AWS_ENDPOINT_URL` / `AWS_ENDPOINT_URL_S3` env resolution → `aws-sdk-go-v2/config/env_config.go`.
- `io.Pipe` + goroutine pattern → standard AWS streaming example.

These stay in code, not just in this spec, because the *why* is load-bearing and non-obvious to the next reader.

## Dependencies

Add to `go.mod` direct requires:

- `github.com/aws/aws-sdk-go-v2` (already indirect → promote)
- `github.com/aws/aws-sdk-go-v2/config` (already indirect → promote)
- `github.com/aws/aws-sdk-go-v2/service/s3` (new)
- `github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager` (new)

## Follow-up (not this PR)

- Malysis wiring: add env-driven driver selection between GCS and S3, switch deployment to S3.
- Expose part-size / concurrency via functional options if a caller needs larger parts.
- Consider `WriterContext(ctx, key)` on both GCS and S3 drivers for per-write cancellation.
- Revisit `transfermanager.GetObject` once the concurrent-reader's close semantics are clarified — could give us concurrent multipart downloads too.

## References

- [aws-sdk-go-v2 env_config.go (`AWS_ENDPOINT_URL` support)](https://github.com/aws/aws-sdk-go-v2/blob/main/config/env_config.go)
- [`feature/s3/transfermanager` package docs](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager)
- [S3 Transfer Manager v2 GA announcement (discussion #3306)](https://github.com/aws/aws-sdk-go-v2/discussions/3306)
- [`transfermanager` migration feedback (issue #3317)](https://github.com/aws/aws-sdk-go-v2/issues/3317)
- [AWS official `upload_arbitrary_sized_stream.go` example](https://github.com/awsdocs/aws-doc-sdk-examples/blob/main/go/example_code/s3/upload_arbitrary_sized_stream.go)
