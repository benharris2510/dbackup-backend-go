package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/config"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/handlers"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/dbackup/backend-go/internal/routes"
	"github.com/dbackup/backend-go/internal/validation"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	if err := database.Initialize(cfg); err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			fmt.Printf("Error closing database: %v\n", err)
		}
	}()

	e := echo.New()

	// Hide Echo banner
	e.HideBanner = true

	// Configure Echo
	e.Debug = cfg.IsDevelopment()

	// Set up validator
	e.Validator = validation.NewValidator()

	// Initialize auth components
	jwtManager := auth.NewJWTManager(
		cfg.JWT.SecretKey,
		cfg.JWT.AccessTokenExpires,
		cfg.JWT.RefreshTokenExpires,
	)
	passwordHasher := auth.NewPasswordHasher()
	totpManager := auth.NewTOTPManager("dbackup")

	// Initialize encryption service
	encryptionService := encryption.NewService(cfg.Encryption.MasterKey)

	// Setup middleware
	setupMiddleware(e, cfg)

	// Setup routes
	setupRoutes(e, jwtManager, passwordHasher, totpManager, encryptionService)

	// Start server with graceful shutdown
	go func() {
		addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func setupMiddleware(e *echo.Echo, cfg *config.Config) {
	// Logger middleware
	e.Use(echoMiddleware.LoggerWithConfig(echoMiddleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"error":"${error}","latency_human":"${latency_human}","bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}))

	// Recover middleware
	e.Use(echoMiddleware.Recover())

	// Request ID middleware
	e.Use(echoMiddleware.RequestID())

	// Timeout middleware
	e.Use(echoMiddleware.TimeoutWithConfig(echoMiddleware.TimeoutConfig{
		Timeout: 30 * time.Second,
	}))

	// CORS middleware
	e.Use(middleware.CORS())

	// Security headers middleware
	e.Use(middleware.SecurityHeaders())
}

func setupRoutes(e *echo.Echo, jm *auth.JWTManager, ph *auth.PasswordHasher, tm *auth.TOTPManager, encService *encryption.Service) {
	// Health check
	e.GET("/health", handlers.HealthCheck)

	// API group
	api := e.Group("/api")

	// API routes will be added here
	api.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "dbackup API v1.0",
			"status":  "operational",
		})
	})

	// Database stats endpoint (for monitoring)
	api.GET("/db/stats", handlers.DatabaseStats)

	// Setup authentication routes
	routes.SetupAuthRoutes(e, jm, ph, tm)

	// Setup database routes
	db := database.GetDB()
	routes.SetupDatabaseRoutes(e, db, jm, encService)
}