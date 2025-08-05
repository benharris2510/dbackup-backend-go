package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DatabaseTable represents a table within a database connection
type DatabaseTable struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	UID       string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name      string `json:"name" gorm:"type:varchar(255);not null"`
	Schema    string `json:"schema" gorm:"type:varchar(255);not null;default:'public'"`
	FullName  string `json:"full_name" gorm:"type:varchar(511);not null"` // schema.table_name
	
	// Table metadata
	TableType    string  `json:"table_type" gorm:"type:varchar(50);default:'BASE TABLE'"` // BASE TABLE, VIEW, etc.
	Engine       *string `json:"engine,omitempty" gorm:"type:varchar(50)"`                // MySQL engine type
	Collation    *string `json:"collation,omitempty" gorm:"type:varchar(100)"`
	Comment      *string `json:"comment,omitempty" gorm:"type:text"`
	
	// Size and statistics
	RowCount       *int64 `json:"row_count,omitempty"`
	DataSize       *int64 `json:"data_size,omitempty"`       // Size in bytes
	IndexSize      *int64 `json:"index_size,omitempty"`      // Index size in bytes
	TotalSize      *int64 `json:"total_size,omitempty"`      // Total size in bytes
	LastAnalyzed   *time.Time `json:"last_analyzed,omitempty"`
	
	// Permissions and access control
	IsBackupEnabled   bool                `json:"is_backup_enabled" gorm:"default:true"`
	HasSelectAccess   bool                `json:"has_select_access" gorm:"default:false"`
	AccessLevel       TableAccessLevel    `json:"access_level" gorm:"type:varchar(20);default:'none'"`
	
	// Relationships
	DatabaseConnectionID uint               `json:"database_connection_id" gorm:"not null;index"`
	DatabaseConnection   DatabaseConnection `json:"database_connection,omitempty" gorm:"foreignKey:DatabaseConnectionID"`
	
	Columns         []DatabaseColumn     `json:"columns,omitempty" gorm:"foreignKey:DatabaseTableID"`
	Indexes         []DatabaseIndex      `json:"indexes,omitempty" gorm:"foreignKey:DatabaseTableID"`
	TablePermissions []TablePermission   `json:"table_permissions,omitempty" gorm:"foreignKey:DatabaseTableID"`
	BackupJobs      []BackupJob          `json:"backup_jobs,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// DatabaseColumn represents a column within a database table
type DatabaseColumn struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	Name         string `json:"name" gorm:"type:varchar(255);not null"`
	DataType     string `json:"data_type" gorm:"type:varchar(100);not null"`
	IsNullable   bool   `json:"is_nullable" gorm:"default:true"`
	IsPrimaryKey bool   `json:"is_primary_key" gorm:"default:false"`
	IsUnique     bool   `json:"is_unique" gorm:"default:false"`
	HasDefault   bool   `json:"has_default" gorm:"default:false"`
	DefaultValue *string `json:"default_value,omitempty" gorm:"type:text"`
	MaxLength    *int   `json:"max_length,omitempty"`
	Precision    *int   `json:"precision,omitempty"`
	Scale        *int   `json:"scale,omitempty"`
	Comment      *string `json:"comment,omitempty" gorm:"type:text"`
	Position     int    `json:"position" gorm:"not null"`
	
	// Relationships
	DatabaseTableID uint          `json:"database_table_id" gorm:"not null;index"`
	DatabaseTable   DatabaseTable `json:"database_table,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// DatabaseIndex represents an index on a database table
