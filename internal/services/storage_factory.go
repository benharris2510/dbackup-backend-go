package services

import (
	"fmt"

	"github.com/dbackup/backend-go/internal/models"
)

// StorageFactory creates storage services from configuration
type StorageFactory struct{}

// NewStorageFactory creates a new storage factory
func NewStorageFactory() *StorageFactory {
	return &StorageFactory{}
}

// CreateS3Service creates an S3 service from a storage configuration
func (f *StorageFactory) CreateS3Service(config *models.StorageConfiguration) (S3ServiceInterface, error) {
	if config == nil {
		return nil, fmt.Errorf("storage configuration is required")
	}

	var endpoint string
	if config.Endpoint != nil {
		endpoint = *config.Endpoint
	}

	// Convert storage configuration to S3Config
	s3Config := &S3Config{
		Region:          config.Region,
		AccessKey:       config.AccessKey,
		SecretKey:       config.SecretKey,
		Endpoint:        endpoint,
		DefaultBucket:   config.Bucket,
		UsePathStyle:    config.ForcePathStyle,
		DisableSSL:      !config.UseSSL,
		MaxUploadSize:   100 * 1024 * 1024, // 100MB default
		UploadTimeout:   config.Timeout,     // Use timeout from config
		DownloadTimeout: config.Timeout,     // Use timeout from config
	}

	// Validate configuration
	if err := ValidateS3Config(s3Config); err != nil {
		return nil, fmt.Errorf("invalid S3 configuration: %w", err)
	}

	// Create S3 service
	service, err := NewS3Service(s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 service: %w", err)
	}

	return service, nil
}

// CreateS3ServiceFromUser creates an S3 service using user's default storage configuration
func (f *StorageFactory) CreateS3ServiceFromUser(user *models.User) (S3ServiceInterface, error) {
	if user == nil {
		return nil, fmt.Errorf("user is required")
	}

	// For now, we'll need to get the user's storage configuration
	// This would typically come from the database
	// TODO: Implement storage configuration retrieval from database
	return nil, fmt.Errorf("user storage configuration not implemented yet")
}

// GetSupportedProviders returns a list of supported storage providers
func (f *StorageFactory) GetSupportedProviders() []string {
	return []string{
		"aws-s3",
		"minio",
		"digitalocean-spaces",
		"backblaze-b2",
		"wasabi",
		"linode-object-storage",
		"custom-s3",
	}
}

// ValidateStorageConfiguration validates a storage configuration
func (f *StorageFactory) ValidateStorageConfiguration(config *models.StorageConfiguration) error {
	if config == nil {
		return fmt.Errorf("storage configuration is required")
	}

	if config.Provider == "" {
		return fmt.Errorf("storage provider is required")
	}

	if config.Region == "" {
		return fmt.Errorf("region is required")
	}

	if config.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}

	if config.SecretKey == "" {
		return fmt.Errorf("secret key is required")
	}

	if config.Bucket == "" {
		return fmt.Errorf("bucket is required")
	}

	// Provider-specific validation
	switch config.Provider {
	case models.StorageProviderAWS:
		// AWS S3 specific validation
		if config.Endpoint != nil && *config.Endpoint != "" {
			return fmt.Errorf("endpoint should not be set for AWS S3")
		}
	case models.StorageProviderMinIO, models.StorageProviderCustom:
		// MinIO and custom S3 specific validation
		if config.Endpoint == nil || *config.Endpoint == "" {
			return fmt.Errorf("endpoint is required for %s", config.Provider)
		}
	case models.StorageProviderDigitalOcean:
		// DigitalOcean Spaces specific validation
		if config.Endpoint == nil || *config.Endpoint == "" {
			endpoint := fmt.Sprintf("https://%s.digitaloceanspaces.com", config.Region)
			config.Endpoint = &endpoint
		}
	case models.StorageProviderBackblaze:
		// Backblaze B2 specific validation
		if config.Endpoint == nil || *config.Endpoint == "" {
			return fmt.Errorf("endpoint is required for Backblaze B2")
		}
	case models.StorageProviderWasabi:
		// Wasabi specific validation
		if config.Endpoint == nil || *config.Endpoint == "" {
			endpoint := fmt.Sprintf("https://s3.%s.wasabisys.com", config.Region)
			config.Endpoint = &endpoint
		}
	default:
		supportedProviders := f.GetSupportedProviders()
		return fmt.Errorf("unsupported storage provider: %s. Supported providers: %v", config.Provider, supportedProviders)
	}

	return nil
}