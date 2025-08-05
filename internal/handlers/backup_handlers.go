package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/services"
	"github.com/dbackup/backend-go/internal/workers"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// BackupWorkerInterface defines the interface for backup worker operations
type BackupWorkerInterface interface {
	EnqueueBackupJob(ctx context.Context, jobType string, payload *workers.BackupTaskPayload, options ...services.JobOption) (*services.JobInfo, error)
	EnqueueScheduledBackupJob(ctx context.Context, payload *workers.BackupTaskPayload, scheduledTime time.Time, options ...services.JobOption) (*services.JobInfo, error)
}

// BackupHandler handles backup-related HTTP requests
type BackupHandler struct {
	db            *gorm.DB
	backupService services.BackupServiceInterface
	s3Service     services.S3ServiceInterface
	queueService  services.QueueServiceInterface
	backupWorker  BackupWorkerInterface
}

// NewBackupHandler creates a new backup handler
func NewBackupHandler(db *gorm.DB, backupService services.BackupServiceInterface, s3Service services.S3ServiceInterface, queueService services.QueueServiceInterface, backupWorker BackupWorkerInterface) *BackupHandler {
	return &BackupHandler{
		db:            db,
		backupService: backupService,
		s3Service:     s3Service,
		queueService:  queueService,
		backupWorker:  backupWorker,
	}
}

// BackupResponse represents a backup job response
type BackupResponse struct {
	ID                   uint                        `json:"id"`
	UID                  string                      `json:"uid"`
	Name                 string                      `json:"name"`
	Type                 models.BackupType           `json:"type"`
	Status               models.BackupStatus         `json:"status"`
	Progress             float64                     `json:"progress"`
	ProgressMessage      string                      `json:"progress_message,omitempty"`
	StartedAt            *time.Time                  `json:"started_at,omitempty"`
	CompletedAt          *time.Time                  `json:"completed_at,omitempty"`
	Duration             *int64                      `json:"duration,omitempty"` // in seconds
	OriginalSize         *int64                      `json:"original_size,omitempty"`
	CompressedSize       *int64                      `json:"compressed_size,omitempty"`
	ErrorMessage         string                      `json:"error_message,omitempty"`
	ErrorCode            string                      `json:"error_code,omitempty"`
	IsScheduled          bool                        `json:"is_scheduled"`
	DatabaseConnection   *DatabaseConnectionResponse `json:"database_connection,omitempty"`
	BackupFiles          []BackupFileResponse        `json:"backup_files,omitempty"`
	CreatedAt            time.Time                   `json:"created_at"`
	UpdatedAt            time.Time                   `json:"updated_at"`
}

// BackupFileResponse represents a backup file response
type BackupFileResponse struct {
	ID               uint       `json:"id"`
	UID              string     `json:"uid"`
	Name             string     `json:"name"`
	OriginalName     string     `json:"original_name"`
	FileType         string     `json:"file_type"`
	Size             *int64     `json:"size,omitempty"`
	OriginalSize     *int64     `json:"original_size,omitempty"`
	CompressionAlgo  string     `json:"compression_algo,omitempty"`
	IsCompressed     bool       `json:"is_compressed"`
	IsEncrypted      bool       `json:"is_encrypted"`
	IsArchived       bool       `json:"is_archived"`
	S3Bucket         string     `json:"s3_bucket,omitempty"`
	S3Key            string     `json:"s3_key,omitempty"`
	S3Region         string     `json:"s3_region,omitempty"`
	S3Endpoint       *string    `json:"s3_endpoint,omitempty"`
	DownloadCount    int        `json:"download_count"`
	LastDownloadedAt *time.Time `json:"last_downloaded_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// DatabaseConnectionResponse represents a database connection response (simplified)
type DatabaseConnectionResponse struct {
	ID       uint   `json:"id"`
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
}

// BackupListQuery represents query parameters for listing backups
type BackupListQuery struct {
	Page             int                   `query:"page"`
	Limit            int                   `query:"limit"`
	Status           models.BackupStatus   `query:"status"`
	Type             models.BackupType     `query:"type"`
	DatabaseUID      string                `query:"database_uid"`
	IsScheduled      *bool                 `query:"is_scheduled"`
	StartDate        string                `query:"start_date"`   // YYYY-MM-DD format
	EndDate          string                `query:"end_date"`     // YYYY-MM-DD format
	Search           string                `query:"search"`       // Search in name, database name
	SortBy           string                `query:"sort_by"`      // created_at, updated_at, started_at, completed_at, name, status
	SortOrder        string                `query:"sort_order"`   // asc, desc
	IncludeFiles     bool                  `query:"include_files"`
	IncludeDatabase  bool                  `query:"include_database"`
}

// BackupListResponse represents the response for listing backups
type BackupListResponse struct {
	Backups    []BackupResponse `json:"backups"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
}

