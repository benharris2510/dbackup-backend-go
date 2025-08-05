package handlers

import (
	"net/http"
	"time"

	"github.com/dbackup/backend-go/internal/database"
	"github.com/labstack/echo/v4"
)

// HealthCheck handles the health check endpoint
func HealthCheck(c echo.Context) error {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "dbackup-api",
		"version":   "1.0.0",
		"checks":    make(map[string]interface{}),
	}

	checks := response["checks"].(map[string]interface{})

	// Database health check
	if err := database.HealthCheck(); err != nil {
		checks["database"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
		response["status"] = "unhealthy"
		return c.JSON(http.StatusServiceUnavailable, response)
	} else {
		checks["database"] = map[string]interface{}{
			"status":    "healthy",
			"connected": database.IsConnected(),
		}
	}

	return c.JSON(http.StatusOK, response)
}
