package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test
	err := HealthCheck(c)

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

	// Verify timestamp is a valid number
	timestamp, ok := response["timestamp"].(float64)
	assert.True(t, ok)
	assert.Greater(t, timestamp, float64(0))
}