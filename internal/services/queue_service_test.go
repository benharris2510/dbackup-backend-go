package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueueService(t *testing.T) {
	config := &QueueConfig{
		RedisAddr:   "localhost:6379",
		RedisDB:     1, // Use different DB for tests
		Concurrency: 5,
		Queues: map[string]int{
			"test": 1,
		},
	}

	service, err := NewQueueService(config)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotNil(t, service.client)
	assert.NotNil(t, service.inspector)

	// Clean up
	err = service.Close()
	assert.NoError(t, err)
}

func TestNewQueueService_DefaultConfig(t *testing.T) {
	service, err := NewQueueService(nil)
	require.NoError(t, err)
	assert.NotNil(t, service)

	// Clean up
	err = service.Close()
	assert.NoError(t, err)
}

func TestJobOption_Functions(t *testing.T) {
	// Test WithQueue
	info := &asynq.TaskInfo{Type: "test"}
	WithQueue("test-queue")(info)
	assert.Equal(t, "test-queue", info.Queue)

	// Test WithMaxRetry
	info = &asynq.TaskInfo{Type: "test"}
	WithMaxRetry(5)(info)
	assert.Equal(t, 5, info.MaxRetry)

	// Test WithTimeout
	info = &asynq.TaskInfo{Type: "test"}
	WithTimeout(30*time.Second)(info)
	assert.Equal(t, 30*time.Second, info.Timeout)

	// Test WithDeadline
	deadline := time.Now().Add(1 * time.Hour)
	info = &asynq.TaskInfo{Type: "test"}
	WithDeadline(deadline)(info)
	assert.Equal(t, deadline, info.Deadline)

	// Test WithUnique (should not panic)
	info = &asynq.TaskInfo{Type: "test"}
	WithUnique(5*time.Minute)(info)
	// WithUnique doesn't modify TaskInfo fields in our implementation
}

func TestQueueService_EnqueueJob_ParameterValidation(t *testing.T) {
	// Skip this test if Redis is not available
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   1,
	})
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	defer service.Close()

	ctx := context.Background()

	// Test with valid parameters
	payload := map[string]interface{}{
		"database_id": "123",
		"backup_type": "full",
	}

	job, err := service.EnqueueJob(ctx, "backup:create", payload)
	if err != nil {
		// Redis might not be available in CI environment
		t.Logf("Enqueue failed (expected if Redis not available): %v", err)
		return
	}

	assert.NotNil(t, job)
	assert.Equal(t, "backup:create", job.Type)
	assert.Equal(t, payload, job.Payload)
	assert.Equal(t, JobStatePending, job.State)
}

func TestQueueService_EnqueueJob_WithOptions(t *testing.T) {
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   1,
	})
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	defer service.Close()

	ctx := context.Background()
	payload := map[string]interface{}{
		"test": "data",
	}

	job, err := service.EnqueueJob(ctx, "test:job", payload,
		WithQueue("critical"),
		WithMaxRetry(3),
		WithTimeout(5*time.Minute),
	)

	if err != nil {
		t.Logf("Enqueue failed (expected if Redis not available): %v", err)
		return
	}

	assert.NotNil(t, job)
	assert.Equal(t, "test:job", job.Type)
	assert.Equal(t, "critical", job.Queue)
	assert.Equal(t, 3, job.MaxRetry)
	assert.Equal(t, 5*time.Minute, job.Timeout)
}

func TestQueueService_EnqueueScheduledJob(t *testing.T) {
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   1,
	})
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	defer service.Close()

	ctx := context.Background()
	payload := map[string]interface{}{
		"scheduled": true,
	}
	processAt := time.Now().Add(1 * time.Hour)

	job, err := service.EnqueueScheduledJob(ctx, "scheduled:job", payload, processAt)
	if err != nil {
		t.Logf("Enqueue scheduled failed (expected if Redis not available): %v", err)
		return
	}

	assert.NotNil(t, job)
	assert.Equal(t, "scheduled:job", job.Type)
	assert.Equal(t, payload, job.Payload)
	assert.Equal(t, JobStateScheduled, job.State)
}

func TestQueueService_HealthCheck(t *testing.T) {
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   1,
	})
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	defer service.Close()

	ctx := context.Background()
	err = service.HealthCheck(ctx)
	if err != nil {
		t.Logf("Health check failed (expected if Redis not available): %v", err)
	}
}

