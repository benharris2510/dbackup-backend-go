package server

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ShutdownManagerTestSuite struct {
	suite.Suite
	server          *echo.Echo
	shutdownManager *ShutdownManager
	db              *gorm.DB
}

func TestShutdownManagerSuite(t *testing.T) {
	suite.Run(t, new(ShutdownManagerTestSuite))
}

func (suite *ShutdownManagerTestSuite) SetupTest() {
	// Create Echo server
	suite.server = echo.New()

	// Create in-memory database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(suite.T(), err)
	suite.db = db

	// Create shutdown manager
	suite.shutdownManager = NewShutdownManager(suite.server)
	suite.shutdownManager.SetDatabase(suite.db)
	suite.shutdownManager.SetTimeout(5 * time.Second)
}

func (suite *ShutdownManagerTestSuite) TearDownTest() {
	if suite.db != nil {
		sqlDB, _ := suite.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

func (suite *ShutdownManagerTestSuite) TestNewShutdownManager() {
	t := suite.T()

	sm := NewShutdownManager(suite.server)
	assert.NotNil(t, sm)
	assert.Equal(t, suite.server, sm.server)
	assert.Equal(t, 30*time.Second, sm.timeout)
	assert.False(t, sm.IsShuttingDown())
	assert.Len(t, sm.hooks, 0)
}

func (suite *ShutdownManagerTestSuite) TestSetters() {
	t := suite.T()

	// Test SetTimeout
	suite.shutdownManager.SetTimeout(10 * time.Second)
	assert.Equal(t, 10*time.Second, suite.shutdownManager.timeout)

	// Test SetDatabase
	suite.shutdownManager.SetDatabase(suite.db)
	assert.Equal(t, suite.db, suite.shutdownManager.db)

	// Test custom logger
	customLogger := &TestLogger{}
	suite.shutdownManager.SetLogger(customLogger)
	assert.Equal(t, customLogger, suite.shutdownManager.logger)
}

func (suite *ShutdownManagerTestSuite) TestAddShutdownHook() {
	t := suite.T()

	// Test adding hooks
	hook1 := func(ctx context.Context) error {
		return nil
	}
	hook2 := func(ctx context.Context) error {
		return fmt.Errorf("test error")
	}

	suite.shutdownManager.AddShutdownHook(hook1)
	assert.Len(t, suite.shutdownManager.hooks, 1)

	suite.shutdownManager.AddShutdownHook(hook2)
	assert.Len(t, suite.shutdownManager.hooks, 2)
}

func (suite *ShutdownManagerTestSuite) TestIsShuttingDown() {
	t := suite.T()

	// Initially should not be shutting down
	assert.False(t, suite.shutdownManager.IsShuttingDown())

	// Mark as shutting down
	suite.shutdownManager.mu.Lock()
	suite.shutdownManager.isShuttingDown = true
	suite.shutdownManager.mu.Unlock()

	assert.True(t, suite.shutdownManager.IsShuttingDown())
}

func (suite *ShutdownManagerTestSuite) TestShutdownWithoutServices() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test shutdown without any services configured
	err := suite.shutdownManager.Shutdown(ctx)
	assert.NoError(t, err)
}

func (suite *ShutdownManagerTestSuite) TestShutdownWithDatabase() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set database
	suite.shutdownManager.SetDatabase(suite.db)

	err := suite.shutdownManager.Shutdown(ctx)
	assert.NoError(t, err)
}

func (suite *ShutdownManagerTestSuite) TestShutdownHooks() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var hookExecuted bool
	var hookOrder []int

	// Add hooks that track execution
	hook1 := func(ctx context.Context) error {
		hookOrder = append(hookOrder, 1)
		return nil
	}
	hook2 := func(ctx context.Context) error {
		hookOrder = append(hookOrder, 2)
		hookExecuted = true
		return nil
	}
	hook3 := func(ctx context.Context) error {
		hookOrder = append(hookOrder, 3)
		return nil
	}

	suite.shutdownManager.AddShutdownHook(hook1)
	suite.shutdownManager.AddShutdownHook(hook2)
	suite.shutdownManager.AddShutdownHook(hook3)

	err := suite.shutdownManager.Shutdown(ctx)
	assert.NoError(t, err)
	assert.True(t, hookExecuted)
	assert.Equal(t, []int{1, 2, 3}, hookOrder)
}

func (suite *ShutdownManagerTestSuite) TestShutdownHooksWithError() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errorHook := func(ctx context.Context) error {
		return fmt.Errorf("test hook error")
	}
	successHook := func(ctx context.Context) error {
		return nil
	}

	suite.shutdownManager.AddShutdownHook(errorHook)
	suite.shutdownManager.AddShutdownHook(successHook)

	err := suite.shutdownManager.Shutdown(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shutdown hooks failed")
}

func (suite *ShutdownManagerTestSuite) TestShutdownTimeout() {
	t := suite.T()

	// Very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Add a hook that takes longer than the timeout
	slowHook := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}

	suite.shutdownManager.AddShutdownHook(slowHook)

	err := suite.shutdownManager.Shutdown(ctx)
	assert.Error(t, err)
}

