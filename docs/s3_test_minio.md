# Run Minio

```bash
docker run -p 9000:9000 -p 9001:9001 -d  --name minio  -e "MINIO_ROOT_USER=admin"  -e "MINIO_ROOT_PASSWORD=password"  quay.io/minio/minio server /data --console-address ":9001"
```

# Run Tests

Bucket Name is the name of the bucket created in minio UI

AWS_REGION is required by AWS SDK, for local testing we can use any value

```bash
SAFEDEP_S3_INTEGRATION_TEST=1 SAFEDEP_S3_INTEGRATION_BUCKET=BUCKET_NAME AWS_REGION=us-east-1 AWS_ENDPOINT_URL=http://localhost:9000 AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=password go test ./storage/... -v -run TestS3StorageDriver_Integration
```