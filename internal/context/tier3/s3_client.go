// Package tier3 provides S3-based Tier 3 (Cold) archival storage for ZenContext.
// This file contains the AWS SDK v2 implementation of the S3Client interface.
package tier3

import (
	"bytes"
	stdctx "context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config holds configuration for the AWS S3 client.
type S3Config struct {
	// Bucket is the S3 bucket name (required).
	Bucket string `json:"bucket" yaml:"bucket"`

	// Region is the AWS region (e.g., "us-east-1").
	Region string `json:"region" yaml:"region"`

	// Endpoint is the custom S3 endpoint (for MinIO, etc.).
	// If empty, uses the default AWS endpoint.
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// AccessKeyID is the AWS access key ID.
	AccessKeyID string `json:"access_key_id" yaml:"access_key_id"`

	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string `json:"secret_access_key" yaml:"secret_access_key"`

	// SessionToken is the AWS session token (optional).
	SessionToken string `json:"session_token" yaml:"session_token"`

	// UsePathStyle forces path-style addressing (required for MinIO).
	UsePathStyle bool `json:"use_path_style" yaml:"use_path_style"`

	// DisableSSL disables SSL/TLS (for local testing).
	DisableSSL bool `json:"disable_ssl" yaml:"disable_ssl"`

	// ForceRenameBucket if true, ensures bucket exists on startup.
	ForceRenameBucket bool `json:"force_rename_bucket" yaml:"force_rename_bucket"`

	// MaxRetries is the maximum number of retries for S3 operations.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// Timeout is the timeout for S3 operations.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// PartSize is the multipart upload part size in bytes.
	PartSize int64 `json:"part_size" yaml:"part_size"`

	// Concurrency is the number of concurrent upload/download parts.
	Concurrency int `json:"concurrency" yaml:"concurrency"`

	// Verbose enables verbose logging.
	Verbose bool `json:"verbose" yaml:"verbose"`
}

// DefaultS3Config returns the default S3 configuration.
func DefaultS3Config() *S3Config {
	return &S3Config{
		Bucket:            "zen-brain-context",
		Region:            "us-east-1",
		Endpoint:          "",
		AccessKeyID:       "",
		SecretAccessKey:   "",
		SessionToken:      "",
		UsePathStyle:      false,
		DisableSSL:        false,
		ForceRenameBucket: false,
		MaxRetries:        3,
		Timeout:           30 * time.Second,
		PartSize:          5 * 1024 * 1024, // 5 MB
		Concurrency:       5,
		Verbose:           false,
	}
}

// awsS3Client implements the S3Client interface using AWS SDK v2.
type awsS3Client struct {
	client *s3.Client
	bucket string
	config *S3Config
}

// NewS3Client creates a new S3Client using AWS SDK v2.
// If config is nil, DefaultS3Config is used.
func NewS3Client(s3Config *S3Config) (S3Client, error) {
	if s3Config == nil {
		s3Config = DefaultS3Config()
	}

	if s3Config.Bucket == "" {
		return nil, errors.New("bucket name is required")
	}

	// Build AWS config
	ctx := stdctx.Background()
	var loadOpts []func(*awsconfig.LoadOptions) error

	// Region
	if s3Config.Region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(s3Config.Region))
	}

	// Credentials (if provided)
	if s3Config.AccessKeyID != "" && s3Config.SecretAccessKey != "" {
		creds := credentials.NewStaticCredentialsProvider(
			s3Config.AccessKeyID,
			s3Config.SecretAccessKey,
			s3Config.SessionToken,
		)
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(creds))
	}

	// Load default config
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Build S3 client options
	var clientOpts []func(*s3.Options)
	
	// Handle custom endpoint
	if s3Config.Endpoint != "" {
		// Ensure endpoint has proper scheme
		endpoint := s3Config.Endpoint
		if s3Config.DisableSSL && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			// Prepend http:// if SSL disabled and no scheme specified
			endpoint = "http://" + endpoint
		} else if !s3Config.DisableSSL && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			// Default to https:// if no scheme
			endpoint = "https://" + endpoint
		}
		
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = s3Config.UsePathStyle
		})
	} else {
		// No custom endpoint, just set path style if requested
		if s3Config.UsePathStyle {
			clientOpts = append(clientOpts, func(o *s3.Options) {
				o.UsePathStyle = s3Config.UsePathStyle
			})
		}
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(cfg, clientOpts...)

	// Ensure bucket exists (optional)
	if s3Config.ForceRenameBucket {
		ctx, cancel := stdctx.WithTimeout(ctx, s3Config.Timeout)
		defer cancel()
		if err := ensureBucketExists(ctx, s3Client, s3Config.Bucket, s3Config.Region); err != nil {
			return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
		}
	}

	return &awsS3Client{
		client: s3Client,
		bucket: s3Config.Bucket,
		config: s3Config,
	}, nil
}

