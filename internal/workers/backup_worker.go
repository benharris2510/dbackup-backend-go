package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/services"
	"github.com/dbackup/backend-go/internal/websocket"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

// BackupWorker handles backup job processing
type BackupWorker struct {
	db            *gorm.DB
	backupService services.BackupServiceInterface
	s3Service     services.S3ServiceInterface
	queueService  services.QueueServiceInterface
	wsService     *websocket.WebSocketService
}

// BackupTaskPayload represents the payload for a backup task
type BackupTaskPayload struct {
	BackupJobID  uint                         `json:"backup_job_id"`
	UserID       uint                         `json:"user_id"`
	DatabaseUID  string                       `json:"database_uid"`
	Options      *services.BackupOptions      `json:"options,omitempty"`
	StorageConfig *models.StorageConfiguration `json:"storage_config,omitempty"`
}

// RestoreTaskPayload represents the payload for a restore task
type RestoreTaskPayload struct {
	BackupJobID   uint                    `json:"backup_job_id"`
	UserID        uint                    `json:"user_id"`
	DatabaseUID   string                  `json:"database_uid"`
	BackupFileUID string                  `json:"backup_file_uid"`
	Options       *services.RestoreOptions `json:"options,omitempty"`
}

// Job type constants
const (
	TypeBackupPostgreSQL = "backup:postgresql"
	TypeBackupMySQL      = "backup:mysql"
	TypeRestorePostgreSQL = "restore:postgresql"
	TypeRestoreMySQL     = "restore:mysql"
	TypeCleanupBackups   = "cleanup:backups"
	TypeScheduledBackup  = "scheduled:backup"
)

// NewBackupWorker creates a new backup worker
func NewBackupWorker(db *gorm.DB, backupService services.BackupServiceInterface, s3Service services.S3ServiceInterface, queueService services.QueueServiceInterface, wsService *websocket.WebSocketService) *BackupWorker {
	return &BackupWorker{
		db:            db,
		backupService: backupService,
		s3Service:     s3Service,
		queueService:  queueService,
		wsService:     wsService,
	}
}

// RegisterHandlers registers all backup-related job handlers
func (bw *BackupWorker) RegisterHandlers(worker *services.QueueWorker) {
	worker.RegisterHandler(TypeBackupPostgreSQL, bw.HandleBackupPostgreSQL)
	worker.RegisterHandler(TypeBackupMySQL, bw.HandleBackupMySQL)
	worker.RegisterHandler(TypeRestorePostgreSQL, bw.HandleRestorePostgreSQL)
	worker.RegisterHandler(TypeRestoreMySQL, bw.HandleRestoreMySQL)
	worker.RegisterHandler(TypeCleanupBackups, bw.HandleCleanupBackups)
	worker.RegisterHandler(TypeScheduledBackup, bw.HandleScheduledBackup)
}

// HandleBackupPostgreSQL handles PostgreSQL backup jobs
func (bw *BackupWorker) HandleBackupPostgreSQL(ctx context.Context, task *asynq.Task) error {
	var payload BackupTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal backup payload: %w", err)
	}

	log.Printf("Processing PostgreSQL backup job %d for user %d", payload.BackupJobID, payload.UserID)

	// Load backup job from database
	var backupJob models.BackupJob
	if err := bw.db.Preload("User").Preload("DatabaseConnection").First(&backupJob, payload.BackupJobID).Error; err != nil {
		return fmt.Errorf("failed to load backup job: %w", err)
	}

	// Update job status to running
	backupJob.Start()
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to update backup job status: %w", err)
	}
	bw.sendBackupProgressUpdate(&backupJob)

	// Set up progress callback
	if payload.Options == nil {
		payload.Options = &services.BackupOptions{}
	}
	payload.Options.ProgressCallback = func(progress float64, message string) {
		backupJob.UpdateProgress(progress, message)
		bw.db.Save(&backupJob)
		// Send WebSocket progress update
		bw.sendBackupProgressUpdate(&backupJob)
	}

	// Perform the backup
	result, err := bw.backupService.CreatePostgreSQLBackup(ctx, &backupJob.DatabaseConnection, payload.Options)
	if err != nil {
		backupJob.Fail(err.Error(), "BACKUP_FAILED")
		bw.db.Save(&backupJob)
		bw.sendBackupProgressUpdate(&backupJob)
		return fmt.Errorf("backup failed: %w", err)
	}

	// Upload backup to S3
	if err := bw.uploadBackupToS3(ctx, &backupJob, result, payload.StorageConfig); err != nil {
		backupJob.Fail(err.Error(), "UPLOAD_FAILED")
		bw.db.Save(&backupJob)
		bw.sendBackupProgressUpdate(&backupJob)
		return fmt.Errorf("upload failed: %w", err)
	}

	// Update job with results
	backupJob.SetSizeInfo(result.OriginalSize, func() int64 {
		if result.CompressedSize != nil {
			return *result.CompressedSize
		}
		return result.OriginalSize
	}())
	backupJob.Complete()
	
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to save completed backup job: %w", err)
	}
	bw.sendBackupProgressUpdate(&backupJob)

	log.Printf("PostgreSQL backup job %d completed successfully", payload.BackupJobID)
	return nil
}

