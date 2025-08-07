package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/queue"
	"github.com/dbackup/backend-go/internal/websocket"
)

// ShutdownManager manages graceful shutdown of the application
type ShutdownManager struct {
	server       *echo.Echo
	db           *gorm.DB
	queueService *queue.Service
	wsHub        *websocket.Hub
	logger       Logger
	timeout      time.Duration
	hooks        []ShutdownHook
	mu           sync.RWMutex
	isShuttingDown bool
}

// ShutdownHook represents a function to call during shutdown
type ShutdownHook func(ctx context.Context) error

// Logger interface for shutdown logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// DefaultShutdownLogger is a simple logger implementation
type DefaultShutdownLogger struct{}

func (l DefaultShutdownLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[SHUTDOWN INFO] "+msg+"\n", args...)
}

func (l DefaultShutdownLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[SHUTDOWN ERROR] "+msg+"\n", args...)
}

func (l DefaultShutdownLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[SHUTDOWN WARN] "+msg+"\n", args...)
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(server *echo.Echo) *ShutdownManager {
	return &ShutdownManager{
		server:  server,
		logger:  DefaultShutdownLogger{},
		timeout: 30 * time.Second,
		hooks:   make([]ShutdownHook, 0),
	}
}

// SetLogger sets a custom logger for the shutdown manager
func (sm *ShutdownManager) SetLogger(logger Logger) {
	sm.logger = logger
}

// SetTimeout sets the shutdown timeout
func (sm *ShutdownManager) SetTimeout(timeout time.Duration) {
	sm.timeout = timeout
}

// SetDatabase sets the database connection for graceful shutdown
func (sm *ShutdownManager) SetDatabase(db *gorm.DB) {
	sm.db = db
}

// SetQueueService sets the queue service for graceful shutdown
func (sm *ShutdownManager) SetQueueService(qs *queue.Service) {
	sm.queueService = qs
}

// SetWebSocketHub sets the WebSocket hub for graceful shutdown
func (sm *ShutdownManager) SetWebSocketHub(hub *websocket.Hub) {
	sm.wsHub = hub
}

// AddShutdownHook adds a custom shutdown hook
func (sm *ShutdownManager) AddShutdownHook(hook ShutdownHook) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.hooks = append(sm.hooks, hook)
}

// IsShuttingDown returns true if the shutdown process has started
func (sm *ShutdownManager) IsShuttingDown() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isShuttingDown
}

// WaitForShutdown waits for shutdown signals and handles graceful shutdown
func (sm *ShutdownManager) WaitForShutdown() {
	// Create channel to receive OS signals
	quit := make(chan os.Signal, 1)
	
	// Register the channel to receive specific OS signals
	signal.Notify(quit, 
		syscall.SIGINT,  // Interrupt (Ctrl+C)
		syscall.SIGTERM, // Terminate
		syscall.SIGQUIT, // Quit
	)

	// Wait for signal
	sig := <-quit
	sm.logger.Info("Received signal: %v. Starting graceful shutdown...", sig)

	// Mark as shutting down
	sm.mu.Lock()
	sm.isShuttingDown = true
	sm.mu.Unlock()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.timeout)
	defer cancel()

	// Perform graceful shutdown
	if err := sm.Shutdown(ctx); err != nil {
		sm.logger.Error("Graceful shutdown failed: %v", err)
		os.Exit(1)
	}

	sm.logger.Info("Graceful shutdown completed successfully")
	os.Exit(0)
}

// Shutdown performs the actual graceful shutdown
func (sm *ShutdownManager) Shutdown(ctx context.Context) error {
	sm.logger.Info("Starting graceful shutdown process...")

	// Create error channel to collect shutdown errors
	errChan := make(chan error, 10)
	var wg sync.WaitGroup

	// Shutdown HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		sm.logger.Info("Shutting down HTTP server...")
		
		if err := sm.shutdownHTTPServer(ctx); err != nil {
			sm.logger.Error("Error shutting down HTTP server: %v", err)
			errChan <- fmt.Errorf("HTTP server shutdown: %w", err)
			return
		}
		
		sm.logger.Info("HTTP server shutdown completed")
	}()

	// Shutdown WebSocket hub
	if sm.wsHub != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.logger.Info("Shutting down WebSocket hub...")
			
			if err := sm.shutdownWebSocketHub(ctx); err != nil {
				sm.logger.Error("Error shutting down WebSocket hub: %v", err)
				errChan <- fmt.Errorf("WebSocket hub shutdown: %w", err)
				return
			}
			
			sm.logger.Info("WebSocket hub shutdown completed")
		}()
	}

	// Shutdown queue service
	if sm.queueService != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.logger.Info("Shutting down queue service...")
			
			if err := sm.shutdownQueueService(ctx); err != nil {
				sm.logger.Error("Error shutting down queue service: %v", err)
				errChan <- fmt.Errorf("queue service shutdown: %w", err)
				return
			}
			
			sm.logger.Info("Queue service shutdown completed")
		}()
	}

	// Run custom shutdown hooks
	if len(sm.hooks) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.logger.Info("Running custom shutdown hooks...")
			
			if err := sm.runShutdownHooks(ctx); err != nil {
				sm.logger.Error("Error running shutdown hooks: %v", err)
				errChan <- fmt.Errorf("shutdown hooks: %w", err)
				return
			}
			
			sm.logger.Info("Custom shutdown hooks completed")
		}()
	}

	// Wait for all shutdown operations to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect any errors
	var shutdownErrors []error
	for err := range errChan {
		shutdownErrors = append(shutdownErrors, err)
	}

	// Shutdown database connections (done last)
	if sm.db != nil {
		sm.logger.Info("Closing database connections...")
		if err := sm.shutdownDatabase(ctx); err != nil {
			sm.logger.Error("Error closing database connections: %v", err)
			shutdownErrors = append(shutdownErrors, fmt.Errorf("database shutdown: %w", err))
		} else {
			sm.logger.Info("Database connections closed")
		}
	}

	// Return combined errors if any
	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown completed with errors: %v", shutdownErrors)
	}

	return nil
}