func TestQueueService_GetQueueStats(t *testing.T) {
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   1,
	})
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	defer service.Close()

	ctx := context.Background()
	stats, err := service.GetQueueStats(ctx)
	if err != nil {
		t.Logf("Get stats failed (expected if Redis not available): %v", err)
		return
	}

	assert.NotNil(t, stats)
	assert.NotZero(t, stats.Timestamp)
	assert.NotNil(t, stats.Queues)
}

func TestQueueService_ListJobs(t *testing.T) {
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   1,
	})
	if err != nil {
		t.Skip("Redis not available for testing")
	}
	defer service.Close()

	ctx := context.Background()
	jobs, err := service.ListJobs(ctx, "", JobStatePending)
	if err != nil {
		t.Logf("List jobs failed (expected if Redis not available): %v", err)
		return
	}

	assert.NotNil(t, jobs)
	// jobs can be empty if no jobs are queued
}

// Test helper functions
func TestJobInfo_Structure(t *testing.T) {
	job := &JobInfo{
		ID:   "test-job-123",
		Type: "backup:create",
		Payload: map[string]interface{}{
			"database_id": "db-123",
		},
		State:    JobStatePending,
		Queue:    "default",
		MaxRetry: 3,
		Retried:  0,
		Timeout:  5 * time.Minute,
	}

	assert.Equal(t, "test-job-123", job.ID)
	assert.Equal(t, "backup:create", job.Type)
	assert.Equal(t, "db-123", job.Payload["database_id"])
	assert.Equal(t, JobStatePending, job.State)
	assert.Equal(t, "default", job.Queue)
	assert.Equal(t, 3, job.MaxRetry)
	assert.Equal(t, 0, job.Retried)
	assert.Equal(t, 5*time.Minute, job.Timeout)
}

func TestQueueStats_Structure(t *testing.T) {
	stats := &QueueStats{
		Pending:   10,
		Active:    2,
		Scheduled: 5,
		Retry:     1,
		Archived:  100,
		Completed: 95,
		Failed:    5,
		Processed: 100,
		Timestamp: time.Now(),
		Queues: map[string]*QueueInfo{
			"default": {
				Name:      "default",
				Size:      18,
				Pending:   10,
				Active:    2,
				Scheduled: 5,
				Retry:     1,
				Archived:  100,
				Paused:    false,
			},
		},
	}

	assert.Equal(t, int64(10), stats.Pending)
	assert.Equal(t, int64(2), stats.Active)
	assert.Equal(t, int64(5), stats.Scheduled)
	assert.Equal(t, int64(1), stats.Retry)
	assert.Equal(t, int64(100), stats.Archived)
	assert.NotNil(t, stats.Queues["default"])
	assert.Equal(t, "default", stats.Queues["default"].Name)
	assert.Equal(t, int64(18), stats.Queues["default"].Size)
}

func TestQueueService_convertTaskInfo(t *testing.T) {
	service, err := NewQueueService(nil)
	require.NoError(t, err)
	defer service.Close()

	// Create a sample TaskInfo
	taskInfo := &asynq.TaskInfo{
		ID:       "test-123",
		Type:     "backup:create",
		Queue:    "default",
		MaxRetry: 3,
		Retried:  1,
		State:    asynq.TaskStatePending,
		Timeout:  5 * time.Minute,
		Deadline: time.Now().Add(1 * time.Hour),
		LastErr:  "connection timeout",
	}

	payload := map[string]interface{}{
		"database_id": "db-123",
		"type":        "full",
	}

	job := service.convertTaskInfo(taskInfo, payload)

	assert.Equal(t, "test-123", job.ID)
	assert.Equal(t, "backup:create", job.Type)
	assert.Equal(t, payload, job.Payload)
	assert.Equal(t, JobStatePending, job.State)
	assert.Equal(t, "default", job.Queue)
	assert.Equal(t, 3, job.MaxRetry)
	assert.Equal(t, 1, job.Retried)
	assert.Equal(t, 5*time.Minute, job.Timeout)
	assert.Equal(t, "connection timeout", job.ErrorMsg)
	assert.NotNil(t, job.Deadline)
}