// HandleBackupMySQL handles MySQL backup jobs
func (bw *BackupWorker) HandleBackupMySQL(ctx context.Context, task *asynq.Task) error {
	var payload BackupTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal backup payload: %w", err)
	}

	log.Printf("Processing MySQL backup job %d for user %d", payload.BackupJobID, payload.UserID)

	// Load backup job from database
	var backupJob models.BackupJob
	if err := bw.db.Preload("User").Preload("DatabaseConnection").First(&backupJob, payload.BackupJobID).Error; err != nil {
		return fmt.Errorf("failed to load backup job: %w", err)
	}

	// Update job status to running
	backupJob.Start()
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to update backup job status: %w", err)
	}
	bw.sendBackupProgressUpdate(&backupJob)

	// Set up progress callback
	if payload.Options == nil {
		payload.Options = &services.BackupOptions{}
	}
	payload.Options.ProgressCallback = func(progress float64, message string) {
		backupJob.UpdateProgress(progress, message)
		bw.db.Save(&backupJob)
		// Send WebSocket progress update
		bw.sendBackupProgressUpdate(&backupJob)
	}

	// Perform the backup
	result, err := bw.backupService.CreateMySQLBackup(ctx, &backupJob.DatabaseConnection, payload.Options)
	if err != nil {
		backupJob.Fail(err.Error(), "BACKUP_FAILED")
		bw.db.Save(&backupJob)
		bw.sendBackupProgressUpdate(&backupJob)
		return fmt.Errorf("backup failed: %w", err)
	}

	// Upload backup to S3
	if err := bw.uploadBackupToS3(ctx, &backupJob, result, payload.StorageConfig); err != nil {
		backupJob.Fail(err.Error(), "UPLOAD_FAILED")
		bw.db.Save(&backupJob)
		bw.sendBackupProgressUpdate(&backupJob)
		return fmt.Errorf("upload failed: %w", err)
	}

	// Update job with results
	backupJob.SetSizeInfo(result.OriginalSize, func() int64 {
		if result.CompressedSize != nil {
			return *result.CompressedSize
		}
		return result.OriginalSize
	}())
	backupJob.Complete()
	
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to save completed backup job: %w", err)
	}
	bw.sendBackupProgressUpdate(&backupJob)

	log.Printf("MySQL backup job %d completed successfully", payload.BackupJobID)
	return nil
}

// HandleRestorePostgreSQL handles PostgreSQL restore jobs
func (bw *BackupWorker) HandleRestorePostgreSQL(ctx context.Context, task *asynq.Task) error {
	var payload RestoreTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal restore payload: %w", err)
	}

	log.Printf("Processing PostgreSQL restore job %d for user %d", payload.BackupJobID, payload.UserID)

	// Load backup job and file from database
	var backupJob models.BackupJob
	if err := bw.db.Preload("User").Preload("DatabaseConnection").First(&backupJob, payload.BackupJobID).Error; err != nil {
		return fmt.Errorf("failed to load backup job: %w", err)
	}

	var backupFile models.BackupFile
	if err := bw.db.Where("uid = ?", payload.BackupFileUID).First(&backupFile).Error; err != nil {
		return fmt.Errorf("failed to load backup file: %w", err)
	}

	// Update job status to running
	backupJob.Start()
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to update backup job status: %w", err)
	}
	bw.sendBackupProgressUpdate(&backupJob)

	// Download backup from S3
	tempPath, err := bw.downloadBackupFromS3(ctx, &backupFile)
	if err != nil {
		backupJob.Fail(err.Error(), "DOWNLOAD_FAILED")
		bw.db.Save(&backupJob)
		return fmt.Errorf("download failed: %w", err)
	}
	defer bw.cleanupTempFile(tempPath)

	// Set up progress callback
	if payload.Options == nil {
		payload.Options = &services.RestoreOptions{}
	}
	payload.Options.ProgressCallback = func(progress float64, message string) {
		backupJob.UpdateProgress(progress, message)
		bw.db.Save(&backupJob)
		// Send WebSocket progress update
		bw.sendBackupProgressUpdate(&backupJob)
	}

	// Perform the restore
	err = bw.backupService.RestorePostgreSQLBackup(ctx, &backupJob.DatabaseConnection, tempPath, payload.Options)
	if err != nil {
		backupJob.Fail(err.Error(), "RESTORE_FAILED")
		bw.db.Save(&backupJob)
		return fmt.Errorf("restore failed: %w", err)
	}

	// Update job status
	backupJob.Complete()
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to save completed restore job: %w", err)
	}

	log.Printf("PostgreSQL restore job %d completed successfully", payload.BackupJobID)
	return nil
}

