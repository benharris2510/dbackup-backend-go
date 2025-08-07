package middleware

import (
	"net/http"

	"github.com/dbackup/backend-go/internal/server"
	"github.com/labstack/echo/v4"
)

// ShutdownMiddleware returns a middleware that rejects requests during shutdown
func ShutdownMiddleware(shutdownManager *server.ShutdownManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if shutdownManager.IsShuttingDown() {
				return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
					"error": "Service is shutting down",
					"code":  "SERVICE_UNAVAILABLE",
				})
			}
			return next(c)
		}
	}
}

// HealthCheckShutdownMiddleware returns a middleware that modifies health check during shutdown
func HealthCheckShutdownMiddleware(shutdownManager *server.ShutdownManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Only apply to health check endpoints
			if c.Path() != "/health" && c.Path() != "/api/health" {
				return next(c)
			}

			if shutdownManager.IsShuttingDown() {
				return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
					"status":  "shutting_down",
					"healthy": false,
					"message": "Service is gracefully shutting down",
				})
			}
			return next(c)
		}
	}
}