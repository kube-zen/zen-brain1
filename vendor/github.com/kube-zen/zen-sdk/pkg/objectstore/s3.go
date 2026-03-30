package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/pkg/errors"
)

// s3Store implements ObjectStore for AWS S3 and S3-compatible services.
type s3Store struct {
	client     *s3.Client
	cfg        *S3Config
	bucketName string
	keyPrefix  string
}

// newS3Store creates a new S3 object store.
func newS3Store(cfg *S3Config) (ObjectStore, error) {
	if cfg == nil {
		return nil, errors.New("s3 config is required")
	}

	// Validate required fields
	if cfg.BucketName == "" {
		return nil, errors.New("s3 bucket_name is required")
	}
	if cfg.Region == "" && cfg.Endpoint == "" {
		return nil, errors.New("s3 region or endpoint is required")
	}

	// Build AWS config
	awsCfg, err := buildAWSConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build aws config")
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	store := &s3Store{
		client:     client,
		cfg:        cfg,
		bucketName: cfg.BucketName,
		keyPrefix:  strings.Trim(cfg.KeyPrefix, "/"),
	}

	// Validate bucket access if not skipped
	if !cfg.SkipBucketCheck {
		if err := store.validateBucket(context.Background()); err != nil {
			return nil, errors.Wrap(err, "failed to validate s3 bucket")
		}
	}

	return store, nil
}

// buildAWSConfig builds the AWS config from S3Config.
func buildAWSConfig(cfg *S3Config) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// If endpoint is specified (MinIO or S3-compatible), use custom resolver
	if cfg.Endpoint != "" {
		// Parse endpoint to extract region if needed
		endpointURL, err := url.Parse(cfg.Endpoint)
		if err != nil {
			return aws.Config{}, errors.Wrap(err, "invalid s3 endpoint")
		}

		// For MinIO/S3-compatible, use the credentials from config
		if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
			))
		}

		// Use the endpoint's host as the base endpoint
		opts = append(opts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{
						URL:           endpointURL.String(),
						SigningRegion: cfg.Region, // Use configured region for signing
						HostnameImmutable: false,
					}, nil
				}
				return aws.Endpoint{}, fmt.Errorf("unknown service: %s", service)
			}),
		))
	} else {
		// For AWS S3, use default credential chain
		if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
			))
		}
	}

	return config.LoadDefaultConfig(context.Background(), opts...)
}

// validateBucket checks if the bucket exists and is accessible.
func (s *s3Store) validateBucket(ctx context.Context) error {
	// Try HeadBucket to check if bucket exists
	headBucketInput := &s3.HeadBucketInput{
		Bucket: aws.String(s.bucketName),
	}

	_, err := s.client.HeadBucket(ctx, headBucketInput)
	if err != nil {
		// Check if bucket exists but we don't have permission
		var noAccess *types.NoSuchBucket
		if errors.As(err, &noAccess) {
			return fmt.Errorf("bucket does not exist: %s", s.bucketName)
		}
		return errors.Wrap(err, "failed to access s3 bucket")
	}

	return nil
}

// PutObject implements ObjectStore.PutObject.
func (s *s3Store) PutObject(ctx context.Context, bucket, key string, data []byte) error {
	if bucket == "" {
		bucket = s.bucketName
	}

	// Add key prefix if configured
	fullKey := s.buildKey(key)

	putInput := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fullKey),
		Body:   bytes.NewReader(data),
	}

	_, err := s.client.PutObject(ctx, putInput)
	if err != nil {
		return errors.Wrap(err, "failed to put s3 object")
	}

	return nil
}

// GetObject implements ObjectStore.GetObject.
func (s *s3Store) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	if bucket == "" {
		bucket = s.bucketName
	}

	fullKey := s.buildKey(key)

	getInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fullKey),
	}

	result, err := s.client.GetObject(ctx, getInput)
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, errors.Wrap(err, "failed to get s3 object")
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read s3 object body")
	}

	return data, nil
}

// DeleteObject implements ObjectStore.DeleteObject.
func (s *s3Store) DeleteObject(ctx context.Context, bucket, key string) error {
	if bucket == "" {
		bucket = s.bucketName
	}

	fullKey := s.buildKey(key)

	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fullKey),
	}

	_, err := s.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		return errors.Wrap(err, "failed to delete s3 object")
	}

	return nil
}

// ListObjects implements ObjectStore.ListObjects.
func (s *s3Store) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	if bucket == "" {
		bucket = s.bucketName
	}

	fullPrefix := s.buildKey(prefix)

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(fullPrefix),
	}

	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, listInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list s3 objects")
		}

		for _, obj := range page.Contents {
			// Strip key prefix from results
			key := aws.ToString(obj.Key)
			if s.keyPrefix != "" {
				key = strings.TrimPrefix(key, s.keyPrefix+"/")
			}
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// HeadObject implements ObjectStore.HeadObject.
func (s *s3Store) HeadObject(ctx context.Context, bucket, key string) (*HeadObjectOutput, error) {
	if bucket == "" {
		bucket = s.bucketName
	}

	fullKey := s.buildKey(key)

	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fullKey),
	}

	result, err := s.client.HeadObject(ctx, headInput)
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, errors.Wrap(err, "failed to head s3 object")
	}

	output := &HeadObjectOutput{
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ContentType:  aws.ToString(result.ContentType),
		ETag:         aws.ToString(result.ETag),
	}

	return output, nil
}

// Close implements ObjectStore.Close.
func (s *s3Store) Close() error {
	// S3 client doesn't need explicit cleanup
	return nil
}

// buildKey builds the full key with prefix if configured.
func (s *s3Store) buildKey(key string) string {
	if s.keyPrefix == "" {
		return strings.Trim(key, "/")
	}
	
	// Trim slashes from both prefix and key, then join
	prefix := strings.Trim(s.keyPrefix, "/")
	cleanKey := strings.Trim(key, "/")
	
	if cleanKey == "" {
		return prefix
	}
	
	return fmt.Sprintf("%s/%s", prefix, cleanKey)
}
