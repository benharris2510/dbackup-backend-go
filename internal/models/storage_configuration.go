package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// StorageProvider represents different S3-compatible storage providers
type StorageProvider string

const (
	StorageProviderAWS        StorageProvider = "aws"
	StorageProviderMinIO      StorageProvider = "minio"
	StorageProviderBackblaze  StorageProvider = "backblaze"
	StorageProviderDigitalOcean StorageProvider = "digitalocean"
	StorageProviderWasabi     StorageProvider = "wasabi"
	StorageProviderGoogle     StorageProvider = "google"
	StorageProviderCustom     StorageProvider = "custom"
)

// StorageConfiguration represents S3-compatible storage configuration
type StorageConfiguration struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	UID  string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// Provider information
	Provider    StorageProvider `json:"provider" gorm:"type:varchar(50);not null"`
	Region      string          `json:"region" gorm:"type:varchar(100);not null"`
	Endpoint    *string         `json:"endpoint,omitempty" gorm:"type:varchar(500)"` // For non-AWS S3 providers
	
	// Authentication
	AccessKey    string  `json:"-" gorm:"type:varchar(255);not null"`        // Encrypted
	SecretKey    string  `json:"-" gorm:"type:text;not null"`                // Encrypted
	SessionToken *string `json:"-" gorm:"type:text"`                         // Encrypted, for temporary credentials
	
	// Bucket configuration
	Bucket          string  `json:"bucket" gorm:"type:varchar(255);not null"`
	PathPrefix      *string `json:"path_prefix,omitempty" gorm:"type:varchar(500)"` // Optional path prefix
	ForcePathStyle  bool    `json:"force_path_style" gorm:"default:false"`
	
	// SSL/TLS configuration
	UseSSL          bool    `json:"use_ssl" gorm:"default:true"`
	SkipSSLVerify   bool    `json:"skip_ssl_verify" gorm:"default:false"`
	CustomCACert    *string `json:"-" gorm:"type:text"` // Encrypted
	
	// Storage class and lifecycle
	DefaultStorageClass string  `json:"default_storage_class" gorm:"type:varchar(50);default:'STANDARD'"`
	LifecyclePolicy     *string `json:"lifecycle_policy,omitempty" gorm:"type:text"`
	
	// Encryption
	EncryptionType      string  `json:"encryption_type" gorm:"type:varchar(50);default:'AES256'"`
	KMSKeyID           *string `json:"kms_key_id,omitempty" gorm:"type:varchar(255)"`
	ClientSideEncryption bool   `json:"client_side_encryption" gorm:"default:true"`
	
	// Connection settings
	MaxRetries        int           `json:"max_retries" gorm:"default:3"`
	Timeout           time.Duration `json:"timeout" gorm:"default:300000000000"` // 5 minutes in nanoseconds
	PartSize          int64         `json:"part_size" gorm:"default:5368709120"` // 5MB in bytes
	Concurrency       int           `json:"concurrency" gorm:"default:5"`
	
	// Status and health
	IsActive         bool       `json:"is_active" gorm:"default:true"`
	LastTestedAt     *time.Time `json:"last_tested_at,omitempty"`
	LastTestError    *string    `json:"last_test_error,omitempty" gorm:"type:text"`
	IsDefault        bool       `json:"is_default" gorm:"default:false"`
	
	// Usage statistics
	TotalObjects     *int64 `json:"total_objects,omitempty"`
	TotalSize        *int64 `json:"total_size,omitempty"`
	LastStatsUpdate  *time.Time `json:"last_stats_update,omitempty"`
	
	// Metadata
	Description *string                `json:"description,omitempty" gorm:"type:text"`
	Tags        map[string]interface{} `json:"tags" gorm:"type:json"`
	
	// Relationships
	TeamID *uint `json:"team_id,omitempty" gorm:"index"`
	Team   *Team `json:"team,omitempty" gorm:"foreignKey:TeamID"`
	UserID uint  `json:"user_id" gorm:"not null;index"`
	User   User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
	
	BackupFiles []BackupFile `json:"backup_files,omitempty" gorm:"foreignKey:S3Bucket;references:Bucket"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the StorageConfiguration model
func (StorageConfiguration) TableName() string {
	return "storage_configurations"
}

// BeforeCreate hook to generate UID before creating storage configuration
func (sc *StorageConfiguration) BeforeCreate(tx *gorm.DB) error {
	if sc.UID == "" {
		sc.UID = generateUID()
	}
	return nil
}

// GetEndpointURL returns the complete endpoint URL
func (sc *StorageConfiguration) GetEndpointURL() string {
	if sc.Endpoint != nil && *sc.Endpoint != "" {
		return *sc.Endpoint
	}
	
	// Default endpoints for known providers
	switch sc.Provider {
	case StorageProviderAWS:
		return fmt.Sprintf("https://s3.%s.amazonaws.com", sc.Region)
	case StorageProviderMinIO:
		return "http://localhost:9000" // Default MinIO endpoint
	case StorageProviderBackblaze:
		return fmt.Sprintf("https://s3.%s.backblazeb2.com", sc.Region)
	case StorageProviderDigitalOcean:
		return fmt.Sprintf("https://%s.digitaloceanspaces.com", sc.Region)
	case StorageProviderWasabi:
		return fmt.Sprintf("https://s3.%s.wasabisys.com", sc.Region)
	case StorageProviderGoogle:
		return "https://storage.googleapis.com"
	default:
		return ""
	}
}

// GetBucketURL returns the full bucket URL
func (sc *StorageConfiguration) GetBucketURL() string {
	endpoint := sc.GetEndpointURL()
	if endpoint == "" {
		return ""
	}
	
	if sc.ForcePathStyle {
		return fmt.Sprintf("%s/%s", endpoint, sc.Bucket)
	}
	
	// Virtual-hosted style (default for AWS)
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com", sc.Bucket, sc.Region)
}

// GetFullPath returns the full S3 path with prefix
func (sc *StorageConfiguration) GetFullPath(key string) string {
	if sc.PathPrefix != nil && *sc.PathPrefix != "" {
		return fmt.Sprintf("%s/%s", *sc.PathPrefix, key)
	}
	return key
}

// IsHealthy checks if the storage configuration is healthy
func (sc *StorageConfiguration) IsHealthy() bool {
	return sc.IsActive && sc.LastTestError == nil
}

// NeedsRetesting checks if the storage configuration should be retested
func (sc *StorageConfiguration) NeedsRetesting() bool {
	if sc.LastTestedAt == nil {
		return true
	}
	// Retest if last test was more than 1 hour ago
	return time.Since(*sc.LastTestedAt) > time.Hour
}

// SetTestResult updates the test result for the storage configuration
func (sc *StorageConfiguration) SetTestResult(success bool, errorMsg string) {
	now := time.Now()
	sc.LastTestedAt = &now
	
	if success {
		sc.LastTestError = nil
	} else {
		sc.LastTestError = &errorMsg
	}
}

// GetProviderDisplayName returns a human-readable name for the provider
func (sc *StorageConfiguration) GetProviderDisplayName() string {
	switch sc.Provider {
	case StorageProviderAWS:
		return "Amazon S3"
	case StorageProviderMinIO:
		return "MinIO"
	case StorageProviderBackblaze:
		return "Backblaze B2"
	case StorageProviderDigitalOcean:
		return "DigitalOcean Spaces"
	case StorageProviderWasabi:
		return "Wasabi"
	case StorageProviderGoogle:
		return "Google Cloud Storage"
	case StorageProviderCustom:
		return "Custom S3-Compatible"
	default:
		return string(sc.Provider)
	}
}

// GetStorageClassOptions returns available storage classes for the provider
func (sc *StorageConfiguration) GetStorageClassOptions() []string {
	switch sc.Provider {
	case StorageProviderAWS:
		return []string{"STANDARD", "REDUCED_REDUNDANCY", "STANDARD_IA", "ONEZONE_IA", "INTELLIGENT_TIERING", "GLACIER", "DEEP_ARCHIVE"}
	case StorageProviderGoogle:
		return []string{"STANDARD", "NEARLINE", "COLDLINE", "ARCHIVE"}
	case StorageProviderBackblaze:
		return []string{"STANDARD"}
	default:
		return []string{"STANDARD", "REDUCED_REDUNDANCY"}
	}
}

// SupportsServerSideEncryption checks if the provider supports server-side encryption
func (sc *StorageConfiguration) SupportsServerSideEncryption() bool {
	switch sc.Provider {
	case StorageProviderAWS, StorageProviderGoogle:
		return true
	default:
		return false
	}
}

// SupportsLifecycleManagement checks if the provider supports lifecycle management
func (sc *StorageConfiguration) SupportsLifecycleManagement() bool {
	switch sc.Provider {
	case StorageProviderAWS, StorageProviderGoogle, StorageProviderBackblaze:
		return true
	default:
		return false
	}
}

// GetMaxPartSize returns the maximum part size for multipart uploads
func (sc *StorageConfiguration) GetMaxPartSize() int64 {
	switch sc.Provider {
	case StorageProviderAWS:
		return 5 * 1024 * 1024 * 1024 // 5GB
	default:
		return 100 * 1024 * 1024 // 100MB
	}
}

// GetMinPartSize returns the minimum part size for multipart uploads
func (sc *StorageConfiguration) GetMinPartSize() int64 {
	switch sc.Provider {
	case StorageProviderAWS:
		return 5 * 1024 * 1024 // 5MB
	default:
		return 1024 * 1024 // 1MB
	}
}

// UpdateStats updates the usage statistics
func (sc *StorageConfiguration) UpdateStats(objects int64, size int64) {
	sc.TotalObjects = &objects
	sc.TotalSize = &size
	now := time.Now()
	sc.LastStatsUpdate = &now
}

// GetFormattedSize returns a human-readable size string
func (sc *StorageConfiguration) GetFormattedSize() string {
	if sc.TotalSize == nil {
		return "Unknown"
	}
	return formatBytes(*sc.TotalSize)
}

// GetObjectCount returns the number of objects in storage
func (sc *StorageConfiguration) GetObjectCount() int64 {
	if sc.TotalObjects == nil {
		return 0
	}
	return *sc.TotalObjects
}

// IsStatsStale checks if the statistics are stale (older than 24 hours)
func (sc *StorageConfiguration) IsStatsStale() bool {
	if sc.LastStatsUpdate == nil {
		return true
	}
	return time.Since(*sc.LastStatsUpdate) > 24*time.Hour
}

// GetConnectionString returns a connection string (without credentials)
func (sc *StorageConfiguration) GetConnectionString() string {
	endpoint := sc.GetEndpointURL()
	return fmt.Sprintf("%s/%s", endpoint, sc.Bucket)
}

// SetAsDefault marks this storage configuration as the default
func (sc *StorageConfiguration) SetAsDefault() {
	sc.IsDefault = true
}

// RemoveAsDefault removes this storage configuration as the default
func (sc *StorageConfiguration) RemoveAsDefault() {
	sc.IsDefault = false
}

// ValidateConfiguration performs basic validation on the configuration
func (sc *StorageConfiguration) ValidateConfiguration() []string {
	var errors []string
	
	if sc.Bucket == "" {
		errors = append(errors, "Bucket name is required")
	}
	
	if sc.Region == "" {
		errors = append(errors, "Region is required")
	}
	
	if sc.AccessKey == "" {
		errors = append(errors, "Access key is required")
	}
	
	if sc.SecretKey == "" {
		errors = append(errors, "Secret key is required")
	}
	
	if sc.PartSize < sc.GetMinPartSize() {
		errors = append(errors, fmt.Sprintf("Part size must be at least %d bytes", sc.GetMinPartSize()))
	}
	
	if sc.PartSize > sc.GetMaxPartSize() {
		errors = append(errors, fmt.Sprintf("Part size cannot exceed %d bytes", sc.GetMaxPartSize()))
	}
	
	return errors
}

// GetAge returns the age of the storage configuration
func (sc *StorageConfiguration) GetAge() time.Duration {
	return time.Since(sc.CreatedAt)
}