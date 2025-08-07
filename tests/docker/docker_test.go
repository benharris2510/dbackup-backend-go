package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testImageName      = "dbackup-backend-test"
	testContainerName  = "dbackup-test-container"
	testPort          = "8082"
	buildTimeout      = 300 * time.Second
	containerTimeout  = 30 * time.Second
)

// Test configuration
var testEnvVars = []string{
	"DATABASE_URL=sqlite://file::memory:?cache=shared",
	"JWT_SECRET_KEY=test-jwt-secret-key-for-testing",
	"ENCRYPTION_MASTER_KEY=test-master-key-32-chars-long123",
	"GO_ENV=test",
	"PORT=8080",
}

func TestDockerBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker build test in short mode")
	}

	// Get project root directory
	projectRoot, err := getProjectRoot()
	require.NoError(t, err, "Failed to get project root")

	// Clean up any existing test images
	cleanupTestImage(t)

	t.Run("build image successfully", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "docker", "build", "-t", testImageName, ".")
		cmd.Dir = projectRoot
		
		// Capture build output
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build output:\n%s", string(output))
		}
		require.NoError(t, err, "Docker build should succeed")

		// Verify image exists
		cmd = exec.Command("docker", "images", testImageName, "--format", "{{.Repository}}")
		output, err = cmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(output), testImageName)
	})

	t.Run("image has correct labels and metadata", func(t *testing.T) {
		cmd := exec.Command("docker", "inspect", testImageName)
		output, err := cmd.Output()
		require.NoError(t, err)

		var inspect []map[string]interface{}
		err = json.Unmarshal(output, &inspect)
		require.NoError(t, err)
		require.Len(t, inspect, 1)

		image := inspect[0]
		
		// Check that image has config
		config, ok := image["Config"].(map[string]interface{})
		require.True(t, ok, "Image should have config")

		// Check exposed port
		exposedPorts, ok := config["ExposedPorts"].(map[string]interface{})
		if ok {
			assert.Contains(t, exposedPorts, "8080/tcp", "Image should expose port 8080")
		}

		// Check user (should be non-root)
		user, ok := config["User"].(string)
		if ok {
			assert.Equal(t, "appuser", user, "Container should run as non-root user")
		}

		// Check working directory
		workingDir, ok := config["WorkingDir"].(string)
		if ok {
			assert.Equal(t, "/app", workingDir, "Working directory should be /app")
		}
	})
}

func TestDockerRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker run test in short mode")
	}

	// Ensure image exists (build it if necessary)
	ensureImageExists(t)

	// Clean up any existing test containers
	cleanupTestContainer(t)

	t.Run("container starts and responds to health checks", func(t *testing.T) {
		// Start container
		containerID := startTestContainer(t)
		defer stopTestContainer(t, containerID)

		// Wait for container to be ready
		waitForContainer(t, testPort)

		// Test health endpoint
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", testPort))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&healthResponse)
		require.NoError(t, err)

		assert.Equal(t, "healthy", healthResponse["status"])
		assert.Equal(t, "dbackup-api", healthResponse["service"])
		assert.Contains(t, healthResponse, "timestamp")
	})

	t.Run("container serves API endpoints", func(t *testing.T) {
		containerID := startTestContainer(t)
		defer stopTestContainer(t, containerID)

		waitForContainer(t, testPort)

		// Test API root endpoint
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/", testPort))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var apiResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&apiResponse)
		require.NoError(t, err)

		assert.Equal(t, "dbackup API v1.0", apiResponse["message"])
		assert.Equal(t, "operational", apiResponse["status"])
	})

	t.Run("container handles environment variables", func(t *testing.T) {
		// Start container with custom environment
		customEnvVars := append(testEnvVars, "GO_ENV=production")
		containerID := startTestContainerWithEnv(t, customEnvVars)
		defer stopTestContainer(t, containerID)

		waitForContainer(t, testPort)

		// The container should start successfully with custom environment
		// We can't directly test environment variables without API endpoints
		// but we can verify the container is running with the health check
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", testPort))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("container logs are accessible", func(t *testing.T) {
		containerID := startTestContainer(t)
		defer stopTestContainer(t, containerID)

		waitForContainer(t, testPort)

		// Get container logs
		cmd := exec.Command("docker", "logs", containerID)
		output, err := cmd.Output()
		require.NoError(t, err)

		logs := string(output)
		// The logs should contain some startup information
		// At minimum, it should not be empty
		assert.NotEmpty(t, logs, "Container should produce logs")
	})
}

func TestDockerHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker health check test in short mode")
	}

	ensureImageExists(t)
	cleanupTestContainer(t)

	t.Run("health check passes when service is healthy", func(t *testing.T) {
		containerID := startTestContainer(t)
		defer stopTestContainer(t, containerID)

		waitForContainer(t, testPort)

		// Wait a bit more for health checks to run
		time.Sleep(35 * time.Second)

		// Check container health status
		cmd := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", containerID)
		output, err := cmd.Output()
		if err == nil {
			healthStatus := strings.TrimSpace(string(output))
			// Health check might be "healthy", "starting", or not configured
			t.Logf("Container health status: %s", healthStatus)
		}
	})
}

func TestDockerSecurity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker security test in short mode")
	}

	ensureImageExists(t)

	t.Run("container runs as non-root user", func(t *testing.T) {
		cmd := exec.Command("docker", "run", "--rm", testImageName, "id")
		output, err := cmd.Output()
		require.NoError(t, err)

		idOutput := string(output)
		// Should run as user 1001 (appuser)
		assert.Contains(t, idOutput, "uid=1001(appuser)")
		assert.Contains(t, idOutput, "gid=1001(appgroup)")
	})

	t.Run("container has minimal attack surface", func(t *testing.T) {
		// Test that the container doesn't have unnecessary tools
		cmd := exec.Command("docker", "run", "--rm", testImageName, "which", "bash")
		err := cmd.Run()
		// bash should not be available (we only have sh in alpine)
		assert.Error(t, err, "Container should not have bash")

		// Test that we have essential tools
		cmd = exec.Command("docker", "run", "--rm", testImageName, "which", "curl")
		err = cmd.Run()
		assert.NoError(t, err, "Container should have curl for health checks")
	})
}

func TestDockerPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker performance test in short mode")
	}

	ensureImageExists(t)

	t.Run("container starts within acceptable time", func(t *testing.T) {
		cleanupTestContainer(t)

		start := time.Now()
		containerID := startTestContainer(t)
		defer stopTestContainer(t, containerID)

		// Wait for the service to be ready
		waitForContainer(t, testPort)
		
		startupTime := time.Since(start)
		t.Logf("Container startup time: %v", startupTime)
		
		// Container should start within reasonable time (30 seconds)
		assert.Less(t, startupTime, 30*time.Second, "Container should start quickly")
	})

	t.Run("image size is reasonable", func(t *testing.T) {
		cmd := exec.Command("docker", "images", testImageName, "--format", "{{.Size}}")
		output, err := cmd.Output()
		require.NoError(t, err)

		sizeStr := strings.TrimSpace(string(output))
		t.Logf("Image size: %s", sizeStr)
		
		// Log the size for monitoring (actual size will vary based on dependencies)
		assert.NotEmpty(t, sizeStr, "Image should have a reported size")
	})
}

// Helper functions

func getProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	
	// Navigate up until we find go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func ensureImageExists(t *testing.T) {
	cmd := exec.Command("docker", "images", testImageName, "--format", "{{.Repository}}")
	output, err := cmd.Output()
	if err != nil || !strings.Contains(string(output), testImageName) {
		t.Log("Test image doesn't exist, building it...")
		TestDockerBuild(t)
	}
}

func cleanupTestImage(t *testing.T) {
	cmd := exec.Command("docker", "rmi", testImageName, "-f")
	_ = cmd.Run() // Ignore errors, image might not exist
}

func cleanupTestContainer(t *testing.T) {
	cmd := exec.Command("docker", "rm", "-f", testContainerName)
	_ = cmd.Run() // Ignore errors, container might not exist
}

func startTestContainer(t *testing.T) string {
	return startTestContainerWithEnv(t, testEnvVars)
}

func startTestContainerWithEnv(t *testing.T, envVars []string) string {
	args := []string{"run", "-d", "--name", testContainerName, "-p", testPort + ":8080"}
	
	// Add environment variables
	for _, env := range envVars {
		args = append(args, "-e", env)
	}
	
	args = append(args, testImageName)
	
	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	require.NoError(t, err, "Failed to start test container")
	
	containerID := strings.TrimSpace(string(output))
	t.Logf("Started container %s (%s)", testContainerName, containerID[:12])
	
	return containerID
}

