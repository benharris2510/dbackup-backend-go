package handlers

import (
	"net/http"

	"github.com/dbackup/backend-go/internal/database"
	"github.com/labstack/echo/v4"
)

// DatabaseStats returns database connection statistics
func DatabaseStats(c echo.Context) error {
	stats, err := database.GetStats()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to get database statistics",
			"details": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   stats,
	})
}