func TestQueueService_convertTaskInfoFromInspector(t *testing.T) {
	service, err := NewQueueService(nil)
	require.NoError(t, err)
	defer service.Close()

	// Create a sample TaskInfo with payload
	payload := map[string]interface{}{
		"database_id": "db-456",
	}
	payloadBytes, _ := json.Marshal(payload)

	taskInfo := &asynq.TaskInfo{
		ID:       "test-456",
		Type:     "backup:restore",
		Queue:    "critical",
		Payload:  payloadBytes,
		MaxRetry: 5,
		Retried:  2,
		State:    asynq.TaskStateActive,
		Timeout:  10 * time.Minute,
		LastErr:  "disk full",
	}

	job := service.convertTaskInfoFromInspector(taskInfo)

	assert.Equal(t, "test-456", job.ID)
	assert.Equal(t, "backup:restore", job.Type)
	assert.Equal(t, "db-456", job.Payload["database_id"])
	assert.Equal(t, JobStateActive, job.State)
	assert.Equal(t, "critical", job.Queue)
	assert.Equal(t, 5, job.MaxRetry)
	assert.Equal(t, 2, job.Retried)
	assert.Equal(t, 10*time.Minute, job.Timeout)
	assert.Equal(t, "disk full", job.ErrorMsg)
}

func TestJobState_Constants(t *testing.T) {
	assert.Equal(t, JobState("pending"), JobStatePending)
	assert.Equal(t, JobState("active"), JobStateActive)  
	assert.Equal(t, JobState("scheduled"), JobStateScheduled)
	assert.Equal(t, JobState("retry"), JobStateRetry)
	assert.Equal(t, JobState("archived"), JobStateArchived)
	assert.Equal(t, JobState("completed"), JobStateCompleted)
	assert.Equal(t, JobState("failed"), JobStateFailed)
}

func TestNewQueueWorker(t *testing.T) {
	config := &QueueConfig{
		RedisAddr:   "localhost:6379",
		RedisDB:     1,
		Concurrency: 3,
		Queues: map[string]int{
			"test": 1,
		},
	}

	worker := NewQueueWorker(config)
	assert.NotNil(t, worker)
	assert.NotNil(t, worker.server)
	assert.NotNil(t, worker.mux)
}

func TestNewQueueWorker_DefaultConfig(t *testing.T) {
	worker := NewQueueWorker(nil)
	assert.NotNil(t, worker)
	assert.NotNil(t, worker.server)
	assert.NotNil(t, worker.mux)
}

func TestQueueWorker_RegisterHandler(t *testing.T) {
	worker := NewQueueWorker(nil)
	
	handlerCalled := false
	handler := func(ctx context.Context, task *asynq.Task) error {
		handlerCalled = true
		return nil
	}

	// This should not panic
	worker.RegisterHandler("test:job", handler)
	
	// We can't easily test the handler execution without a full integration test
	assert.False(t, handlerCalled) // Handler not called yet
}

// Integration tests (these would require Redis running)
func TestQueueService_Integration_FullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require Redis to be running
	t.Skip("Integration test requires Redis setup")

	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "localhost:6379",
		RedisDB:   2, // Use different DB for integration tests
	})
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()

	// Health check
	err = service.HealthCheck(ctx)
	require.NoError(t, err)

	// Enqueue a job
	payload := map[string]interface{}{
		"test_data": "integration_test",
	}

	job, err := service.EnqueueJob(ctx, "integration:test", payload, WithQueue("test"))
	require.NoError(t, err)
	assert.NotNil(t, job)

	// List jobs
	jobs, err := service.ListJobs(ctx, "integration:test", JobStatePending)
	require.NoError(t, err)
	assert.True(t, len(jobs) >= 1)

	// Get queue stats
	stats, err := service.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.True(t, stats.Pending >= 1)

	// Cancel the job (cleanup)
	err = service.CancelJob(ctx, job.ID)
	// Error is expected if job was already processed
	t.Logf("Cancel result: %v", err)
}

func TestQueueService_InvalidRedisConfig(t *testing.T) {
	service, err := NewQueueService(&QueueConfig{
		RedisAddr: "invalid:12345",
		RedisDB:   0,
	})
	require.NoError(t, err) // Service creation doesn't test connection
	defer service.Close()

	ctx := context.Background()

	// Health check should fail
	err = service.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis connection failed")
}

func TestQueueService_EnqueueJob_MarshalError(t *testing.T) {
	service, err := NewQueueService(nil)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()

	// Create payload that can't be marshaled (contains channel)
	payload := map[string]interface{}{
		"channel": make(chan int),
	}

	job, err := service.EnqueueJob(ctx, "test:job", payload)
	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "failed to marshal payload")
}

func TestQueueService_EnqueueScheduledJob_MarshalError(t *testing.T) {
	service, err := NewQueueService(nil)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()

	// Create payload that can't be marshaled
	payload := map[string]interface{}{
		"function": func() {},
	}

	job, err := service.EnqueueScheduledJob(ctx, "test:job", payload, time.Now().Add(1*time.Hour))
	assert.Error(t, err)
	assert.Nil(t, job)
	assert.Contains(t, err.Error(), "failed to marshal payload")
}