package models

import (
	"time"

	"gorm.io/gorm"
)

// TableType represents the type of database table
type TableType string

const (
	TableTypeTable        TableType = "table"
	TableTypeView         TableType = "view"
	TableTypeMaterialized TableType = "materialized_view"
	TableTypeSequence     TableType = "sequence" 
	TableTypeIndex        TableType = "index"
	TableTypePartition    TableType = "partition"
)

// DatabaseTable represents a table discovered in a database connection
type DatabaseTable struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	UID  string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name string `json:"name" gorm:"type:varchar(255);not null;index"`
	
	// Table metadata
	Schema      string    `json:"schema" gorm:"type:varchar(255);not null;index"`
	Type        TableType `json:"type" gorm:"type:varchar(50);not null;default:'table'"`
	Comment     *string   `json:"comment,omitempty" gorm:"type:text"`
	Engine      *string   `json:"engine,omitempty" gorm:"type:varchar(100)"` // MySQL engine
	Collation   *string   `json:"collation,omitempty" gorm:"type:varchar(100)"`
	
	// Size and row information
	RowCount          *int64  `json:"row_count,omitempty" gorm:"bigint"`
	DataLength        *int64  `json:"data_length,omitempty" gorm:"bigint"`        // Bytes
	IndexLength       *int64  `json:"index_length,omitempty" gorm:"bigint"`       // Bytes
	DataFree          *int64  `json:"data_free,omitempty" gorm:"bigint"`          // Free bytes
	AutoIncrement     *int64  `json:"auto_increment,omitempty" gorm:"bigint"`
	AvgRowLength      *int64  `json:"avg_row_length,omitempty" gorm:"bigint"`
	
	// Table structure hash for change detection
	StructureHash *string `json:"structure_hash,omitempty" gorm:"type:varchar(64)"`
	
	// Discovery metadata
	LastDiscoveredAt time.Time `json:"last_discovered_at" gorm:"not null"`
	DiscoveryError   *string   `json:"discovery_error,omitempty" gorm:"type:text"`
	
	// Backup settings
	IsBackupEnabled   bool     `json:"is_backup_enabled" gorm:"default:false"`
	BackupPriority    int      `json:"backup_priority" gorm:"default:100"` // Lower = higher priority
	ExcludeFromBackup bool     `json:"exclude_from_backup" gorm:"default:false"`
	BackupSchedule    *string  `json:"backup_schedule,omitempty" gorm:"type:varchar(100)"` // Cron expression
	
	// Access control
	HasSelectAccess bool             `json:"has_select_access" gorm:"default:false"`
	AccessLevel     TableAccessLevel `json:"access_level" gorm:"type:varchar(20);default:'none'"`
	
	// Relationships
	DatabaseConnectionID uint               `json:"database_connection_id" gorm:"not null;index"`
	DatabaseConnection   DatabaseConnection `json:"database_connection,omitempty" gorm:"foreignKey:DatabaseConnectionID"`
	
	Columns         []DatabaseColumn     `json:"columns,omitempty" gorm:"foreignKey:DatabaseTableID"`
	Indexes         []DatabaseIndex      `json:"indexes,omitempty" gorm:"foreignKey:DatabaseTableID"`
	ForeignKeys     []DatabaseForeignKey `json:"foreign_keys,omitempty" gorm:"foreignKey:DatabaseTableID"`
	TablePermissions []TablePermission   `json:"table_permissions,omitempty" gorm:"foreignKey:DatabaseTableID"`
	BackupJobs      []BackupJob          `json:"backup_jobs,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// DatabaseColumn represents a column in a database table
type DatabaseColumn struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// Column metadata
	DataType         string  `json:"data_type" gorm:"type:varchar(100);not null"`
	IsNullable       bool    `json:"is_nullable" gorm:"default:true"`
	DefaultValue     *string `json:"default_value,omitempty" gorm:"type:text"`
	MaxLength        *int    `json:"max_length,omitempty"`
	NumericPrecision *int    `json:"numeric_precision,omitempty"`
	NumericScale     *int    `json:"numeric_scale,omitempty"`
	CharacterSet     *string `json:"character_set,omitempty" gorm:"type:varchar(100)"`
	Collation        *string `json:"collation,omitempty" gorm:"type:varchar(100)"`
	Comment          *string `json:"comment,omitempty" gorm:"type:text"`
	
	// Column properties
	IsPrimaryKey   bool `json:"is_primary_key" gorm:"default:false"`
	IsAutoIncrement bool `json:"is_auto_increment" gorm:"default:false"`
	IsUnique       bool `json:"is_unique" gorm:"default:false"`
	IsIndexed      bool `json:"is_indexed" gorm:"default:false"`
	
	// Position in table
	OrdinalPosition int `json:"ordinal_position" gorm:"not null"`
	
	// Relationships
	DatabaseTableID uint          `json:"database_table_id" gorm:"not null;index"`
	DatabaseTable   DatabaseTable `json:"database_table,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// IndexType represents the type of database index
