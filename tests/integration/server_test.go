package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/handlers"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Debug = true

	// Setup middleware
	e.Use(echoMiddleware.LoggerWithConfig(echoMiddleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"error":"${error}","latency_human":"${latency_human}","bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02T15:04:05.000Z07:00",
		Output: os.Stdout,
	}))
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.RequestID())
	e.Use(echoMiddleware.TimeoutWithConfig(echoMiddleware.TimeoutConfig{
		Timeout: 30 * time.Second,
	}))
	e.Use(middleware.CORS())
	e.Use(middleware.SecurityHeaders())

	// Setup routes
	e.GET("/health", handlers.HealthCheck)
	api := e.Group("/api")
	api.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "dbackup API v1.0",
			"status":  "operational",
		})
	})

	return e
}

func TestServerSetup(t *testing.T) {
	// Set up test environment
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	server := setupTestServer()

	t.Run("Health endpoint returns correct response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "dbackup-api", response["service"])
	})

	t.Run("API root endpoint returns correct response", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/", nil)
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "dbackup API v1.0", response["message"])
		assert.Equal(t, "operational", response["status"])
	})

	t.Run("CORS headers are set correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Security headers are set correctly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"))
		assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'self'")
	})

	t.Run("Request ID is generated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.NotEmpty(t, rec.Header().Get("X-Request-Id"))
	})

	t.Run("404 for unknown routes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("OPTIONS request for CORS preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		
		server.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestServerMiddlewareOrder(t *testing.T) {
	server := setupTestServer()

	// Test that panic recovery works
	server.GET("/panic", func(c echo.Context) error {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	
	// Should not panic due to recover middleware
	assert.NotPanics(t, func() {
		server.ServeHTTP(rec, req)
	})
	
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}