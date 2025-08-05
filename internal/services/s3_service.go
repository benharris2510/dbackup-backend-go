package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3ServiceInterface defines the interface for S3 operations
type S3ServiceInterface interface {
	// File operations
	UploadFile(ctx context.Context, bucket, key string, data io.Reader, contentType string) (*S3UploadResult, error)
	DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	DeleteFile(ctx context.Context, bucket, key string) error

	// File information
	GetFileInfo(ctx context.Context, bucket, key string) (*S3FileInfo, error)
	FileExists(ctx context.Context, bucket, key string) (bool, error)

	// Bucket operations
	CreateBucket(ctx context.Context, bucket, region string) error
	BucketExists(ctx context.Context, bucket string) (bool, error)
	ListFiles(ctx context.Context, bucket, prefix string, maxKeys int) (*S3ListResult, error)

	// URL operations
	GeneratePresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	GenerateUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)

	// Configuration
	TestConnection(ctx context.Context) error
}

// S3Service implements S3 storage operations
type S3Service struct {
	client   *s3.Client
	uploader *manager.Uploader
	config   *S3Config
}

// S3Config holds S3 configuration
type S3Config struct {
	Region          string
	AccessKey       string
	SecretKey       string
	Endpoint        string
	UsePathStyle    bool
	DisableSSL      bool
	DefaultBucket   string
	MaxUploadSize   int64
	UploadTimeout   time.Duration
	DownloadTimeout time.Duration
}

// S3UploadResult contains the result of an upload operation
type S3UploadResult struct {
	Bucket      string
	Key         string
	ETag        string
	Location    string
	Size        int64
	ContentType string
	UploadedAt  time.Time
}

// S3FileInfo contains information about a file in S3
type S3FileInfo struct {
	Bucket       string
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
	StorageClass string
	Metadata     map[string]string
}

// S3ListResult contains the result of a list operation
type S3ListResult struct {
	Bucket     string
	Prefix     string
	Objects    []S3Object
	Truncated  bool
	NextMarker string
}

// S3Object represents an object in S3
type S3Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
}

// NewS3Service creates a new S3 service instance
func NewS3Service(cfg *S3Config) (*S3Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("S3 configuration is required")
	}

	// Set default values
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.MaxUploadSize <= 0 {
		cfg.MaxUploadSize = 100 * 1024 * 1024 // 100MB default
	}
	if cfg.UploadTimeout <= 0 {
		cfg.UploadTimeout = 10 * time.Minute
	}
	if cfg.DownloadTimeout <= 0 {
		cfg.DownloadTimeout = 5 * time.Minute
	}

	var awsCfg aws.Config
	var err error

	// Configure AWS SDK
	if cfg.Endpoint != "" {
		// Custom endpoint (MinIO, LocalStack, etc.)
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: true,
			}, nil
		})

		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.Region),
			config.WithEndpointResolverWithOptions(customResolver),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKey, cfg.SecretKey, "",
			)),
		)
	} else {
		// AWS S3
		if cfg.AccessKey != "" && cfg.SecretKey != "" {
			awsCfg, err = config.LoadDefaultConfig(context.TODO(),
				config.WithRegion(cfg.Region),
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
					cfg.AccessKey, cfg.SecretKey, "",
				)),
			)
		} else {
			// Use default credentials (IAM role, environment variables, etc.)
			awsCfg, err = config.LoadDefaultConfig(context.TODO(),
				config.WithRegion(cfg.Region),
			)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom options
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.UsePathStyle {
			o.UsePathStyle = true
		}
		if cfg.DisableSSL {
			o.EndpointOptions.DisableHTTPS = true
		}
	})

	// Create uploader
	uploader := manager.NewUploader(client)

	return &S3Service{
		client:   client,
		uploader: uploader,
		config:   cfg,
	}, nil
}

// UploadFile uploads a file to S3
func (s *S3Service) UploadFile(ctx context.Context, bucket, key string, data io.Reader, contentType string) (*S3UploadResult, error) {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Create context with timeout
	uploadCtx, cancel := context.WithTimeout(ctx, s.config.UploadTimeout)
	defer cancel()

	// Count bytes if possible
	var size int64
	if seeker, ok := data.(io.Seeker); ok {
		currentPos, _ := seeker.Seek(0, io.SeekCurrent)
		endPos, _ := seeker.Seek(0, io.SeekEnd)
		size = endPos - currentPos
		seeker.Seek(currentPos, io.SeekStart)
	}

	// Upload the file
	result, err := s.uploader.Upload(uploadCtx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"uploaded-by": "dbackup",
			"uploaded-at": time.Now().UTC().Format(time.RFC3339),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return &S3UploadResult{
		Bucket:      bucket,
		Key:         key,
		ETag:        strings.Trim(aws.ToString(result.ETag), "\""),
		Location:    result.Location,
		Size:        size,
		ContentType: contentType,
		UploadedAt:  time.Now().UTC(),
	}, nil
}

// DownloadFile downloads a file from S3
func (s *S3Service) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}

	// Create context with timeout
	downloadCtx, cancel := context.WithTimeout(ctx, s.config.DownloadTimeout)
	defer cancel()

	result, err := s.client.GetObject(downloadCtx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to download file from S3: %w", err)
	}

	return result.Body, nil
}

// DeleteFile deletes a file from S3
func (s *S3Service) DeleteFile(ctx context.Context, bucket, key string) error {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	if key == "" {
		return fmt.Errorf("object key is required")
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}

	return nil
}

