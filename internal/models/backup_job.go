package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// BackupStatus represents the status of a backup job
type BackupStatus string

const (
	BackupStatusPending    BackupStatus = "pending"
	BackupStatusRunning    BackupStatus = "running"
	BackupStatusCompleted  BackupStatus = "completed"
	BackupStatusFailed     BackupStatus = "failed"
	BackupStatusCancelled  BackupStatus = "cancelled"
	BackupStatusPartial    BackupStatus = "partial"
)

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
	BackupTypeDifferential BackupType = "differential"
	BackupTypeSchema      BackupType = "schema"
	BackupTypeData        BackupType = "data"
)

// BackupJob represents a backup job
type BackupJob struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	UID  string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// Job configuration
	Type        BackupType `json:"type" gorm:"type:varchar(50);not null;default:'full'"`
	Status      BackupStatus `json:"status" gorm:"type:varchar(50);not null;default:'pending'"`
	Priority    int        `json:"priority" gorm:"default:5"` // 1-10, higher is more important
	
	// Backup scope
	IsTableSpecific bool     `json:"is_table_specific" gorm:"default:false"`
	Tables         []string `json:"tables" gorm:"type:json"` // Table names to backup
	ExcludeTables  []string `json:"exclude_tables" gorm:"type:json"` // Tables to exclude
	
	// Scheduling
	IsScheduled     bool       `json:"is_scheduled" gorm:"default:false"`
	ScheduleExpression *string `json:"schedule_expression,omitempty" gorm:"type:varchar(255)"` // Cron expression
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	
	// Execution details
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Duration     *int64     `json:"duration,omitempty"` // Duration in seconds
	
	// Progress tracking
	Progress        float64 `json:"progress" gorm:"default:0"`           // 0-100
	CurrentStep     string  `json:"current_step" gorm:"type:varchar(255)"`
	TotalSteps      int     `json:"total_steps" gorm:"default:1"`
	CompletedSteps  int     `json:"completed_steps" gorm:"default:0"`
	
	// Size and compression
	OriginalSize   *int64  `json:"original_size,omitempty"`   // Original data size in bytes
	CompressedSize *int64  `json:"compressed_size,omitempty"` // Compressed backup size in bytes
	CompressionRatio *float64 `json:"compression_ratio,omitempty"` // Compression ratio (0-1)
	
	// Error handling
	ErrorMessage    *string `json:"error_message,omitempty" gorm:"type:text"`
	ErrorCode       *string `json:"error_code,omitempty" gorm:"type:varchar(50)"`
	RetryCount      int     `json:"retry_count" gorm:"default:0"`
	MaxRetries      int     `json:"max_retries" gorm:"default:3"`
	
	// Metadata
	Description *string                `json:"description,omitempty" gorm:"type:text"`
	Tags        map[string]interface{} `json:"tags" gorm:"type:json"`
	
	// Relationships
	UserID               uint                `json:"user_id" gorm:"not null;index"`
	User                 User                `json:"user,omitempty" gorm:"foreignKey:UserID"`
	DatabaseConnectionID uint                `json:"database_connection_id" gorm:"not null;index"`
	DatabaseConnection   DatabaseConnection  `json:"database_connection,omitempty" gorm:"foreignKey:DatabaseConnectionID"`
	DatabaseTableID      *uint               `json:"database_table_id,omitempty" gorm:"index"`
	DatabaseTable        *DatabaseTable      `json:"database_table,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	BackupFiles []BackupFile `json:"backup_files,omitempty" gorm:"foreignKey:BackupJobID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the BackupJob model
func (BackupJob) TableName() string {
	return "backup_jobs"
}

// BeforeCreate hook to generate UID before creating backup job
func (bj *BackupJob) BeforeCreate(tx *gorm.DB) error {
	if bj.UID == "" {
		bj.UID = generateUID()
	}
	return nil
}

// IsRunning checks if the backup job is currently running
func (bj *BackupJob) IsRunning() bool {
	return bj.Status == BackupStatusRunning
}

// IsCompleted checks if the backup job has completed successfully
func (bj *BackupJob) IsCompleted() bool {
	return bj.Status == BackupStatusCompleted
}

// IsFailed checks if the backup job has failed
func (bj *BackupJob) IsFailed() bool {
	return bj.Status == BackupStatusFailed
}

// CanRetry checks if the backup job can be retried
func (bj *BackupJob) CanRetry() bool {
	return bj.IsFailed() && bj.RetryCount < bj.MaxRetries
}

// CanCancel checks if the backup job can be cancelled
func (bj *BackupJob) CanCancel() bool {
	return bj.Status == BackupStatusPending || bj.Status == BackupStatusRunning
}

// GetDuration returns the duration of the backup job
func (bj *BackupJob) GetDuration() time.Duration {
	if bj.Duration != nil {
		return time.Duration(*bj.Duration) * time.Second
	}
	
	if bj.StartedAt != nil && bj.CompletedAt != nil {
		return bj.CompletedAt.Sub(*bj.StartedAt)
	}
	
	if bj.StartedAt != nil && bj.IsRunning() {
		return time.Since(*bj.StartedAt)
	}
	
	return 0
}

