package models

import (
	"database/sql/driver"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MigrationStatus represents the status of a migration
type MigrationStatus string

const (
	MigrationStatusPending   MigrationStatus = "pending"
	MigrationStatusApplied   MigrationStatus = "applied"
	MigrationStatusFailed    MigrationStatus = "failed"
	MigrationStatusRollback  MigrationStatus = "rollback"
)

// Value implements the driver.Valuer interface for MigrationStatus
func (ms MigrationStatus) Value() (driver.Value, error) {
	return string(ms), nil
}

// Scan implements the sql.Scanner interface for MigrationStatus
func (ms *MigrationStatus) Scan(value interface{}) error {
	if value == nil {
		*ms = MigrationStatusPending
		return nil
	}

	switch s := value.(type) {
	case string:
		*ms = MigrationStatus(s)
	case []byte:
		*ms = MigrationStatus(s)
	default:
		return fmt.Errorf("cannot scan %T into MigrationStatus", value)
	}

	return nil
}

// Migration represents a database migration record
type Migration struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Version     string          `gorm:"uniqueIndex;not null" json:"version"`
	Name        string          `gorm:"not null" json:"name"`
	Description string          `json:"description"`
	Status      MigrationStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`
	AppliedAt   *time.Time      `json:"applied_at,omitempty"`
	RolledAt    *time.Time      `json:"rolled_at,omitempty"`
	Checksum    string          `gorm:"not null" json:"checksum"`
	ExecutionTime int64         `gorm:"default:0" json:"execution_time"` // in milliseconds
	ErrorMessage  string        `json:"error_message,omitempty"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName returns the table name for the Migration model
func (Migration) TableName() string {
	return "schema_migrations"
}

// BeforeCreate GORM hook called before creating a migration record
func (m *Migration) BeforeCreate(tx *gorm.DB) error {
	if m.Status == "" {
		m.Status = MigrationStatusPending
	}
	return nil
}

// MarkApplied marks the migration as successfully applied
func (m *Migration) MarkApplied(executionTime time.Duration) {
	now := time.Now()
	m.Status = MigrationStatusApplied
	m.AppliedAt = &now
	m.ExecutionTime = executionTime.Milliseconds()
	m.ErrorMessage = ""
}

// MarkFailed marks the migration as failed with an error message
func (m *Migration) MarkFailed(errMsg string) {
	m.Status = MigrationStatusFailed
	m.ErrorMessage = errMsg
	m.AppliedAt = nil
}

// MarkRollback marks the migration as rolled back
func (m *Migration) MarkRollback() {
	now := time.Now()
	m.Status = MigrationStatusRollback
	m.RolledAt = &now
	m.AppliedAt = nil
	m.ErrorMessage = ""
}

// IsApplied returns true if the migration has been successfully applied
func (m *Migration) IsApplied() bool {
	return m.Status == MigrationStatusApplied
}

// IsFailed returns true if the migration failed
func (m *Migration) IsFailed() bool {
	return m.Status == MigrationStatusFailed
}

// IsRolledBack returns true if the migration was rolled back
func (m *Migration) IsRolledBack() bool {
	return m.Status == MigrationStatusRollback
}

// MigrationBatch represents a group of migrations executed together
type MigrationBatch struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	BatchNumber int       `gorm:"uniqueIndex;not null" json:"batch_number"`
	StartedAt   time.Time `gorm:"autoCreateTime" json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string    `gorm:"type:varchar(20);default:'running'" json:"status"`
	TotalCount  int       `gorm:"default:0" json:"total_count"`
	SuccessCount int      `gorm:"default:0" json:"success_count"`
	FailureCount int      `gorm:"default:0" json:"failure_count"`
}

// TableName returns the table name for the MigrationBatch model
func (MigrationBatch) TableName() string {
	return "migration_batches"
}

// MarkCompleted marks the batch as completed
func (mb *MigrationBatch) MarkCompleted() {
	now := time.Now()
	mb.CompletedAt = &now
	if mb.FailureCount == 0 {
		mb.Status = "completed"
	} else {
		mb.Status = "partial"
	}
}