// CreateBackupRequest represents a request to create a backup
type CreateBackupRequest struct {
	Name                     string                  `json:"name" validate:"required,min=1,max=255"`
	DatabaseUID              string                  `json:"database_uid" validate:"required"`
	Type                     models.BackupType       `json:"type" validate:"required"`
	Options                  *services.BackupOptions `json:"options,omitempty"`
	StorageConfigurationUID  *string                 `json:"storage_configuration_uid,omitempty"`
	ScheduleAt               *time.Time              `json:"schedule_at,omitempty"`
}

// GetBackups handles GET /api/backups
func (h *BackupHandler) GetBackups(c echo.Context) error {
	user := c.Get("user").(*models.User)
	
	// Parse query parameters
	var query BackupListQuery
	if err := c.Bind(&query); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query parameters")
	}

	// Set defaults
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.SortBy == "" {
		query.SortBy = "created_at"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	// Build query
	db := h.db.Where("user_id = ?", user.ID)

	// Apply filters
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.IsScheduled != nil {
		db = db.Where("is_scheduled = ?", *query.IsScheduled)
	}
	if query.DatabaseUID != "" {
		db = db.Joins("JOIN database_connections dc ON backup_jobs.database_connection_id = dc.id").
			Where("dc.uid = ?", query.DatabaseUID)
	}

	// Date filters
	if query.StartDate != "" {
		if startDate, err := time.Parse("2006-01-02", query.StartDate); err == nil {
			db = db.Where("created_at >= ?", startDate)
		}
	}
	if query.EndDate != "" {
		if endDate, err := time.Parse("2006-01-02", query.EndDate); err == nil {
			// Add one day to include the entire end date
			endDate = endDate.AddDate(0, 0, 1)
			db = db.Where("created_at < ?", endDate)
		}
	}

	// Search filter
	if query.Search != "" {
		searchTerm := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(name) LIKE ? OR EXISTS (SELECT 1 FROM database_connections dc WHERE dc.id = backup_jobs.database_connection_id AND (LOWER(dc.name) LIKE ? OR LOWER(dc.database) LIKE ?))", 
			searchTerm, searchTerm, searchTerm)
	}

	// Count total
	var total int64
	if err := db.Model(&models.BackupJob{}).Count(&total).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to count backups")
	}

	// Apply sorting
	orderClause := query.SortBy
	if query.SortOrder == "desc" {
		orderClause += " DESC"
	} else {
		orderClause += " ASC"
	}
	db = db.Order(orderClause)

	// Apply pagination
	offset := (query.Page - 1) * query.Limit
	db = db.Offset(offset).Limit(query.Limit)

	// Include associations if requested
	if query.IncludeDatabase {
		db = db.Preload("DatabaseConnection")
	}
	if query.IncludeFiles {
		db = db.Preload("BackupFiles")
	}

	// Execute query
	var backupJobs []models.BackupJob
	if err := db.Find(&backupJobs).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch backups")
	}

	// Convert to response format
	backups := make([]BackupResponse, len(backupJobs))
	for i, job := range backupJobs {
		backups[i] = h.convertBackupJobToResponse(job)
	}

	// Calculate total pages
	totalPages := int((total + int64(query.Limit) - 1) / int64(query.Limit))

	response := BackupListResponse{
		Backups:    backups,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// GetBackup handles GET /api/backups/:uid
func (h *BackupHandler) GetBackup(c echo.Context) error {
	user := c.Get("user").(*models.User)
	backupUID := c.Param("uid")

	if backupUID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Backup UID is required")
	}

	var backupJob models.BackupJob
	if err := h.db.Preload("DatabaseConnection").Preload("BackupFiles").
		Where("uid = ? AND user_id = ?", backupUID, user.ID).
		First(&backupJob).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Backup not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch backup")
	}

	response := h.convertBackupJobToResponse(backupJob)
	return c.JSON(http.StatusOK, response)
}

