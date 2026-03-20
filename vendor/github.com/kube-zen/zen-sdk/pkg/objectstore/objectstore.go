// Package objectstore provides a unified interface for object storage backends.
//
// Supported backends:
//   - S3 (AWS S3 and S3-compatible services like MinIO, Wasabi, DigitalOcean Spaces)
//   - Azure Blob Storage (planned)
//   - Google Cloud Storage (planned)
//
// Usage:
//
//	cfg := &objectstore.Config{
//	    Type: "s3",
//	    S3: &objectstore.S3Config{
//	        BucketName:       "my-bucket",
//	        Region:           "us-east-1",
//	        AccessKeyID:      "AKIA...",
//	        SecretAccessKey:  "secret...",
//	    },
//	}
//
//	store, err := objectstore.NewObjectStore(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Upload a file
//	err = store.PutObject(ctx, "my-bucket", "path/to/file.txt", data)
package objectstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ObjectStore defines the interface for object storage backends.
type ObjectStore interface {
	// PutObject stores data in the specified bucket at the given key.
	PutObject(ctx context.Context, bucket, key string, data []byte) error

	// GetObject retrieves data from the specified bucket at the given key.
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)

	// DeleteObject removes the object at the given bucket/key.
	DeleteObject(ctx context.Context, bucket, key string) error

	// ListObjects lists objects in the bucket with the given prefix.
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)

	// HeadObject checks if an object exists and returns metadata.
	HeadObject(ctx context.Context, bucket, key string) (*HeadObjectOutput, error)

	// Close releases any resources held by the object store.
	Close() error
}

// HeadObjectOutput contains metadata from a HeadObject operation.
type HeadObjectOutput struct {
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
}

// Config defines the configuration for creating an ObjectStore.
type Config struct {
	// Type specifies the backend type: "s3", "minio", "azure", "gcs"
	Type string `yaml:"type" json:"type"`

	// S3 configuration for AWS S3 or S3-compatible services
	S3 *S3Config `yaml:"s3,omitempty" json:"s3,omitempty"`

	// Azure Blob configuration (planned)
	Azure *AzureConfig `yaml:"azure,omitempty" json:"azure,omitempty"`

	// GCS configuration (planned)
	GCS *GCSConfig `yaml:"gcs,omitempty" json:"gcs,omitempty"`
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is required")
	}

	if c.Type == "" {
		return errors.New("config type is required")
	}

	switch strings.ToLower(c.Type) {
	case "s3", "minio":
		if c.S3 == nil {
			return errors.New("s3 config is required for s3/minio type")
		}
		if c.S3.BucketName == "" {
			return errors.New("s3 bucket_name is required")
		}
		if c.S3.Region == "" && c.S3.Endpoint == "" {
			return errors.New("s3 region or endpoint is required")
		}
		if c.S3.AccessKeyID == "" && !c.S3.SkipBucketCheck {
			return errors.New("s3 access_key_id is required")
		}
		if c.S3.SecretAccessKey == "" && !c.S3.SkipBucketCheck {
			return errors.New("s3 secret_access_key is required")
		}

	case "azure":
		if c.Azure == nil {
			return errors.New("azure config is required for azure type")
		}
		if c.Azure.AccountName == "" {
			return errors.New("azure account_name is required")
		}
		if c.Azure.AccountKey == "" {
			return errors.New("azure account_key is required")
		}
		if c.Azure.ContainerName == "" {
			return errors.New("azure container_name is required")
		}

	case "gcs":
		if c.GCS == nil {
			return errors.New("gcs config is required for gcs type")
		}
		if c.GCS.ProjectID == "" {
			return errors.New("gcs project_id is required")
		}
		if c.GCS.BucketName == "" {
			return errors.New("gcs bucket_name is required")
		}

	default:
		return fmt.Errorf("unsupported type: %s (supported: s3, minio, azure, gcs)", c.Type)
	}

	return nil
}

// S3Config defines configuration for S3/MinIO backends.
type S3Config struct {
	// BucketName is the S3 bucket name (required)
	BucketName string `yaml:"bucket_name" json:"bucket_name"`

	// Region is the AWS region (required for AWS S3, optional for MinIO)
	Region string `yaml:"region" json:"region"`

	// AccessKeyID is the AWS access key ID (required for AWS S3)
	AccessKeyID string `yaml:"access_key_id" json:"access_key_id"`

	// SecretAccessKey is the AWS secret access key (required for AWS S3)
	SecretAccessKey string `yaml:"secret_access_key" json:"secret_access_key"`

	// KeyPrefix is an optional prefix to prepend to all object keys
	KeyPrefix string `yaml:"key_prefix" json:"key_prefix"`

	// Endpoint is the custom endpoint URL (optional, for MinIO/S3-compatible services)
	Endpoint string `yaml:"endpoint" json:"endpoint"`

	// UseSSL specifies whether to use HTTPS (optional, for MinIO)
	UseSSL bool `yaml:"use_ssl" json:"use_ssl"`

	// SkipBucketCheck skips bucket existence validation (for testing)
	SkipBucketCheck bool `yaml:"skip_bucket_check" json:"skip_bucket_check"`
}

// AzureConfig defines configuration for Azure Blob backend.
type AzureConfig struct {
	// AccountName is the Azure Storage account name
	AccountName string `yaml:"account_name" json:"account_name"`

	// AccountKey is the Azure Storage account key
	AccountKey string `yaml:"account_key" json:"account_key"`

	// ContainerName is the Azure Blob container name
	ContainerName string `yaml:"container_name" json:"container_name"`
}

// GCSConfig defines configuration for GCS backend.
type GCSConfig struct {
	// ProjectID is the Google Cloud project ID
	ProjectID string `yaml:"project_id" json:"project_id"`

	// BucketName is the GCS bucket name
	BucketName string `yaml:"bucket_name" json:"bucket_name"`

	// CredentialsJSON is the service account credentials in JSON format
	CredentialsJSON string `yaml:"credentials_json" json:"credentials_json"`
}

// NewObjectStore creates an ObjectStore based on the configuration.
func NewObjectStore(cfg *Config) (ObjectStore, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch strings.ToLower(cfg.Type) {
	case "s3", "minio":
		return newS3Store(cfg.S3)
	case "azure":
		return nil, errors.New("azure backend not yet implemented")
	case "gcs":
		return nil, errors.New("gcs backend not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported type: %s", cfg.Type)
	}
}

// ValidateS3Credentials validates S3 credentials and bucket access.
// This is a convenience function for validating S3 configuration before
// creating an ObjectStore.
func ValidateS3Credentials(cfg *S3Config) error {
	store, err := newS3Store(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	// Try to check bucket existence
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = store.HeadObject(ctx, cfg.BucketName, "") // Check bucket
	return err
}
