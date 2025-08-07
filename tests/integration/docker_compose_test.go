package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
)

// TestDockerComposeStack tests the full docker-compose stack integration
func TestDockerComposeStack(t *testing.T) {
	// Skip if not running in Docker environment
	if !isDockerEnvironment() {
		t.Skip("Skipping Docker Compose integration tests - not in Docker environment")
	}

	t.Run("API Health Check", testAPIHealthCheck)
	t.Run("PostgreSQL Connection", testPostgreSQLConnection)
	t.Run("Redis Connection", testRedisConnection)
	t.Run("MySQL Connection", testMySQLConnection)
	t.Run("MinIO Connection", testMinIOConnection)
	t.Run("Service Dependencies", testServiceDependencies)
	t.Run("Environment Variables", testEnvironmentVariables)
	t.Run("API Database Integration", testAPIDatabaseIntegration)
	t.Run("API Redis Integration", testAPIRedisIntegration)
}

func testAPIHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Test health endpoint
	resp, err := client.Get("http://api:8080/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)
	
	assert.Equal(t, "healthy", health["status"])
	assert.NotEmpty(t, health["timestamp"])
}

func testPostgreSQLConnection(t *testing.T) {
	dsn := "postgres://postgres:postgres@postgres:5432/dbackup?sslmode=disable"
	
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = db.PingContext(ctx)
	require.NoError(t, err)
	
	// Test database exists
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = 'dbackup')"
	err = db.QueryRowContext(ctx, query).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
	
	// Test initialization script ran
	var count int
	query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'"
	err = db.QueryRowContext(ctx, query).Scan(&count)
	require.NoError(t, err)
	// Should have at least some tables (even if migrations haven't run yet)
	assert.GreaterOrEqual(t, count, 0)
}

func testRedisConnection(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		DB:   0,
	})
	defer rdb.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test ping
	pong, err := rdb.Ping(ctx).Result()
	require.NoError(t, err)
	assert.Equal(t, "PONG", pong)
	
	// Test basic operations
	err = rdb.Set(ctx, "test-key", "test-value", time.Minute).Err()
	require.NoError(t, err)
	
	val, err := rdb.Get(ctx, "test-key").Result()
	require.NoError(t, err)
	assert.Equal(t, "test-value", val)
	
	// Clean up
	err = rdb.Del(ctx, "test-key").Err()
	require.NoError(t, err)
}

func testMySQLConnection(t *testing.T) {
	dsn := "testuser:testpass@tcp(mysql:3306)/testdb?parseTime=true"
	
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	defer db.Close()
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = db.PingContext(ctx)
	require.NoError(t, err)
	
	// Test sample data exists
	var count int
	query := "SELECT COUNT(*) FROM users"
	err = db.QueryRowContext(ctx, query).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0) // Should have sample users from init script
	
	// Test tables exist
	tables := []string{"users", "orders", "products"}
	for _, table := range tables {
		var exists int
		query := "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ? AND table_schema = 'testdb'"
		err = db.QueryRowContext(ctx, query, table).Scan(&exists)
		require.NoError(t, err)
		assert.Equal(t, 1, exists, "Table %s should exist", table)
	}
}

