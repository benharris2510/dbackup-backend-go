package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/dbackup/backend-go/internal/encryption"
	"gorm.io/gorm"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
	DatabaseTypeRedis      DatabaseType = "redis"
	DatabaseTypeSQLServer  DatabaseType = "sqlserver"
	DatabaseTypeOracle     DatabaseType = "oracle"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
)

// DatabaseConnection represents a database connection configuration
type DatabaseConnection struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	UID  string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// Database connection details
	Type     DatabaseType `json:"type" gorm:"type:varchar(50);not null"`
	Host     string       `json:"host" gorm:"type:varchar(255);not null"`
	Port     int          `json:"port" gorm:"not null"`
	Database string       `json:"database" gorm:"type:varchar(255);not null"`
	Username string       `json:"username" gorm:"type:varchar(255);not null"`
	Password string       `json:"-" gorm:"type:text;not null"` // Encrypted
	
	// SSL Configuration
	SSLEnabled   bool    `json:"ssl_enabled" gorm:"default:false"`
	SSLMode      *string `json:"ssl_mode,omitempty" gorm:"type:varchar(50)"`
	SSLCert      *string `json:"-" gorm:"type:text"` // Encrypted
	SSLKey       *string `json:"-" gorm:"type:text"` // Encrypted
	SSLRootCert  *string `json:"-" gorm:"type:text"` // Encrypted
	
	// Connection settings
	MaxConnections     int           `json:"max_connections" gorm:"default:10"`
	ConnectionTimeout  time.Duration `json:"connection_timeout" gorm:"default:30000000000"` // 30 seconds in nanoseconds
	QueryTimeout       time.Duration `json:"query_timeout" gorm:"default:300000000000"`     // 5 minutes in nanoseconds
	
	// Status and health
	IsActive      bool       `json:"is_active" gorm:"default:true"`
	LastTestedAt  *time.Time `json:"last_tested_at"`
	LastTestError *string    `json:"last_test_error,omitempty" gorm:"type:text"`
	
	// Metadata
	Description *string                `json:"description,omitempty" gorm:"type:text"`
	Tags        []DatabaseTag          `json:"tags,omitempty" gorm:"many2many:database_connection_tags;"`
	
	// Relationships
	UserID uint `json:"user_id" gorm:"not null;index"`
	User   User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	TeamID *uint `json:"team_id" gorm:"index"`
	Team   *Team `json:"team,omitempty" gorm:"foreignKey:TeamID"`
	
	Tables     []DatabaseTable `json:"tables,omitempty" gorm:"foreignKey:DatabaseConnectionID"`
	BackupJobs []BackupJob     `json:"backup_jobs,omitempty" gorm:"foreignKey:DatabaseConnectionID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// DatabaseTag represents a tag for organizing database connections
type DatabaseTag struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	Name  string `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	Color string `json:"color" gorm:"type:varchar(7);default:'#3B82F6'"` // Hex color
	
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the DatabaseConnection model
func (DatabaseConnection) TableName() string {
	return "database_connections"
}

// TableName returns the table name for the DatabaseTag model
func (DatabaseTag) TableName() string {
	return "database_tags"
}

// BeforeCreate hook to generate UID before creating database connection
func (dc *DatabaseConnection) BeforeCreate(tx *gorm.DB) error {
	if dc.UID == "" {
		dc.UID = generateUID()
	}
	return nil
}

// GetConnectionString returns the database connection string (without password)
func (dc *DatabaseConnection) GetConnectionString() string {
	switch dc.Type {
	case DatabaseTypePostgreSQL:
		sslMode := "disable"
		if dc.SSLEnabled && dc.SSLMode != nil {
			sslMode = *dc.SSLMode
		}
		return fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=%s", 
			dc.Username, dc.Host, dc.Port, dc.Database, sslMode)
	case DatabaseTypeMySQL:
		ssl := "false"
		if dc.SSLEnabled {
			ssl = "true"
		}
		return fmt.Sprintf("mysql://%s:***@%s:%d/%s?tls=%s",
			dc.Username, dc.Host, dc.Port, dc.Database, ssl)
	case DatabaseTypeMongoDB:
		ssl := "false"
		if dc.SSLEnabled {
			ssl = "true"
		}
		return fmt.Sprintf("mongodb://%s:***@%s:%d/%s?ssl=%s",
			dc.Username, dc.Host, dc.Port, dc.Database, ssl)
	default:
		return fmt.Sprintf("%s://%s:***@%s:%d/%s",
			string(dc.Type), dc.Username, dc.Host, dc.Port, dc.Database)
	}
}

