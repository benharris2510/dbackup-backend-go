package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// BackupFile represents a file stored in S3 for a backup job
type BackupFile struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	UID  string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// File information
	OriginalName string  `json:"original_name" gorm:"type:varchar(255);not null"`
	MimeType     string  `json:"mime_type" gorm:"type:varchar(100);default:'application/octet-stream'"`
	Size         *int64  `json:"size,omitempty"`
	Checksum     *string `json:"checksum,omitempty" gorm:"type:varchar(64)"` // SHA256 hash
	
	// S3 storage information
	S3Bucket     string  `json:"s3_bucket" gorm:"type:varchar(255);not null"`
	S3Key        string  `json:"s3_key" gorm:"type:varchar(1000);not null"`
	S3Region     string  `json:"s3_region" gorm:"type:varchar(50);not null"`
	S3Endpoint   *string `json:"s3_endpoint,omitempty" gorm:"type:varchar(255)"` // For S3-compatible services
	StorageClass string  `json:"storage_class" gorm:"type:varchar(50);default:'STANDARD'"`
	
	// Encryption
	IsEncrypted     bool    `json:"is_encrypted" gorm:"default:true"`
	EncryptionKey   *string `json:"-" gorm:"type:varchar(255)"` // Encrypted encryption key
	EncryptionAlgo  string  `json:"encryption_algo" gorm:"type:varchar(50);default:'AES256'"`
	
	// Compression
	IsCompressed   bool    `json:"is_compressed" gorm:"default:true"`
	CompressionAlgo string `json:"compression_algo" gorm:"type:varchar(50);default:'gzip'"`
	OriginalSize   *int64  `json:"original_size,omitempty"`
	
	// File metadata
	FileType     string                 `json:"file_type" gorm:"type:varchar(50);not null"` // dump, schema, data, log
	ContentType  string                 `json:"content_type" gorm:"type:varchar(100);default:'application/sql'"`
	Metadata     map[string]interface{} `json:"metadata" gorm:"type:json"`
	
	// Retention and lifecycle
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	IsArchived   bool       `json:"is_archived" gorm:"default:false"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
	
	// Access control
	IsPublic      bool   `json:"is_public" gorm:"default:false"`
	AccessToken   *string `json:"-" gorm:"type:varchar(255)"` // For presigned URL generation
	DownloadCount int    `json:"download_count" gorm:"default:0"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	
	// Relationships
	BackupJobID uint      `json:"backup_job_id" gorm:"not null;index"`
	BackupJob   BackupJob `json:"backup_job,omitempty" gorm:"foreignKey:BackupJobID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the BackupFile model
func (BackupFile) TableName() string {
	return "backup_files"
}

// BeforeCreate hook to generate UID before creating backup file
func (bf *BackupFile) BeforeCreate(tx *gorm.DB) error {
	if bf.UID == "" {
		bf.UID = generateUID()
	}
	return nil
}

// GetS3URL returns the full S3 URL for the file
func (bf *BackupFile) GetS3URL() string {
	if bf.S3Endpoint != nil && *bf.S3Endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", *bf.S3Endpoint, bf.S3Bucket, bf.S3Key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bf.S3Bucket, bf.S3Region, bf.S3Key)
}

// GetFormattedSize returns a human-readable size string
func (bf *BackupFile) GetFormattedSize() string {
	size := bf.Size
	if size == nil {
		return "Unknown"
	}
	
	return formatBytes(*size)
}

// GetCompressionRatio calculates the compression ratio if both sizes are available
func (bf *BackupFile) GetCompressionRatio() float64 {
	if bf.OriginalSize != nil && bf.Size != nil && *bf.OriginalSize > 0 {
		return 1.0 - (float64(*bf.Size) / float64(*bf.OriginalSize))
	}
	return 0.0
}

// IsExpired checks if the backup file has expired
func (bf *BackupFile) IsExpired() bool {
	return bf.ExpiresAt != nil && bf.ExpiresAt.Before(time.Now())
}

// ShouldBeArchived checks if the file should be archived based on age
func (bf *BackupFile) ShouldBeArchived(archiveAfterDays int) bool {
	if bf.IsArchived || archiveAfterDays <= 0 {
		return false
	}
	
	archiveTime := bf.CreatedAt.AddDate(0, 0, archiveAfterDays)
	return time.Now().After(archiveTime)
}

// Archive marks the file as archived
func (bf *BackupFile) Archive() {
	bf.IsArchived = true
	now := time.Now()
	bf.ArchivedAt = &now
}

// IncrementDownloadCount increments the download counter
func (bf *BackupFile) IncrementDownloadCount() {
	bf.DownloadCount++
	now := time.Now()
	bf.LastAccessedAt = &now
}

// GetFileExtension returns the file extension based on the file type and compression
func (bf *BackupFile) GetFileExtension() string {
	var ext string
	
	switch bf.FileType {
	case "dump":
		ext = ".sql"
	case "schema":
		ext = ".schema.sql"
	case "data":
		ext = ".data.sql"
	case "log":
		ext = ".log"
	default:
		ext = ".dump"
	}
	
	if bf.IsCompressed {
		switch bf.CompressionAlgo {
		case "gzip":
			ext += ".gz"
		case "bzip2":
			ext += ".bz2"
		case "xz":
			ext += ".xz"
		}
	}
	
	return ext
}

// GetStorageInfo returns a summary of storage information
func (bf *BackupFile) GetStorageInfo() map[string]interface{} {
	info := map[string]interface{}{
		"bucket":        bf.S3Bucket,
		"key":          bf.S3Key,
		"region":       bf.S3Region,
		"storage_class": bf.StorageClass,
		"encrypted":    bf.IsEncrypted,
		"compressed":   bf.IsCompressed,
	}
	
	if bf.S3Endpoint != nil {
		info["endpoint"] = *bf.S3Endpoint
	}
	
	return info
}

// ValidateChecksum validates the file checksum
func (bf *BackupFile) ValidateChecksum(actualChecksum string) bool {
	return bf.Checksum != nil && *bf.Checksum == actualChecksum
}

// SetChecksum sets the file checksum
func (bf *BackupFile) SetChecksum(checksum string) {
	bf.Checksum = &checksum
}

// IsAccessible checks if the file is accessible (not expired, not deleted)
func (bf *BackupFile) IsAccessible() bool {
	return !bf.IsExpired() && bf.DeletedAt.Time.IsZero()
}

// GetAge returns the age of the backup file
func (bf *BackupFile) GetAge() time.Duration {
	return time.Since(bf.CreatedAt)
}

// GetFormattedAge returns a human-readable age string
func (bf *BackupFile) GetFormattedAge() string {
	age := bf.GetAge()
	
	if age < time.Hour {
		return fmt.Sprintf("%.0f minutes", age.Minutes())
	} else if age < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", age.Hours())
	} else {
		days := int(age.Hours() / 24)
		return fmt.Sprintf("%d days", days)
	}
}

// SetRetentionPolicy sets the expiration date based on retention policy
func (bf *BackupFile) SetRetentionPolicy(retentionDays int) {
	if retentionDays > 0 {
		expiry := bf.CreatedAt.AddDate(0, 0, retentionDays)
		bf.ExpiresAt = &expiry
	}
}

// GetTypeIcon returns an icon name for the file type
func (bf *BackupFile) GetTypeIcon() string {
	switch bf.FileType {
	case "dump":
		return "database"
	case "schema":
		return "table"
	case "data":
		return "data"
	case "log":
		return "document-text"
	default:
		return "document"
	}
}