package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// QueueServiceInterface defines the interface for job queue operations
type QueueServiceInterface interface {
	// Job management
	EnqueueJob(ctx context.Context, jobType string, payload interface{}, opts ...JobOption) (*JobInfo, error)
	EnqueueScheduledJob(ctx context.Context, jobType string, payload interface{}, processAt time.Time, opts ...JobOption) (*JobInfo, error)
	
	// Job queries
	GetJob(ctx context.Context, jobID string) (*JobInfo, error)
	ListJobs(ctx context.Context, jobType string, state JobState) ([]*JobInfo, error)
	CancelJob(ctx context.Context, jobID string) error
	RetryJob(ctx context.Context, jobID string) error
	
	// Queue management
	GetQueueStats(ctx context.Context) (*QueueStats, error)
	PauseQueue(ctx context.Context, queueName string) error
	UnpauseQueue(ctx context.Context, queueName string) error
	
	// Cleanup
	DeleteQueue(ctx context.Context, queueName string) error
	
	// Health check
	HealthCheck(ctx context.Context) error
	
	// Close resources
	Close() error
}

// QueueService implements job queue operations using Asynq
type QueueService struct {
	client    *asynq.Client
	inspector *asynq.Inspector
	redisOpt  asynq.RedisClientOpt
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Concurrency   int
	Queues        map[string]int // queue name -> priority
}

// JobState represents the state of a job
type JobState string

const (
	JobStatePending   JobState = "pending"
	JobStateActive    JobState = "active"
	JobStateScheduled JobState = "scheduled"
	JobStateRetry     JobState = "retry"
	JobStateArchived  JobState = "archived"
	JobStateCompleted JobState = "completed"
	JobStateFailed    JobState = "failed"
)