// CreateBackup handles POST /api/backups
func (h *BackupHandler) CreateBackup(c echo.Context) error {
	user := c.Get("user").(*models.User)

	var req CreateBackupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Find database connection
	var dbConn models.DatabaseConnection
	if err := h.db.Where("uid = ? AND user_id = ?", req.DatabaseUID, user.ID).
		First(&dbConn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Database connection not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to find database connection")
	}

	// Find storage configuration if specified
	var storageConfig *models.StorageConfiguration
	if req.StorageConfigurationUID != nil {
		storageConfig = &models.StorageConfiguration{}
		if err := h.db.Where("uid = ? AND user_id = ?", *req.StorageConfigurationUID, user.ID).
			First(storageConfig).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Storage configuration not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to find storage configuration")
		}
	}

	// Create backup job
	backupJob := &models.BackupJob{
		Name:                 req.Name,
		Type:                 req.Type,
		Status:               models.BackupStatusPending,
		UserID:               user.ID,
		DatabaseConnectionID: dbConn.ID,
		IsScheduled:          req.ScheduleAt != nil,
	}

	if err := h.db.Create(backupJob).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create backup job")
	}

	// Prepare backup task payload
	payload := &workers.BackupTaskPayload{
		BackupJobID:   backupJob.ID,
		UserID:        user.ID,
		DatabaseUID:   req.DatabaseUID,
		Options:       req.Options,
		StorageConfig: storageConfig,
	}

	// Determine job type based on database type
	var jobType string
	switch dbConn.Type {
	case models.DatabaseTypePostgreSQL:
		jobType = workers.TypeBackupPostgreSQL
	case models.DatabaseTypeMySQL:
		jobType = workers.TypeBackupMySQL
	default:
		// Clean up the created backup job
		h.db.Delete(backupJob)
		return echo.NewHTTPError(http.StatusBadRequest, "Unsupported database type")
	}

	// Enqueue the backup job
	var err error

	if req.ScheduleAt != nil {
		// Schedule for later
		_, err = h.backupWorker.EnqueueScheduledBackupJob(c.Request().Context(), payload, *req.ScheduleAt)
	} else {
		// Execute immediately
		_, err = h.backupWorker.EnqueueBackupJob(c.Request().Context(), jobType, payload)
	}

	if err != nil {
		// Clean up the created backup job
		h.db.Delete(backupJob)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enqueue backup job: "+err.Error())
	}

	// Note: BackupJob model doesn't have QueueJobID field yet
	// This would need to be added to the model or stored separately
	h.db.Save(backupJob)

	// Return the created backup job
	response := h.convertBackupJobToResponse(*backupJob)
	return c.JSON(http.StatusCreated, response)
}

// CancelBackup handles DELETE /api/backups/:uid
func (h *BackupHandler) CancelBackup(c echo.Context) error {
	user := c.Get("user").(*models.User)
	backupUID := c.Param("uid")

	if backupUID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Backup UID is required")
	}

	var backupJob models.BackupJob
	if err := h.db.Where("uid = ? AND user_id = ?", backupUID, user.ID).
		First(&backupJob).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Backup not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch backup")
	}

	// Only allow cancellation of pending or running jobs
	if backupJob.Status != models.BackupStatusPending && backupJob.Status != models.BackupStatusRunning {
		return echo.NewHTTPError(http.StatusBadRequest, "Cannot cancel completed or failed backup")
	}

	// Cancel the queue job if it exists
	// Note: BackupJob model doesn't have QueueJobID field yet
	// This would need to be added to the model or stored separately

	// Update backup job status
	backupJob.Cancel()
	if err := h.db.Save(&backupJob).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to cancel backup")
	}

	response := h.convertBackupJobToResponse(backupJob)
	return c.JSON(http.StatusOK, response)
}

// RetryBackup handles POST /api/backups/:uid/retry
func (h *BackupHandler) RetryBackup(c echo.Context) error {
	user := c.Get("user").(*models.User)
	backupUID := c.Param("uid")

	if backupUID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Backup UID is required")
	}

	var backupJob models.BackupJob
	if err := h.db.Preload("DatabaseConnection").
		Where("uid = ? AND user_id = ?", backupUID, user.ID).
		First(&backupJob).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Backup not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch backup")
	}

	// Only allow retry of failed or cancelled jobs
	if backupJob.Status != models.BackupStatusFailed && backupJob.Status != models.BackupStatusCancelled {
		return echo.NewHTTPError(http.StatusBadRequest, "Can only retry failed or cancelled backups")
	}

	// Reset backup job status
	backupJob.Status = models.BackupStatusPending
	backupJob.Progress = 0
	backupJob.CurrentStep = ""
	backupJob.ErrorMessage = nil
	backupJob.ErrorCode = nil
	backupJob.StartedAt = nil
	backupJob.CompletedAt = nil

	if err := h.db.Save(&backupJob).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to reset backup job")
	}

	// Determine job type
	var jobType string
	switch backupJob.DatabaseConnection.Type {
	case models.DatabaseTypePostgreSQL:
		jobType = workers.TypeBackupPostgreSQL
	case models.DatabaseTypeMySQL:
		jobType = workers.TypeBackupMySQL
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "Unsupported database type")
	}

	// Re-enqueue the backup job
	payload := &workers.BackupTaskPayload{
		BackupJobID: backupJob.ID,
		UserID:      user.ID,
		DatabaseUID: backupJob.DatabaseConnection.UID,
	}

	_, err := h.backupWorker.EnqueueBackupJob(c.Request().Context(), jobType, payload)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to retry backup job: "+err.Error())
	}

	// Note: BackupJob model doesn't have QueueJobID field yet
	// This would need to be added to the model or stored separately
	h.db.Save(&backupJob)

	response := h.convertBackupJobToResponse(backupJob)
	return c.JSON(http.StatusOK, response)
}

