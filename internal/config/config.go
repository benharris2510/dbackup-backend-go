package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Server ServerConfig

	// Database configuration
	Database DatabaseConfig

	// JWT configuration
	JWT JWTConfig

	// Redis configuration
	Redis RedisConfig

	// Encryption configuration
	Encryption EncryptionConfig

	// OAuth configuration
	OAuth OAuthConfig

	// S3 configuration
	S3 S3Config

	// Backup configuration
	Backup BackupConfig

	// CORS configuration
	CORS CORSConfig

	// Rate limiting configuration
	RateLimit RateLimitConfig

	// Logging configuration
	Log LogConfig

	// Monitoring configuration
	Monitoring MonitoringConfig

	// WebSocket configuration
	WebSocket WebSocketConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port string
	Host string
	Env  string
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	URL                   string
	MaxConnections        int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
}

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	SecretKey           string
	AccessTokenExpires  time.Duration
	RefreshTokenExpires time.Duration
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	URL        string
	Password   string
	MaxRetries int
	PoolSize   int
}

// EncryptionConfig holds encryption-related configuration
type EncryptionConfig struct {
	MasterKey string
}

// OAuthConfig holds OAuth provider configuration
type OAuthConfig struct {
	Google GoogleOAuthConfig
}

// GoogleOAuthConfig holds Google OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// S3Config holds S3 storage configuration
type S3Config struct {
	DefaultEndpoint    string
	DefaultRegion      string
	ForcePathStyle     bool
}

// BackupConfig holds backup-related configuration
type BackupConfig struct {
	MaxSize            int64
	RetentionDays      int
	MaxConcurrent      int
	Timeout            time.Duration
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled            bool
	RequestsPerMinute  int
	Burst              int
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string
	Format string
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	MetricsEnabled bool
	MetricsPort    int
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingPeriod      time.Duration
	PongWait        time.Duration
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/dbackup/")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Set default values
	setDefaults()

	// Enable environment variable override
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	
	// Explicitly bind environment variables for nested structures
	bindEnvVars()

	// Unmarshal configuration
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.env", "development")

	// Database defaults
	viper.SetDefault("database.url", "postgres://dbackup:dbackup@localhost:5432/dbackup_dev?sslmode=disable")
	viper.SetDefault("database.maxconnections", 25)
	viper.SetDefault("database.maxidleconnections", 5)
	viper.SetDefault("database.connectionmaxlifetime", "5m")

	// JWT defaults
	viper.SetDefault("jwt.accesstokenexpires", "168h") // 7 days
	viper.SetDefault("jwt.refreshtokenexpires", "720h") // 30 days

	// Redis defaults
	viper.SetDefault("redis.url", "redis://localhost:6379/0")
	viper.SetDefault("redis.maxretries", 3)
	viper.SetDefault("redis.poolsize", 10)

	// S3 defaults
	viper.SetDefault("s3.defaultendpoint", "https://s3.amazonaws.com")
	viper.SetDefault("s3.defaultregion", "us-east-1")
	viper.SetDefault("s3.forcepathstyle", false)

	// Backup defaults
	viper.SetDefault("backup.maxsize", 10737418240) // 10GB
	viper.SetDefault("backup.retentiondays", 30)
	viper.SetDefault("backup.maxconcurrent", 5)
	viper.SetDefault("backup.timeout", "3600s")

	// CORS defaults
	viper.SetDefault("cors.allowedorigins", []string{"http://localhost:3000"})
	viper.SetDefault("cors.allowedmethods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"})
	viper.SetDefault("cors.allowedheaders", []string{"Content-Type", "Authorization", "X-Requested-With"})
	viper.SetDefault("cors.allowcredentials", true)

	// Rate limit defaults
	viper.SetDefault("ratelimit.enabled", true)
	viper.SetDefault("ratelimit.requestsperminute", 60)
	viper.SetDefault("ratelimit.burst", 10)

	// Log defaults
	viper.SetDefault("log.level", "debug")
	viper.SetDefault("log.format", "json")

	// Monitoring defaults
	viper.SetDefault("monitoring.metricsenabled", true)
	viper.SetDefault("monitoring.metricsport", 9090)

	// WebSocket defaults
	viper.SetDefault("websocket.readbuffersize", 1024)
	viper.SetDefault("websocket.writebuffersize", 1024)
	viper.SetDefault("websocket.pingperiod", "54s")
	viper.SetDefault("websocket.pongwait", "60s")
}