// GetFullConnectionString returns the complete connection string with password
func (dc *DatabaseConnection) GetFullConnectionString(decryptedPassword string) string {
	switch dc.Type {
	case DatabaseTypePostgreSQL:
		sslMode := "disable"
		if dc.SSLEnabled && dc.SSLMode != nil {
			sslMode = *dc.SSLMode
		}
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", 
			dc.Username, decryptedPassword, dc.Host, dc.Port, dc.Database, sslMode)
	case DatabaseTypeMySQL:
		ssl := "false"
		if dc.SSLEnabled {
			ssl = "true"
		}
		return fmt.Sprintf("mysql://%s:%s@%s:%d/%s?tls=%s",
			dc.Username, decryptedPassword, dc.Host, dc.Port, dc.Database, ssl)
	case DatabaseTypeMongoDB:
		ssl := "false"
		if dc.SSLEnabled {
			ssl = "true"
		}
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?ssl=%s",
			dc.Username, decryptedPassword, dc.Host, dc.Port, dc.Database, ssl)
	default:
		return fmt.Sprintf("%s://%s:%s@%s:%d/%s",
			string(dc.Type), dc.Username, decryptedPassword, dc.Host, dc.Port, dc.Database)
	}
}

// IsHealthy checks if the database connection is healthy
func (dc *DatabaseConnection) IsHealthy() bool {
	return dc.IsActive && dc.LastTestError == nil
}

// NeedsRetesting checks if the connection should be retested
func (dc *DatabaseConnection) NeedsRetesting() bool {
	if dc.LastTestedAt == nil {
		return true
	}
	// Retest if last test was more than 1 hour ago
	return time.Since(*dc.LastTestedAt) > time.Hour
}

// SetTestResult updates the test result for the connection
func (dc *DatabaseConnection) SetTestResult(success bool, errorMsg string) {
	now := time.Now()
	dc.LastTestedAt = &now
	
	if success {
		dc.LastTestError = nil
	} else {
		dc.LastTestError = &errorMsg
	}
}

// GetTableCount returns the number of discovered tables
func (dc *DatabaseConnection) GetTableCount() int {
	return len(dc.Tables)
}

// GetActiveBackupJobsCount returns the number of active backup jobs
func (dc *DatabaseConnection) GetActiveBackupJobsCount() int {
	count := 0
	for _, job := range dc.BackupJobs {
		if job.Status == BackupStatusPending || job.Status == BackupStatusRunning {
			count++
		}
	}
	return count
}

// CanCreateBackup checks if a new backup can be created for this connection
func (dc *DatabaseConnection) CanCreateBackup() bool {
	return dc.IsActive && dc.IsHealthy() && dc.GetActiveBackupJobsCount() < 5
}

// GetDefaultPort returns the default port for the database type
func (dt DatabaseType) GetDefaultPort() int {
	switch dt {
	case DatabaseTypePostgreSQL:
		return 5432
	case DatabaseTypeMySQL:
		return 3306
	case DatabaseTypeMongoDB:
		return 27017
	case DatabaseTypeRedis:
		return 6379
	case DatabaseTypeSQLServer:
		return 1433
	case DatabaseTypeOracle:
		return 1521
	default:
		return 3306
	}
}

// IsValid checks if the database type is valid
func (dt DatabaseType) IsValid() bool {
	switch dt {
	case DatabaseTypePostgreSQL, DatabaseTypeMySQL, DatabaseTypeMongoDB, 
		 DatabaseTypeRedis, DatabaseTypeSQLServer, DatabaseTypeOracle, DatabaseTypeSQLite:
		return true
	default:
		return false
	}
}

