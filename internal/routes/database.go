package routes

import (
	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/handlers"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// SetupDatabaseRoutes sets up database connection management routes
func SetupDatabaseRoutes(e *echo.Echo, db *gorm.DB, jm *auth.JWTManager, encService *encryption.Service) {
	// Create database handler
	dbHandler := handlers.NewDatabaseHandler(db, encService)

	// Database routes group with authentication required (cookie-based)
	dbGroup := e.Group("/api/databases", middleware.CookieJWT(jm))

	// CRUD operations for database connections
	dbGroup.GET("", dbHandler.ListDatabaseConnections)
	dbGroup.POST("", dbHandler.CreateDatabaseConnection)
	dbGroup.GET("/:uid", dbHandler.GetDatabaseConnection)
	dbGroup.PUT("/:uid", dbHandler.UpdateDatabaseConnection)
	dbGroup.DELETE("/:uid", dbHandler.DeleteDatabaseConnection)

	// Database connection operations
	dbGroup.POST("/:uid/test", dbHandler.TestDatabaseConnection)
	dbGroup.POST("/:uid/discover", dbHandler.DiscoverTables)

	// Database statistics (public endpoint for health checks)
	e.GET("/api/stats/database", handlers.DatabaseStats)
}