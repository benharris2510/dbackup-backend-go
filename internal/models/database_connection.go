package models

import (
	"fmt"
	"time"

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
		 DatabaseTypeRedis, DatabaseTypeSQLServer, DatabaseTypeOracle:
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
	default:
		return string(dt)
	}
}