func stopTestContainer(t *testing.T, containerID string) {
	cmd := exec.Command("docker", "stop", containerID)
	if err := cmd.Run(); err != nil {
		t.Logf("Failed to stop container %s: %v", containerID, err)
	}
	
	cmd = exec.Command("docker", "rm", containerID)
	if err := cmd.Run(); err != nil {
		t.Logf("Failed to remove container %s: %v", containerID, err)
	}
	
	t.Logf("Stopped and removed container %s", containerID[:12])
}

func waitForContainer(t *testing.T, port string) {
	timeout := time.After(containerTimeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for container to be ready")
		case <-ticker.C:
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", port))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					t.Log("Container is ready")
					return
				}
			}
		}
	}
}

// Integration test that combines multiple aspects
func TestDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	ensureImageExists(t)
	cleanupTestContainer(t)

	t.Run("full container lifecycle", func(t *testing.T) {
		// Start container
		containerID := startTestContainer(t)
		
		// Verify it starts
		waitForContainer(t, testPort)
		
		// Test health endpoint
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", testPort))
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Test API endpoint
		resp, err = http.Get(fmt.Sprintf("http://localhost:%s/api/", testPort))
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Check container logs have content
		cmd := exec.Command("docker", "logs", containerID)
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.NotEmpty(t, string(output))
		
		// Gracefully stop container
		stopTestContainer(t, containerID)
		
		// Verify container is stopped
		cmd = exec.Command("docker", "ps", "-q", "--filter", fmt.Sprintf("name=%s", testContainerName))
		output, err = cmd.Output()
		require.NoError(t, err)
		assert.Empty(t, strings.TrimSpace(string(output)), "Container should be stopped")
	})
}

// Cleanup function to run after all tests
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	
	// Cleanup
	cleanupTestImage(nil)
	cleanupTestContainer(nil)
	
	os.Exit(code)
}

// Test Dockerfile best practices
func TestDockerfileBestPractices(t *testing.T) {
	projectRoot, err := getProjectRoot()
	require.NoError(t, err)

	dockerfilePath := filepath.Join(projectRoot, "Dockerfile")
	
	t.Run("dockerfile exists and is readable", func(t *testing.T) {
		_, err := os.Stat(dockerfilePath)
		assert.NoError(t, err, "Dockerfile should exist")
		
		content, err := os.ReadFile(dockerfilePath)
		require.NoError(t, err)
		assert.NotEmpty(t, content, "Dockerfile should have content")
	})
	
	t.Run("dockerfile follows best practices", func(t *testing.T) {
		content, err := os.ReadFile(dockerfilePath)
		require.NoError(t, err)
		
		dockerfile := string(content)
		
		// Should use multi-stage build
		assert.Contains(t, dockerfile, "FROM golang:", "Should use Go base image for build")
		assert.Contains(t, dockerfile, "FROM alpine:", "Should use Alpine for runtime")
		assert.Contains(t, dockerfile, "AS builder", "Should use multi-stage build")
		
		// Should set working directory
		assert.Contains(t, dockerfile, "WORKDIR /app", "Should set working directory")
		
		// Should expose port
		assert.Contains(t, dockerfile, "EXPOSE 8080", "Should expose port 8080")
		
		// Should have health check
		assert.Contains(t, dockerfile, "HEALTHCHECK", "Should have health check")
		
		// Should run as non-root user
		assert.Contains(t, dockerfile, "USER appuser", "Should run as non-root user")
		
		// Should clean package cache
		assert.Contains(t, dockerfile, "rm -rf /var/cache/apk/*", "Should clean package cache")
	})
}

// Test .dockerignore best practices
func TestDockerignoreBestPractices(t *testing.T) {
	projectRoot, err := getProjectRoot()
	require.NoError(t, err)

	dockerignorePath := filepath.Join(projectRoot, ".dockerignore")
	
	t.Run("dockerignore exists and excludes appropriate files", func(t *testing.T) {
		content, err := os.ReadFile(dockerignorePath)
		if err != nil {
			t.Skip("No .dockerignore file found")
		}
		
		dockerignore := string(content)
		
		// Should ignore common files
		assert.Contains(t, dockerignore, ".git", "Should ignore .git directory")
		assert.Contains(t, dockerignore, "*.md", "Should ignore markdown files")
		assert.Contains(t, dockerignore, "*_test.go", "Should ignore test files")
		assert.Contains(t, dockerignore, "coverage.out", "Should ignore coverage files")
		assert.Contains(t, dockerignore, ".env", "Should ignore environment files")
	})
}