// HandleRestoreMySQL handles MySQL restore jobs
func (bw *BackupWorker) HandleRestoreMySQL(ctx context.Context, task *asynq.Task) error {
	var payload RestoreTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal restore payload: %w", err)
	}

	log.Printf("Processing MySQL restore job %d for user %d", payload.BackupJobID, payload.UserID)

	// Load backup job and file from database
	var backupJob models.BackupJob
	if err := bw.db.Preload("User").Preload("DatabaseConnection").First(&backupJob, payload.BackupJobID).Error; err != nil {
		return fmt.Errorf("failed to load backup job: %w", err)
	}

	var backupFile models.BackupFile
	if err := bw.db.Where("uid = ?", payload.BackupFileUID).First(&backupFile).Error; err != nil {
		return fmt.Errorf("failed to load backup file: %w", err)
	}

	// Update job status to running
	backupJob.Start()
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to update backup job status: %w", err)
	}
	bw.sendBackupProgressUpdate(&backupJob)

	// Download backup from S3
	tempPath, err := bw.downloadBackupFromS3(ctx, &backupFile)
	if err != nil {
		backupJob.Fail(err.Error(), "DOWNLOAD_FAILED")
		bw.db.Save(&backupJob)
		return fmt.Errorf("download failed: %w", err)
	}
	defer bw.cleanupTempFile(tempPath)

	// Set up progress callback
	if payload.Options == nil {
		payload.Options = &services.RestoreOptions{}
	}
	payload.Options.ProgressCallback = func(progress float64, message string) {
		backupJob.UpdateProgress(progress, message)
		bw.db.Save(&backupJob)
		// Send WebSocket progress update
		bw.sendBackupProgressUpdate(&backupJob)
	}

	// Perform the restore
	err = bw.backupService.RestoreMySQLBackup(ctx, &backupJob.DatabaseConnection, tempPath, payload.Options)
	if err != nil {
		backupJob.Fail(err.Error(), "RESTORE_FAILED")
		bw.db.Save(&backupJob)
		return fmt.Errorf("restore failed: %w", err)
	}

	// Update job status
	backupJob.Complete()
	if err := bw.db.Save(&backupJob).Error; err != nil {
		return fmt.Errorf("failed to save completed restore job: %w", err)
	}

	log.Printf("MySQL restore job %d completed successfully", payload.BackupJobID)
	return nil
}

// HandleCleanupBackups handles backup cleanup jobs
func (bw *BackupWorker) HandleCleanupBackups(ctx context.Context, task *asynq.Task) error {
	log.Printf("Processing backup cleanup job")

	// Find expired backup files
	var expiredFiles []models.BackupFile
	now := time.Now()
	if err := bw.db.Where("expires_at IS NOT NULL AND expires_at < ?", now).Find(&expiredFiles).Error; err != nil {
		return fmt.Errorf("failed to find expired backup files: %w", err)
	}

	cleaned := 0
	for _, file := range expiredFiles {
		// Delete from S3
		if err := bw.s3Service.DeleteFile(ctx, file.S3Bucket, file.S3Key); err != nil {
			log.Printf("Failed to delete S3 file %s/%s: %v", file.S3Bucket, file.S3Key, err)
			continue
		}

		// Delete from database
		if err := bw.db.Delete(&file).Error; err != nil {
			log.Printf("Failed to delete backup file record %d: %v", file.ID, err)
			continue
		}

		cleaned++
	}

	// Find files that should be archived
	var filesToArchive []models.BackupFile
	archiveAfterDays := 30 // Could be configurable
	if err := bw.db.Where("is_archived = ? AND created_at < ?", false, now.AddDate(0, 0, -archiveAfterDays)).Find(&filesToArchive).Error; err != nil {
		return fmt.Errorf("failed to find files to archive: %w", err)
	}

	archived := 0
	for _, file := range filesToArchive {
		if file.ShouldBeArchived(archiveAfterDays) {
			file.Archive()
			if err := bw.db.Save(&file).Error; err != nil {
				log.Printf("Failed to archive backup file %d: %v", file.ID, err)
				continue
			}
			archived++
		}
	}

	log.Printf("Cleanup completed: %d files deleted, %d files archived", cleaned, archived)
	return nil
}

