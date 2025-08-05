package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// HealthCheck handles the health check endpoint
func HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "dbackup-api",
		"version":   "1.0.0",
	})
}