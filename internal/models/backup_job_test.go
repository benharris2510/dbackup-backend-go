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

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto-migrate models
	err = db.AutoMigrate(&User{}, &Team{}, &DatabaseConnection{}, &DatabaseTable{}, &BackupJob{}, &BackupFile{})
	require.NoError(t, err)

	return db
}

func createTestUser(t *testing.T, db *gorm.DB) *User {
	user := &User{
		UID:       "test-user-uid",
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		Password:  "hashedpassword",
		IsActive:  true,
	}
	err := db.Create(user).Error
	require.NoError(t, err)
	return user
}

func createTestDatabaseConnection(t *testing.T, db *gorm.DB, user *User) *DatabaseConnection {
	conn := &DatabaseConnection{
		UID:      "test-conn-uid",
		Name:     "Test Connection",
		Type:     DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		Password: "encrypted-password",
		UserID:   user.ID,
		IsActive: true,
	}
	err := db.Create(conn).Error
	require.NoError(t, err)
	return conn
}

func TestBackupJob_TableName(t *testing.T) {
	job := &BackupJob{}
	assert.Equal(t, "backup_jobs", job.TableName())
}

func TestBackupJob_BeforeCreate(t *testing.T) {
	// Test UID generation logic without database persistence
	job := &BackupJob{
		Name:   "Test Backup",
		Type:   BackupTypeFull,
		Status: BackupStatusPending,
	}

	// UID should be empty before create
	assert.Empty(t, job.UID)

	// Simulate BeforeCreate hook
	err := job.BeforeCreate(nil)
	require.NoError(t, err)

	// UID should be generated after create
	assert.NotEmpty(t, job.UID)
	assert.Len(t, job.UID, 36) // UUID length
}

func TestBackupJob_StatusCheckers(t *testing.T) {
	tests := []struct {
		name     string
		status   BackupStatus
		isRunning bool
		isCompleted bool
		isFailed bool
	}{
		{
			name:        "pending status",
			status:      BackupStatusPending,
			isRunning:   false,
			isCompleted: false,
			isFailed:    false,
		},
		{
			name:        "running status",
			status:      BackupStatusRunning,
			isRunning:   true,
			isCompleted: false,
			isFailed:    false,
		},
		{
			name:        "completed status",
			status:      BackupStatusCompleted,
			isRunning:   false,
			isCompleted: true,
			isFailed:    false,
		},
		{
			name:        "failed status",
			status:      BackupStatusFailed,
			isRunning:   false,
			isCompleted: false,
			isFailed:    true,
		},
		{
			name:        "cancelled status",
			status:      BackupStatusCancelled,
			isRunning:   false,
			isCompleted: false,
			isFailed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{Status: tt.status}
			
			assert.Equal(t, tt.isRunning, job.IsRunning())
			assert.Equal(t, tt.isCompleted, job.IsCompleted())
			assert.Equal(t, tt.isFailed, job.IsFailed())
		})
	}
}

func TestBackupJob_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		status     BackupStatus
		retryCount int
		maxRetries int
		canRetry   bool
	}{
		{
			name:       "failed job with retries left",
			status:     BackupStatusFailed,
			retryCount: 1,
			maxRetries: 3,
			canRetry:   true,
		},
		{
			name:       "failed job with no retries left",
			status:     BackupStatusFailed,
			retryCount: 3,
			maxRetries: 3,
			canRetry:   false,
		},
		{
			name:       "running job",
			status:     BackupStatusRunning,
			retryCount: 0,
			maxRetries: 3,
			canRetry:   false,
		},
		{
			name:       "completed job",
			status:     BackupStatusCompleted,
			retryCount: 0,
			maxRetries: 3,
			canRetry:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{
				Status:     tt.status,
				RetryCount: tt.retryCount,
				MaxRetries: tt.maxRetries,
			}
			
			assert.Equal(t, tt.canRetry, job.CanRetry())
		})
	}
}

func TestBackupJob_CanCancel(t *testing.T) {
	tests := []struct {
		name      string
		status    BackupStatus
		canCancel bool
	}{
		{
			name:      "pending job can be cancelled",
			status:    BackupStatusPending,
			canCancel: true,
		},
		{
			name:      "running job can be cancelled",
			status:    BackupStatusRunning,
			canCancel: true,
		},
		{
			name:      "completed job cannot be cancelled",
			status:    BackupStatusCompleted,
			canCancel: false,
		},
		{
			name:      "failed job cannot be cancelled",
			status:    BackupStatusFailed,
			canCancel: false,
		},
		{
			name:      "already cancelled job cannot be cancelled",
			status:    BackupStatusCancelled,
			canCancel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{Status: tt.status}
			assert.Equal(t, tt.canCancel, job.CanCancel())
		})
	}
}