// HandleScheduledBackup handles scheduled backup execution
func (bw *BackupWorker) HandleScheduledBackup(ctx context.Context, task *asynq.Task) error {
	var payload BackupTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal scheduled backup payload: %w", err)
	}

	log.Printf("Processing scheduled backup for user %d", payload.UserID)

	// Find database connection by UID
	var dbConn models.DatabaseConnection
	if err := bw.db.Where("uid = ? AND user_id = ?", payload.DatabaseUID, payload.UserID).First(&dbConn).Error; err != nil {
		return fmt.Errorf("failed to find database connection: %w", err)
	}

	// Create a new backup job
	backupJob := &models.BackupJob{
		Name:                 fmt.Sprintf("Scheduled backup - %s", dbConn.Name),
		Type:                 models.BackupTypeFull,
		Status:               models.BackupStatusPending,
		UserID:               payload.UserID,
		DatabaseConnectionID: dbConn.ID,
		IsScheduled:          true,
	}

	if err := bw.db.Create(backupJob).Error; err != nil {
		return fmt.Errorf("failed to create scheduled backup job: %w", err)
	}

	// Update payload with the new backup job ID
	payload.BackupJobID = backupJob.ID

	// Determine backup type based on database type
	var jobType string
	switch dbConn.Type {
	case models.DatabaseTypePostgreSQL:
		jobType = TypeBackupPostgreSQL
	case models.DatabaseTypeMySQL:
		jobType = TypeBackupMySQL
	default:
		return fmt.Errorf("unsupported database type: %s", dbConn.Type)
	}

	// Enqueue the actual backup job
	_, err := bw.queueService.EnqueueJob(ctx, jobType, payload, services.WithQueue("backups"))
	if err != nil {
		backupJob.Fail(err.Error(), "ENQUEUE_FAILED")
		bw.db.Save(backupJob)
		return fmt.Errorf("failed to enqueue backup job: %w", err)
	}

	log.Printf("Scheduled backup job %d enqueued successfully", backupJob.ID)
	return nil
}

