package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/services"
	"github.com/dbackup/backend-go/internal/workers"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// CustomValidator wraps the validator
type CustomValidator struct {
	validator *validator.Validate
}

// Validate validates structs
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// Mock services
type MockBackupService struct {
	mock.Mock
}

func (m *MockBackupService) CreatePostgreSQLBackup(ctx context.Context, conn *models.DatabaseConnection, options *services.BackupOptions) (*services.BackupResult, error) {
	args := m.Called(ctx, conn, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.BackupResult), args.Error(1)
}

func (m *MockBackupService) CreateMySQLBackup(ctx context.Context, conn *models.DatabaseConnection, options *services.BackupOptions) (*services.BackupResult, error) {
	args := m.Called(ctx, conn, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.BackupResult), args.Error(1)
}

func (m *MockBackupService) RestorePostgreSQLBackup(ctx context.Context, conn *models.DatabaseConnection, backupPath string, options *services.RestoreOptions) error {
	args := m.Called(ctx, conn, backupPath, options)
	return args.Error(0)
}

func (m *MockBackupService) RestoreMySQLBackup(ctx context.Context, conn *models.DatabaseConnection, backupPath string, options *services.RestoreOptions) error {
	args := m.Called(ctx, conn, backupPath, options)
	return args.Error(0)
}

func (m *MockBackupService) ValidateBackupTools() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockBackupService) GetBackupEstimate(ctx context.Context, conn *models.DatabaseConnection, options *services.BackupOptions) (*services.BackupEstimate, error) {
	args := m.Called(ctx, conn, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.BackupEstimate), args.Error(1)
}

func (m *MockBackupService) CompressBackup(ctx context.Context, inputPath, outputPath string, algorithm string) error {
	args := m.Called(ctx, inputPath, outputPath, algorithm)
	return args.Error(0)
}

func (m *MockBackupService) DecompressBackup(ctx context.Context, inputPath, outputPath string) error {
	args := m.Called(ctx, inputPath, outputPath)
	return args.Error(0)
}

type MockS3Service struct {
	mock.Mock
}

func (m *MockS3Service) UploadFile(ctx context.Context, bucket, key string, data io.Reader, contentType string) (*services.S3UploadResult, error) {
	args := m.Called(ctx, bucket, key, data, contentType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.S3UploadResult), args.Error(1)
}

func (m *MockS3Service) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockS3Service) DeleteFile(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

func (m *MockS3Service) GetFileInfo(ctx context.Context, bucket, key string) (*services.S3FileInfo, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.S3FileInfo), args.Error(1)
}

func (m *MockS3Service) FileExists(ctx context.Context, bucket, key string) (bool, error) {
	args := m.Called(ctx, bucket, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockS3Service) CreateBucket(ctx context.Context, bucket, region string) error {
	args := m.Called(ctx, bucket, region)
	return args.Error(0)
}

func (m *MockS3Service) BucketExists(ctx context.Context, bucket string) (bool, error) {
	args := m.Called(ctx, bucket)
	return args.Bool(0), args.Error(1)
}

func (m *MockS3Service) ListFiles(ctx context.Context, bucket, prefix string, maxKeys int) (*services.S3ListResult, error) {
	args := m.Called(ctx, bucket, prefix, maxKeys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.S3ListResult), args.Error(1)
}

func (m *MockS3Service) GeneratePresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	args := m.Called(ctx, bucket, key, expiry)
	return args.String(0), args.Error(1)
}

func (m *MockS3Service) GenerateUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	args := m.Called(ctx, bucket, key, expiry)
	return args.String(0), args.Error(1)
}

func (m *MockS3Service) TestConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockQueueService struct {
	mock.Mock
}

func (m *MockQueueService) EnqueueJob(ctx context.Context, jobType string, payload interface{}, opts ...services.JobOption) (*services.JobInfo, error) {
	args := m.Called(ctx, jobType, payload, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.JobInfo), args.Error(1)
}

func (m *MockQueueService) EnqueueScheduledJob(ctx context.Context, jobType string, payload interface{}, processAt time.Time, opts ...services.JobOption) (*services.JobInfo, error) {
	args := m.Called(ctx, jobType, payload, processAt, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.JobInfo), args.Error(1)
}

func (m *MockQueueService) GetJob(ctx context.Context, jobID string) (*services.JobInfo, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.JobInfo), args.Error(1)
}