// validate validates the configuration
func validate(cfg *Config) error {
	// Server validation
	if cfg.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	// Database validation
	if cfg.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	// JWT validation
	if cfg.JWT.SecretKey == "" && !cfg.IsDevelopment() {
		return fmt.Errorf("JWT secret key is required in non-development environments")
	}

	// Encryption validation
	if cfg.Encryption.MasterKey == "" && !cfg.IsDevelopment() {
		return fmt.Errorf("master encryption key is required in non-development environments")
	}

	// Backup validation
	if cfg.Backup.MaxSize <= 0 {
		return fmt.Errorf("backup max size must be positive")
	}
	if cfg.Backup.RetentionDays <= 0 {
		return fmt.Errorf("backup retention days must be positive")
	}
	if cfg.Backup.MaxConcurrent <= 0 {
		return fmt.Errorf("max concurrent backups must be positive")
	}

	// Rate limit validation
	if cfg.RateLimit.Enabled {
		if cfg.RateLimit.RequestsPerMinute <= 0 {
			return fmt.Errorf("rate limit requests per minute must be positive")
		}
		if cfg.RateLimit.Burst <= 0 {
			return fmt.Errorf("rate limit burst must be positive")
		}
	}

	// WebSocket validation
	if cfg.WebSocket.ReadBufferSize <= 0 {
		return fmt.Errorf("websocket read buffer size must be positive")
	}
	if cfg.WebSocket.WriteBufferSize <= 0 {
		return fmt.Errorf("websocket write buffer size must be positive")
	}

	return nil
}

// bindEnvVars explicitly binds environment variables to config keys
func bindEnvVars() {
	// Server
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("server.host", "SERVER_HOST")
	viper.BindEnv("server.env", "SERVER_ENV", "ENV")
	
	// Database
	viper.BindEnv("database.url", "DATABASE_URL")
	viper.BindEnv("database.maxconnections", "DATABASE_MAX_CONNECTIONS")
	viper.BindEnv("database.maxidleconnections", "DATABASE_MAX_IDLE_CONNECTIONS")
	viper.BindEnv("database.connectionmaxlifetime", "DATABASE_CONNECTION_MAX_LIFETIME")
	
	// JWT
	viper.BindEnv("jwt.secretkey", "JWT_SECRET_KEY")
	viper.BindEnv("jwt.accesstokenexpires", "JWT_ACCESS_TOKEN_EXPIRES")
	viper.BindEnv("jwt.refreshtokenexpires", "JWT_REFRESH_TOKEN_EXPIRES")
	
	// Redis
	viper.BindEnv("redis.url", "REDIS_URL")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.maxretries", "REDIS_MAX_RETRIES")
	viper.BindEnv("redis.poolsize", "REDIS_POOL_SIZE")
	
	// Encryption
	viper.BindEnv("encryption.masterkey", "ENCRYPTION_MASTER_KEY")
	
	// OAuth
	viper.BindEnv("oauth.google.clientid", "GOOGLE_CLIENT_ID")
	viper.BindEnv("oauth.google.clientsecret", "GOOGLE_CLIENT_SECRET")
	viper.BindEnv("oauth.google.redirecturi", "GOOGLE_REDIRECT_URI")
	
	// S3
	viper.BindEnv("s3.defaultendpoint", "S3_DEFAULT_ENDPOINT", "DEFAULT_S3_ENDPOINT")
	viper.BindEnv("s3.defaultregion", "S3_DEFAULT_REGION", "DEFAULT_S3_REGION")
	viper.BindEnv("s3.forcepathstyle", "S3_FORCE_PATH_STYLE")
	
	// Backup
	viper.BindEnv("backup.maxsize", "BACKUP_MAX_SIZE")
	viper.BindEnv("backup.retentiondays", "BACKUP_RETENTION_DAYS")
	viper.BindEnv("backup.maxconcurrent", "BACKUP_MAX_CONCURRENT")
	viper.BindEnv("backup.timeout", "BACKUP_TIMEOUT")
	
	// CORS
	viper.BindEnv("cors.allowedorigins", "CORS_ALLOWED_ORIGINS")
	viper.BindEnv("cors.allowedmethods", "CORS_ALLOWED_METHODS")
	viper.BindEnv("cors.allowedheaders", "CORS_ALLOWED_HEADERS")
	viper.BindEnv("cors.allowcredentials", "CORS_ALLOW_CREDENTIALS")
	
	// Rate Limit
	viper.BindEnv("ratelimit.enabled", "RATE_LIMIT_ENABLED")
	viper.BindEnv("ratelimit.requestsperminute", "RATE_LIMIT_REQUESTS_PER_MINUTE")
	viper.BindEnv("ratelimit.burst", "RATE_LIMIT_BURST")
	
	// Log
	viper.BindEnv("log.level", "LOG_LEVEL")
	viper.BindEnv("log.format", "LOG_FORMAT")
	
	// Monitoring
	viper.BindEnv("monitoring.metricsenabled", "METRICS_ENABLED")
	viper.BindEnv("monitoring.metricsport", "METRICS_PORT")
	
	// WebSocket
	viper.BindEnv("websocket.readbuffersize", "WEBSOCKET_READ_BUFFER_SIZE")
	viper.BindEnv("websocket.writebuffersize", "WEBSOCKET_WRITE_BUFFER_SIZE")
	viper.BindEnv("websocket.pingperiod", "WEBSOCKET_PING_PERIOD")
	viper.BindEnv("websocket.pongwait", "WEBSOCKET_PONG_WAIT")
}

// IsDevelopment returns true if the application is running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

// IsProduction returns true if the application is running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

// IsTest returns true if the application is running in test mode
func (c *Config) IsTest() bool {
	return c.Server.Env == "test"
}