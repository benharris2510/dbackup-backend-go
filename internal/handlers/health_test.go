package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/config"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	t.Run("health check without database", func(t *testing.T) {
		// Setup without database initialization
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test
		err := HealthCheck(c)

		// Assertions - should return unhealthy due to no database
		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		// Parse response
		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify response fields
		assert.Equal(t, "unhealthy", response["status"])
		assert.Equal(t, "dbackup-api", response["service"])
		assert.Equal(t, "1.0.0", response["version"])
		assert.NotNil(t, response["timestamp"])

		// Verify checks section exists and database is unhealthy
		checks, ok := response["checks"].(map[string]interface{})
		assert.True(t, ok)
		assert.NotNil(t, checks)

		dbCheck, ok := checks["database"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "unhealthy", dbCheck["status"])
		assert.NotNil(t, dbCheck["error"])

		// Verify timestamp is a valid number
		timestamp, ok := response["timestamp"].(float64)
		assert.True(t, ok)
		assert.Greater(t, timestamp, float64(0))
	})

	t.Run("health check with database", func(t *testing.T) {
		// Skip if we don't have a test database
		cfg := &config.Config{
			Server: config.ServerConfig{
				Env: "test",
			},
			Database: config.DatabaseConfig{
				URL:                   "postgres://postgres:postgres@localhost:5432/dbackup_test",
				MaxConnections:        10,
				MaxIdleConnections:    5,
				ConnectionMaxLifetime: 30 * time.Minute,
			},
		}

		err := database.Initialize(cfg)
		if err != nil {
			t.Skipf("Skipping test due to database connection error: %v", err)
		}
		defer database.Close()

		// Setup
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test
		err = HealthCheck(c)

		// Assertions
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse response
		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify response fields
		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "dbackup-api", response["service"])
		assert.Equal(t, "1.0.0", response["version"])
		assert.NotNil(t, response["timestamp"])

		// Verify checks section exists and database is healthy
		checks, ok := response["checks"].(map[string]interface{})
		assert.True(t, ok)
		assert.NotNil(t, checks)

		dbCheck, ok := checks["database"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "healthy", dbCheck["status"])
		assert.Equal(t, true, dbCheck["connected"])

		// Verify timestamp is a valid number
		timestamp, ok := response["timestamp"].(float64)
		assert.True(t, ok)
		assert.Greater(t, timestamp, float64(0))
	})
}