func (suite *ShutdownManagerTestSuite) TestConcurrentShutdown() {
	t := suite.T()

	var wg sync.WaitGroup
	var shutdownCount int32

	// Test concurrent shutdown calls
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := suite.shutdownManager.Shutdown(ctx)
			if err == nil {
				// Use atomic operations for thread safety in real scenarios
				shutdownCount++
			}
		}()
	}

	wg.Wait()
	// All should succeed since we're not actually starting the server
	assert.True(t, shutdownCount > 0)
}

func (suite *ShutdownManagerTestSuite) TestGetDefaultShutdownManager() {
	t := suite.T()

	sm := GetDefaultShutdownManager(suite.server)
	assert.NotNil(t, sm)
	assert.Equal(t, suite.server, sm.server)
	assert.Equal(t, 30*time.Second, sm.timeout)
	// Should have default hooks
	assert.Greater(t, len(sm.hooks), 0)
}

// Test the provided shutdown hooks
func (suite *ShutdownManagerTestSuite) TestBuiltinShutdownHooks() {
	t := suite.T()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testCases := []struct {
		name string
		hook ShutdownHook
	}{
		{
			name: "HealthcheckShutdownHook",
			hook: HealthcheckShutdownHook(),
		},
		{
			name: "CleanupTempFilesHook",
			hook: CleanupTempFilesHook("/tmp/test"),
		},
		{
			name: "LogRotationShutdownHook",
			hook: LogRotationShutdownHook(),
		},
		{
			name: "MetricsFlushHook",
			hook: MetricsFlushHook(),
		},
		{
			name: "NotifyExternalServicesHook",
			hook: NotifyExternalServicesHook("http://example.com/webhook"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.hook(ctx)
			assert.NoError(t, err)
		})
	}
}

func (suite *ShutdownManagerTestSuite) TestBuiltinHooksWithTimeout() {
	t := suite.T()

	// Very short timeout to test timeout handling
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	cancel() // Cancel immediately

	testCases := []struct {
		name string
		hook ShutdownHook
	}{
		{
			name: "HealthcheckShutdownHook",
			hook: HealthcheckShutdownHook(),
		},
		{
			name: "CleanupTempFilesHook",
			hook: CleanupTempFilesHook("/tmp/test"),
		},
		{
			name: "LogRotationShutdownHook",
			hook: LogRotationShutdownHook(),
		},
		{
			name: "MetricsFlushHook",
			hook: MetricsFlushHook(),
		},
		{
			name: "NotifyExternalServicesHook",
			hook: NotifyExternalServicesHook("http://example.com/webhook"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.hook(ctx)
			// Should return context error due to timeout/cancellation
			assert.Error(t, err)
			assert.Equal(t, context.Canceled, err)
		})
	}
}

// Test logger for testing purposes
type TestLogger struct {
	InfoMessages  []string
	ErrorMessages []string
	WarnMessages  []string
	mu            sync.RWMutex
}

func (l *TestLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.InfoMessages = append(l.InfoMessages, fmt.Sprintf(msg, args...))
}

func (l *TestLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ErrorMessages = append(l.ErrorMessages, fmt.Sprintf(msg, args...))
}

func (l *TestLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.WarnMessages = append(l.WarnMessages, fmt.Sprintf(msg, args...))
}

func (l *TestLogger) GetInfoMessages() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]string{}, l.InfoMessages...)
}

func (l *TestLogger) GetErrorMessages() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]string{}, l.ErrorMessages...)
}

func (l *TestLogger) GetWarnMessages() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]string{}, l.WarnMessages...)
}

func (suite *ShutdownManagerTestSuite) TestShutdownManagerLogging() {
	t := suite.T()

	testLogger := &TestLogger{}
	suite.shutdownManager.SetLogger(testLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add a hook for testing
	suite.shutdownManager.AddShutdownHook(func(ctx context.Context) error {
		return nil
	})

	err := suite.shutdownManager.Shutdown(ctx)
	assert.NoError(t, err)

	// Check that log messages were recorded
	infoMessages := testLogger.GetInfoMessages()
	assert.Greater(t, len(infoMessages), 0)
	
	// Should contain startup message
	assert.Contains(t, infoMessages[0], "Starting graceful shutdown process")
}

// Benchmark tests for shutdown performance
func BenchmarkShutdownManager(b *testing.B) {
	server := echo.New()
	sm := NewShutdownManager(server)
	
	// Add some hooks to simulate real-world scenario
	sm.AddShutdownHook(HealthcheckShutdownHook())
	sm.AddShutdownHook(CleanupTempFilesHook("/tmp"))
	sm.AddShutdownHook(LogRotationShutdownHook())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		sm.Shutdown(ctx)
		cancel()
	}
}

func BenchmarkShutdownHookExecution(b *testing.B) {
	hook := HealthcheckShutdownHook()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hook(ctx)
	}
}

// Integration test that simulates real shutdown scenario
func TestShutdownIntegration(t *testing.T) {
	// This test would be more comprehensive in a real scenario
	// where we actually start the server and test shutdown

	server := echo.New()
	sm := GetDefaultShutdownManager(server)
	
	// Add custom hook
	var customHookExecuted bool
	sm.AddShutdownHook(func(ctx context.Context) error {
		customHookExecuted = true
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := sm.Shutdown(ctx)
	assert.NoError(t, err)
	assert.True(t, customHookExecuted)
}