func (m *MockQueueService) ListJobs(ctx context.Context, jobType string, state services.JobState) ([]*services.JobInfo, error) {
	args := m.Called(ctx, jobType, state)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*services.JobInfo), args.Error(1)
}

func (m *MockQueueService) CancelJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockQueueService) RetryJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockQueueService) GetQueueStats(ctx context.Context) (*services.QueueStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.QueueStats), args.Error(1)
}

func (m *MockQueueService) PauseQueue(ctx context.Context, queueName string) error {
	args := m.Called(ctx, queueName)
	return args.Error(0)
}

func (m *MockQueueService) UnpauseQueue(ctx context.Context, queueName string) error {
	args := m.Called(ctx, queueName)
	return args.Error(0)
}

func (m *MockQueueService) DeleteQueue(ctx context.Context, queueName string) error {
	args := m.Called(ctx, queueName)
	return args.Error(0)
}

func (m *MockQueueService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQueueService) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockBackupWorker struct {
	mock.Mock
}

func (m *MockBackupWorker) EnqueueBackupJob(ctx context.Context, jobType string, payload *workers.BackupTaskPayload, options ...services.JobOption) (*services.JobInfo, error) {
	args := m.Called(ctx, jobType, payload, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.JobInfo), args.Error(1)
}

func (m *MockBackupWorker) EnqueueScheduledBackupJob(ctx context.Context, payload *workers.BackupTaskPayload, scheduledTime time.Time, options ...services.JobOption) (*services.JobInfo, error) {
	args := m.Called(ctx, payload, scheduledTime, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.JobInfo), args.Error(1)
}


// Test helper functions
func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(
		&models.User{},
		&models.DatabaseConnection{},
		&models.BackupJob{},
		&models.BackupFile{},
		&models.StorageConfiguration{},
	)

	return db
}

func createTestUser(db *gorm.DB) *models.User {
	user := &models.User{
		Email:    "test@example.com",
		Password: "hashed_password",
		IsActive: true,
	}
	db.Create(user)
	return user
}

func createTestDatabaseConnection(db *gorm.DB, userID uint) *models.DatabaseConnection {
	dbConn := &models.DatabaseConnection{
		Name:     "Test Database",
		Type:     models.DatabaseTypePostgreSQL,
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		Password: "testpass",
		UserID:   userID,
	}
	db.Create(dbConn)
	return dbConn
}

func createTestBackupJob(db *gorm.DB, userID, dbConnID uint) *models.BackupJob {
	job := &models.BackupJob{
		Name:                 "Test Backup",
		Type:                 models.BackupTypeFull,
		Status:               models.BackupStatusCompleted,
		Progress:             100.0,
		UserID:               userID,
		DatabaseConnectionID: dbConnID,
		IsScheduled:          false,
		Tags:                 nil, // Set to nil to avoid SQLite map serialization issues
	}
	db.Create(job)
	return job
}

func TestBackupHandler_GetBackups_Success(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	createTestBackupJob(db, user.ID, dbConn.ID)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", user)

	// Execute
	err := handler.GetBackups(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response BackupListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, int64(1), response.Total)
	assert.Equal(t, 1, len(response.Backups))
	assert.Equal(t, "Test Backup", response.Backups[0].Name)
	assert.Equal(t, models.BackupStatusCompleted, response.Backups[0].Status)
}

func TestBackupHandler_GetBackups_WithFilters(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	
	// Create multiple backup jobs with different statuses
	job1 := createTestBackupJob(db, user.ID, dbConn.ID)
	job1.Status = models.BackupStatusCompleted
	job1.Type = models.BackupTypeFull
	db.Save(job1)

	job2 := &models.BackupJob{
		Name:                 "Test Backup 2",
		Type:                 models.BackupTypeIncremental,
		Status:               models.BackupStatusPending,
		UserID:               user.ID,
		DatabaseConnectionID: dbConn.ID,
		IsScheduled:          true,
		Tags:                 nil,
	}
	db.Create(job2)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	tests := []struct {
		name           string
		queryParams    string
		expectedCount  int
		expectedStatus models.BackupStatus
	}{
		{
			name:          "Filter by status completed",
			queryParams:   "status=completed",
			expectedCount: 1,
			expectedStatus: models.BackupStatusCompleted,
		},
		{
			name:          "Filter by status pending",
			queryParams:   "status=pending",
			expectedCount: 1,
			expectedStatus: models.BackupStatusPending,
		},
		{
			name:          "Filter by type full",
			queryParams:   "type=full",
			expectedCount: 1,
		},
		{
			name:          "Filter by scheduled jobs",
			queryParams:   "is_scheduled=true",
			expectedCount: 1,
		},
		{
			name:          "Filter by non-scheduled jobs",
			queryParams:   "is_scheduled=false",
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/backups?"+tt.queryParams, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("user", user)

			err := handler.GetBackups(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			var response BackupListResponse
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedCount, len(response.Backups))
			if tt.expectedStatus != "" && len(response.Backups) > 0 {
				assert.Equal(t, tt.expectedStatus, response.Backups[0].Status)
			}
		})
	}
}

