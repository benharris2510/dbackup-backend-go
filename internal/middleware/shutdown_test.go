package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dbackup/backend-go/internal/server"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownMiddleware(t *testing.T) {
	e := echo.New()
	shutdownManager := server.NewShutdownManager(e)

	middleware := ShutdownMiddleware(shutdownManager)

	t.Run("Normal Operation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		nextCalled := false
		next := func(c echo.Context) error {
			nextCalled = true
			return c.String(http.StatusOK, "success")
		}

		err := middleware(next)(c)
		assert.NoError(t, err)
		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "success", rec.Body.String())
	})

	t.Run("During Shutdown", func(t *testing.T) {
		// Mark as shutting down
		shutdownManager.mu.Lock()
		shutdownManager.isShuttingDown = true
		shutdownManager.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		nextCalled := false
		next := func(c echo.Context) error {
			nextCalled = true
			return c.String(http.StatusOK, "success")
		}

		err := middleware(next)(c)
		assert.NoError(t, err)
		assert.False(t, nextCalled) // Next should not be called
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "Service is shutting down")
	})
}

func TestHealthCheckShutdownMiddleware(t *testing.T) {
	e := echo.New()
	shutdownManager := server.NewShutdownManager(e)

	middleware := HealthCheckShutdownMiddleware(shutdownManager)

	t.Run("Health Check - Normal Operation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/health")

		nextCalled := false
		next := func(c echo.Context) error {
			nextCalled = true
			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":  "healthy",
				"healthy": true,
			})
		}

		err := middleware(next)(c)
		assert.NoError(t, err)
		assert.True(t, nextCalled)
	})

	t.Run("Health Check - During Shutdown", func(t *testing.T) {
		// Mark as shutting down
		shutdownManager.mu.Lock()
		shutdownManager.isShuttingDown = true
		shutdownManager.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/health")

		nextCalled := false
		next := func(c echo.Context) error {
			nextCalled = true
			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":  "healthy",
				"healthy": true,
			})
		}

		err := middleware(next)(c)
		assert.NoError(t, err)
		assert.False(t, nextCalled) // Next should not be called
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "shutting_down")
	})

	t.Run("API Health Check - During Shutdown", func(t *testing.T) {
		// Mark as shutting down
		shutdownManager.mu.Lock()
		shutdownManager.isShuttingDown = true
		shutdownManager.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/api/health")

		nextCalled := false
		next := func(c echo.Context) error {
			nextCalled = true
			return c.JSON(http.StatusOK, map[string]interface{}{
				"status":  "healthy",
				"healthy": true,
			})
		}

		err := middleware(next)(c)
		assert.NoError(t, err)
		assert.False(t, nextCalled) // Next should not be called
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("Non-Health Endpoint - Not Affected During Shutdown", func(t *testing.T) {
		// Mark as shutting down
		shutdownManager.mu.Lock()
		shutdownManager.isShuttingDown = true
		shutdownManager.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath("/api/users")

		nextCalled := false
		next := func(c echo.Context) error {
			nextCalled = true
			return c.String(http.StatusOK, "users data")
		}

		// Health check middleware should pass through non-health endpoints
		err := middleware(next)(c)
		assert.NoError(t, err)
		assert.True(t, nextCalled) // Next should be called
	})
}

func TestShutdownMiddlewareIntegration(t *testing.T) {
	e := echo.New()
	shutdownManager := server.NewShutdownManager(e)

	// Setup both middleware in order
	e.Use(HealthCheckShutdownMiddleware(shutdownManager))
	e.Use(ShutdownMiddleware(shutdownManager))

	// Add routes
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "healthy",
			"healthy": true,
		})
	})

	e.GET("/api/data", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"data": "test"})
	})

	t.Run("Normal Operation - Health Check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "healthy")
	})

	t.Run("Normal Operation - API Endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "test")
	})

	t.Run("During Shutdown - Health Check", func(t *testing.T) {
		// Mark as shutting down
		shutdownManager.mu.Lock()
		shutdownManager.isShuttingDown = true
		shutdownManager.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "shutting_down")
	})

	t.Run("During Shutdown - API Endpoint", func(t *testing.T) {
		// Mark as shutting down (should already be set from previous test)
		assert.True(t, shutdownManager.IsShuttingDown())

		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "Service is shutting down")
	})
}

func BenchmarkShutdownMiddleware(b *testing.B) {
	e := echo.New()
	shutdownManager := server.NewShutdownManager(e)
	middleware := ShutdownMiddleware(shutdownManager)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	next := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset recorder for each iteration
		rec.Body.Reset()
		rec.Code = 0

		middleware(next)(c)
	}
}

func BenchmarkHealthCheckShutdownMiddleware(b *testing.B) {
	e := echo.New()
	shutdownManager := server.NewShutdownManager(e)
	middleware := HealthCheckShutdownMiddleware(shutdownManager)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/health")

	next := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "healthy",
			"healthy": true,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset recorder for each iteration
		rec.Body.Reset()
		rec.Code = 0

		middleware(next)(c)
	}
}

// Test concurrent access to shutdown status
func TestShutdownMiddlewareConcurrency(t *testing.T) {
	e := echo.New()
	shutdownManager := server.NewShutdownManager(e)
	middleware := ShutdownMiddleware(shutdownManager)

	// Run multiple goroutines to test concurrent access
	concurrency := 10
	results := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			next := func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			}

			err := middleware(next)(c)
			results <- (err == nil && rec.Code == http.StatusOK)
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			successCount++
		}
	}

	// All should succeed when not shutting down
	assert.Equal(t, concurrency, successCount)

	// Now mark as shutting down and test again
	shutdownManager.mu.Lock()
	shutdownManager.isShuttingDown = true
	shutdownManager.mu.Unlock()

	for i := 0; i < concurrency; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			next := func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			}

			err := middleware(next)(c)
			results <- (err == nil && rec.Code == http.StatusServiceUnavailable)
		}()
	}

	// Collect results for shutdown scenario
	shutdownCount := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			shutdownCount++
		}
	}

	// All should return service unavailable when shutting down
	assert.Equal(t, concurrency, shutdownCount)
}