// GetFileInfo gets information about a file in S3
func (s *S3Service) GetFileInfo(ctx context.Context, bucket, key string) (*S3FileInfo, error) {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get file info from S3: %w", err)
	}

	return &S3FileInfo{
		Bucket:       bucket,
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		LastModified: aws.ToTime(result.LastModified),
		ETag:         strings.Trim(aws.ToString(result.ETag), "\""),
		ContentType:  aws.ToString(result.ContentType),
		StorageClass: string(result.StorageClass),
		Metadata:     result.Metadata,
	}, nil
}

// FileExists checks if a file exists in S3
func (s *S3Service) FileExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := s.GetFileInfo(ctx, bucket, key)
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) || strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateBucket creates a new S3 bucket
func (s *S3Service) CreateBucket(ctx context.Context, bucket, region string) error {
	if bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	if region == "" {
		region = s.config.Region
	}

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	// Add location constraint for regions other than us-east-1
	if region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}

	_, err := s.client.CreateBucket(ctx, input)
	if err != nil {
		var bucketAlreadyExists *types.BucketAlreadyExists
		var bucketAlreadyOwnedByYou *types.BucketAlreadyOwnedByYou
		if errors.As(err, &bucketAlreadyExists) || errors.As(err, &bucketAlreadyOwnedByYou) {
			return nil // Bucket already exists, which is fine
		}
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// BucketExists checks if a bucket exists
func (s *S3Service) BucketExists(ctx context.Context, bucket string) (bool, error) {
	if bucket == "" {
		return false, fmt.Errorf("bucket name is required")
	}

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		var noSuchBucket *types.NoSuchBucket
		if errors.As(err, &noSuchBucket) || strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// ListFiles lists files in an S3 bucket
func (s *S3Service) ListFiles(ctx context.Context, bucket, prefix string, maxKeys int) (*S3ListResult, error) {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(int32(maxKeys)),
	}

	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	result, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in S3: %w", err)
	}

	objects := make([]S3Object, len(result.Contents))
	for i, obj := range result.Contents {
		objects[i] = S3Object{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         strings.Trim(aws.ToString(obj.ETag), "\""),
			StorageClass: string(obj.StorageClass),
		}
	}

	listResult := &S3ListResult{
		Bucket:    bucket,
		Prefix:    prefix,
		Objects:   objects,
		Truncated: aws.ToBool(result.IsTruncated),
	}

	if result.NextContinuationToken != nil {
		listResult.NextMarker = aws.ToString(result.NextContinuationToken)
	}

	return listResult, nil
}

// GeneratePresignedURL generates a presigned URL for downloading a file
func (s *S3Service) GeneratePresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return "", fmt.Errorf("bucket name is required")
	}

	if key == "" {
		return "", fmt.Errorf("object key is required")
	}

	if expiry <= 0 {
		expiry = 1 * time.Hour // Default to 1 hour
	}

	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// GenerateUploadURL generates a presigned URL for uploading a file
func (s *S3Service) GenerateUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if bucket == "" {
		return "", fmt.Errorf("bucket name is required")
	}

	if key == "" {
		return "", fmt.Errorf("object key is required")
	}

	if expiry <= 0 {
		expiry = 1 * time.Hour // Default to 1 hour
	}

	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate upload URL: %w", err)
	}

	return request.URL, nil
}

// TestConnection tests the S3 connection
func (s *S3Service) TestConnection(ctx context.Context) error {
	// Try to list buckets to test the connection
	_, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("S3 connection test failed: %w", err)
	}

	return nil
}

// UploadFromBytes is a convenience method to upload data from bytes
func (s *S3Service) UploadFromBytes(ctx context.Context, bucket, key string, data []byte, contentType string) (*S3UploadResult, error) {
	return s.UploadFile(ctx, bucket, key, bytes.NewReader(data), contentType)
}

// DownloadToBytes is a convenience method to download data to bytes
func (s *S3Service) DownloadToBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	reader, err := s.DownloadFile(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// GetPublicURL returns the public URL for a file (if bucket is public)
func (s *S3Service) GetPublicURL(bucket, key string) string {
	if bucket == "" {
		bucket = s.config.DefaultBucket
	}

	if s.config.Endpoint != "" {
		// Custom endpoint
		u, _ := url.Parse(s.config.Endpoint)
		if s.config.UsePathStyle {
			return fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(s.config.Endpoint, "/"), bucket, key)
		}
		return fmt.Sprintf("%s://%s.%s/%s", u.Scheme, bucket, u.Host, key)
	}

	// AWS S3
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, s.config.Region, key)
}

// ValidateConfig validates S3 configuration
func ValidateS3Config(cfg *S3Config) error {
	if cfg == nil {
		return fmt.Errorf("S3 configuration is required")
	}

	if cfg.Endpoint != "" {
		// Custom endpoint validation
		if cfg.AccessKey == "" || cfg.SecretKey == "" {
			return fmt.Errorf("access key and secret key are required for custom S3 endpoint")
		}

		if _, err := url.Parse(cfg.Endpoint); err != nil {
			return fmt.Errorf("invalid S3 endpoint URL: %w", err)
		}
	}

	if cfg.MaxUploadSize < 0 {
		return fmt.Errorf("max upload size cannot be negative")
	}

	if cfg.UploadTimeout < 0 {
		return fmt.Errorf("upload timeout cannot be negative")
	}

	if cfg.DownloadTimeout < 0 {
		return fmt.Errorf("download timeout cannot be negative")
	}

	return nil
}