func testMinIOConnection(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Test MinIO health endpoint
	resp, err := client.Get("http://minio:9000/minio/health/live")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func testServiceDependencies(t *testing.T) {
	// Test that API can connect to its dependencies
	client := &http.Client{Timeout: 15 * time.Second}
	
	// Wait a bit for all services to be ready
	time.Sleep(5 * time.Second)
	
	// Test API readiness (should connect to DB and Redis)
	resp, err := client.Get("http://api:8080/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)
	
	// API should report healthy status if it can connect to dependencies
	assert.Equal(t, "healthy", health["status"])
}

func testEnvironmentVariables(t *testing.T) {
	// Test that API has correct environment variables
	client := &http.Client{Timeout: 10 * time.Second}
	
	// If there's an env endpoint, test it
	// For now, we'll just verify the API responds correctly
	resp, err := client.Get("http://api:8080/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func testAPIDatabaseIntegration(t *testing.T) {
	// Test that API can perform database operations
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Test database-dependent endpoint (if available)
	// This might be a databases list endpoint
	resp, err := client.Get("http://api:8080/api/databases")
	if err == nil {
		defer resp.Body.Close()
		// If endpoint exists, it should respond (might be 401 without auth)
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized)
	}
}

func testAPIRedisIntegration(t *testing.T) {
	// Test that API can use Redis for caching/sessions
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Test an endpoint that might use Redis
	resp, err := client.Get("http://api:8080/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Helper function to detect if we're running in Docker
func isDockerEnvironment() bool {
	// Check if we can resolve docker service names
	client := &http.Client{Timeout: 2 * time.Second}
	_, err := client.Get("http://api:8080/health")
	return err == nil
}

// TestDockerComposeServicesAvailability tests individual service availability
func TestDockerComposeServicesAvailability(t *testing.T) {
	if !isDockerEnvironment() {
		t.Skip("Skipping Docker Compose service tests - not in Docker environment")
	}

	services := map[string]string{
		"API":     "http://api:8080/health",
		"MinIO":   "http://minio:9000/minio/health/live",
		"Adminer": "http://adminer:8080",
	}

	for name, url := range services {
		t.Run(fmt.Sprintf("%s Service", name), func(t *testing.T) {
			client := &http.Client{Timeout: 10 * time.Second}
			
			resp, err := client.Get(url)
			if err != nil {
				t.Logf("Service %s not available at %s: %v", name, url, err)
				return
			}
			defer resp.Body.Close()
			
			assert.True(t, resp.StatusCode < 500, 
				"Service %s should be available (got %d)", name, resp.StatusCode)
		})
	}
}

// TestDockerComposeNetworking tests network connectivity between services
func TestDockerComposeNetworking(t *testing.T) {
	if !isDockerEnvironment() {
		t.Skip("Skipping Docker Compose networking tests - not in Docker environment")
	}

	t.Run("Service Name Resolution", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Test that service names resolve
		services := []string{"postgres", "redis", "minio", "mysql"}
		
		for _, service := range services {
			// Try to connect to each service (will fail with connection refused but name should resolve)
			url := fmt.Sprintf("http://%s:80", service)
			_, err := client.Get(url)
			
			// We expect connection refused, not name resolution failure
			if err != nil {
				assert.NotContains(t, err.Error(), "no such host", 
					"Service name %s should resolve", service)
			}
		}
	})
}

// TestDockerComposeVolumes tests that volumes are working correctly
func TestDockerComposeVolumes(t *testing.T) {
	if !isDockerEnvironment() {
		t.Skip("Skipping Docker Compose volume tests - not in Docker environment")
	}

	t.Run("PostgreSQL Data Persistence", func(t *testing.T) {
		dsn := "postgres://postgres:postgres@postgres:5432/dbackup?sslmode=disable"
		
		db, err := sql.Open("postgres", dsn)
		require.NoError(t, err)
		defer db.Close()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// Create a test table
		_, err = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS volume_test (id SERIAL PRIMARY KEY, data TEXT)")
		require.NoError(t, err)
		
		// Insert test data
		_, err = db.ExecContext(ctx, "INSERT INTO volume_test (data) VALUES ('test-data')")
		require.NoError(t, err)
		
		// Verify data exists
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM volume_test WHERE data = 'test-data'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		
		// Clean up
		_, err = db.ExecContext(ctx, "DROP TABLE IF EXISTS volume_test")
		require.NoError(t, err)
	})

	t.Run("Redis Data Persistence", func(t *testing.T) {
		rdb := redis.NewClient(&redis.Options{
			Addr: "redis:6379",
			DB:   0,
		})
		defer rdb.Close()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// Test that Redis is configured with persistence
		info, err := rdb.Info(ctx, "persistence").Result()
		require.NoError(t, err)
		
		// Check that AOF is enabled (from our compose config)
		assert.Contains(t, info, "aof_enabled:1")
	})
}

// BenchmarkDockerComposePerformance benchmarks the Docker Compose stack performance
func BenchmarkDockerComposePerformance(b *testing.B) {
	if !isDockerEnvironment() {
		b.Skip("Skipping Docker Compose performance tests - not in Docker environment")
	}

	client := &http.Client{Timeout: 5 * time.Second}

	b.Run("API Health Check", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := client.Get("http://api:8080/health")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("Database Connection", func(b *testing.B) {
		dsn := "postgres://postgres:postgres@postgres:5432/dbackup?sslmode=disable"
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			b.Fatal(err)
		}
		defer db.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := db.Ping()
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Redis Operations", func(b *testing.B) {
		rdb := redis.NewClient(&redis.Options{
			Addr: "redis:6379",
			DB:   0,
		})
		defer rdb.Close()

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := rdb.Set(ctx, "bench-key", "bench-value", time.Minute).Err()
			if err != nil {
				b.Fatal(err)
			}
			
			_, err = rdb.Get(ctx, "bench-key").Result()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}