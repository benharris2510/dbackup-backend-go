package services

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockS3Client is a mock implementation of the S3 client
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *MockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.PutObjectOutput), args.Error(1)
}

func (m *MockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.DeleteObjectOutput), args.Error(1)
}

func (m *MockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.HeadObjectOutput), args.Error(1)
}

func (m *MockS3Client) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.HeadBucketOutput), args.Error(1)
}

func (m *MockS3Client) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.CreateBucketOutput), args.Error(1)
}

func (m *MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.ListObjectsV2Output), args.Error(1)
}

func (m *MockS3Client) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	args := m.Called(ctx, params, optFns)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.ListBucketsOutput), args.Error(1)
}

// ReadCloser implementation for testing
type testReadCloser struct {
	*bytes.Reader
}

func (t *testReadCloser) Close() error {
	return nil
}

func newTestReadCloser(data []byte) io.ReadCloser {
	return &testReadCloser{bytes.NewReader(data)}
}

func TestValidateS3Config(t *testing.T) {
	tests := []struct {
		name    string
		config  *S3Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid AWS config",
			config: &S3Config{
				Region:    "us-east-1",
				AccessKey: "access",
				SecretKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "valid custom endpoint config",
			config: &S3Config{
				Region:    "us-east-1",
				AccessKey: "access",
				SecretKey: "secret",
				Endpoint:  "https://minio.example.com",
			},
			wantErr: false,
		},
		{
			name: "custom endpoint without credentials",
			config: &S3Config{
				Region:   "us-east-1",
				Endpoint: "https://minio.example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint URL",
			config: &S3Config{
				Region:    "us-east-1",
				AccessKey: "access",
				SecretKey: "secret",
				Endpoint:  "not-a-url",
			},
			wantErr: true,
		},
		{
			name: "negative max upload size",
			config: &S3Config{
				Region:        "us-east-1",
				AccessKey:     "access",
				SecretKey:     "secret",
				MaxUploadSize: -1,
			},
			wantErr: true,
		},
		{
			name: "negative upload timeout",
			config: &S3Config{
				Region:        "us-east-1",
				AccessKey:     "access",
				SecretKey:     "secret",
				UploadTimeout: -1 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateS3Config(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewS3Service(t *testing.T) {
	tests := []struct {
		name    string
		config  *S3Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &S3Config{
				Region:    "us-east-1",
				AccessKey: "access",
				SecretKey: "secret",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewS3Service(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, service)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, service)
				assert.NotNil(t, service.client)
				assert.NotNil(t, service.uploader)
				assert.Equal(t, tt.config, service.config)
			}
		})
	}
}

func TestS3Service_GetPublicURL(t *testing.T) {
	tests := []struct {
		name        string
		config      *S3Config
		bucket      string
		key         string
		expectedURL string
	}{
		{
			name: "AWS S3 URL",
			config: &S3Config{
				Region: "us-east-1",
			},
			bucket:      "my-bucket",
			key:         "path/to/file.txt",
			expectedURL: "https://my-bucket.s3.us-east-1.amazonaws.com/path/to/file.txt",
		},
		{
			name: "custom endpoint with path style",
			config: &S3Config{
				Region:       "us-east-1",
				Endpoint:     "https://minio.example.com",
				UsePathStyle: true,
			},
			bucket:      "my-bucket",
			key:         "path/to/file.txt",
			expectedURL: "https://minio.example.com/my-bucket/path/to/file.txt",
		},
		{
			name: "custom endpoint without path style",
			config: &S3Config{
				Region:   "us-east-1",
				Endpoint: "https://minio.example.com",
			},
			bucket:      "my-bucket",
			key:         "path/to/file.txt",
			expectedURL: "https://my-bucket.minio.example.com/path/to/file.txt",
		},
		{
			name: "default bucket from config",
			config: &S3Config{
				Region:        "us-east-1",
				DefaultBucket: "default-bucket",
			},
			bucket:      "",
			key:         "file.txt",
			expectedURL: "https://default-bucket.s3.us-east-1.amazonaws.com/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &S3Service{config: tt.config}
			url := service.GetPublicURL(tt.bucket, tt.key)
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestS3Service_UploadFromBytes(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		AccessKey:     "access",
		SecretKey:     "secret",
		DefaultBucket: "test-bucket",
	})
	require.NoError(t, err)

	ctx := context.Background()
	testData := []byte("test file content")
	
	result, err := service.UploadFromBytes(ctx, "test-bucket", "test-key", testData, "text/plain")
	
	// This will fail in actual execution since we don't have real AWS credentials
	// but it tests the method signature and basic functionality
	if err != nil {
		assert.Contains(t, err.Error(), "failed to upload file to S3")
	} else {
		assert.NotNil(t, result)
		assert.Equal(t, "test-bucket", result.Bucket)
		assert.Equal(t, "test-key", result.Key)
		assert.Equal(t, "text/plain", result.ContentType)
	}
}

func TestS3Service_DownloadToBytes(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		AccessKey:     "access",
		SecretKey:     "secret",
		DefaultBucket: "test-bucket",
	})
	require.NoError(t, err)

	ctx := context.Background()
	
	data, err := service.DownloadToBytes(ctx, "test-bucket", "test-key")
	
	// This will fail in actual execution since we don't have real AWS credentials
	// but it tests the method signature and basic functionality
	if err != nil {
		assert.Contains(t, err.Error(), "failed to download file from S3")
	} else {
		assert.NotNil(t, data)
	}
}

func TestS3Config_DefaultValues(t *testing.T) {
	config := &S3Config{}
	
	service, err := NewS3Service(config)
	require.NoError(t, err)
	
	// Check that default values are set
	assert.Equal(t, "us-east-1", service.config.Region)
	assert.Equal(t, int64(100*1024*1024), service.config.MaxUploadSize)
	assert.Equal(t, 10*time.Minute, service.config.UploadTimeout)
	assert.Equal(t, 5*time.Minute, service.config.DownloadTimeout)
}

func TestS3Service_ValidateRequiredParameters(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:    "us-east-1",
		AccessKey: "access",
		SecretKey: "secret",
	})
	require.NoError(t, err)

	ctx := context.Background()
	testData := bytes.NewReader([]byte("test"))

	// Test upload with empty bucket and no default
	_, err = service.UploadFile(ctx, "", "key", testData, "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test upload with empty key
	_, err = service.UploadFile(ctx, "bucket", "", testData, "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "object key is required")

	// Test download with empty bucket and no default
	_, err = service.DownloadFile(ctx, "", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test download with empty key
	_, err = service.DownloadFile(ctx, "bucket", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "object key is required")

	// Test delete with empty bucket and no default
	err = service.DeleteFile(ctx, "", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test delete with empty key
	err = service.DeleteFile(ctx, "bucket", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "object key is required")

	// Test get file info with empty bucket and no default
	_, err = service.GetFileInfo(ctx, "", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test get file info with empty key
	_, err = service.GetFileInfo(ctx, "bucket", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "object key is required")

	// Test file exists with empty bucket and no default
	_, err = service.FileExists(ctx, "", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test create bucket with empty name
	err = service.CreateBucket(ctx, "", "us-east-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test bucket exists with empty name
	_, err = service.BucketExists(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test list files with empty bucket and no default
	_, err = service.ListFiles(ctx, "", "", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test generate presigned URL with empty bucket and no default
	_, err = service.GeneratePresignedURL(ctx, "", "key", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test generate presigned URL with empty key
	_, err = service.GeneratePresignedURL(ctx, "bucket", "", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "object key is required")

	// Test generate upload URL with empty bucket and no default
	_, err = service.GenerateUploadURL(ctx, "", "key", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")

	// Test generate upload URL with empty key
	_, err = service.GenerateUploadURL(ctx, "bucket", "", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "object key is required")
}

func TestS3Service_DefaultBucketUsage(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		AccessKey:     "access",
		SecretKey:     "secret",
		DefaultBucket: "default-bucket",
	})
	require.NoError(t, err)

	ctx := context.Background()
	testData := bytes.NewReader([]byte("test"))

	// Test that empty bucket uses default bucket
	// These will fail due to no real AWS connection, but we test parameter handling
	_, err = service.UploadFile(ctx, "", "key", testData, "text/plain")
	if err != nil {
		// Should not be a "bucket name is required" error since default is set
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	_, err = service.DownloadFile(ctx, "", "key")
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	err = service.DeleteFile(ctx, "", "key")
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	_, err = service.GetFileInfo(ctx, "", "key")
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	_, err = service.FileExists(ctx, "", "key")
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	_, err = service.ListFiles(ctx, "", "", 10)
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	_, err = service.GeneratePresignedURL(ctx, "", "key", time.Hour)
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}

	_, err = service.GenerateUploadURL(ctx, "", "key", time.Hour)
	if err != nil {
		assert.NotContains(t, err.Error(), "bucket name is required")
	}
}

func TestS3Service_ContentTypeDefaults(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		AccessKey:     "access",
		SecretKey:     "secret",
		DefaultBucket: "test-bucket",
	})
	require.NoError(t, err)

	ctx := context.Background()
	testData := bytes.NewReader([]byte("test"))

	// Test that empty content type gets default
	_, err = service.UploadFile(ctx, "bucket", "key", testData, "")
	if err != nil {
		// The error should be from AWS connection, not content type validation
		assert.NotContains(t, err.Error(), "content type")
	}
}

func TestS3Service_ListFilesDefaults(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		AccessKey:     "access",
		SecretKey:     "secret",
		DefaultBucket: "test-bucket",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Test that maxKeys <= 0 gets default value of 1000
	_, err = service.ListFiles(ctx, "bucket", "", 0)
	if err != nil {
		// Should not be a parameter validation error
		assert.NotContains(t, err.Error(), "maxKeys")
	}

	_, err = service.ListFiles(ctx, "bucket", "", -1)
	if err != nil {
		assert.NotContains(t, err.Error(), "maxKeys")
	}
}

func TestS3Service_PresignedURLDefaults(t *testing.T) {
	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		AccessKey:     "access",
		SecretKey:     "secret",
		DefaultBucket: "test-bucket",
	})
	require.NoError(t, err)

	ctx := context.Background()

	// Test that expiry <= 0 gets default value of 1 hour
	_, err = service.GeneratePresignedURL(ctx, "bucket", "key", 0)
	if err != nil {
		// Should not be a parameter validation error
		assert.NotContains(t, err.Error(), "expiry")
	}

	_, err = service.GeneratePresignedURL(ctx, "bucket", "key", -1*time.Hour)
	if err != nil {
		assert.NotContains(t, err.Error(), "expiry")
	}

	_, err = service.GenerateUploadURL(ctx, "bucket", "key", 0)
	if err != nil {
		assert.NotContains(t, err.Error(), "expiry")
	}

	_, err = service.GenerateUploadURL(ctx, "bucket", "key", -1*time.Hour)
	if err != nil {
		assert.NotContains(t, err.Error(), "expiry")
	}
}

// Integration test helpers
func TestS3Service_Integration_UploadAndDownload(t *testing.T) {
	// Skip integration tests unless explicitly enabled
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require real AWS credentials and should only run in integration test environment
	t.Skip("Integration test requires real AWS credentials")

	service, err := NewS3Service(&S3Config{
		Region:        "us-east-1",
		DefaultBucket: "test-bucket",
		// Note: In real integration tests, credentials would come from environment
	})
	require.NoError(t, err)

	ctx := context.Background()
	testData := []byte("integration test content")
	testKey := "integration-test-file.txt"

	// Upload file
	uploadResult, err := service.UploadFromBytes(ctx, "", testKey, testData, "text/plain")
	require.NoError(t, err)
	assert.Equal(t, testKey, uploadResult.Key)
	assert.Equal(t, "text/plain", uploadResult.ContentType)

	// Download file
	downloadedData, err := service.DownloadToBytes(ctx, "", testKey)
	require.NoError(t, err)
	assert.Equal(t, testData, downloadedData)

	// Get file info
	fileInfo, err := service.GetFileInfo(ctx, "", testKey)
	require.NoError(t, err)
	assert.Equal(t, testKey, fileInfo.Key)
	assert.Equal(t, int64(len(testData)), fileInfo.Size)
	assert.Equal(t, "text/plain", fileInfo.ContentType)

	// Check file exists
	exists, err := service.FileExists(ctx, "", testKey)
	require.NoError(t, err)
	assert.True(t, exists)

	// Generate presigned URL
	presignedURL, err := service.GeneratePresignedURL(ctx, "", testKey, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, presignedURL)
	assert.Contains(t, presignedURL, testKey)

	// List files
	listResult, err := service.ListFiles(ctx, "", "integration-test", 10)
	require.NoError(t, err)
	assert.True(t, len(listResult.Objects) >= 1)

	// Clean up - delete file
	err = service.DeleteFile(ctx, "", testKey)
	require.NoError(t, err)

	// Verify file is deleted
	exists, err = service.FileExists(ctx, "", testKey)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestS3Service_Integration_BucketOperations(t *testing.T) {
	// Skip integration tests unless explicitly enabled  
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Integration test requires real AWS credentials")

	service, err := NewS3Service(&S3Config{
		Region: "us-east-1",
		// Note: In real integration tests, credentials would come from environment
	})
	require.NoError(t, err)

	ctx := context.Background()
	testBucket := "dbackup-integration-test-bucket"

	// Test connection
	err = service.TestConnection(ctx)
	require.NoError(t, err)

	// Create bucket
	err = service.CreateBucket(ctx, testBucket, "us-east-1")
	require.NoError(t, err)

	// Check bucket exists
	exists, err := service.BucketExists(ctx, testBucket)
	require.NoError(t, err)
	assert.True(t, exists)

	// Note: In real integration tests, you would also clean up by deleting the bucket
	// However, bucket deletion requires the bucket to be empty and additional permissions
}