type IndexType string

const (
	IndexTypeBtree   IndexType = "btree"
	IndexTypeHash    IndexType = "hash"
	IndexTypeGin     IndexType = "gin"
	IndexTypeGist    IndexType = "gist"
	IndexTypeBrin    IndexType = "brin"
	IndexTypeSpgist  IndexType = "spgist"
	IndexTypeFulltext IndexType = "fulltext"
	IndexTypeSpatial IndexType = "spatial"
)

// DatabaseIndex represents an index in a database table
type DatabaseIndex struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// Index metadata
	Type      IndexType `json:"type" gorm:"type:varchar(50);not null;default:'btree'"`
	IsUnique  bool      `json:"is_unique" gorm:"default:false"`
	IsPrimary bool      `json:"is_primary" gorm:"default:false"`
	Columns   string    `json:"columns" gorm:"type:text;not null"` // JSON array of column names
	Comment   *string   `json:"comment,omitempty" gorm:"type:text"`
	
	// Size information
	Size *int64 `json:"size,omitempty" gorm:"bigint"` // Index size in bytes
	
	// Relationships
	DatabaseTableID uint          `json:"database_table_id" gorm:"not null;index"`
	DatabaseTable   DatabaseTable `json:"database_table,omitempty" gorm:"foreignKey:DatabaseTableID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// DatabaseForeignKey represents a foreign key constraint
type DatabaseForeignKey struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	
	// Foreign key metadata
	ColumnName           string  `json:"column_name" gorm:"type:varchar(255);not null"`
	ReferencedSchema     string  `json:"referenced_schema" gorm:"type:varchar(255);not null"`
	ReferencedTable      string  `json:"referenced_table" gorm:"type:varchar(255);not null"`
	ReferencedColumn     string  `json:"referenced_column" gorm:"type:varchar(255);not null"`
	OnUpdateAction       string  `json:"on_update_action" gorm:"type:varchar(50);default:'RESTRICT'"`
	OnDeleteAction       string  `json:"on_delete_action" gorm:"type:varchar(50);default:'RESTRICT'"`
	MatchOption          *string `json:"match_option,omitempty" gorm:"type:varchar(50)"`
	IsDeferrable         bool    `json:"is_deferrable" gorm:"default:false"`
	InitiallyDeferred    bool    `json:"initially_deferred" gorm:"default:false"`
	
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

// TableName returns the table name for the DatabaseForeignKey model
func (DatabaseForeignKey) TableName() string {
	return "database_foreign_keys"
}

// BeforeCreate hook to generate UID before creating database table
func (dt *DatabaseTable) BeforeCreate(tx *gorm.DB) error {
	if dt.UID == "" {
		dt.UID = generateUID()
	}
	return nil
}

// GetFullName returns the full table name including schema
func (dt *DatabaseTable) GetFullName() string {
	if dt.Schema != "" && dt.Schema != "public" {
		return dt.Schema + "." + dt.Name
	}
	return dt.Name
}

// GetSizeInBytes returns the total size of the table in bytes
func (dt *DatabaseTable) GetSizeInBytes() int64 {
	var size int64
	if dt.DataLength != nil {
		size += *dt.DataLength
	}
	if dt.IndexLength != nil {
		size += *dt.IndexLength
	}
	return size
}

// GetFormattedSize returns a human-readable size string
func (dt *DatabaseTable) GetFormattedSize() string {
	return formatBytes(dt.GetSizeInBytes())
}

// IsEmpty returns true if the table has no rows
func (dt *DatabaseTable) IsEmpty() bool {
	return dt.RowCount != nil && *dt.RowCount == 0
}

// HasStructureChanged returns true if the table structure has changed
func (dt *DatabaseTable) HasStructureChanged(newHash string) bool {
	return dt.StructureHash == nil || *dt.StructureHash != newHash
}

// UpdateStructureHash updates the structure hash for change detection
func (dt *DatabaseTable) UpdateStructureHash(hash string) {
	dt.StructureHash = &hash
}

// CanBackup returns true if the table can be backed up
func (dt *DatabaseTable) CanBackup() bool {
	return dt.IsBackupEnabled && !dt.ExcludeFromBackup && dt.Type == TableTypeTable && dt.HasSelectAccess
}

// GetBackupPriorityLevel returns a descriptive priority level
func (dt *DatabaseTable) GetBackupPriorityLevel() string {
	switch {
	case dt.BackupPriority <= 25:
		return "critical"
	case dt.BackupPriority <= 50:
		return "high"
	case dt.BackupPriority <= 75:
		return "medium"
	default:
		return "low"
	}
}

// GetActiveBackupJobsCount returns the number of active backup jobs for this table
func (dt *DatabaseTable) GetActiveBackupJobsCount() int {
	count := 0
	for _, job := range dt.BackupJobs {
		if job.Status == BackupStatusPending || job.Status == BackupStatusRunning {
			count++
		}
	}
	return count
}

// HasActiveBackups returns true if there are active backup jobs
func (dt *DatabaseTable) HasActiveBackups() bool {
	return dt.GetActiveBackupJobsCount() > 0
}

// GetColumnCount returns the number of columns in the table
func (dt *DatabaseTable) GetColumnCount() int {
	return len(dt.Columns)
}

// GetIndexCount returns the number of indexes on the table
func (dt *DatabaseTable) GetIndexCount() int {
	return len(dt.Indexes)
}

// GetForeignKeyCount returns the number of foreign keys on the table
func (dt *DatabaseTable) GetForeignKeyCount() int {
	return len(dt.ForeignKeys)
}

// GetPrimaryKeyColumns returns the columns that make up the primary key
func (dt *DatabaseTable) GetPrimaryKeyColumns() []DatabaseColumn {
	var pkColumns []DatabaseColumn
	for _, column := range dt.Columns {
		if column.IsPrimaryKey {
			pkColumns = append(pkColumns, column)
		}
	}
	return pkColumns
}

// HasPrimaryKey returns true if the table has a primary key
func (dt *DatabaseTable) HasPrimaryKey() bool {
	return len(dt.GetPrimaryKeyColumns()) > 0
}

// GetColumnByName returns a column by its name
func (dt *DatabaseTable) GetColumnByName(name string) *DatabaseColumn {
	for i, column := range dt.Columns {
		if column.Name == name {
			return &dt.Columns[i]
		}
	}
	return nil
}

// GetIndexByName returns an index by its name
func (dt *DatabaseTable) GetIndexByName(name string) *DatabaseIndex {
	for i, index := range dt.Indexes {
		if index.Name == name {
			return &dt.Indexes[i]
		}
	}
	return nil
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

// IsView checks if the table is actually a view
func (dt *DatabaseTable) IsView() bool {
	return dt.Type == TableTypeView || dt.Type == TableTypeMaterialized
}

// IsStale checks if the table statistics are stale (older than 24 hours)
func (dt *DatabaseTable) IsStale() bool {
	return time.Since(dt.LastDiscoveredAt) > 24*time.Hour
}

// UpdateStatistics updates the table statistics
func (dt *DatabaseTable) UpdateStatistics(rowCount, dataLength, indexLength int64) {
	dt.RowCount = &rowCount
	dt.DataLength = &dataLength
	dt.IndexLength = &indexLength
	dt.LastDiscoveredAt = time.Now()
}

// ToPublic returns a public representation of the table without sensitive data
func (dt *DatabaseTable) ToPublic() *DatabaseTablePublic {
	public := &DatabaseTablePublic{
		ID:                   dt.ID,
		UID:                  dt.UID,
		Name:                 dt.Name,
		Schema:               dt.Schema,
		Type:                 dt.Type,
		RowCount:             dt.RowCount,
		DataLength:           dt.DataLength,
		IndexLength:          dt.IndexLength,
		AutoIncrement:        dt.AutoIncrement,
		LastDiscoveredAt:     dt.LastDiscoveredAt,
		IsBackupEnabled:      dt.IsBackupEnabled,
		BackupPriority:       dt.BackupPriority,
		ExcludeFromBackup:    dt.ExcludeFromBackup,
		HasSelectAccess:      dt.HasSelectAccess,
		AccessLevel:          dt.AccessLevel,
		FullName:             dt.GetFullName(),
		SizeInBytes:          dt.GetSizeInBytes(),
		FormattedSize:        dt.GetFormattedSize(),
		IsEmpty:              dt.IsEmpty(),
		CanBackup:            dt.CanBackup(),
		BackupPriorityLevel:  dt.GetBackupPriorityLevel(),
		HasActiveBackups:     dt.HasActiveBackups(),
		ColumnCount:          dt.GetColumnCount(),
		IndexCount:           dt.GetIndexCount(),
		ForeignKeyCount:      dt.GetForeignKeyCount(),
		HasPrimaryKey:        dt.HasPrimaryKey(),
		IsView:               dt.IsView(),
		IsStale:              dt.IsStale(),
		CreatedAt:            dt.CreatedAt,
		UpdatedAt:            dt.UpdatedAt,
	}

	if dt.Comment != nil {
		public.Comment = *dt.Comment
	}

	if dt.Engine != nil {
		public.Engine = *dt.Engine
	}

	if dt.Collation != nil {
		public.Collation = *dt.Collation
	}

	if dt.BackupSchedule != nil {
		public.BackupSchedule = *dt.BackupSchedule
	}

	if dt.DiscoveryError != nil {
		public.DiscoveryError = *dt.DiscoveryError
	}

	return public
}

// DatabaseTablePublic represents the public view of a database table
type DatabaseTablePublic struct {
	ID                  uint             `json:"id"`
	UID                 string           `json:"uid"`
	Name                string           `json:"name"`
	Schema              string           `json:"schema"`
	Type                TableType        `json:"type"`
	Comment             string           `json:"comment,omitempty"`
	Engine              string           `json:"engine,omitempty"`
	Collation           string           `json:"collation,omitempty"`
	RowCount            *int64           `json:"row_count,omitempty"`
	DataLength          *int64           `json:"data_length,omitempty"`
	IndexLength         *int64           `json:"index_length,omitempty"`
	AutoIncrement       *int64           `json:"auto_increment,omitempty"`
	LastDiscoveredAt    time.Time        `json:"last_discovered_at"`
	DiscoveryError      string           `json:"discovery_error,omitempty"`
	IsBackupEnabled     bool             `json:"is_backup_enabled"`
	BackupPriority      int              `json:"backup_priority"`
	ExcludeFromBackup   bool             `json:"exclude_from_backup"`
	BackupSchedule      string           `json:"backup_schedule,omitempty"`
	HasSelectAccess     bool             `json:"has_select_access"`
	AccessLevel         TableAccessLevel `json:"access_level"`
	FullName            string           `json:"full_name"`
	SizeInBytes         int64            `json:"size_in_bytes"`
	FormattedSize       string           `json:"formatted_size"`
	IsEmpty             bool             `json:"is_empty"`
	CanBackup           bool             `json:"can_backup"`
	BackupPriorityLevel string           `json:"backup_priority_level"`
	HasActiveBackups    bool             `json:"has_active_backups"`
	ColumnCount         int              `json:"column_count"`
	IndexCount          int              `json:"index_count"`
	ForeignKeyCount     int              `json:"foreign_key_count"`
	HasPrimaryKey       bool             `json:"has_primary_key"`
	IsView              bool             `json:"is_view"`
	IsStale             bool             `json:"is_stale"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`
}

// TableDiscoveryRequest represents a request to discover tables in a database
type TableDiscoveryRequest struct {
	SchemaPattern *string `json:"schema_pattern,omitempty" validate:"max=255"`
	TablePattern  *string `json:"table_pattern,omitempty" validate:"max=255"`
	IncludeViews  bool    `json:"include_views"`
	IncludeSystem bool    `json:"include_system"`
}

// TableUpdateRequest represents a request to update table settings
type TableUpdateRequest struct {
	IsBackupEnabled   *bool   `json:"is_backup_enabled,omitempty"`
	BackupPriority    *int    `json:"backup_priority,omitempty" validate:"omitempty,min=1,max=999"`
	ExcludeFromBackup *bool   `json:"exclude_from_backup,omitempty"`
	BackupSchedule    *string `json:"backup_schedule,omitempty" validate:"omitempty,max=100"`
}

// TableListResponse represents the response for listing tables
type TableListResponse struct {
	Tables  []*DatabaseTablePublic `json:"tables"`
	Count   int                    `json:"count"`
	Filters TableListFilters       `json:"filters"`
}

// TableListFilters represents the filters applied when listing tables
type TableListFilters struct {
	IncludeViews  bool   `json:"include_views"`
	IncludeSystem bool   `json:"include_system"`
	SchemaPattern string `json:"schema_pattern,omitempty"`
	TablePattern  string `json:"table_pattern,omitempty"`
}

// TableDiscoveryResponse represents the response for discovering and saving tables
type TableDiscoveryResponse struct {
	Tables []*DatabaseTablePublic `json:"tables"`
	Count  int                    `json:"count"`
}

// ApplyUpdate applies the update request to the table
func (dt *DatabaseTable) ApplyUpdate(req *TableUpdateRequest) {
	if req.IsBackupEnabled != nil {
		dt.IsBackupEnabled = *req.IsBackupEnabled
	}
	if req.BackupPriority != nil {
		dt.BackupPriority = *req.BackupPriority
	}
	if req.ExcludeFromBackup != nil {
		dt.ExcludeFromBackup = *req.ExcludeFromBackup
	}
	if req.BackupSchedule != nil {
		if *req.BackupSchedule == "" {
			dt.BackupSchedule = nil
		} else {
			dt.BackupSchedule = req.BackupSchedule
		}
	}
}

// IsValid checks if the table type is valid
func (tt TableType) IsValid() bool {
	switch tt {
	case TableTypeTable, TableTypeView, TableTypeMaterialized, 
		 TableTypeSequence, TableTypeIndex, TableTypePartition:
		return true
	default:
		return false
	}
}

// GetDisplayName returns a human-readable name for the table type
func (tt TableType) GetDisplayName() string {
	switch tt {
	case TableTypeTable:
		return "Table"
	case TableTypeView:
		return "View"
	case TableTypeMaterialized:
		return "Materialized View"
	case TableTypeSequence:
		return "Sequence"
	case TableTypeIndex:
		return "Index"
	case TableTypePartition:
		return "Partition"
	default:
		return string(tt)
	}
}

// IsValid checks if the index type is valid
func (it IndexType) IsValid() bool {
	switch it {
	case IndexTypeBtree, IndexTypeHash, IndexTypeGin, IndexTypeGist,
		 IndexTypeBrin, IndexTypeSpgist, IndexTypeFulltext, IndexTypeSpatial:
		return true
	default:
		return false
	}
}

// GetDisplayName returns a human-readable name for the index type
func (it IndexType) GetDisplayName() string {
	switch it {
	case IndexTypeBtree:
		return "B-Tree"
	case IndexTypeHash:
		return "Hash"
	case IndexTypeGin:
		return "GIN"
	case IndexTypeGist:
		return "GiST"
	case IndexTypeBrin:
		return "BRIN"
	case IndexTypeSpgist:
		return "SP-GiST"
	case IndexTypeFulltext:
		return "Full Text"
	case IndexTypeSpatial:
		return "Spatial"
	default:
		return string(it)
	}
}

// IsValid checks if the table access level is valid
func (tal TableAccessLevel) IsValid() bool {
	switch tal {
	case TableAccessNone, TableAccessRead, TableAccessWrite, TableAccessAdmin:
		return true
	default:
		return false
	}
}

// GetDisplayName returns a human-readable name for the table access level
func (tal TableAccessLevel) GetDisplayName() string {
	switch tal {
	case TableAccessNone:
		return "No Access"
	case TableAccessRead:
		return "Read Only"
	case TableAccessWrite:
		return "Read/Write"
	case TableAccessAdmin:
		return "Administrator"
	default:
		return string(tal)
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

