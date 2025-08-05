package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Reset singleton before each test
	defer Reset()

	tests := []struct {
		name           string
		envVars        map[string]string
		expectedValues map[string]interface{}
		expectError    bool
	}{
		{
			name:    "Default configuration",
			envVars: map[string]string{},
			expectedValues: map[string]interface{}{
				"server.port":         "8080",
				"server.host":         "0.0.0.0",
				"server.env":          "development",
				"database.maxconnections": 25,
				"redis.maxretries":    3,
				"backup.retentiondays": 30,
			},
			expectError: false,
		},
		{
			name: "Environment variable override",
			envVars: map[string]string{
				"SERVER_PORT":         "9000",
				"SERVER_ENV":          "production",
				"DATABASE_URL":        "postgres://prod:prod@prod-db:5432/dbackup_prod",
				"JWT_SECRET_KEY":      "production-secret-key",
				"ENCRYPTION_MASTER_KEY": "production-encryption-key-32-bytes",
			},
			expectedValues: map[string]interface{}{
				"server.port": "9000",
				"server.env":  "production",
				"database.url": "postgres://prod:prod@prod-db:5432/dbackup_prod",
				"jwt.secretkey": "production-secret-key",
				"encryption.masterkey": "production-encryption-key-32-bytes",
			},
			expectError: false,
		},
		{
			name: "Custom backup configuration",
			envVars: map[string]string{
				"BACKUP_MAX_SIZE":       "21474836480", // 20GB
				"BACKUP_RETENTION_DAYS": "90",
				"BACKUP_MAX_CONCURRENT": "10",
				"BACKUP_TIMEOUT":        "7200s",
			},
			expectedValues: map[string]interface{}{
				"backup.maxsize":       int64(21474836480),
				"backup.retentiondays": 90,
				"backup.maxconcurrent": 10,
				"backup.timeout":       "2h0m0s",
			},
			expectError: false,
		},
		{
			name: "CORS configuration",
			envVars: map[string]string{
				"CORS_ALLOWED_ORIGINS":     "https://app.example.com,https://admin.example.com",
				"CORS_ALLOWED_METHODS":     "GET,POST,PUT,DELETE",
				"CORS_ALLOWED_HEADERS":     "Content-Type,Authorization,X-API-Key",
				"CORS_ALLOW_CREDENTIALS":   "false",
			},
			expectedValues: map[string]interface{}{
				"cors.allowedorigins":   []string{"https://app.example.com", "https://admin.example.com"},
				"cors.allowedmethods":   []string{"GET", "POST", "PUT", "DELETE"},
				"cors.allowedheaders":   []string{"Content-Type", "Authorization", "X-API-Key"},
				"cors.allowcredentials": false,
			},
			expectError: false,
		},
		{
			name: "WebSocket configuration",
			envVars: map[string]string{
				"WEBSOCKET_READ_BUFFER_SIZE":  "2048",
				"WEBSOCKET_WRITE_BUFFER_SIZE": "2048",
				"WEBSOCKET_PING_PERIOD":       "30s",
				"WEBSOCKET_PONG_WAIT":         "45s",
			},
			expectedValues: map[string]interface{}{
				"websocket.readbuffersize":  2048,
				"websocket.writebuffersize": 2048,
				"websocket.pingperiod":      "30s",
				"websocket.pongwait":        "45s",
			},
			expectError: false,
		},
		{
			name: "Rate limiting configuration",
			envVars: map[string]string{
				"RATE_LIMIT_ENABLED":             "false",
				"RATE_LIMIT_REQUESTS_PER_MINUTE": "120",
				"RATE_LIMIT_BURST":               "20",
			},
			expectedValues: map[string]interface{}{
				"ratelimit.enabled":            false,
				"ratelimit.requestsperminute":  120,
				"ratelimit.burst":              20,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Reset singleton for clean test
			Reset()

			cfg, err := Load()

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Verify expected values
			for path, expected := range tt.expectedValues {
				switch path {
				case "server.port":
					assert.Equal(t, expected, cfg.Server.Port)
				case "server.host":
					assert.Equal(t, expected, cfg.Server.Host)
				case "server.env":
					assert.Equal(t, expected, cfg.Server.Env)
				case "database.url":
					assert.Equal(t, expected, cfg.Database.URL)
				case "database.maxconnections":
					assert.Equal(t, expected, cfg.Database.MaxConnections)
				case "jwt.secretkey":
					assert.Equal(t, expected, cfg.JWT.SecretKey)
				case "encryption.masterkey":
					assert.Equal(t, expected, cfg.Encryption.MasterKey)
				case "backup.maxsize":
					assert.Equal(t, expected, cfg.Backup.MaxSize)
				case "backup.retentiondays":
					assert.Equal(t, expected, cfg.Backup.RetentionDays)
				case "backup.maxconcurrent":
					assert.Equal(t, expected, cfg.Backup.MaxConcurrent)
				case "backup.timeout":
					expectedDuration, _ := time.ParseDuration(expected.(string))
					assert.Equal(t, expectedDuration, cfg.Backup.Timeout)
				case "cors.allowedorigins":
					assert.Equal(t, expected, cfg.CORS.AllowedOrigins)
				case "cors.allowedmethods":
					assert.Equal(t, expected, cfg.CORS.AllowedMethods)
				case "cors.allowedheaders":
					assert.Equal(t, expected, cfg.CORS.AllowedHeaders)
				case "cors.allowcredentials":
					assert.Equal(t, expected, cfg.CORS.AllowCredentials)
				case "websocket.readbuffersize":
					assert.Equal(t, expected, cfg.WebSocket.ReadBufferSize)
				case "websocket.writebuffersize":
					assert.Equal(t, expected, cfg.WebSocket.WriteBufferSize)
				case "websocket.pingperiod":
					expectedDuration, _ := time.ParseDuration(expected.(string))
					assert.Equal(t, expectedDuration, cfg.WebSocket.PingPeriod)
				case "websocket.pongwait":
					expectedDuration, _ := time.ParseDuration(expected.(string))
					assert.Equal(t, expectedDuration, cfg.WebSocket.PongWait)
				case "ratelimit.enabled":
					assert.Equal(t, expected, cfg.RateLimit.Enabled)
				case "ratelimit.requestsperminute":
					assert.Equal(t, expected, cfg.RateLimit.RequestsPerMinute)
				case "ratelimit.burst":
					assert.Equal(t, expected, cfg.RateLimit.Burst)
				case "redis.maxretries":
					assert.Equal(t, expected, cfg.Redis.MaxRetries)
				}
			}
		})
	}
}

