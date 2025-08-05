package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dbackup/backend-go/internal/handlers"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()

	// Hide Echo banner
	e.HideBanner = true

	// Configure Echo
	e.Debug = os.Getenv("ENV") == "development"

	// Setup middleware
	setupMiddleware(e)

	// Setup routes
	setupRoutes(e)

	// Start server
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	// Start server with graceful shutdown
	go func() {
		if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
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

func setupMiddleware(e *echo.Echo) {
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

func setupRoutes(e *echo.Echo) {
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
}