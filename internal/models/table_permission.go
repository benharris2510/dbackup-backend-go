package models

import (
	"time"

	"gorm.io/gorm"
)

// TablePermission represents user permissions for specific database tables
type TablePermission struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	AccessLevel TableAccessLevel `json:"access_level" gorm:"type:varchar(20);not null"`
	
	// Permission details
	CanRead   bool `json:"can_read" gorm:"default:false"`
	CanWrite  bool `json:"can_write" gorm:"default:false"`
	CanBackup bool `json:"can_backup" gorm:"default:false"`
	CanRestore bool `json:"can_restore" gorm:"default:false"`
	CanDelete  bool `json:"can_delete" gorm:"default:false"`
	
	// Conditions and restrictions
	RowLevelFilter *string `json:"row_level_filter,omitempty" gorm:"type:text"` // SQL WHERE clause
	ColumnMask     []string `json:"column_mask" gorm:"type:json"`               // Columns to mask/hide
	TimeRestriction *string `json:"time_restriction,omitempty" gorm:"type:varchar(255)"` // Time-based access
	
	// Metadata
	GrantedBy   *uint   `json:"granted_by,omitempty" gorm:"index"`             // User ID who granted permission
	GrantedAt   time.Time `json:"granted_at" gorm:"autoCreateTime"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Description *string `json:"description,omitempty" gorm:"type:text"`
	
	// Relationships
	UserID          uint          `json:"user_id" gorm:"not null;index"`
	User            User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	DatabaseTableID uint          `json:"database_table_id" gorm:"not null;index"`
	DatabaseTable   DatabaseTable `json:"database_table,omitempty" gorm:"foreignKey:DatabaseTableID"`
	GrantedByUser   *User         `json:"granted_by_user,omitempty" gorm:"foreignKey:GrantedBy"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the TablePermission model
func (TablePermission) TableName() string {
	return "table_permissions"
}

// IsExpired checks if the permission has expired
func (tp *TablePermission) IsExpired() bool {
	return tp.ExpiresAt != nil && tp.ExpiresAt.Before(time.Now())
}

// IsActive checks if the permission is currently active
func (tp *TablePermission) IsActive() bool {
	return !tp.IsExpired() && tp.DeletedAt.Time.IsZero()
}

// HasPermission checks if the permission includes a specific action
func (tp *TablePermission) HasPermission(action string) bool {
	if !tp.IsActive() {
		return false
	}
	
	switch action {
	case "read":
		return tp.CanRead
	case "write":
		return tp.CanWrite
	case "backup":
		return tp.CanBackup
	case "restore":
		return tp.CanRestore
	case "delete":
		return tp.CanDelete
	default:
		return false
	}
}

// GetPermissionSummary returns a summary of granted permissions
func (tp *TablePermission) GetPermissionSummary() []string {
	var permissions []string
	
	if tp.CanRead {
		permissions = append(permissions, "read")
	}
	if tp.CanWrite {
		permissions = append(permissions, "write")
	}
	if tp.CanBackup {
		permissions = append(permissions, "backup")
	}
	if tp.CanRestore {
		permissions = append(permissions, "restore")
	}
	if tp.CanDelete {
		permissions = append(permissions, "delete")
	}
	
	return permissions
}

// SetPermissionLevel sets permissions based on access level
func (tp *TablePermission) SetPermissionLevel(level TableAccessLevel) {
	tp.AccessLevel = level
	
	switch level {
	case TableAccessNone:
		tp.CanRead = false
		tp.CanWrite = false
		tp.CanBackup = false
		tp.CanRestore = false
		tp.CanDelete = false
	case TableAccessRead:
		tp.CanRead = true
		tp.CanWrite = false
		tp.CanBackup = true
		tp.CanRestore = false
		tp.CanDelete = false
	case TableAccessWrite:
		tp.CanRead = true
		tp.CanWrite = true
		tp.CanBackup = true
		tp.CanRestore = true
		tp.CanDelete = false
	case TableAccessAdmin:
		tp.CanRead = true
		tp.CanWrite = true
		tp.CanBackup = true
		tp.CanRestore = true
		tp.CanDelete = true
	}
}