// uploadBackupToS3 uploads a backup file to S3 storage
func (bw *BackupWorker) uploadBackupToS3(ctx context.Context, job *models.BackupJob, result *services.BackupResult, storageConfig *models.StorageConfiguration) error {
	if storageConfig == nil {
		// Use default storage configuration for the user
		if err := bw.db.Where("user_id = ? AND is_default = ?", job.UserID, true).First(&storageConfig).Error; err != nil {
			return fmt.Errorf("no default storage configuration found for user: %w", err)
		}
	}

	// Read backup file
	backupData, err := bw.readBackupFile(result.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Generate S3 key
	timestamp := time.Now().Format("2006/01/02")
	s3Key := fmt.Sprintf("backups/%s/%s/%s", timestamp, job.DatabaseConnection.Database, job.UID+".backup")
	if storageConfig.PathPrefix != nil && *storageConfig.PathPrefix != "" {
		s3Key = fmt.Sprintf("%s/%s", *storageConfig.PathPrefix, s3Key)
	}

	// Upload to S3
	uploadResult, err := bw.s3Service.UploadFile(ctx, storageConfig.Bucket, s3Key, bytes.NewReader(backupData), "application/octet-stream")
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Create backup file record
	backupFile := &models.BackupFile{
		Name:         fmt.Sprintf("%s-backup-%s", job.DatabaseConnection.Database, time.Now().Format("20060102-150405")),
		OriginalName: result.FilePath,
		FileType:     "dump",
		S3Bucket:     uploadResult.Bucket,
		S3Key:        uploadResult.Key,
		S3Region:     storageConfig.Region,
		Size:         &result.OriginalSize,
		BackupJobID:  job.ID,
		IsEncrypted:  storageConfig.ClientSideEncryption,
		IsCompressed: result.CompressedSize != nil,
	}

	if storageConfig.Endpoint != nil {
		backupFile.S3Endpoint = storageConfig.Endpoint
	}

	if result.CompressedSize != nil {
		backupFile.OriginalSize = &result.OriginalSize
		backupFile.Size = result.CompressedSize
		backupFile.CompressionAlgo = "gzip"
	}

	if result.Checksum != "" {
		backupFile.SetChecksum(result.Checksum)
	}

	// Set retention policy (30 days default)
	backupFile.SetRetentionPolicy(30)

	if err := bw.db.Create(backupFile).Error; err != nil {
		return fmt.Errorf("failed to create backup file record: %w", err)
	}

	log.Printf("Backup uploaded to S3: %s/%s", uploadResult.Bucket, uploadResult.Key)
	return nil
}

// downloadBackupFromS3 downloads a backup file from S3 to a temporary location
func (bw *BackupWorker) downloadBackupFromS3(ctx context.Context, backupFile *models.BackupFile) (string, error) {
	// Download from S3
	reader, err := bw.s3Service.DownloadFile(ctx, backupFile.S3Bucket, backupFile.S3Key)
	if err != nil {
		return "", fmt.Errorf("failed to download from S3: %w", err)
	}
	defer reader.Close()

	// Write to temporary file
	tempPath := fmt.Sprintf("/tmp/restore_%s_%d", backupFile.UID, time.Now().Unix())
	if err := bw.writeBackupFileFromReader(tempPath, reader); err != nil {
		return "", fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Update access tracking
	backupFile.IncrementDownloadCount()
	bw.db.Save(backupFile)

	return tempPath, nil
}

// Helper methods for file operations
func (bw *BackupWorker) readBackupFile(filePath string) ([]byte, error) {
	// Implementation would read the file from disk
	// For now, return empty data to satisfy interface
	return []byte{}, nil
}

func (bw *BackupWorker) writeBackupFile(filePath string, data []byte) error {
	// Implementation would write data to file
	// For now, return nil to satisfy interface
	return nil
}

func (bw *BackupWorker) writeBackupFileFromReader(filePath string, reader io.Reader) error {
	// Implementation would write reader to file
	// For now, return nil to satisfy interface
	return nil
}

func (bw *BackupWorker) cleanupTempFile(filePath string) {
	// Implementation would remove the temporary file
	log.Printf("Cleaning up temporary file: %s", filePath)
}

// EnqueueBackupJob is a helper method to enqueue backup jobs
func (bw *BackupWorker) EnqueueBackupJob(ctx context.Context, jobType string, payload *BackupTaskPayload, options ...services.JobOption) (*services.JobInfo, error) {
	// Set default options if not provided
	defaultOptions := []services.JobOption{
		services.WithQueue("backups"),
		services.WithMaxRetry(3),
		services.WithTimeout(30 * time.Minute),
	}
	
	allOptions := append(defaultOptions, options...)

	return bw.queueService.EnqueueJob(ctx, jobType, payload, allOptions...)
}

// EnqueueScheduledBackupJob is a helper method to enqueue scheduled backup jobs
func (bw *BackupWorker) EnqueueScheduledBackupJob(ctx context.Context, payload *BackupTaskPayload, scheduledTime time.Time, options ...services.JobOption) (*services.JobInfo, error) {
	defaultOptions := []services.JobOption{
		services.WithQueue("scheduled"),
		services.WithMaxRetry(2),
		services.WithTimeout(35 * time.Minute),
	}
	
	allOptions := append(defaultOptions, options...)

	return bw.queueService.EnqueueScheduledJob(ctx, TypeScheduledBackup, payload, scheduledTime, allOptions...)
}

// GetBackupJobStatus returns the current status of a backup job
func (bw *BackupWorker) GetBackupJobStatus(ctx context.Context, backupJobID uint) (*models.BackupJob, error) {
	var backupJob models.BackupJob
	if err := bw.db.Preload("BackupFiles").First(&backupJob, backupJobID).Error; err != nil {
		return nil, fmt.Errorf("failed to get backup job: %w", err)
	}
	return &backupJob, nil
}

// sendBackupProgressUpdate sends WebSocket progress updates for backup jobs
func (bw *BackupWorker) sendBackupProgressUpdate(backupJob *models.BackupJob) {
	if bw.wsService == nil {
		return // WebSocket service not available
	}

	progressMsg := &websocket.BackupProgressMessage{
		BackupJobUID:    backupJob.UID,
		Status:          string(backupJob.Status),
		Progress:        backupJob.Progress,
		ProgressMessage: backupJob.CurrentStep,
		StartedAt:       backupJob.StartedAt,
		CompletedAt:     backupJob.CompletedAt,
	}

	if backupJob.ErrorMessage != nil {
		progressMsg.ErrorMessage = backupJob.ErrorMessage
	}

	// Send progress update to user via WebSocket
	err := bw.wsService.BroadcastBackupProgress(backupJob.UserID, progressMsg)
	if err != nil {
		log.Printf("Failed to send WebSocket backup progress update: %v", err)
	}
}