// GetFormattedDuration returns a human-readable duration string
func (bj *BackupJob) GetFormattedDuration() string {
	duration := bj.GetDuration()
	if duration == 0 {
		return "N/A"
	}
	
	if duration < time.Minute {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.1fm", duration.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
}

// GetFormattedSize returns a human-readable size string
func (bj *BackupJob) GetFormattedSize() string {
	size := bj.CompressedSize
	if size == nil {
		size = bj.OriginalSize
	}
	
	if size == nil {
		return "Unknown"
	}
	
	return formatBytes(*size)
}

// GetCompressionRatio calculates the compression ratio
func (bj *BackupJob) GetCompressionRatio() float64 {
	if bj.CompressionRatio != nil {
		return *bj.CompressionRatio
	}
	
	if bj.OriginalSize != nil && bj.CompressedSize != nil && *bj.OriginalSize > 0 {
		ratio := 1.0 - (float64(*bj.CompressedSize) / float64(*bj.OriginalSize))
		bj.CompressionRatio = &ratio
		return ratio
	}
	
	return 0.0
}

// Start marks the backup job as started
func (bj *BackupJob) Start() {
	now := time.Now()
	bj.Status = BackupStatusRunning
	bj.StartedAt = &now
	bj.Progress = 0
	bj.CurrentStep = "Starting backup"
}

// Complete marks the backup job as completed
func (bj *BackupJob) Complete() {
	now := time.Now()
	bj.Status = BackupStatusCompleted
	bj.CompletedAt = &now
	bj.Progress = 100
	bj.CompletedSteps = bj.TotalSteps
	bj.CurrentStep = "Backup completed"
	
	if bj.StartedAt != nil {
		duration := int64(now.Sub(*bj.StartedAt).Seconds())
		bj.Duration = &duration
	}
}

// Fail marks the backup job as failed
func (bj *BackupJob) Fail(errorMsg, errorCode string) {
	now := time.Now()
	bj.Status = BackupStatusFailed
	bj.CompletedAt = &now
	bj.ErrorMessage = &errorMsg
	bj.ErrorCode = &errorCode
	bj.CurrentStep = "Backup failed"
	
	if bj.StartedAt != nil {
		duration := int64(now.Sub(*bj.StartedAt).Seconds())
		bj.Duration = &duration
	}
}

// Cancel marks the backup job as cancelled
func (bj *BackupJob) Cancel() {
	now := time.Now()
	bj.Status = BackupStatusCancelled
	bj.CompletedAt = &now
	bj.CurrentStep = "Backup cancelled"
	
	if bj.StartedAt != nil {
		duration := int64(now.Sub(*bj.StartedAt).Seconds())
		bj.Duration = &duration
	}
}

// UpdateProgress updates the progress of the backup job
func (bj *BackupJob) UpdateProgress(progress float64, currentStep string) {
	bj.Progress = progress
	bj.CurrentStep = currentStep
	
	// Update completed steps based on progress
	if bj.TotalSteps > 0 {
		bj.CompletedSteps = int(float64(bj.TotalSteps) * (progress / 100.0))
	}
}

// IncrementStep increments the completed steps and updates progress
func (bj *BackupJob) IncrementStep(stepName string) {
	bj.CompletedSteps++
	bj.CurrentStep = stepName
	
	if bj.TotalSteps > 0 {
		bj.Progress = (float64(bj.CompletedSteps) / float64(bj.TotalSteps)) * 100.0
	}
}

// SetSizeInfo sets the size information for the backup
func (bj *BackupJob) SetSizeInfo(originalSize, compressedSize int64) {
	bj.OriginalSize = &originalSize
	bj.CompressedSize = &compressedSize
}

// IsScheduledForNow checks if the backup job is scheduled to run now
func (bj *BackupJob) IsScheduledForNow() bool {
	if !bj.IsScheduled || bj.NextRunAt == nil {
		return false
	}
	return bj.NextRunAt.Before(time.Now()) || bj.NextRunAt.Equal(time.Now())
}

// GetBackupFileCount returns the number of backup files
func (bj *BackupJob) GetBackupFileCount() int {
	return len(bj.BackupFiles)
}

// GetTotalBackupSize returns the total size of all backup files
func (bj *BackupJob) GetTotalBackupSize() int64 {
	var total int64
	for _, file := range bj.BackupFiles {
		if file.Size != nil {
			total += *file.Size
		}
	}
	return total
}

// HasBackupFiles checks if the backup job has any backup files
func (bj *BackupJob) HasBackupFiles() bool {
	return len(bj.BackupFiles) > 0
}

// GetStatusColor returns a color code for the backup status
func (bj *BackupJob) GetStatusColor() string {
	switch bj.Status {
	case BackupStatusCompleted:
		return "#10B981" // Green
	case BackupStatusRunning:
		return "#3B82F6" // Blue
	case BackupStatusFailed:
		return "#EF4444" // Red
	case BackupStatusCancelled:
		return "#6B7280" // Gray
	case BackupStatusPartial:
		return "#F59E0B" // Yellow
	default: // Pending
		return "#8B5CF6" // Purple
	}
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else if bytes < 1024*1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	} else {
		return fmt.Sprintf("%.1f TB", float64(bytes)/(1024*1024*1024*1024))
	}
}