func TestBackupHandler_GetBackups_Pagination(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)

	// Create multiple backup jobs
	for i := 0; i < 25; i++ {
		job := &models.BackupJob{
			Name:                 "Test Backup " + strconv.Itoa(i+1),
			Type:                 models.BackupTypeFull,
			Status:               models.BackupStatusCompleted,
			UserID:               user.ID,
			DatabaseConnectionID: dbConn.ID,
			Tags:                 nil,
		}
		db.Create(job)
	}

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Test first page
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/backups?page=1&limit=10", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", user)

	err := handler.GetBackups(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response BackupListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, int64(25), response.Total)
	assert.Equal(t, 10, len(response.Backups))
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 10, response.Limit)
	assert.Equal(t, 3, response.TotalPages)

	// Test second page
	req = httptest.NewRequest(http.MethodGet, "/api/backups?page=2&limit=10", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.Set("user", user)

	err = handler.GetBackups(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 10, len(response.Backups))
	assert.Equal(t, 2, response.Page)
}

func TestBackupHandler_GetBackup_Success(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	job := createTestBackupJob(db, user.ID, dbConn.ID)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/backups/"+job.UID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("uid")
	c.SetParamValues(job.UID)
	c.Set("user", user)

	// Execute
	err := handler.GetBackup(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response BackupResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, job.UID, response.UID)
	assert.Equal(t, "Test Backup", response.Name)
	assert.Equal(t, models.BackupStatusCompleted, response.Status)
	assert.NotNil(t, response.DatabaseConnection)
	assert.Equal(t, dbConn.UID, response.DatabaseConnection.UID)
}

func TestBackupHandler_GetBackup_NotFound(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/backups/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("uid")
	c.SetParamValues("nonexistent")
	c.Set("user", user)

	// Execute
	err := handler.GetBackup(c)

	// Assert
	assert.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, httpErr.Code)
}

func TestBackupHandler_CreateBackup_Success(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	// Set up mock expectations
	expectedJobInfo := &services.JobInfo{
		ID:   "test-job-123",
		Type: workers.TypeBackupPostgreSQL,
	}
	mockBackupWorker.On("EnqueueBackupJob", mock.Anything, workers.TypeBackupPostgreSQL, mock.Anything, mock.Anything).Return(expectedJobInfo, nil)

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create request payload
	reqPayload := CreateBackupRequest{
		Name:        "Test Backup",
		DatabaseUID: dbConn.UID,
		Type:        models.BackupTypeFull,
	}

	payloadBytes, _ := json.Marshal(reqPayload)

	// Create Echo context with validator
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/backups", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", user)

	// Execute
	err := handler.CreateBackup(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var response BackupResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Test Backup", response.Name)
	assert.Equal(t, models.BackupTypeFull, response.Type)
	assert.Equal(t, models.BackupStatusPending, response.Status)

	// Verify backup job was created in database
	var createdJob models.BackupJob
	err = db.Where("uid = ?", response.UID).First(&createdJob).Error
	require.NoError(t, err)
	assert.Equal(t, "Test Backup", createdJob.Name)
	assert.Equal(t, user.ID, createdJob.UserID)
	assert.Equal(t, dbConn.ID, createdJob.DatabaseConnectionID)

	// Verify mock was called
	mockBackupWorker.AssertExpectations(t)
}

func TestBackupHandler_CreateBackup_DatabaseNotFound(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create request payload with non-existent database UID
	reqPayload := CreateBackupRequest{
		Name:        "Test Backup",
		DatabaseUID: "nonexistent",
		Type:        models.BackupTypeFull,
	}

	payloadBytes, _ := json.Marshal(reqPayload)

	// Create Echo context with validator
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/backups", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", user)

	// Execute
	err := handler.CreateBackup(c)

	// Assert
	assert.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, httpErr.Code)
	assert.Contains(t, httpErr.Message, "Database connection not found")
}