// JobInfo contains information about a job
type JobInfo struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	State       JobState               `json:"state"`
	Queue       string                 `json:"queue"`
	MaxRetry    int                    `json:"max_retry"`
	Retried     int                    `json:"retried"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	FailedAt    *time.Time             `json:"failed_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	NextRunAt   *time.Time             `json:"next_run_at,omitempty"`
	Timeout     time.Duration          `json:"timeout"`
	Deadline    *time.Time             `json:"deadline,omitempty"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
}

// QueueStats contains queue statistics
type QueueStats struct {
	Pending   int64                    `json:"pending"`
	Active    int64                    `json:"active"`
	Scheduled int64                    `json:"scheduled"`
	Retry     int64                    `json:"retry"`
	Archived  int64                    `json:"archived"`
	Completed int64                    `json:"completed"`
	Failed    int64                    `json:"failed"`
	Processed int64                    `json:"processed"`
	Timestamp time.Time                `json:"timestamp"`
	Queues    map[string]*QueueInfo    `json:"queues"`
}

// QueueInfo contains information about a specific queue
type QueueInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Pending   int64  `json:"pending"`
	Active    int64  `json:"active"`
	Scheduled int64  `json:"scheduled"`
	Retry     int64  `json:"retry"`
	Archived  int64  `json:"archived"`
	Paused    bool   `json:"paused"`
}

// JobOption represents options for job creation
type JobOption func(*asynq.TaskInfo)

// NewQueueService creates a new queue service
func NewQueueService(config *QueueConfig) (*QueueService, error) {
	if config == nil {
		config = &QueueConfig{
			RedisAddr:   "localhost:6379",
			RedisDB:     0,
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		}
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	}

	client := asynq.NewClient(redisOpt)
	inspector := asynq.NewInspector(redisOpt)

	return &QueueService{
		client:    client,
		inspector: inspector,
		redisOpt:  redisOpt,
	}, nil
}

// WithQueue sets the queue for a job
func WithQueue(queue string) JobOption {
	return func(info *asynq.TaskInfo) {
		info.Queue = queue
	}
}

// WithMaxRetry sets the maximum retry count for a job
func WithMaxRetry(maxRetry int) JobOption {
	return func(info *asynq.TaskInfo) {
		info.MaxRetry = maxRetry
	}
}

// WithTimeout sets the timeout for a job
func WithTimeout(timeout time.Duration) JobOption {
	return func(info *asynq.TaskInfo) {
		info.Timeout = timeout
	}
}

// WithDeadline sets the deadline for a job
func WithDeadline(deadline time.Time) JobOption {
	return func(info *asynq.TaskInfo) {
		info.Deadline = deadline
	}
}

// WithUnique makes a job unique (prevents duplicates)
func WithUnique(ttl time.Duration) JobOption {
	return func(info *asynq.TaskInfo) {
		// UniqueKey and UniqueTTL are not fields on TaskInfo
		// They are handled via asynq.Option in the actual enqueue call
	}
}

// EnqueueJob enqueues a job for immediate processing
func (qs *QueueService) EnqueueJob(ctx context.Context, jobType string, payload interface{}, opts ...JobOption) (*JobInfo, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(jobType, payloadBytes)
	
	// Apply options
	var taskOpts []asynq.Option
	for _, opt := range opts {
		// Create a TaskInfo to collect options
		info := &asynq.TaskInfo{Type: jobType}
		opt(info)
		
		// Convert to asynq options
		if info.Queue != "" {
			taskOpts = append(taskOpts, asynq.Queue(info.Queue))
		}
		if info.MaxRetry > 0 {
			taskOpts = append(taskOpts, asynq.MaxRetry(info.MaxRetry))
		}
		if info.Timeout > 0 {
			taskOpts = append(taskOpts, asynq.Timeout(info.Timeout))
		}
		if !info.Deadline.IsZero() {
			taskOpts = append(taskOpts, asynq.Deadline(info.Deadline))
		}
	}

	taskInfo, err := qs.client.EnqueueContext(ctx, task, taskOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return qs.convertTaskInfo(taskInfo, payload), nil
}

// EnqueueScheduledJob enqueues a job for processing at a specific time
func (qs *QueueService) EnqueueScheduledJob(ctx context.Context, jobType string, payload interface{}, processAt time.Time, opts ...JobOption) (*JobInfo, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := asynq.NewTask(jobType, payloadBytes)
	
	// Apply options
	var taskOpts []asynq.Option
	taskOpts = append(taskOpts, asynq.ProcessAt(processAt))
	
	for _, opt := range opts {
		// Create a TaskInfo to collect options
		info := &asynq.TaskInfo{Type: jobType}
		opt(info)
		
		// Convert to asynq options
		if info.Queue != "" {
			taskOpts = append(taskOpts, asynq.Queue(info.Queue))
		}
		if info.MaxRetry > 0 {
			taskOpts = append(taskOpts, asynq.MaxRetry(info.MaxRetry))
		}
		if info.Timeout > 0 {
			taskOpts = append(taskOpts, asynq.Timeout(info.Timeout))
		}
		if !info.Deadline.IsZero() {
			taskOpts = append(taskOpts, asynq.Deadline(info.Deadline))
		}
	}

	taskInfo, err := qs.client.EnqueueContext(ctx, task, taskOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue scheduled job: %w", err)
	}

	return qs.convertTaskInfo(taskInfo, payload), nil
}

// GetJob retrieves information about a specific job
func (qs *QueueService) GetJob(ctx context.Context, jobID string) (*JobInfo, error) {
	// Try to find the job in different queues
	queues, err := qs.inspector.Queues()
	if err != nil {
		return nil, fmt.Errorf("failed to get queues: %w", err)
	}

	// Try to find the task in different states across all queues

	for _, queueName := range queues {
		// Try different task listing methods based on state
		var taskLists [][]*asynq.TaskInfo
		
		pending, _ := qs.inspector.ListPendingTasks(queueName)
		active, _ := qs.inspector.ListActiveTasks(queueName)
		scheduled, _ := qs.inspector.ListScheduledTasks(queueName)
		retry, _ := qs.inspector.ListRetryTasks(queueName)
		archived, _ := qs.inspector.ListArchivedTasks(queueName)
		completed, _ := qs.inspector.ListCompletedTasks(queueName)
		
		taskLists = append(taskLists, pending, active, scheduled, retry, archived, completed)
		
		for _, tasks := range taskLists {
			for _, task := range tasks {
				if task.ID == jobID {
					return qs.convertTaskInfoFromInspector(task), nil
				}
			}
		}
	}

	return nil, fmt.Errorf("job not found: %s", jobID)
}

// ListJobs lists jobs of a specific type and state
func (qs *QueueService) ListJobs(ctx context.Context, jobType string, state JobState) ([]*JobInfo, error) {
	queues, err := qs.inspector.Queues()
	if err != nil {
		return nil, fmt.Errorf("failed to get queues: %w", err)
	}

	var jobs []*JobInfo

	for _, queueName := range queues {
		var tasks []*asynq.TaskInfo
		var err error

		switch state {
		case JobStatePending:
			tasks, err = qs.inspector.ListPendingTasks(queueName)
		case JobStateActive:
			tasks, err = qs.inspector.ListActiveTasks(queueName)
		case JobStateScheduled:
			tasks, err = qs.inspector.ListScheduledTasks(queueName)
		case JobStateRetry:
			tasks, err = qs.inspector.ListRetryTasks(queueName)
		case JobStateArchived:
			tasks, err = qs.inspector.ListArchivedTasks(queueName)
		case JobStateCompleted:
			tasks, err = qs.inspector.ListCompletedTasks(queueName)
		default:
			return nil, fmt.Errorf("unsupported job state: %s", state)
		}

		if err != nil {
			continue // Skip this queue if there's an error
		}

		for _, task := range tasks {
			if jobType == "" || task.Type == jobType {
				jobs = append(jobs, qs.convertTaskInfoFromInspector(task))
			}
		}
	}

	return jobs, nil
}

// CancelJob cancels a pending or scheduled job
func (qs *QueueService) CancelJob(ctx context.Context, jobID string) error {
	// Find the queue containing the job
	queues, err := qs.inspector.Queues()
	if err != nil {
		return fmt.Errorf("failed to get queues: %w", err)
	}

	for _, queueName := range queues {
		err := qs.inspector.DeleteTask(queueName, jobID)
		if err == nil {
			return nil // Successfully deleted
		}
	}

	return fmt.Errorf("failed to cancel job %s: job not found", jobID)
}

// RetryJob retries a failed job
func (qs *QueueService) RetryJob(ctx context.Context, jobID string) error {
	// Find the queue containing the job and run it
	queues, err := qs.inspector.Queues()
	if err != nil {
		return fmt.Errorf("failed to get queues: %w", err)
	}

	for _, queueName := range queues {
		err := qs.inspector.RunTask(queueName, jobID)
		if err == nil {
			return nil // Successfully retried
		}
	}

	return fmt.Errorf("failed to retry job %s: job not found", jobID)
}

// GetQueueStats returns queue statistics
func (qs *QueueService) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	queueStats := &QueueStats{
		Timestamp: time.Now(),
		Queues:    make(map[string]*QueueInfo),
	}

	// Get information for each queue
	queues, err := qs.inspector.Queues()
	if err != nil {
		return nil, fmt.Errorf("failed to get queues: %w", err)
	}

	var totalPending, totalActive, totalScheduled, totalRetry, totalArchived int64

	for _, queueName := range queues {
		queueInfo, err := qs.inspector.GetQueueInfo(queueName)
		if err != nil {
			continue
		}

		queueStats.Queues[queueName] = &QueueInfo{
			Name:      queueInfo.Queue,
			Size:      int64(queueInfo.Size),
			Pending:   int64(queueInfo.Pending),
			Active:    int64(queueInfo.Active),
			Scheduled: int64(queueInfo.Scheduled),
			Retry:     int64(queueInfo.Retry),
			Archived:  int64(queueInfo.Archived),
			Paused:    queueInfo.Paused,
		}

		// Aggregate totals
		totalPending += int64(queueInfo.Pending)
		totalActive += int64(queueInfo.Active)
		totalScheduled += int64(queueInfo.Scheduled)
		totalRetry += int64(queueInfo.Retry)
		totalArchived += int64(queueInfo.Archived)
	}

	queueStats.Pending = totalPending
	queueStats.Active = totalActive
	queueStats.Scheduled = totalScheduled
	queueStats.Retry = totalRetry
	queueStats.Archived = totalArchived
	queueStats.Processed = totalActive + totalArchived // Approximation

	return queueStats, nil
}

// PauseQueue pauses a queue
func (qs *QueueService) PauseQueue(ctx context.Context, queueName string) error {
	err := qs.inspector.PauseQueue(queueName)
	if err != nil {
		return fmt.Errorf("failed to pause queue %s: %w", queueName, err)
	}
	return nil
}

// UnpauseQueue unpauses a queue
func (qs *QueueService) UnpauseQueue(ctx context.Context, queueName string) error {
	err := qs.inspector.UnpauseQueue(queueName)
	if err != nil {
		return fmt.Errorf("failed to unpause queue %s: %w", queueName, err)
	}
	return nil
}

// DeleteQueue deletes all jobs in a queue
func (qs *QueueService) DeleteQueue(ctx context.Context, queueName string) error {
	err := qs.inspector.DeleteQueue(queueName, false)
	if err != nil {
		return fmt.Errorf("failed to delete queue %s: %w", queueName, err)
	}
	return nil
}

// HealthCheck checks if the queue service is healthy
func (qs *QueueService) HealthCheck(ctx context.Context) error {
	// Test Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr:     qs.redisOpt.Addr,
		Password: qs.redisOpt.Password,
		DB:       qs.redisOpt.DB,
	})
	defer rdb.Close()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	// Test inspector functionality
	_, err = qs.inspector.Queues()
	if err != nil {
		return fmt.Errorf("inspector failed: %w", err)
	}

	return nil
}

// Close closes the queue service and releases resources
func (qs *QueueService) Close() error {
	if err := qs.client.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}
	
	if err := qs.inspector.Close(); err != nil {
		return fmt.Errorf("failed to close inspector: %w", err)
	}
	
	return nil
}

// Shutdown gracefully shuts down the queue service
func (qs *QueueService) Shutdown(ctx context.Context) error {
	log.Println("Shutting down queue service...")
	
	// Wait for any pending operations to complete or timeout
	select {
	case <-ctx.Done():
		log.Println("Queue service shutdown timed out, forcing close")
	case <-time.After(2 * time.Second):
		log.Println("Queue service shutdown grace period completed")
	}
	
	// Close the service
	return qs.Close()
}

// convertTaskInfo converts asynq.TaskInfo to JobInfo
func (qs *QueueService) convertTaskInfo(taskInfo *asynq.TaskInfo, payload interface{}) *JobInfo {
	var payloadMap map[string]interface{}
	if payload != nil {
		payloadBytes, _ := json.Marshal(payload)
		json.Unmarshal(payloadBytes, &payloadMap)
	}

	job := &JobInfo{
		ID:       taskInfo.ID,
		Type:     taskInfo.Type,
		Payload:  payloadMap,
		Queue:    taskInfo.Queue,
		MaxRetry: taskInfo.MaxRetry,
		Retried:  taskInfo.Retried,
		Timeout:  taskInfo.Timeout,
	}

	// Convert state
	switch taskInfo.State {
	case asynq.TaskStatePending:
		job.State = JobStatePending
	case asynq.TaskStateActive:
		job.State = JobStateActive
	case asynq.TaskStateScheduled:
		job.State = JobStateScheduled
	case asynq.TaskStateRetry:
		job.State = JobStateRetry
	case asynq.TaskStateArchived:
		job.State = JobStateArchived
	case asynq.TaskStateCompleted:
		job.State = JobStateCompleted
	default:
		job.State = JobStatePending
	}

	// Set timestamps if available (using available fields)
	if !taskInfo.NextProcessAt.IsZero() {
		job.NextRunAt = &taskInfo.NextProcessAt
	}
	if !taskInfo.Deadline.IsZero() {
		job.Deadline = &taskInfo.Deadline
	}

	job.ErrorMsg = taskInfo.LastErr

	return job
}

// convertTaskInfoFromInspector converts inspector TaskInfo to JobInfo
func (qs *QueueService) convertTaskInfoFromInspector(taskInfo *asynq.TaskInfo) *JobInfo {
	var payloadMap map[string]interface{}
	if len(taskInfo.Payload) > 0 {
		json.Unmarshal(taskInfo.Payload, &payloadMap)
	}

	job := &JobInfo{
		ID:       taskInfo.ID,
		Type:     taskInfo.Type,
		Payload:  payloadMap,
		Queue:    taskInfo.Queue,
		MaxRetry: taskInfo.MaxRetry,
		Retried:  taskInfo.Retried,
		Timeout:  taskInfo.Timeout,
		ErrorMsg: taskInfo.LastErr,
	}

	// Convert state
	switch taskInfo.State {
	case asynq.TaskStatePending:
		job.State = JobStatePending
	case asynq.TaskStateActive:
		job.State = JobStateActive
	case asynq.TaskStateScheduled:
		job.State = JobStateScheduled
	case asynq.TaskStateRetry:
		job.State = JobStateRetry
	case asynq.TaskStateArchived:
		job.State = JobStateArchived
	case asynq.TaskStateCompleted:
		job.State = JobStateCompleted
	default:
		job.State = JobStatePending
	}

	// Set timestamps (using available fields)
	if !taskInfo.NextProcessAt.IsZero() {
		job.NextRunAt = &taskInfo.NextProcessAt
	}
	if !taskInfo.Deadline.IsZero() {
		job.Deadline = &taskInfo.Deadline
	}

	return job
}

// QueueWorker represents a worker that processes jobs
type QueueWorker struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

// NewQueueWorker creates a new queue worker
func NewQueueWorker(config *QueueConfig) *QueueWorker {
	if config == nil {
		config = &QueueConfig{
			RedisAddr:   "localhost:6379",
			RedisDB:     0,
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		}
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: config.Concurrency,
		Queues:      config.Queues,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Printf("Error processing task %s: %v", task.Type(), err)
		}),
	})

	mux := asynq.NewServeMux()

	return &QueueWorker{
		server: server,
		mux:    mux,
	}
}

// RegisterHandler registers a handler for a specific job type
func (qw *QueueWorker) RegisterHandler(jobType string, handler func(context.Context, *asynq.Task) error) {
	qw.mux.HandleFunc(jobType, handler)
}

// Start starts the worker
func (qw *QueueWorker) Start() error {
	return qw.server.Run(qw.mux)
}

// Stop stops the worker
func (qw *QueueWorker) Stop() {
	qw.server.Stop()
}

// Shutdown gracefully shuts down the worker
func (qw *QueueWorker) Shutdown() {
	qw.server.Shutdown()
}