type DatabaseIndex struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	Name       string `json:"name" gorm:"type:varchar(255);not null"`
	Type       string `json:"type" gorm:"type:varchar(50);not null"`        // BTREE, HASH, etc.
	IsUnique   bool   `json:"is_unique" gorm:"default:false"`
	IsPrimary  bool   `json:"is_primary" gorm:"default:false"`
	Columns    string `json:"columns" gorm:"type:text;not null"`            // JSON array of column names
	Definition string `json:"definition" gorm:"type:text"`                  // Full index definition
	
	// Relationships
	DatabaseTableID uint          `json:"database_table_id" gorm:"not null;index"`
	DatabaseTable   DatabaseTable `json:"database_table,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableAccessLevel represents the level of access to a table
type TableAccessLevel string

const (
	TableAccessNone   TableAccessLevel = "none"
	TableAccessRead   TableAccessLevel = "read"
	TableAccessWrite  TableAccessLevel = "write"
	TableAccessAdmin  TableAccessLevel = "admin"
)

// TableName returns the table name for the DatabaseTable model
func (DatabaseTable) TableName() string {
	return "database_tables"
}

// TableName returns the table name for the DatabaseColumn model
func (DatabaseColumn) TableName() string {
	return "database_columns"
}

// TableName returns the table name for the DatabaseIndex model
func (DatabaseIndex) TableName() string {
	return "database_indexes"
}

// BeforeCreate hook to generate UID and full name before creating table
func (dt *DatabaseTable) BeforeCreate(tx *gorm.DB) error {
	if dt.UID == "" {
		dt.UID = generateUID()
	}
	
	// Generate full name if not set
	if dt.FullName == "" {
		if dt.Schema != "" && dt.Schema != "public" {
			dt.FullName = dt.Schema + "." + dt.Name
		} else {
			dt.FullName = dt.Name
		}
	}
	
	return nil
}

// GetPrimaryKeyColumns returns the primary key columns for the table
func (dt *DatabaseTable) GetPrimaryKeyColumns() []DatabaseColumn {
	var pkColumns []DatabaseColumn
	for _, column := range dt.Columns {
		if column.IsPrimaryKey {
			pkColumns = append(pkColumns, column)
		}
	}
	return pkColumns
}

// GetColumnByName returns a column by its name
func (dt *DatabaseTable) GetColumnByName(name string) *DatabaseColumn {
	for _, column := range dt.Columns {
		if column.Name == name {
			return &column
		}
	}
	return nil
}

// GetIndexByName returns an index by its name
func (dt *DatabaseTable) GetIndexByName(name string) *DatabaseIndex {
	for _, index := range dt.Indexes {
		if index.Name == name {
			return &index
		}
	}
	return nil
}

// HasPrimaryKey checks if the table has a primary key
func (dt *DatabaseTable) HasPrimaryKey() bool {
	for _, column := range dt.Columns {
		if column.IsPrimaryKey {
			return true
		}
	}
	return false
}

// GetColumnCount returns the number of columns in the table
func (dt *DatabaseTable) GetColumnCount() int {
	return len(dt.Columns)
}

// GetIndexCount returns the number of indexes on the table
func (dt *DatabaseTable) GetIndexCount() int {
	return len(dt.Indexes)
}

// IsView checks if the table is actually a view
func (dt *DatabaseTable) IsView() bool {
	return dt.TableType == "VIEW" || dt.TableType == "MATERIALIZED VIEW"
}

// CanBackup checks if the table can be backed up
func (dt *DatabaseTable) CanBackup() bool {
	return dt.IsBackupEnabled && dt.HasSelectAccess && dt.AccessLevel != TableAccessNone
}

// GetFormattedSize returns a human-readable size string
func (dt *DatabaseTable) GetFormattedSize() string {
	if dt.TotalSize == nil {
		return "Unknown"
	}
	
	size := *dt.TotalSize
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	}
}

// UpdateStatistics updates the table statistics
func (dt *DatabaseTable) UpdateStatistics(rowCount, dataSize, indexSize int64) {
	dt.RowCount = &rowCount
	dt.DataSize = &dataSize
	dt.IndexSize = &indexSize
	totalSize := dataSize + indexSize
	dt.TotalSize = &totalSize
	now := time.Now()
	dt.LastAnalyzed = &now
}

// IsStale checks if the table statistics are stale (older than 24 hours)
func (dt *DatabaseTable) IsStale() bool {
	if dt.LastAnalyzed == nil {
		return true
	}
	return time.Since(*dt.LastAnalyzed) > 24*time.Hour
}

// GetQualifiedName returns the fully qualified table name
func (dt *DatabaseTable) GetQualifiedName() string {
	return dt.FullName
}

// IsAccessible checks if the table is accessible for the given access level
func (dt *DatabaseTable) IsAccessible(requiredLevel TableAccessLevel) bool {
	switch requiredLevel {
	case TableAccessNone:
		return true
	case TableAccessRead:
		return dt.AccessLevel == TableAccessRead || dt.AccessLevel == TableAccessWrite || dt.AccessLevel == TableAccessAdmin
	case TableAccessWrite:
		return dt.AccessLevel == TableAccessWrite || dt.AccessLevel == TableAccessAdmin
	case TableAccessAdmin:
		return dt.AccessLevel == TableAccessAdmin
	default:
		return false
	}
}

// GetDataTypeCategory returns the category of a data type
func (dc *DatabaseColumn) GetDataTypeCategory() string {
	switch dc.DataType {
	case "int", "integer", "bigint", "smallint", "tinyint", "mediumint",
		 "serial", "bigserial", "smallserial":
		return "integer"
	case "decimal", "numeric", "float", "double", "real", "money":
		return "decimal"
	case "varchar", "char", "text", "longtext", "mediumtext", "tinytext",
		 "character", "character varying":
		return "string"
	case "date", "datetime", "timestamp", "time", "timestamptz", "timetz":
		return "datetime"
	case "boolean", "bool", "bit":
		return "boolean"
	case "json", "jsonb":
		return "json"
	case "uuid":
		return "uuid"
	case "bytea", "blob", "longblob", "mediumblob", "tinyblob", "varbinary", "binary":
		return "binary"
	default:
		return "other"
	}
}

// IsNumeric checks if the column is numeric
func (dc *DatabaseColumn) IsNumeric() bool {
	category := dc.GetDataTypeCategory()
	return category == "integer" || category == "decimal"
}

// IsString checks if the column is a string type
func (dc *DatabaseColumn) IsString() bool {
	return dc.GetDataTypeCategory() == "string"
}

// IsDateTime checks if the column is a date/time type
func (dc *DatabaseColumn) IsDateTime() bool {
	return dc.GetDataTypeCategory() == "datetime"
}