// GetBackupProgress handles GET /api/backups/:uid/progress
func (h *BackupHandler) GetBackupProgress(c echo.Context) error {
	user := c.Get("user").(*models.User)
	backupUID := c.Param("uid")

	if backupUID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Backup UID is required")
	}

	var backupJob models.BackupJob
	if err := h.db.Where("uid = ? AND user_id = ?", backupUID, user.ID).
		First(&backupJob).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Backup not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch backup")
	}

	progressResponse := map[string]interface{}{
		"uid":              backupJob.UID,
		"status":           backupJob.Status,
		"progress":         backupJob.Progress,
		"progress_message": backupJob.CurrentStep,
		"started_at":       backupJob.StartedAt,
		"completed_at":     backupJob.CompletedAt,
		"error_message":    backupJob.ErrorMessage,
		"error_code":       backupJob.ErrorCode,
	}

	return c.JSON(http.StatusOK, progressResponse)
}

// convertBackupJobToResponse converts a BackupJob model to response format
func (h *BackupHandler) convertBackupJobToResponse(job models.BackupJob) BackupResponse {
	var errorMessage, errorCode string
	if job.ErrorMessage != nil {
		errorMessage = *job.ErrorMessage
	}
	if job.ErrorCode != nil {
		errorCode = *job.ErrorCode
	}

	response := BackupResponse{
		ID:              job.ID,
		UID:             job.UID,
		Name:            job.Name,
		Type:            job.Type,
		Status:          job.Status,
		Progress:        job.Progress,
		ProgressMessage: job.CurrentStep,
		StartedAt:       job.StartedAt,
		CompletedAt:     job.CompletedAt,
		OriginalSize:    job.OriginalSize,
		CompressedSize:  job.CompressedSize,
		ErrorMessage:    errorMessage,
		ErrorCode:       errorCode,
		IsScheduled:     job.IsScheduled,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}

	// Calculate duration if available
	if job.StartedAt != nil && job.CompletedAt != nil {
		duration := int64(job.CompletedAt.Sub(*job.StartedAt).Seconds())
		response.Duration = &duration
	}

	// Include database connection if loaded
	if job.DatabaseConnection.ID != 0 {
		response.DatabaseConnection = &DatabaseConnectionResponse{
			ID:       job.DatabaseConnection.ID,
			UID:      job.DatabaseConnection.UID,
			Name:     job.DatabaseConnection.Name,
			Type:     string(job.DatabaseConnection.Type),
			Host:     job.DatabaseConnection.Host,
			Port:     job.DatabaseConnection.Port,
			Database: job.DatabaseConnection.Database,
			Username: job.DatabaseConnection.Username,
		}
	}

	// Include backup files if loaded
	if len(job.BackupFiles) > 0 {
		response.BackupFiles = make([]BackupFileResponse, len(job.BackupFiles))
		for i, file := range job.BackupFiles {
			response.BackupFiles[i] = BackupFileResponse{
				ID:               file.ID,
				UID:              file.UID,
				Name:             file.Name,
				OriginalName:     file.OriginalName,
				FileType:         file.FileType,
				Size:             file.Size,
				OriginalSize:     file.OriginalSize,
				CompressionAlgo:  file.CompressionAlgo,
				IsCompressed:     file.IsCompressed,
				IsEncrypted:      file.IsEncrypted,
				IsArchived:       file.IsArchived,
				S3Bucket:         file.S3Bucket,
				S3Key:            file.S3Key,
				S3Region:         file.S3Region,
				S3Endpoint:       file.S3Endpoint,
				DownloadCount:    file.DownloadCount,
				LastDownloadedAt: nil, // field not available in model yet
				ExpiresAt:        file.ExpiresAt,
				CreatedAt:        file.CreatedAt,
				UpdatedAt:        file.UpdatedAt,
			}
		}
	}

	return response
}

// RegisterBackupRoutes registers backup-related routes
func (h *BackupHandler) RegisterRoutes(g *echo.Group) {
	backups := g.Group("/backups")
	
	backups.GET("", h.GetBackups)
	backups.POST("", h.CreateBackup)
	backups.GET("/:uid", h.GetBackup)
	backups.DELETE("/:uid", h.CancelBackup)
	backups.POST("/:uid/retry", h.RetryBackup)
	backups.GET("/:uid/progress", h.GetBackupProgress)
}