// shutdownHTTPServer gracefully shuts down the HTTP server
func (sm *ShutdownManager) shutdownHTTPServer(ctx context.Context) error {
	if sm.server == nil {
		return nil
	}

	// Echo's Shutdown method handles graceful shutdown
	return sm.server.Shutdown(ctx)
}

// shutdownWebSocketHub gracefully shuts down the WebSocket hub
func (sm *ShutdownManager) shutdownWebSocketHub(ctx context.Context) error {
	if sm.wsHub == nil {
		return nil
	}

	// Close all WebSocket connections gracefully
	return sm.wsHub.Shutdown(ctx)
}

// shutdownQueueService gracefully shuts down the queue service
func (sm *ShutdownManager) shutdownQueueService(ctx context.Context) error {
	if sm.queueService == nil {
		return nil
	}

	// Stop accepting new jobs and wait for current jobs to complete
	return sm.queueService.Shutdown(ctx)
}

// shutdownDatabase closes database connections
func (sm *ShutdownManager) shutdownDatabase(ctx context.Context) error {
	if sm.db == nil {
		return nil
	}

	// Get the underlying sql.DB
	sqlDB, err := sm.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Close the connection pool
	return sqlDB.Close()
}

// runShutdownHooks executes all registered shutdown hooks
func (sm *ShutdownManager) runShutdownHooks(ctx context.Context) error {
	sm.mu.RLock()
	hooks := make([]ShutdownHook, len(sm.hooks))
	copy(hooks, sm.hooks)
	sm.mu.RUnlock()

	var hookErrors []error
	
	for i, hook := range hooks {
		sm.logger.Info("Running shutdown hook %d/%d", i+1, len(hooks))
		
		if err := hook(ctx); err != nil {
			sm.logger.Error("Shutdown hook %d failed: %v", i+1, err)
			hookErrors = append(hookErrors, fmt.Errorf("hook %d: %w", i+1, err))
		} else {
			sm.logger.Info("Shutdown hook %d completed successfully", i+1)
		}
	}

	if len(hookErrors) > 0 {
		return fmt.Errorf("shutdown hooks failed: %v", hookErrors)
	}

	return nil
}

// ForceShutdown forcefully terminates the application
func (sm *ShutdownManager) ForceShutdown() {
	sm.logger.Warn("Force shutdown initiated")
	os.Exit(1)
}

// HealthcheckShutdownHook creates a shutdown hook that marks the service as unhealthy
func HealthcheckShutdownHook() ShutdownHook {
	return func(ctx context.Context) error {
		// This would typically update a health check endpoint or external service
		// to indicate the service is shutting down and should not receive new traffic
		fmt.Println("Marking service as unhealthy for load balancer removal")
		
		// Give load balancers time to remove this instance
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}
}

// CleanupTempFilesHook creates a shutdown hook that cleans up temporary files
func CleanupTempFilesHook(tempDir string) ShutdownHook {
	return func(ctx context.Context) error {
		if tempDir == "" {
			return nil
		}

		fmt.Printf("Cleaning up temporary files in %s\n", tempDir)
		
		// Implementation would clean up temp files
		// For now, just simulate the operation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}
}

// LogRotationShutdownHook creates a shutdown hook that rotates logs
func LogRotationShutdownHook() ShutdownHook {
	return func(ctx context.Context) error {
		fmt.Println("Rotating logs before shutdown")
		
		// Implementation would rotate/flush logs
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	}
}

// MetricsFlushHook creates a shutdown hook that flushes metrics
func MetricsFlushHook() ShutdownHook {
	return func(ctx context.Context) error {
		fmt.Println("Flushing metrics before shutdown")
		
		// Implementation would flush any pending metrics
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}
}

// NotifyExternalServicesHook creates a shutdown hook that notifies external services
func NotifyExternalServicesHook(webhookURL string) ShutdownHook {
	return func(ctx context.Context) error {
		if webhookURL == "" {
			return nil
		}

		fmt.Printf("Notifying external services at %s\n", webhookURL)
		
		// Implementation would send shutdown notification to external services
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
			return nil
		}
	}
}

// GetDefaultShutdownManager creates a shutdown manager with common configuration
func GetDefaultShutdownManager(server *echo.Echo) *ShutdownManager {
	sm := NewShutdownManager(server)
	sm.SetTimeout(30 * time.Second)
	
	// Add default database connection if available
	if db := database.GetDB(); db != nil {
		sm.SetDatabase(db)
	}
	
	// Add common shutdown hooks
	sm.AddShutdownHook(HealthcheckShutdownHook())
	sm.AddShutdownHook(CleanupTempFilesHook("/tmp"))
	sm.AddShutdownHook(LogRotationShutdownHook())
	sm.AddShutdownHook(MetricsFlushHook())
	
	return sm
}