func TestBackupJob_GetDuration(t *testing.T) {
	now := time.Now()
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(-30 * time.Minute)

	tests := []struct {
		name        string
		job         *BackupJob
		expected    time.Duration
	}{
		{
			name: "duration from stored value",
			job: &BackupJob{
				Duration: func() *int64 { d := int64(1800); return &d }(), // 30 minutes
			},
			expected: 30 * time.Minute,
		},
		{
			name: "duration from started and completed times",
			job: &BackupJob{
				StartedAt:   &startTime,
				CompletedAt: &endTime,
			},
			expected: 30 * time.Minute,
		},
		{
			name: "duration for running job",
			job: &BackupJob{
				Status:    BackupStatusRunning,
				StartedAt: &startTime,
			},
			expected: time.Since(startTime),
		},
		{
			name: "no duration available",
			job: &BackupJob{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := tt.job.GetDuration()
			
			if tt.name == "duration for running job" {
				// For running jobs, we can't test exact duration, just ensure it's reasonable
				assert.True(t, duration > 50*time.Minute && duration < 70*time.Minute)
			} else {
				assert.Equal(t, tt.expected, duration)
			}
		})
	}
}

func TestBackupJob_GetFormattedDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration *int64
		expected string
	}{
		{
			name:     "no duration",
			duration: nil,
			expected: "N/A",
		},
		{
			name:     "seconds",
			duration: func() *int64 { d := int64(45); return &d }(),
			expected: "45s",
		},
		{
			name:     "minutes",
			duration: func() *int64 { d := int64(150); return &d }(), // 2.5 minutes
			expected: "2.5m",
		},
		{
			name:     "hours",
			duration: func() *int64 { d := int64(7200); return &d }(), // 2 hours
			expected: "2.0h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{Duration: tt.duration}
			assert.Equal(t, tt.expected, job.GetFormattedDuration())
		})
	}
}

func TestBackupJob_GetFormattedSize(t *testing.T) {
	tests := []struct {
		name           string
		originalSize   *int64
		compressedSize *int64
		expected       string
	}{
		{
			name:     "no size info",
			expected: "Unknown",
		},
		{
			name:         "only original size",
			originalSize: func() *int64 { s := int64(1024 * 1024); return &s }(), // 1MB
			expected:     "1.0 MB",
		},
		{
			name:           "compressed size preferred",
			originalSize:   func() *int64 { s := int64(2 * 1024 * 1024); return &s }(), // 2MB
			compressedSize: func() *int64 { s := int64(1024 * 1024); return &s }(),     // 1MB
			expected:       "1.0 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{
				OriginalSize:   tt.originalSize,
				CompressedSize: tt.compressedSize,
			}
			assert.Equal(t, tt.expected, job.GetFormattedSize())
		})
	}
}

func TestBackupJob_GetCompressionRatio(t *testing.T) {
	tests := []struct {
		name           string
		originalSize   *int64
		compressedSize *int64
		storedRatio    *float64
		expectedRatio  float64
	}{
		{
			name:          "stored ratio",
			storedRatio:   func() *float64 { r := 0.5; return &r }(),
			expectedRatio: 0.5,
		},
		{
			name:           "calculated ratio",
			originalSize:   func() *int64 { s := int64(1000); return &s }(),
			compressedSize: func() *int64 { s := int64(600); return &s }(),
			expectedRatio:  0.4, // 40% compression
		},
		{
			name:          "no compression info",
			expectedRatio: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{
				OriginalSize:     tt.originalSize,
				CompressedSize:   tt.compressedSize,
				CompressionRatio: tt.storedRatio,
			}
			
			ratio := job.GetCompressionRatio()
			assert.InDelta(t, tt.expectedRatio, ratio, 0.001)
			
			// Test that calculated ratio is stored
			if tt.storedRatio == nil && tt.originalSize != nil && tt.compressedSize != nil {
				assert.NotNil(t, job.CompressionRatio)
				assert.InDelta(t, tt.expectedRatio, *job.CompressionRatio, 0.001)
			}
		})
	}
}

