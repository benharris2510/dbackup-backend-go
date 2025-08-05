package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AuditAction represents the type of action being audited
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionRead   AuditAction = "read"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionLogin  AuditAction = "login"
	AuditActionLogout AuditAction = "logout"
	AuditActionBackup AuditAction = "backup"
	AuditActionRestore AuditAction = "restore"
	AuditActionExport AuditAction = "export"
	AuditActionImport AuditAction = "import"
	AuditActionPermissionGrant AuditAction = "permission_grant"
	AuditActionPermissionRevoke AuditAction = "permission_revoke"
)

// AuditResource represents the type of resource being accessed
type AuditResource string

const (
	AuditResourceUser               AuditResource = "user"
	AuditResourceTeam               AuditResource = "team"
	AuditResourceDatabaseConnection AuditResource = "database_connection"
	AuditResourceDatabaseTable      AuditResource = "database_table"
	AuditResourceBackupJob          AuditResource = "backup_job"
	AuditResourceBackupFile         AuditResource = "backup_file"
	AuditResourceTablePermission    AuditResource = "table_permission"
	AuditResourceStorageConfig      AuditResource = "storage_config"
	AuditResourceSession            AuditResource = "session"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID uint `json:"id" gorm:"primaryKey"`
	
	// Action details
	Action      AuditAction   `json:"action" gorm:"type:varchar(50);not null;index"`
	Resource    AuditResource `json:"resource" gorm:"type:varchar(50);not null;index"`
	ResourceID  *uint         `json:"resource_id,omitempty" gorm:"index"`
	ResourceUID *string       `json:"resource_uid,omitempty" gorm:"type:varchar(36);index"`
	
	// Request details
	Method      string  `json:"method" gorm:"type:varchar(10);not null"`        // HTTP method
	Path        string  `json:"path" gorm:"type:varchar(500);not null"`         // Request path
	UserAgent   *string `json:"user_agent,omitempty" gorm:"type:text"`
	IPAddress   string  `json:"ip_address" gorm:"type:varchar(45);not null;index"`
	SessionID   *string `json:"session_id,omitempty" gorm:"type:varchar(255);index"`
	RequestID   *string `json:"request_id,omitempty" gorm:"type:varchar(255)"`
	
	// Response details
	StatusCode   int     `json:"status_code" gorm:"not null"`
	Duration     int64   `json:"duration" gorm:"not null"` // Duration in milliseconds
	ErrorMessage *string `json:"error_message,omitempty" gorm:"type:text"`
	
	// Data changes
	OldValues map[string]interface{} `json:"old_values,omitempty" gorm:"type:json"`
	NewValues map[string]interface{} `json:"new_values,omitempty" gorm:"type:json"`
	Changes   []string               `json:"changes,omitempty" gorm:"type:json"` // List of changed fields
	
	// Additional metadata
	Metadata    map[string]interface{} `json:"metadata,omitempty" gorm:"type:json"`
	Description *string                `json:"description,omitempty" gorm:"type:text"`
	
	// Risk assessment
	RiskLevel    string  `json:"risk_level" gorm:"type:varchar(20);default:'low';index"` // low, medium, high, critical
	IsSuspicious bool    `json:"is_suspicious" gorm:"default:false;index"`
	IsBlocked    bool    `json:"is_blocked" gorm:"default:false"`
	
	// Relationships
	UserID *uint `json:"user_id,omitempty" gorm:"index"`
	User   *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	TeamID *uint `json:"team_id,omitempty" gorm:"index"`
	Team   *Team `json:"team,omitempty" gorm:"foreignKey:TeamID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the AuditLog model
func (AuditLog) TableName() string {
	return "audit_logs"
}

// IsSuccess checks if the action was successful
func (al *AuditLog) IsSuccess() bool {
	return al.StatusCode >= 200 && al.StatusCode < 300
}

// IsError checks if the action resulted in an error
func (al *AuditLog) IsError() bool {
	return al.StatusCode >= 400
}

// GetFormattedDuration returns a human-readable duration string
func (al *AuditLog) GetFormattedDuration() string {
	if al.Duration < 1000 {
		return fmt.Sprintf("%dms", al.Duration)
	}
	return fmt.Sprintf("%.2fs", float64(al.Duration)/1000)
}

// GetRiskLevelColor returns a color code for the risk level
func (al *AuditLog) GetRiskLevelColor() string {
	switch al.RiskLevel {
	case "critical":
		return "#DC2626" // Red
	case "high":
		return "#EA580C" // Orange
	case "medium":
		return "#D97706" // Amber
	case "low":
		return "#059669" // Green
	default:
		return "#6B7280" // Gray
	}
}

// GetActionDescription returns a human-readable description of the action
func (al *AuditLog) GetActionDescription() string {
	switch al.Action {
	case AuditActionCreate:
		return "Created"
	case AuditActionRead:
		return "Viewed"
	case AuditActionUpdate:
		return "Updated"
	case AuditActionDelete:
		return "Deleted"
	case AuditActionLogin:
		return "Logged in"
	case AuditActionLogout:
		return "Logged out"
	case AuditActionBackup:
		return "Created backup"
	case AuditActionRestore:
		return "Restored from backup"
	case AuditActionExport:
		return "Exported data"
	case AuditActionImport:
		return "Imported data"
	case AuditActionPermissionGrant:
		return "Granted permission"
	case AuditActionPermissionRevoke:
		return "Revoked permission"
	default:
		return string(al.Action)
	}
}

// GetResourceDescription returns a human-readable description of the resource
func (al *AuditLog) GetResourceDescription() string {
	switch al.Resource {
	case AuditResourceUser:
		return "User"
	case AuditResourceTeam:
		return "Team"
	case AuditResourceDatabaseConnection:
		return "Database Connection"
	case AuditResourceDatabaseTable:
		return "Database Table"
	case AuditResourceBackupJob:
		return "Backup Job"
	case AuditResourceBackupFile:
		return "Backup File"
	case AuditResourceTablePermission:
		return "Table Permission"
	case AuditResourceStorageConfig:
		return "Storage Configuration"
	case AuditResourceSession:
		return "Session"
	default:
		return string(al.Resource)
	}
}

// HasChanges checks if the audit log contains data changes
func (al *AuditLog) HasChanges() bool {
	return len(al.Changes) > 0 || len(al.OldValues) > 0 || len(al.NewValues) > 0
}

// GetChangedFields returns the list of changed fields
func (al *AuditLog) GetChangedFields() []string {
	if len(al.Changes) > 0 {
		return al.Changes
	}
	
	// Extract changed fields from old/new values
	var changes []string
	for field := range al.NewValues {
		if _, exists := al.OldValues[field]; exists {
			if al.OldValues[field] != al.NewValues[field] {
				changes = append(changes, field)
			}
		} else {
			changes = append(changes, field)
		}
	}
	
	return changes
}

// SetMetadata sets a metadata value
func (al *AuditLog) SetMetadata(key string, value interface{}) {
	if al.Metadata == nil {
		al.Metadata = make(map[string]interface{})
	}
	al.Metadata[key] = value
}

// GetMetadata gets a metadata value
func (al *AuditLog) GetMetadata(key string) (interface{}, bool) {
	if al.Metadata == nil {
		return nil, false
	}
	value, exists := al.Metadata[key]
	return value, exists
}

// SetChanges sets the old and new values for change tracking
func (al *AuditLog) SetChanges(oldValues, newValues map[string]interface{}) {
	al.OldValues = oldValues
	al.NewValues = newValues
	
	// Calculate changed fields
	var changes []string
	for field, newValue := range newValues {
		if oldValue, exists := oldValues[field]; exists {
			if oldValue != newValue {
				changes = append(changes, field)
			}
		} else {
			changes = append(changes, field)
		}
	}
	
	al.Changes = changes
}

// MarkAsSuspicious marks the audit log as suspicious
func (al *AuditLog) MarkAsSuspicious(reason string) {
	al.IsSuspicious = true
	al.SetMetadata("suspicious_reason", reason)
	
	// Automatically increase risk level
	if al.RiskLevel == "low" {
		al.RiskLevel = "medium"
	}
}

// Block marks the audit log as blocked
func (al *AuditLog) Block(reason string) {
	al.IsBlocked = true
	al.SetMetadata("blocked_reason", reason)
	al.RiskLevel = "high"
}

// GetAge returns the age of the audit log entry
func (al *AuditLog) GetAge() time.Duration {
	return time.Since(al.CreatedAt)
}

// IsRecent checks if the audit log entry is recent (within specified duration)
func (al *AuditLog) IsRecent(duration time.Duration) bool {
	return al.GetAge() < duration
}

// GetSummary returns a summary of the audit log entry
func (al *AuditLog) GetSummary() string {
	action := al.GetActionDescription()
	resource := al.GetResourceDescription()
	
	summary := fmt.Sprintf("%s %s", action, resource)
	
	if al.ResourceUID != nil {
		summary += fmt.Sprintf(" (%s)", *al.ResourceUID)
	}
	
	return summary
}

// GetLocationInfo returns location information from IP address
func (al *AuditLog) GetLocationInfo() map[string]interface{} {
	location := make(map[string]interface{})
	location["ip_address"] = al.IPAddress
	
	// This would typically integrate with a GeoIP service
	// For now, just return the IP address
	if locationData, exists := al.GetMetadata("location"); exists {
		if loc, ok := locationData.(map[string]interface{}); ok {
			for k, v := range loc {
				location[k] = v
			}
		}
	}
	
	return location
}

// IsFromSameSession checks if this audit log is from the same session as another
func (al *AuditLog) IsFromSameSession(other *AuditLog) bool {
	return al.SessionID != nil && other.SessionID != nil && *al.SessionID == *other.SessionID
}

// IsFromSameUser checks if this audit log is from the same user as another
func (al *AuditLog) IsFromSameUser(other *AuditLog) bool {
	return al.UserID != nil && other.UserID != nil && *al.UserID == *other.UserID
}