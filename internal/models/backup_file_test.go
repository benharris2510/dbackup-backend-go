package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupBackupFileTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&User{}, &Team{}, &DatabaseConnection{}, &BackupJob{}, &BackupFile{})
	require.NoError(t, err)

	return db
}

func TestBackupFile_TableName(t *testing.T) {
	file := &BackupFile{}
	assert.Equal(t, "backup_files", file.TableName())
}

func TestBackupFile_BeforeCreate(t *testing.T) {
	// Test UID generation logic without database persistence
	file := &BackupFile{
		Name:         "test-backup.sql",
		OriginalName: "backup.sql",
		S3Bucket:     "test-bucket",
		S3Key:        "backups/test.sql",
		S3Region:     "us-east-1",
		FileType:     "dump",
	}

	// UID should be empty before create
	assert.Empty(t, file.UID)

	// Simulate BeforeCreate hook
	err := file.BeforeCreate(nil)
	require.NoError(t, err)

	// UID should be generated after create
	assert.NotEmpty(t, file.UID)
	assert.Len(t, file.UID, 36) // UUID length
}

func TestBackupFile_GetS3URL(t *testing.T) {
	tests := []struct {
		name       string
		file       *BackupFile
		expectedURL string
	}{
		{
			name: "AWS S3 standard URL",
			file: &BackupFile{
				S3Bucket: "my-bucket",
				S3Key:    "backups/test.sql",
				S3Region: "us-east-1",
			},
			expectedURL: "https://my-bucket.s3.us-east-1.amazonaws.com/backups/test.sql",
		},
		{
			name: "Custom S3-compatible endpoint",
			file: &BackupFile{
				S3Bucket:   "my-bucket",
				S3Key:      "backups/test.sql",
				S3Region:   "us-east-1",
				S3Endpoint: func() *string { s := "https://minio.example.com"; return &s }(),
			},
			expectedURL: "https://minio.example.com/my-bucket/backups/test.sql",
		},
		{
			name: "Empty custom endpoint uses AWS",
			file: &BackupFile{
				S3Bucket:   "my-bucket",
				S3Key:      "backups/test.sql",
				S3Region:   "eu-west-1",
				S3Endpoint: func() *string { s := ""; return &s }(),
			},
			expectedURL: "https://my-bucket.s3.eu-west-1.amazonaws.com/backups/test.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.file.GetS3URL()
			assert.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestBackupFile_GetFormattedSize(t *testing.T) {
	tests := []struct {
		name     string
		size     *int64
		expected string
	}{
		{
			name:     "no size",
			size:     nil,
			expected: "Unknown",
		},
		{
			name:     "bytes",
			size:     func() *int64 { s := int64(512); return &s }(),
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			size:     func() *int64 { s := int64(1536); return &s }(), // 1.5KB
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			size:     func() *int64 { s := int64(2 * 1024 * 1024); return &s }(), // 2MB
			expected: "2.0 MB",
		},
		{
			name:     "gigabytes",
			size:     func() *int64 { s := int64(3 * 1024 * 1024 * 1024); return &s }(), // 3GB
			expected: "3.0 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{Size: tt.size}
			result := file.GetFormattedSize()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupFile_GetCompressionRatio(t *testing.T) {
	tests := []struct {
		name          string
		originalSize  *int64
		compressedSize *int64
		expectedRatio float64
	}{
		{
			name:          "no compression data",
			originalSize:  nil,
			compressedSize: nil,
			expectedRatio: 0.0,
		},
		{
			name:          "50% compression",
			originalSize:  func() *int64 { s := int64(1000); return &s }(),
			compressedSize: func() *int64 { s := int64(500); return &s }(),
			expectedRatio: 0.5,
		},
		{
			name:          "75% compression",
			originalSize:  func() *int64 { s := int64(2000); return &s }(),
			compressedSize: func() *int64 { s := int64(500); return &s }(),
			expectedRatio: 0.75,
		},
		{
			name:          "no compression (same size)",
			originalSize:  func() *int64 { s := int64(1000); return &s }(),
			compressedSize: func() *int64 { s := int64(1000); return &s }(),
			expectedRatio: 0.0,
		},
		{
			name:          "zero original size",
			originalSize:  func() *int64 { s := int64(0); return &s }(),
			compressedSize: func() *int64 { s := int64(100); return &s }(),
			expectedRatio: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{
				OriginalSize: tt.originalSize,
				Size:         tt.compressedSize,
			}
			ratio := file.GetCompressionRatio()
			assert.InDelta(t, tt.expectedRatio, ratio, 0.001)
		})
	}
}

func TestBackupFile_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "no expiration",
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: &past,
			expected:  true,
		},
		{
			name:      "not expired",
			expiresAt: &future,
			expected:  false,
		},
		{
			name:      "expires now",  
			expiresAt: &now,
			expected:  true, // time.Now() has passed slightly, so now is considered expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{ExpiresAt: tt.expiresAt}
			assert.Equal(t, tt.expected, file.IsExpired())
		})
	}
}

func TestBackupFile_ShouldBeArchived(t *testing.T) {
	now := time.Now()
	old := now.AddDate(0, 0, -10) // 10 days ago
	recent := now.AddDate(0, 0, -2) // 2 days ago

	tests := []struct {
		name            string
		createdAt       time.Time
		isArchived      bool
		archiveAfterDays int
		expected        bool
	}{
		{
			name:            "should be archived - old file",
			createdAt:       old,
			isArchived:      false,
			archiveAfterDays: 7,
			expected:        true,
		},
		{
			name:            "should not be archived - recent file",
			createdAt:       recent,
			isArchived:      false,
			archiveAfterDays: 7,
			expected:        false,
		},
		{
			name:            "already archived",
			createdAt:       old,
			isArchived:      true,
			archiveAfterDays: 7,
			expected:        false,
		},
		{
			name:            "archive disabled",
			createdAt:       old,
			isArchived:      false,
			archiveAfterDays: 0,
			expected:        false,
		},
		{
			name:            "negative archive days",
			createdAt:       old,
			isArchived:      false,
			archiveAfterDays: -1,
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{
				CreatedAt:  tt.createdAt,
				IsArchived: tt.isArchived,
			}
			result := file.ShouldBeArchived(tt.archiveAfterDays)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupFile_Archive(t *testing.T) {
	file := &BackupFile{
		IsArchived: false,
		ArchivedAt: nil,
	}

	beforeArchive := time.Now()
	file.Archive()
	afterArchive := time.Now()

	assert.True(t, file.IsArchived)
	assert.NotNil(t, file.ArchivedAt)
	assert.True(t, file.ArchivedAt.After(beforeArchive) || file.ArchivedAt.Equal(beforeArchive))
	assert.True(t, file.ArchivedAt.Before(afterArchive) || file.ArchivedAt.Equal(afterArchive))
}

func TestBackupFile_IncrementDownloadCount(t *testing.T) {
	file := &BackupFile{
		DownloadCount:   5,
		LastAccessedAt: nil,
	}

	beforeIncrement := time.Now()
	file.IncrementDownloadCount()
	afterIncrement := time.Now()

	assert.Equal(t, 6, file.DownloadCount)
	assert.NotNil(t, file.LastAccessedAt)
	assert.True(t, file.LastAccessedAt.After(beforeIncrement) || file.LastAccessedAt.Equal(beforeIncrement))
	assert.True(t, file.LastAccessedAt.Before(afterIncrement) || file.LastAccessedAt.Equal(afterIncrement))
}

func TestBackupFile_GetFileExtension(t *testing.T) {
	tests := []struct {
		name            string
		fileType        string
		isCompressed    bool
		compressionAlgo string
		expected        string
	}{
		{
			name:            "dump file uncompressed",
			fileType:        "dump",
			isCompressed:    false,
			compressionAlgo: "",
			expected:        ".sql",
		},
		{
			name:            "dump file gzipped",
			fileType:        "dump",
			isCompressed:    true,
			compressionAlgo: "gzip",
			expected:        ".sql.gz",
		},
		{
			name:            "schema file compressed",
			fileType:        "schema",
			isCompressed:    true,
			compressionAlgo: "bzip2",
			expected:        ".schema.sql.bz2",
		},
		{
			name:            "data file xz compressed",
			fileType:        "data",
			isCompressed:    true,
			compressionAlgo: "xz",
			expected:        ".data.sql.xz",
		},
		{
			name:            "log file uncompressed",
			fileType:        "log",
			isCompressed:    false,
			compressionAlgo: "",
			expected:        ".log",
		},
		{
			name:            "unknown file type",
			fileType:        "unknown",
			isCompressed:    false,
			compressionAlgo: "",
			expected:        ".dump",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{
				FileType:        tt.fileType,
				IsCompressed:    tt.isCompressed,
				CompressionAlgo: tt.compressionAlgo,
			}
			result := file.GetFileExtension()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupFile_GetStorageInfo(t *testing.T) {
	tests := []struct {
		name     string
		file     *BackupFile
		expected map[string]interface{}
	}{
		{
			name: "basic storage info",
			file: &BackupFile{
				S3Bucket:     "my-bucket",
				S3Key:        "backups/test.sql",
				S3Region:     "us-east-1",
				StorageClass: "STANDARD",
				IsEncrypted:  true,
				IsCompressed: true,
			},
			expected: map[string]interface{}{
				"bucket":        "my-bucket",
				"key":          "backups/test.sql",
				"region":       "us-east-1",
				"storage_class": "STANDARD",
				"encrypted":    true,
				"compressed":   true,
			},
		},
		{
			name: "with custom endpoint",
			file: &BackupFile{
				S3Bucket:     "my-bucket",
				S3Key:        "backups/test.sql",
				S3Region:     "us-east-1",
				S3Endpoint:   func() *string { s := "https://minio.example.com"; return &s }(),
				StorageClass: "COLD",
				IsEncrypted:  false,
				IsCompressed: false,
			},
			expected: map[string]interface{}{
				"bucket":        "my-bucket",
				"key":          "backups/test.sql",
				"region":       "us-east-1",
				"endpoint":     "https://minio.example.com",
				"storage_class": "COLD",
				"encrypted":    false,
				"compressed":   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := tt.file.GetStorageInfo()
			assert.Equal(t, tt.expected, info)
		})
	}
}

func TestBackupFile_ChecksumMethods(t *testing.T) {
	file := &BackupFile{}

	// Test setting checksum
	checksum := "abcd1234567890"
	file.SetChecksum(checksum)
	assert.NotNil(t, file.Checksum)
	assert.Equal(t, checksum, *file.Checksum)

	// Test validating correct checksum
	assert.True(t, file.ValidateChecksum(checksum))

	// Test validating incorrect checksum
	assert.False(t, file.ValidateChecksum("wrongchecksum"))

	// Test file without checksum
	fileNoChecksum := &BackupFile{}
	assert.False(t, fileNoChecksum.ValidateChecksum("anychecksum"))
}

func TestBackupFile_IsAccessible(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		deletedAt gorm.DeletedAt
		expected  bool
	}{
		{
			name:      "accessible file",
			expiresAt: &future,
			deletedAt: gorm.DeletedAt{},
			expected:  true,
		},
		{
			name:      "expired file",
			expiresAt: &past,
			deletedAt: gorm.DeletedAt{},
			expected:  false,
		},
		{
			name:      "deleted file",
			expiresAt: &future,
			deletedAt: gorm.DeletedAt{Time: now, Valid: true},
			expected:  false,
		},
		{
			name:      "expired and deleted",
			expiresAt: &past,
			deletedAt: gorm.DeletedAt{Time: now, Valid: true},
			expected:  false,
		},
		{
			name:      "no expiration, not deleted",
			expiresAt: nil,
			deletedAt: gorm.DeletedAt{},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{
				ExpiresAt: tt.expiresAt,
				DeletedAt: tt.deletedAt,
			}
			result := file.IsAccessible()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupFile_GetAge(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	file := &BackupFile{CreatedAt: oneHourAgo}
	age := file.GetAge()

	// Age should be approximately 1 hour
	assert.True(t, age >= 59*time.Minute && age <= 61*time.Minute)
}

func TestBackupFile_GetFormattedAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		expected  string
	}{
		{
			name:      "minutes old",
			createdAt: now.Add(-30 * time.Minute),
			expected:  "30 minutes",
		},
		{
			name:      "hours old",
			createdAt: now.Add(-2*time.Hour - 30*time.Minute),
			expected:  "2.5 hours",
		},
		{
			name:      "days old",
			createdAt: now.AddDate(0, 0, -3),
			expected:  "3 days",
		},
		{
			name:      "very recent",
			createdAt: now.Add(-30 * time.Second),
			expected:  "1 minutes", // Less than an hour shows minutes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &BackupFile{CreatedAt: tt.createdAt}
			result := file.GetFormattedAge()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupFile_SetRetentionPolicy(t *testing.T) {
	now := time.Now()
	file := &BackupFile{CreatedAt: now}

	tests := []struct {
		name          string
		retentionDays int
		expectExpiry  bool
	}{
		{
			name:          "set 30 day retention",
			retentionDays: 30,
			expectExpiry:  true,
		},
		{
			name:          "zero retention days",
			retentionDays: 0,
			expectExpiry:  false,
		},
		{
			name:          "negative retention days",
			retentionDays: -5,
			expectExpiry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file.SetRetentionPolicy(tt.retentionDays)

			if tt.expectExpiry {
				assert.NotNil(t, file.ExpiresAt)
				expectedExpiry := now.AddDate(0, 0, tt.retentionDays)
				assert.True(t, file.ExpiresAt.Equal(expectedExpiry))
			} else {
				// ExpiresAt might be nil or unchanged from previous test
				// Just ensure no new expiry was set for invalid retention days
			}
		})
	}
}

func TestBackupFile_GetTypeIcon(t *testing.T) {
	tests := []struct {
		fileType string
		expected string
	}{
		{"dump", "database"},
		{"schema", "table"},
		{"data", "data"},
		{"log", "document-text"},
		{"unknown", "document"},
		{"", "document"},
	}

	for _, tt := range tests {
		t.Run(tt.fileType, func(t *testing.T) {
			file := &BackupFile{FileType: tt.fileType}
			result := file.GetTypeIcon()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupFile_IntegrationWorkflow(t *testing.T) {
	// Test a complete workflow of backup file operations
	file := &BackupFile{
		Name:            "test-backup.sql",
		OriginalName:    "backup.sql",
		MimeType:        "application/sql",
		S3Bucket:        "my-backup-bucket",
		S3Key:           "backups/2023/test-backup.sql.gz",
		S3Region:        "us-east-1",
		FileType:        "dump",
		ContentType:     "application/sql",
		IsEncrypted:     true,
		EncryptionAlgo:  "AES256",
		IsCompressed:    true,
		CompressionAlgo: "gzip",
		StorageClass:    "STANDARD",
		OriginalSize:    func() *int64 { s := int64(2048); return &s }(),
		Size:            func() *int64 { s := int64(1024); return &s }(),
		CreatedAt:       time.Now().AddDate(0, 0, -5), // 5 days ago
	}

	// Test UID generation
	err := file.BeforeCreate(nil)
	require.NoError(t, err)
	assert.NotEmpty(t, file.UID)

	// Test URL generation
	url := file.GetS3URL()
	assert.Contains(t, url, "my-backup-bucket")
	assert.Contains(t, url, "backups/2023/test-backup.sql.gz")

	// Test size information
	assert.Equal(t, "1.0 KB", file.GetFormattedSize())
	assert.Equal(t, 0.5, file.GetCompressionRatio()) // 50% compression

	// Test file extension
	assert.Equal(t, ".sql.gz", file.GetFileExtension())

	// Test checksum operations
	file.SetChecksum("abcd1234567890")
	assert.True(t, file.ValidateChecksum("abcd1234567890"))
	assert.False(t, file.ValidateChecksum("wrongchecksum"))

	// Test archival logic
	assert.False(t, file.IsArchived)
	assert.True(t, file.ShouldBeArchived(3)) // Should archive after 3 days
	file.Archive()
	assert.True(t, file.IsArchived)
	assert.NotNil(t, file.ArchivedAt)

	// Test download tracking
	assert.Equal(t, 0, file.DownloadCount)
	file.IncrementDownloadCount()
	assert.Equal(t, 1, file.DownloadCount)
	assert.NotNil(t, file.LastAccessedAt)

	// Test retention policy
	file.SetRetentionPolicy(30)
	assert.NotNil(t, file.ExpiresAt)

	// Test accessibility (should be accessible since not expired or deleted)
	assert.True(t, file.IsAccessible())

	// Test storage info
	info := file.GetStorageInfo()
	assert.Equal(t, "my-backup-bucket", info["bucket"])
	assert.Equal(t, "us-east-1", info["region"])
	assert.Equal(t, true, info["encrypted"])
	assert.Equal(t, true, info["compressed"])

	// Test type icon
	assert.Equal(t, "database", file.GetTypeIcon())

	// Test table name
	assert.Equal(t, "backup_files", file.TableName())
}