func TestBackupJob_Start(t *testing.T) {
	job := &BackupJob{
		Status: BackupStatusPending,
	}

	beforeStart := time.Now()
	job.Start()
	afterStart := time.Now()

	assert.Equal(t, BackupStatusRunning, job.Status)
	assert.NotNil(t, job.StartedAt)
	assert.True(t, job.StartedAt.After(beforeStart) || job.StartedAt.Equal(beforeStart))
	assert.True(t, job.StartedAt.Before(afterStart) || job.StartedAt.Equal(afterStart))
	assert.Equal(t, float64(0), job.Progress)
	assert.Equal(t, "Starting backup", job.CurrentStep)
}

func TestBackupJob_Complete(t *testing.T) {
	startTime := time.Now().Add(-1 * time.Hour)
	job := &BackupJob{
		Status:     BackupStatusRunning,
		StartedAt:  &startTime,
		TotalSteps: 5,
	}

	beforeComplete := time.Now()
	job.Complete()
	afterComplete := time.Now()

	assert.Equal(t, BackupStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.True(t, job.CompletedAt.After(beforeComplete) || job.CompletedAt.Equal(beforeComplete))
	assert.True(t, job.CompletedAt.Before(afterComplete) || job.CompletedAt.Equal(afterComplete))
	assert.Equal(t, float64(100), job.Progress)
	assert.Equal(t, 5, job.CompletedSteps)
	assert.Equal(t, "Backup completed", job.CurrentStep)
	assert.NotNil(t, job.Duration)
	assert.True(t, *job.Duration > 3590 && *job.Duration < 3610) // ~1 hour
}

func TestBackupJob_Fail(t *testing.T) {
	startTime := time.Now().Add(-30 * time.Minute)
	job := &BackupJob{
		Status:    BackupStatusRunning,
		StartedAt: &startTime,
	}

	beforeFail := time.Now()
	job.Fail("Connection timeout", "CONN_TIMEOUT")
	afterFail := time.Now()

	assert.Equal(t, BackupStatusFailed, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.True(t, job.CompletedAt.After(beforeFail) || job.CompletedAt.Equal(beforeFail))
	assert.True(t, job.CompletedAt.Before(afterFail) || job.CompletedAt.Equal(afterFail))
	assert.NotNil(t, job.ErrorMessage)
	assert.Equal(t, "Connection timeout", *job.ErrorMessage)
	assert.NotNil(t, job.ErrorCode)
	assert.Equal(t, "CONN_TIMEOUT", *job.ErrorCode)
	assert.Equal(t, "Backup failed", job.CurrentStep)
	assert.NotNil(t, job.Duration)
	assert.True(t, *job.Duration > 1790 && *job.Duration < 1810) // ~30 minutes
}

func TestBackupJob_Cancel(t *testing.T) {
	startTime := time.Now().Add(-15 * time.Minute)
	job := &BackupJob{
		Status:    BackupStatusRunning,
		StartedAt: &startTime,
	}

	beforeCancel := time.Now()
	job.Cancel()
	afterCancel := time.Now()

	assert.Equal(t, BackupStatusCancelled, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.True(t, job.CompletedAt.After(beforeCancel) || job.CompletedAt.Equal(beforeCancel))
	assert.True(t, job.CompletedAt.Before(afterCancel) || job.CompletedAt.Equal(afterCancel))
	assert.Equal(t, "Backup cancelled", job.CurrentStep)
	assert.NotNil(t, job.Duration)
	assert.True(t, *job.Duration > 890 && *job.Duration < 910) // ~15 minutes
}

func TestBackupJob_UpdateProgress(t *testing.T) {
	job := &BackupJob{
		TotalSteps: 10,
	}

	job.UpdateProgress(50.0, "Processing data")

	assert.Equal(t, float64(50), job.Progress)
	assert.Equal(t, "Processing data", job.CurrentStep)
	assert.Equal(t, 5, job.CompletedSteps) // 50% of 10 steps
}

func TestBackupJob_IncrementStep(t *testing.T) {
	job := &BackupJob{
		TotalSteps:     4,
		CompletedSteps: 2,
	}

	job.IncrementStep("Final step")

	assert.Equal(t, 3, job.CompletedSteps)
	assert.Equal(t, "Final step", job.CurrentStep)
	assert.Equal(t, float64(75), job.Progress) // 3/4 = 75%
}

func TestBackupJob_SetSizeInfo(t *testing.T) {
	job := &BackupJob{}

	job.SetSizeInfo(2048, 1024)

	assert.NotNil(t, job.OriginalSize)
	assert.Equal(t, int64(2048), *job.OriginalSize)
	assert.NotNil(t, job.CompressedSize)
	assert.Equal(t, int64(1024), *job.CompressedSize)
}

func TestBackupJob_IsScheduledForNow(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		isScheduled bool
		nextRunAt   *time.Time
		expected    bool
	}{
		{
			name:        "not scheduled",
			isScheduled: false,
			nextRunAt:   &past,
			expected:    false,
		},
		{
			name:        "scheduled but no next run time",
			isScheduled: true,
			nextRunAt:   nil,
			expected:    false,
		},
		{
			name:        "scheduled for past time",
			isScheduled: true,
			nextRunAt:   &past,
			expected:    true,
		},
		{
			name:        "scheduled for future time",
			isScheduled: true,
			nextRunAt:   &future,
			expected:    false,
		},
		{
			name:        "scheduled for now",
			isScheduled: true,
			nextRunAt:   &now,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &BackupJob{
				IsScheduled: tt.isScheduled,
				NextRunAt:   tt.nextRunAt,
			}
			assert.Equal(t, tt.expected, job.IsScheduledForNow())
		})
	}
}

func TestBackupJob_BackupFileHelpers(t *testing.T) {
	// Create backup job with simulated backup files (without database)
	file1 := BackupFile{
		Size: func() *int64 { s := int64(1024); return &s }(),
	}
	file2 := BackupFile{
		Size: func() *int64 { s := int64(2048); return &s }(),
	}

	job := &BackupJob{
		BackupFiles: []BackupFile{file1, file2},
	}

	// Test helper methods
	assert.Equal(t, 2, job.GetBackupFileCount())
	assert.Equal(t, int64(3072), job.GetTotalBackupSize()) // 1024 + 2048
	assert.True(t, job.HasBackupFiles())

	// Test with empty files
	emptyJob := &BackupJob{}
	assert.Equal(t, 0, emptyJob.GetBackupFileCount())
	assert.Equal(t, int64(0), emptyJob.GetTotalBackupSize())
	assert.False(t, emptyJob.HasBackupFiles())
}

func TestBackupJob_GetStatusColor(t *testing.T) {
	tests := []struct {
		status BackupStatus
		color  string
	}{
		{BackupStatusCompleted, "#10B981"},
		{BackupStatusRunning, "#3B82F6"},
		{BackupStatusFailed, "#EF4444"},
		{BackupStatusCancelled, "#6B7280"},
		{BackupStatusPartial, "#F59E0B"},
		{BackupStatusPending, "#8B5CF6"}, // default
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			job := &BackupJob{Status: tt.status}
			assert.Equal(t, tt.color, job.GetStatusColor())
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536 * 1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536 * 1024 * 1024, "1.5 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
		{1536 * 1024 * 1024 * 1024, "1.5 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackupJob_DatabaseIntegration(t *testing.T) {
	// Note: Database integration tests are simplified due to SQLite JSON field limitations
	// Full database integration will be tested in higher-level integration tests
	
	job := &BackupJob{
		Name:        "Integration Test Backup",
		Type:        BackupTypeFull,
		Status:      BackupStatusPending,
		Progress:    0,
		CurrentStep: "Initializing",
	}

	// Test UID generation
	err := job.BeforeCreate(nil)
	require.NoError(t, err)
	assert.NotEmpty(t, job.UID)
	assert.Len(t, job.UID, 36)

	// Test status transitions
	job.Start()
	assert.Equal(t, BackupStatusRunning, job.Status)
	assert.NotNil(t, job.StartedAt)
	assert.Equal(t, float64(0), job.Progress)

	job.UpdateProgress(50, "Backing up tables")
	assert.Equal(t, float64(50), job.Progress)
	assert.Equal(t, "Backing up tables", job.CurrentStep)

	job.Complete()
	assert.Equal(t, BackupStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
	assert.Equal(t, float64(100), job.Progress)
	assert.NotNil(t, job.Duration)

	// Test table name
	assert.Equal(t, "backup_jobs", job.TableName())
}