// IsRestrictedByTime checks if access is restricted by time
func (tp *TablePermission) IsRestrictedByTime() bool {
	return tp.TimeRestriction != nil && *tp.TimeRestriction != ""
}

// HasColumnMask checks if columns are masked
func (tp *TablePermission) HasColumnMask() bool {
	return len(tp.ColumnMask) > 0
}

// HasRowLevelFilter checks if row-level filtering is applied
func (tp *TablePermission) HasRowLevelFilter() bool {
	return tp.RowLevelFilter != nil && *tp.RowLevelFilter != ""
}

// GetMaskedColumns returns the list of masked columns
func (tp *TablePermission) GetMaskedColumns() []string {
	if tp.ColumnMask == nil {
		return []string{}
	}
	return tp.ColumnMask
}

// AddMaskedColumn adds a column to the mask list
func (tp *TablePermission) AddMaskedColumn(column string) {
	if tp.ColumnMask == nil {
		tp.ColumnMask = []string{}
	}
	
	// Check if column is already masked
	for _, masked := range tp.ColumnMask {
		if masked == column {
			return
		}
	}
	
	tp.ColumnMask = append(tp.ColumnMask, column)
}

// RemoveMaskedColumn removes a column from the mask list
func (tp *TablePermission) RemoveMaskedColumn(column string) {
	if tp.ColumnMask == nil {
		return
	}
	
	for i, masked := range tp.ColumnMask {
		if masked == column {
			tp.ColumnMask = append(tp.ColumnMask[:i], tp.ColumnMask[i+1:]...)
			return
		}
	}
}

// SetRowLevelFilter sets the row-level security filter
func (tp *TablePermission) SetRowLevelFilter(filter string) {
	if filter == "" {
		tp.RowLevelFilter = nil
	} else {
		tp.RowLevelFilter = &filter
	}
}

// SetTimeRestriction sets time-based access restriction
func (tp *TablePermission) SetTimeRestriction(restriction string) {
	if restriction == "" {
		tp.TimeRestriction = nil
	} else {
		tp.TimeRestriction = &restriction
	}
}

// SetExpiration sets the expiration time for the permission
func (tp *TablePermission) SetExpiration(expiresAt time.Time) {
	tp.ExpiresAt = &expiresAt
}

// RemoveExpiration removes the expiration time
func (tp *TablePermission) RemoveExpiration() {
	tp.ExpiresAt = nil
}

// GetRemainingTime returns the time remaining before expiration
func (tp *TablePermission) GetRemainingTime() *time.Duration {
	if tp.ExpiresAt == nil {
		return nil
	}
	
	remaining := time.Until(*tp.ExpiresAt)
	return &remaining
}

// IsGrantedBy checks if the permission was granted by a specific user
func (tp *TablePermission) IsGrantedBy(userID uint) bool {
	return tp.GrantedBy != nil && *tp.GrantedBy == userID
}

// CanBeRevokedBy checks if the permission can be revoked by a specific user
func (tp *TablePermission) CanBeRevokedBy(userID uint) bool {
	// Can be revoked by the user who granted it or by an admin
	return tp.IsGrantedBy(userID) || (tp.GrantedByUser != nil && tp.GrantedByUser.IsAdmin)
}

// GetAccessLevelDescription returns a human-readable description of the access level
func (tp *TablePermission) GetAccessLevelDescription() string {
	switch tp.AccessLevel {
	case TableAccessNone:
		return "No access"
	case TableAccessRead:
		return "Read-only access"
	case TableAccessWrite:
		return "Read and write access"
	case TableAccessAdmin:
		return "Full administrative access"
	default:
		return "Unknown access level"
	}
}

// Clone creates a copy of the table permission
func (tp *TablePermission) Clone() *TablePermission {
	clone := *tp
	clone.ID = 0 // Reset ID for new record
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.DeletedAt = gorm.DeletedAt{}
	return &clone
}