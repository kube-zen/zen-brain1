# ObjectStore Package

The `objectstore` package provides a unified interface for object storage backends in zen-platform.

## Supported Backends

- **S3** - AWS S3 and S3-compatible services (MinIO, Wasabi, DigitalOcean Spaces, etc.)
- **Azure Blob Storage** - Planned
- **Google Cloud Storage** - Planned

## Installation

```go
import "github.com/kube-zen/zen-sdk/pkg/objectstore"
```

## Quick Start

### AWS S3

```go
cfg := &objectstore.Config{
    Type: "s3",
    S3: &objectstore.S3Config{
        BucketName:       "my-bucket",
        Region:           "us-east-1",
        AccessKeyID:      "AKIAIOSFODNN7EXAMPLE",
        SecretAccessKey:  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
        KeyPrefix:        "webhooks", // Optional: prefix all object keys
    },
}

store, err := objectstore.NewObjectStore(cfg)
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

### MinIO (S3-compatible)

```go
cfg := &objectstore.Config{
    Type: "minio",
    S3: &objectstore.S3Config{
        BucketName:       "my-bucket",
        Endpoint:         "http://localhost:9000",
        AccessKeyID:      "minioadmin",
        SecretAccessKey:  "minioadmin",
        UseSSL:          false,
        KeyPrefix:        "events",
        SkipBucketCheck: true, // Skip bucket validation for local testing
    },
}

store, err := objectstore.NewObjectStore(cfg)
if err != nil {
    log.Fatal(err)
}
defer store.Close()
```

## Usage

### Put Object

```go
data := []byte("Hello, World!")
err := store.PutObject(ctx, "my-bucket", "path/to/file.txt", data)
```

### Get Object

```go
data, err := store.GetObject(ctx, "my-bucket", "path/to/file.txt")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Retrieved: %s\n", string(data))
```

### Delete Object

```go
err := store.DeleteObject(ctx, "my-bucket", "path/to/file.txt")
```

### List Objects

```go
keys, err := store.ListObjects(ctx, "my-bucket", "path/to/")
for _, key := range keys {
    fmt.Println(key)
}
```

### Head Object (Check Existence)

```go
meta, err := store.HeadObject(ctx, "my-bucket", "path/to/file.txt")
if err != nil {
    if errors.Is(err, ErrObjectNotFound) {
        log.Printf("Object does not exist")
    } else {
        log.Fatal(err)
    }
} else {
    log.Printf("Size: %d, LastModified: %v", meta.Size, meta.LastModified)
}
```

## Configuration

### S3Config

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BucketName` | string | Yes | S3 bucket name |
| `Region` | string | Yes* | AWS region (not required for MinIO with endpoint) |
| `AccessKeyID` | string | Yes* | AWS access key ID |
| `SecretAccessKey` | string | Yes* | AWS secret access key |
| `KeyPrefix` | string | No | Prefix to prepend to all object keys |
| `Endpoint` | string | No | Custom endpoint for S3-compatible services |
| `UseSSL` | bool | No | Use HTTPS for custom endpoint (default: true) |
| `SkipBucketCheck` | bool | No | Skip bucket existence check (for testing) |

*Required for AWS S3, optional for S3-compatible services if credentials are not needed.

## Validation

Before creating an ObjectStore, you can validate the configuration:

```go
cfg := &objectstore.Config{...}

if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
```

For S3, you can also validate credentials and bucket access:

```go
s3Cfg := &objectstore.S3Config{...}

if err := objectstore.ValidateS3Credentials(s3Cfg); err != nil {
    log.Fatal(err)
}
```

## Error Handling

The package wraps AWS SDK errors with context:

```go
data, err := store.GetObject(ctx, "my-bucket", "nonexistent.txt")
if err != nil {
    // Check if object doesn't exist
    var noSuchKey *types.NoSuchKey
    if errors.As(err, &noSuchKey) {
        log.Printf("Object not found")
        return
    }
    
    // Other errors
    log.Printf("Failed to get object: %v", err)
    return
}
```

## Integration with zen-ingester and zen-egress

### zen-ingester Example

The object store can be used in zen-ingester to store raw webhooks:

```go
// In ingester controller
cfg := &objectstore.Config{
    Type: "s3",
    S3: &objectstore.S3Config{
        BucketName:  ingesterConfig.S3Bucket,
        Region:      ingesterConfig.S3Region,
        KeyPrefix:   "raw-webhooks",
    },
}

store, err := objectstore.NewObjectStore(cfg)
if err != nil {
    log.Error(err, "failed to create object store")
    return err
}
defer store.Close()

// Store raw webhook
key := fmt.Sprintf("%s/%s.json", eventType, eventID)
err = store.PutObject(ctx, cfg.S3.BucketName, key, webhookPayload)
```

### zen-egress Example

The object store can be used in zen-egress as a destination:

```yaml
apiVersion: zen.kube-zen.io/v1alpha1
kind: Destination
metadata:
  name: s3-destination
spec:
  type: s3
  s3:
    bucketName: my-bucket
    region: us-east-1
    accessKeyID:
      secretRef:
        name: s3-credentials
        key: access-key-id
    secretAccessKey:
      secretRef:
        name: s3-credentials
        key: secret-access-key
    keyPrefix: events
```

## Testing

Run unit tests:

```bash
go test ./pkg/objectstore/...
```

Run integration tests (requires AWS credentials):

```bash
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1
export TEST_BUCKET=your-test-bucket

go test -v -run TestS3StoreOperations ./pkg/objectstore
```

Run integration tests against MinIO:

```bash
export MINIO_ENDPOINT=http://localhost:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY=minioadmin
export TEST_BUCKET=test-bucket

go test -v -run TestS3StoreOperations -tags minio ./pkg/objectstore
```

## Roadmap

- [ ] Azure Blob Storage backend
- [ ] Google Cloud Storage backend
- [ ] Batch operations for multiple objects
- [ ] Multipart upload support for large files
- [ ] Presigned URL generation
- [ ] Object lifecycle management
- [ ] Retry and backoff configuration
- [ ] Prometheus metrics for operations

## See Also

- [AWS SDK v2 Documentation](https://aws.github.io/aws-sdk-go-v2/)
- [MinIO Documentation](https://min.io/docs/minio/linux/index.html)
