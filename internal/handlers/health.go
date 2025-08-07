package handlers

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/dbackup/backend-go/internal/database"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Status      string            `json:"status"`
	Timestamp   time.Time         `json:"timestamp"`
	Version     string            `json:"version,omitempty"`
	Uptime      string            `json:"uptime,omitempty"`
	Services    map[string]Health `json:"services,omitempty"`
	Environment string            `json:"environment,omitempty"`
}

// Health represents the health of an individual service
type Health struct {
	Status    string        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Error     string        `json:"error,omitempty"`
	Latency   time.Duration `json:"latency,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

var startTime = time.Now()

// Liveness returns a simple liveness check
// GET /api/health/live
func (h *HealthHandler) Liveness(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now(),
	})
}

// Readiness returns a readiness check with dependencies
// GET /api/health/ready
func (h *HealthHandler) Readiness(c echo.Context) error {
	ctx := c.Request().Context()
	status := &HealthStatus{
		Status:      "healthy",
		Timestamp:   time.Now(),
		Uptime:      time.Since(startTime).String(),
		Environment: getEnvironment(),
		Services:    make(map[string]Health),
	}

	// Check database connectivity
	dbHealth := h.checkDatabase(ctx)
	status.Services["database"] = dbHealth

	// Check Redis connectivity (if available)
	if h.redis != nil {
		redisHealth := h.checkRedis(ctx)
		status.Services["redis"] = redisHealth
	}

	// Determine overall status
	overallHealthy := true
	for _, service := range status.Services {
		if service.Status != "healthy" {
			overallHealthy = false
			break
		}
	}

	if !overallHealthy {
		status.Status = "degraded"
		return c.JSON(http.StatusServiceUnavailable, status)
	}

	return c.JSON(http.StatusOK, status)
}

// Health returns a comprehensive health check
// GET /api/health
func (h *HealthHandler) Health(c echo.Context) error {
	ctx := c.Request().Context()
	status := &HealthStatus{
		Status:      "healthy",
		Timestamp:   time.Now(),
		Version:     getVersion(),
		Uptime:      time.Since(startTime).String(),
		Environment: getEnvironment(),
		Services:    make(map[string]Health),
	}

	// Check all services
	dbHealth := h.checkDatabase(ctx)
	status.Services["database"] = dbHealth

	if h.redis != nil {
		redisHealth := h.checkRedis(ctx)
		status.Services["redis"] = redisHealth
	}

	// Check external dependencies
	s3Health := h.checkS3Connectivity(ctx)
	status.Services["s3"] = s3Health

	// Determine overall status
	overallHealthy := true
	degraded := false

	for serviceName, service := range status.Services {
		switch service.Status {
		case "unhealthy":
			// Critical services
			if serviceName == "database" {
				overallHealthy = false
			} else {
				degraded = true
			}
		case "degraded":
			degraded = true
		}
	}

	if !overallHealthy {
		status.Status = "unhealthy"
		return c.JSON(http.StatusServiceUnavailable, status)
	} else if degraded {
		status.Status = "degraded"
		return c.JSON(http.StatusOK, status)
	}

	return c.JSON(http.StatusOK, status)
}

// checkDatabase checks database connectivity and performance
func (h *HealthHandler) checkDatabase(ctx context.Context) Health {
	start := time.Now()
	health := Health{
		Timestamp: start,
	}

	if h.db == nil {
		health.Status = "unhealthy"
		health.Error = "database connection not configured"
		return health
	}

	// Get underlying SQL DB for ping
	sqlDB, err := h.db.DB()
	if err != nil {
		health.Status = "unhealthy"
		health.Error = "failed to get database connection: " + err.Error()
		health.Latency = time.Since(start)
		return health
	}

	// Create context with timeout
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Ping database
	if err := sqlDB.PingContext(dbCtx); err != nil {
		health.Status = "unhealthy"
		health.Error = "database ping failed: " + err.Error()
		health.Latency = time.Since(start)
		return health
	}

	// Check database stats
	stats := sqlDB.Stats()
	health.Latency = time.Since(start)

	// Evaluate database health based on connection pool
	if stats.MaxOpenConnections > 0 && stats.OpenConnections >= 0 && stats.OpenConnections <= stats.MaxOpenConnections {
		health.Status = "healthy"
		health.Message = "database connection pool healthy"
	} else if stats.MaxOpenConnections == 0 && stats.OpenConnections >= 0 {
		// No max limit configured, consider healthy if we have connections
		health.Status = "healthy"
		health.Message = "database connection pool healthy (no max limit)"
	} else {
		health.Status = "degraded"
		health.Message = "database connection pool may be stressed"
	}

	return health
}

// checkRedis checks Redis connectivity
func (h *HealthHandler) checkRedis(ctx context.Context) Health {
	start := time.Now()
	health := Health{
		Timestamp: start,
	}

	if h.redis == nil {
		health.Status = "unhealthy"
		health.Error = "redis connection not configured"
		return health
	}

	// Create context with timeout
	redisCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Ping Redis
	if err := h.redis.Ping(redisCtx).Err(); err != nil {
		health.Status = "unhealthy"
		health.Error = "redis ping failed: " + err.Error()
		health.Latency = time.Since(start)
		return health
	}

	health.Status = "healthy"
	health.Message = "redis connection healthy"
	health.Latency = time.Since(start)
	return health
}

// checkS3Connectivity checks S3 service availability
func (h *HealthHandler) checkS3Connectivity(ctx context.Context) Health {
	start := time.Now()
	health := Health{
		Timestamp: start,
		Status:    "healthy",
		Message:   "s3 check skipped - configuration dependent",
		Latency:   time.Since(start),
	}

	// Note: S3 health check would depend on storage configuration
	// which is user-specific. For now, we return a neutral status.
	// In a real implementation, you might check if default S3 
	// credentials are configured and test connectivity.

	return health
}

// getVersion returns the application version
func getVersion() string {
	// In a real application, this would be injected at build time
	// via ldflags: go build -ldflags "-X main.version=1.0.0"
	return "1.0.0-dev"
}

// getEnvironment returns the current environment
func getEnvironment() string {
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = "development"
	}
	return env
}

// RegisterRoutes registers health check routes
func (h *HealthHandler) RegisterRoutes(e *echo.Echo) {
	health := e.Group("/api/health")
	
	// Kubernetes-style probes
	health.GET("/live", h.Liveness)   // Liveness probe
	health.GET("/ready", h.Readiness) // Readiness probe
	
	// Comprehensive health check
	health.GET("", h.Health)          // Detailed health status
	health.GET("/", h.Health)         // Alternative path
}

// Legacy health check for backward compatibility
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