func TestBackupHandler_CreateBackup_ScheduledBackup(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	// Set up mock expectations for scheduled job
	scheduleTime := time.Now().Add(1 * time.Hour)
	expectedJobInfo := &services.JobInfo{
		ID:   "scheduled-job-123",
		Type: workers.TypeScheduledBackup,
	}
	mockBackupWorker.On("EnqueueScheduledBackupJob", mock.Anything, mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(expectedJobInfo, nil)

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create request payload
	reqPayload := CreateBackupRequest{
		Name:        "Scheduled Backup",
		DatabaseUID: dbConn.UID,
		Type:        models.BackupTypeFull,
		ScheduleAt:  &scheduleTime,
	}

	payloadBytes, _ := json.Marshal(reqPayload)

	// Create Echo context with validator
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/backups", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user", user)

	// Execute
	err := handler.CreateBackup(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var response BackupResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.IsScheduled)

	// Verify backup job was created as scheduled
	var createdJob models.BackupJob
	err = db.Where("uid = ?", response.UID).First(&createdJob).Error
	require.NoError(t, err)
	assert.True(t, createdJob.IsScheduled)

	// Verify mock was called
	mockBackupWorker.AssertExpectations(t)
}

func TestBackupHandler_CancelBackup_Success(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	job := &models.BackupJob{
		Name:                 "Test Backup",
		Type:                 models.BackupTypeFull,
		Status:               models.BackupStatusPending,
		UserID:               user.ID,
		DatabaseConnectionID: dbConn.ID,
		Tags:                 nil,
	}
	db.Create(job)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	// Set up mock expectations (no queue job ID available yet)

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/backups/"+job.UID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("uid")
	c.SetParamValues(job.UID)
	c.Set("user", user)

	// Execute
	err := handler.CancelBackup(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response BackupResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, models.BackupStatusCancelled, response.Status)

	// Verify job was updated in database
	var updatedJob models.BackupJob
	err = db.Where("uid = ?", job.UID).First(&updatedJob).Error
	require.NoError(t, err)
	assert.Equal(t, models.BackupStatusCancelled, updatedJob.Status)

	// Note: Queue service mock not called since QueueJobID not implemented yet
}

func TestBackupHandler_CancelBackup_AlreadyCompleted(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	job := &models.BackupJob{
		Name:                 "Test Backup",
		Type:                 models.BackupTypeFull,
		Status:               models.BackupStatusCompleted,
		UserID:               user.ID,
		DatabaseConnectionID: dbConn.ID,
		Tags:                 nil,
	}
	db.Create(job)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/backups/"+job.UID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("uid")
	c.SetParamValues(job.UID)
	c.Set("user", user)

	// Execute
	err := handler.CancelBackup(c)

	// Assert
	assert.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	assert.Contains(t, httpErr.Message, "Cannot cancel completed")
}

func TestBackupHandler_RetryBackup_Success(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	errorMsg := "Previous error"
	errorCode := "ERROR_CODE"
	job := &models.BackupJob{
		Name:                 "Test Backup",
		Type:                 models.BackupTypeFull,
		Status:               models.BackupStatusFailed,
		UserID:               user.ID,
		DatabaseConnectionID: dbConn.ID,
		ErrorMessage:         &errorMsg,
		ErrorCode:            &errorCode,
		Tags:                 nil,
	}
	db.Create(job)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	// Set up mock expectations
	expectedJobInfo := &services.JobInfo{
		ID:   "retry-job-123",
		Type: workers.TypeBackupPostgreSQL,
	}
	mockBackupWorker.On("EnqueueBackupJob", mock.Anything, workers.TypeBackupPostgreSQL, mock.Anything, mock.Anything).Return(expectedJobInfo, nil)

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/backups/"+job.UID+"/retry", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("uid")
	c.SetParamValues(job.UID)
	c.Set("user", user)

	// Execute
	err := handler.RetryBackup(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response BackupResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, models.BackupStatusPending, response.Status)
	assert.Equal(t, "", response.ErrorMessage)
	assert.Equal(t, "", response.ErrorCode)

	// Verify job was reset in database
	var updatedJob models.BackupJob
	err = db.Where("uid = ?", job.UID).First(&updatedJob).Error
	require.NoError(t, err)
	assert.Equal(t, models.BackupStatusPending, updatedJob.Status)
	assert.Equal(t, float64(0), updatedJob.Progress)
	assert.Nil(t, updatedJob.ErrorMessage)
	assert.Nil(t, updatedJob.ErrorCode)

	// Verify mock was called
	mockBackupWorker.AssertExpectations(t)
}