func TestValidation(t *testing.T) {
	defer Reset()

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		errorString string
	}{
		{
			name: "Missing JWT secret in production",
			envVars: map[string]string{
				"SERVER_ENV": "production",
			},
			expectError: true,
			errorString: "JWT secret key is required in non-development environments",
		},
		{
			name: "Missing encryption key in production",
			envVars: map[string]string{
				"SERVER_ENV":     "production",
				"JWT_SECRET_KEY": "valid-jwt-secret",
			},
			expectError: true,
			errorString: "master encryption key is required in non-development environments",
		},
		{
			name: "Invalid backup max size",
			envVars: map[string]string{
				"BACKUP_MAX_SIZE": "-1",
			},
			expectError: true,
			errorString: "backup max size must be positive",
		},
		{
			name: "Invalid backup retention days",
			envVars: map[string]string{
				"BACKUP_RETENTION_DAYS": "0",
			},
			expectError: true,
			errorString: "backup retention days must be positive",
		},
		{
			name: "Invalid max concurrent backups",
			envVars: map[string]string{
				"BACKUP_MAX_CONCURRENT": "-1",
			},
			expectError: true,
			errorString: "max concurrent backups must be positive",
		},
		{
			name: "Invalid rate limit requests per minute",
			envVars: map[string]string{
				"RATE_LIMIT_ENABLED":             "true",
				"RATE_LIMIT_REQUESTS_PER_MINUTE": "0",
			},
			expectError: true,
			errorString: "rate limit requests per minute must be positive",
		},
		{
			name: "Invalid rate limit burst",
			envVars: map[string]string{
				"RATE_LIMIT_ENABLED": "true",
				"RATE_LIMIT_BURST":   "-1",
			},
			expectError: true,
			errorString: "rate limit burst must be positive",
		},
		{
			name: "Invalid websocket read buffer size",
			envVars: map[string]string{
				"WEBSOCKET_READ_BUFFER_SIZE": "0",
			},
			expectError: true,
			errorString: "websocket read buffer size must be positive",
		},
		{
			name: "Invalid websocket write buffer size",
			envVars: map[string]string{
				"WEBSOCKET_WRITE_BUFFER_SIZE": "-1",
			},
			expectError: true,
			errorString: "websocket write buffer size must be positive",
		},
		{
			name: "Valid production configuration",
			envVars: map[string]string{
				"SERVER_ENV":            "production",
				"JWT_SECRET_KEY":        "production-jwt-secret-key",
				"ENCRYPTION_MASTER_KEY": "production-encryption-key-32-bytes",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Reset singleton for clean test
			Reset()

			cfg, err := Load()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorString)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestConfigMethods(t *testing.T) {
	defer Reset()

	tests := []struct {
		name              string
		env               string
		expectDevelopment bool
		expectProduction  bool
		expectTest        bool
	}{
		{
			name:              "Development environment",
			env:               "development",
			expectDevelopment: true,
			expectProduction:  false,
			expectTest:        false,
		},
		{
			name:              "Production environment",
			env:               "production",
			expectDevelopment: false,
			expectProduction:  true,
			expectTest:        false,
		},
		{
			name:              "Test environment",
			env:               "test",
			expectDevelopment: false,
			expectProduction:  false,
			expectTest:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SERVER_ENV", tt.env)
			defer os.Unsetenv("SERVER_ENV")

			// Set required keys for non-development environments
			if tt.env != "development" {
				os.Setenv("JWT_SECRET_KEY", "test-jwt-secret")
				os.Setenv("ENCRYPTION_MASTER_KEY", "test-encryption-key")
				defer func() {
					os.Unsetenv("JWT_SECRET_KEY")
					os.Unsetenv("ENCRYPTION_MASTER_KEY")
				}()
			}

			Reset()
			cfg, err := Load()
			require.NoError(t, err)

			assert.Equal(t, tt.expectDevelopment, cfg.IsDevelopment())
			assert.Equal(t, tt.expectProduction, cfg.IsProduction())
			assert.Equal(t, tt.expectTest, cfg.IsTest())
		})
	}
}