// ensureBucketExists checks if bucket exists and creates it if needed.
func ensureBucketExists(ctx stdctx.Context, client *s3.Client, bucket, region string) error {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		return nil // Bucket exists
	}

	// Try to create bucket
	createInput := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	// Set location constraint for regions other than us-east-1
	if region != "" && region != "us-east-1" {
		createInput.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}

	_, err = client.CreateBucket(ctx, createInput)
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
	}

	return nil
}

// PutObject uploads an object to S3.
func (c *awsS3Client) PutObject(ctx stdctx.Context, key string, body io.Reader, contentType string, metadata map[string]string) error {
	if c.config.Verbose {
		fmt.Printf("[S3Client] PutObject: bucket=%s, key=%s, contentType=%s\n",
			c.bucket, key, contentType)
	}

	// Read entire body into bytes (for simplicity; for large files consider multipart upload)
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	if metadata != nil {
		input.Metadata = metadata
	}

	ctx, cancel := stdctx.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	_, err = c.client.PutObject(ctx, input)
	return err
}

// GetObject retrieves an object from S3.
func (c *awsS3Client) GetObject(ctx stdctx.Context, key string) ([]byte, error) {
	if c.config.Verbose {
		fmt.Printf("[S3Client] GetObject: bucket=%s, key=%s\n", c.bucket, key)
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	ctx, cancel := stdctx.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	output, err := c.client.GetObject(ctx, input)
	if err != nil {
		return nil, translateS3Error(err)
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

// DeleteObject deletes an object from S3.
func (c *awsS3Client) DeleteObject(ctx stdctx.Context, key string) error {
	if c.config.Verbose {
		fmt.Printf("[S3Client] DeleteObject: bucket=%s, key=%s\n", c.bucket, key)
	}

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	ctx, cancel := stdctx.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	_, err := c.client.DeleteObject(ctx, input)
	return err
}

// ListObjects lists objects matching a prefix.
func (c *awsS3Client) ListObjects(ctx stdctx.Context, prefix string) ([]string, error) {
	if c.config.Verbose {
		fmt.Printf("[S3Client] ListObjects: bucket=%s, prefix=%s\n", c.bucket, prefix)
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	ctx, cancel := stdctx.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}
	return keys, nil
}

// ObjectExists checks if an object exists.
func (c *awsS3Client) ObjectExists(ctx stdctx.Context, key string) (bool, error) {
	if c.config.Verbose {
		fmt.Printf("[S3Client] ObjectExists: bucket=%s, key=%s\n", c.bucket, key)
	}

	input := &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	ctx, cancel := stdctx.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	_, err := c.client.HeadObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NotFound", "NoSuchKey":
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

// Close closes the S3 client.
// AWS SDK v2 clients don't need explicit close, but we implement for interface.
func (c *awsS3Client) Close() error {
	// Nothing to close
	return nil
}

// translateS3Error translates AWS S3 errors to more generic errors.
func translateS3Error(err error) error {
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return fmt.Errorf("object not found: %w", err)
		case "AccessDenied":
			return fmt.Errorf("access denied: %w", err)
		case "BucketAlreadyExists":
			return fmt.Errorf("bucket already exists: %w", err)
		case "BucketAlreadyOwnedByYou":
			return fmt.Errorf("bucket already owned by you: %w", err)
		}
	}

	// Check for timeout
	if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
		return fmt.Errorf("S3 operation timeout: %w", err)
	}

	return err
}

// IsS3UnavailableError returns true if the error indicates S3 is unavailable.
func IsS3UnavailableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "NoSuchBucket") ||
		strings.Contains(errStr, "AccessDenied")
}