// GetDisplayName returns a human-readable name for the database type
func (dt DatabaseType) GetDisplayName() string {
	switch dt {
	case DatabaseTypePostgreSQL:
		return "PostgreSQL"
	case DatabaseTypeMySQL:
		return "MySQL"
	case DatabaseTypeMongoDB:
		return "MongoDB"
	case DatabaseTypeRedis:
		return "Redis"
	case DatabaseTypeSQLServer:
		return "SQL Server"
	case DatabaseTypeOracle:
		return "Oracle"
	case DatabaseTypeSQLite:
		return "SQLite"
	default:
		return string(dt)
	}
}

// EncryptCredentials encrypts sensitive connection data before saving to database
func (dc *DatabaseConnection) EncryptCredentials(encService *encryption.Service) error {
	if encService == nil {
		return errors.New("encryption service is required")
	}
	
	// Encrypt password
	if dc.Password != "" {
		encrypted, err := encService.Encrypt(dc.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		dc.Password = encrypted
	}
	
	// Encrypt SSL certificates if present
	if dc.SSLCert != nil && *dc.SSLCert != "" {
		encrypted, err := encService.Encrypt(*dc.SSLCert)
		if err != nil {
			return fmt.Errorf("failed to encrypt SSL cert: %w", err)
		}
		*dc.SSLCert = encrypted
	}
	
	if dc.SSLKey != nil && *dc.SSLKey != "" {
		encrypted, err := encService.Encrypt(*dc.SSLKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt SSL key: %w", err)
		}
		*dc.SSLKey = encrypted
	}
	
	if dc.SSLRootCert != nil && *dc.SSLRootCert != "" {
		encrypted, err := encService.Encrypt(*dc.SSLRootCert)
		if err != nil {
			return fmt.Errorf("failed to encrypt SSL root cert: %w", err)
		}
		*dc.SSLRootCert = encrypted
	}
	
	return nil
}

// DecryptCredentials decrypts sensitive connection data after loading from database
func (dc *DatabaseConnection) DecryptCredentials(encService *encryption.Service) error {
	if encService == nil {
		return errors.New("encryption service is required")
	}
	
	// Decrypt password
	if dc.Password != "" {
		decrypted, err := encService.Decrypt(dc.Password)
		if err != nil {
			return fmt.Errorf("failed to decrypt password: %w", err)
		}
		dc.Password = decrypted
	}
	
	// Decrypt SSL certificates if present
	if dc.SSLCert != nil && *dc.SSLCert != "" {
		decrypted, err := encService.Decrypt(*dc.SSLCert)
		if err != nil {
			return fmt.Errorf("failed to decrypt SSL cert: %w", err)
		}
		*dc.SSLCert = decrypted
	}
	
	if dc.SSLKey != nil && *dc.SSLKey != "" {
		decrypted, err := encService.Decrypt(*dc.SSLKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt SSL key: %w", err)
		}
		*dc.SSLKey = decrypted
	}
	
	if dc.SSLRootCert != nil && *dc.SSLRootCert != "" {
		decrypted, err := encService.Decrypt(*dc.SSLRootCert)
		if err != nil {
			return fmt.Errorf("failed to decrypt SSL root cert: %w", err)
		}
		*dc.SSLRootCert = decrypted
	}
	
	return nil
}

// GetDecryptedPassword returns the decrypted password
func (dc *DatabaseConnection) GetDecryptedPassword(encService *encryption.Service) (string, error) {
	if encService == nil {
		return "", errors.New("encryption service is required")
	}
	
	if dc.Password == "" {
		return "", nil
	}
	
	return encService.Decrypt(dc.Password)
}

// ValidateConnection validates the database connection configuration
func (dc *DatabaseConnection) ValidateConnection() error {
	if dc.Name == "" {
		return errors.New("connection name is required")
	}
	
	if !dc.Type.IsValid() {
		return fmt.Errorf("invalid database type: %s", dc.Type)
	}
	
	if dc.Host == "" && dc.Type != DatabaseTypeSQLite {
		return errors.New("host is required for non-SQLite databases")
	}
	
	if dc.Database == "" {
		return errors.New("database name is required")
	}
	
	if dc.Username == "" && dc.Type != DatabaseTypeSQLite {
		return errors.New("username is required for non-SQLite databases")
	}
	
	if dc.Password == "" && dc.Type != DatabaseTypeSQLite {
		return errors.New("password is required for non-SQLite databases")
	}
	
	// Validate port (not required for SQLite)
	if dc.Type != DatabaseTypeSQLite {
		if dc.Port <= 0 || dc.Port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
	}
	
	return nil
}

// SetDefaultValues sets default values for the connection
func (dc *DatabaseConnection) SetDefaultValues() {
	if dc.Port == 0 {
		dc.Port = dc.Type.GetDefaultPort()
	}
	
	if dc.MaxConnections == 0 {
		dc.MaxConnections = 10
	}
	
	if dc.ConnectionTimeout == 0 {
		dc.ConnectionTimeout = 30 * time.Second
	}
	
	if dc.QueryTimeout == 0 {
		dc.QueryTimeout = 5 * time.Minute
	}
}

// ToPublic returns a public representation of the connection without sensitive data
func (dc *DatabaseConnection) ToPublic() *DatabaseConnectionPublic {
	public := &DatabaseConnectionPublic{
		ID:                dc.ID,
		UID:               dc.UID,
		Name:              dc.Name,
		Type:              dc.Type,
		Host:              dc.Host,
		Port:              dc.Port,
		Database:          dc.Database,
		Username:          dc.Username,
		SSLEnabled:        dc.SSLEnabled,
		MaxConnections:    dc.MaxConnections,
		ConnectionTimeout: dc.ConnectionTimeout,
		QueryTimeout:      dc.QueryTimeout,
		IsActive:          dc.IsActive,
		LastTestedAt:      dc.LastTestedAt,
		HasSSLCert:        dc.SSLCert != nil && *dc.SSLCert != "",
		HasSSLKey:         dc.SSLKey != nil && *dc.SSLKey != "",
		HasSSLRootCert:    dc.SSLRootCert != nil && *dc.SSLRootCert != "",
		TableCount:        len(dc.Tables),
		BackupJobsCount:   len(dc.BackupJobs),
		ConnectionString:  dc.GetConnectionString(),
		IsHealthy:         dc.IsHealthy(),
		NeedsRetesting:    dc.NeedsRetesting(),
		CreatedAt:         dc.CreatedAt,
		UpdatedAt:         dc.UpdatedAt,
	}
	
	if dc.Description != nil {
		public.Description = *dc.Description
	}
	
	if dc.LastTestError != nil {
		public.LastTestError = *dc.LastTestError
	}
	
	if dc.SSLMode != nil {
		public.SSLMode = *dc.SSLMode
	}
	
	// Convert tags
	public.Tags = make([]DatabaseTagPublic, len(dc.Tags))
	for i, tag := range dc.Tags {
		public.Tags[i] = DatabaseTagPublic{
			ID:    tag.ID,
			Name:  tag.Name,
			Color: tag.Color,
		}
	}
	
	return public
}

// DatabaseConnectionPublic represents the public view of a database connection
type DatabaseConnectionPublic struct {
	ID                uint                `json:"id"`
	UID               string              `json:"uid"`
	Name              string              `json:"name"`
	Description       string              `json:"description,omitempty"`
	Type              DatabaseType        `json:"type"`
	Host              string              `json:"host"`
	Port              int                 `json:"port"`
	Database          string              `json:"database"`
	Username          string              `json:"username"`
	SSLEnabled        bool                `json:"ssl_enabled"`
	SSLMode           string              `json:"ssl_mode,omitempty"`
	HasSSLCert        bool                `json:"has_ssl_cert"`
	HasSSLKey         bool                `json:"has_ssl_key"`
	HasSSLRootCert    bool                `json:"has_ssl_root_cert"`
	MaxConnections    int                 `json:"max_connections"`
	ConnectionTimeout time.Duration       `json:"connection_timeout"`
	QueryTimeout      time.Duration       `json:"query_timeout"`
	IsActive          bool                `json:"is_active"`
	LastTestedAt      *time.Time          `json:"last_tested_at"`
	LastTestError     string              `json:"last_test_error,omitempty"`
	Tags              []DatabaseTagPublic `json:"tags"`
	TableCount        int                 `json:"table_count"`
	BackupJobsCount   int                 `json:"backup_jobs_count"`
	ConnectionString  string              `json:"connection_string"`
	IsHealthy         bool                `json:"is_healthy"`
	NeedsRetesting    bool                `json:"needs_retesting"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// DatabaseTagPublic represents the public view of a database tag
type DatabaseTagPublic struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ConnectionRequest represents a request to create/update a database connection
type ConnectionRequest struct {
	Name              string              `json:"name" validate:"required,min=1,max=255"`
	Description       string              `json:"description,omitempty" validate:"max=1000"`
	Type              DatabaseType        `json:"type" validate:"required"`
	Host              string              `json:"host" validate:"required_unless=Type sqlite"`
	Port              int                 `json:"port" validate:"min=1,max=65535"`
	Database          string              `json:"database" validate:"required,min=1,max=255"`
	Username          string              `json:"username" validate:"required_unless=Type sqlite"`
	Password          string              `json:"password" validate:"required_unless=Type sqlite"`
	SSLEnabled        bool                `json:"ssl_enabled"`
	SSLMode           string              `json:"ssl_mode,omitempty"`
	SSLCert           string              `json:"ssl_cert,omitempty"`
	SSLKey            string              `json:"ssl_key,omitempty"`
	SSLRootCert       string              `json:"ssl_root_cert,omitempty"`
	MaxConnections    int                 `json:"max_connections" validate:"min=1,max=100"`
	ConnectionTimeout int                 `json:"connection_timeout" validate:"min=1,max=300"` // seconds
	QueryTimeout      int                 `json:"query_timeout" validate:"min=1,max=3600"`     // seconds
	TagIDs            []uint              `json:"tag_ids,omitempty"`
}

// ToModel converts ConnectionRequest to DatabaseConnection model
func (cr *ConnectionRequest) ToModel() *DatabaseConnection {
	dc := &DatabaseConnection{
		Name:              cr.Name,
		Type:              cr.Type,
		Host:              cr.Host,
		Port:              cr.Port,
		Database:          cr.Database,
		Username:          cr.Username,
		Password:          cr.Password,
		SSLEnabled:        cr.SSLEnabled,
		MaxConnections:    cr.MaxConnections,
		ConnectionTimeout: time.Duration(cr.ConnectionTimeout) * time.Second,
		QueryTimeout:      time.Duration(cr.QueryTimeout) * time.Second,
		IsActive:          true,
	}
	
	if cr.Description != "" {
		dc.Description = &cr.Description
	}
	
	if cr.SSLMode != "" {
		dc.SSLMode = &cr.SSLMode
	}
	
	if cr.SSLCert != "" {
		dc.SSLCert = &cr.SSLCert
	}
	
	if cr.SSLKey != "" {
		dc.SSLKey = &cr.SSLKey
	}
	
	if cr.SSLRootCert != "" {
		dc.SSLRootCert = &cr.SSLRootCert
	}
	
	return dc
}

// TestConnectionResult represents the result of testing a database connection
type TestConnectionResult struct {
	Success      bool                   `json:"success"`
	Message      string                 `json:"message"`
	Error        string                 `json:"error,omitempty"`
	ResponseTime time.Duration          `json:"response_time"`
	DatabaseInfo *DatabaseInfoResult    `json:"database_info,omitempty"`
}

// DatabaseInfoResult represents database information gathered during connection test
type DatabaseInfoResult struct {
	Version   string `json:"version"`
	Charset   string `json:"charset,omitempty"`
	Collation string `json:"collation,omitempty"`
	Size      string `json:"size,omitempty"`
}