func TestBackupHandler_GetBackupProgress_Success(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	startTime := time.Now().Add(-5 * time.Minute)
	job := &models.BackupJob{
		Name:                 "Test Backup",
		Type:                 models.BackupTypeFull,
		Status:               models.BackupStatusRunning,
		Progress:             75.5,
		CurrentStep:          "Processing table users",
		StartedAt:            &startTime,
		UserID:               user.ID,
		DatabaseConnectionID: dbConn.ID,
		Tags:                 nil,
	}
	db.Create(job)

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Create Echo context
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/backups/"+job.UID+"/progress", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("uid")
	c.SetParamValues(job.UID)
	c.Set("user", user)

	// Execute
	err := handler.GetBackupProgress(c)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, job.UID, response["uid"])
	assert.Equal(t, string(models.BackupStatusRunning), response["status"])
	assert.Equal(t, 75.5, response["progress"])
	assert.Equal(t, "Processing table users", response["progress_message"])
	assert.NotNil(t, response["started_at"])
}

func TestBackupHandler_convertBackupJobToResponse(t *testing.T) {
	db := setupTestDB()
	user := createTestUser(db)
	dbConn := createTestDatabaseConnection(db, user.ID)
	
	startTime := time.Now().Add(-10 * time.Minute)
	endTime := time.Now().Add(-5 * time.Minute)
	
	job := models.BackupJob{
		Name:            "Test Backup",
		Type:            models.BackupTypeFull,
		Status:          models.BackupStatusCompleted,
		Progress:        100.0,
		CurrentStep:     "Completed",
		StartedAt:       &startTime,
		CompletedAt:     &endTime,
		OriginalSize:    func() *int64 { v := int64(1024000); return &v }(),
		CompressedSize:  func() *int64 { v := int64(512000); return &v }(),
		IsScheduled:     false,
		Tags:            nil,
		DatabaseConnection: *dbConn,
		BackupFiles: []models.BackupFile{
			{
				Name:         "backup-file-1",
				FileType:     "dump",
				Size:         func() *int64 { v := int64(512000); return &v }(),
				IsCompressed: true,
				S3Bucket:     "test-bucket",
				S3Key:        "backups/test-key",
				S3Region:     "us-east-1",
			},
		},
	}

	// Create mocks
	mockBackupService := &MockBackupService{}
	mockS3Service := &MockS3Service{}
	mockQueueService := &MockQueueService{}
	mockBackupWorker := &MockBackupWorker{}

	handler := NewBackupHandler(db, mockBackupService, mockS3Service, mockQueueService, mockBackupWorker)

	// Execute
	response := handler.convertBackupJobToResponse(job)

	// Assert
	assert.Equal(t, job.UID, response.UID)
	assert.Equal(t, "Test Backup", response.Name)
	assert.Equal(t, models.BackupTypeFull, response.Type)
	assert.Equal(t, models.BackupStatusCompleted, response.Status)
	assert.Equal(t, 100.0, response.Progress)
	assert.Equal(t, "Completed", response.ProgressMessage)
	assert.Equal(t, startTime, *response.StartedAt)
	assert.Equal(t, endTime, *response.CompletedAt)
	assert.Equal(t, int64(1024000), *response.OriginalSize)
	assert.Equal(t, int64(512000), *response.CompressedSize)
	assert.False(t, response.IsScheduled)

	// Duration should be calculated
	assert.NotNil(t, response.Duration)
	expectedDuration := int64(endTime.Sub(startTime).Seconds())
	assert.Equal(t, expectedDuration, *response.Duration)

	// Database connection should be included
	assert.NotNil(t, response.DatabaseConnection)
	assert.Equal(t, dbConn.UID, response.DatabaseConnection.UID)
	assert.Equal(t, dbConn.Name, response.DatabaseConnection.Name)
	assert.Equal(t, string(dbConn.Type), response.DatabaseConnection.Type)

	// Backup files should be included
	assert.Equal(t, 1, len(response.BackupFiles))
	assert.Equal(t, "backup-file-1", response.BackupFiles[0].Name)
	assert.Equal(t, "dump", response.BackupFiles[0].FileType)
	assert.True(t, response.BackupFiles[0].IsCompressed)
	assert.Equal(t, "test-bucket", response.BackupFiles[0].S3Bucket)
	assert.Equal(t, "backups/test-key", response.BackupFiles[0].S3Key)
}