package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UID       string         `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Email     string         `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	FirstName string         `json:"first_name" gorm:"type:varchar(100);not null"`
	LastName  string         `json:"last_name" gorm:"type:varchar(100);not null"`
	Password  string         `json:"-" gorm:"type:varchar(255);not null"`
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	IsAdmin   bool           `json:"is_admin" gorm:"default:false"`
	
	// Email verification
	IsEmailVerified    bool       `json:"is_email_verified" gorm:"default:false"`
	EmailVerifyToken   *string    `json:"-" gorm:"type:varchar(255)"`
	EmailVerifyExpires *time.Time `json:"-"`
	
	// Password reset
	PasswordResetToken   *string    `json:"-" gorm:"type:varchar(255)"`
	PasswordResetExpires *time.Time `json:"-"`
	
	// 2FA settings
	TwoFactorEnabled    bool    `json:"two_factor_enabled" gorm:"default:false"`
	TwoFactorSecret     *string `json:"-" gorm:"type:varchar(255)"`
	TwoFactorBackupCode *string `json:"-" gorm:"type:varchar(255)"`
	
	// Avatar and profile
	Avatar   *string `json:"avatar" gorm:"type:text"`
	Timezone string  `json:"timezone" gorm:"type:varchar(50);default:'UTC'"`
	
	// Subscription and limits
	SubscriptionTier     string `json:"subscription_tier" gorm:"type:varchar(50);default:'free'"`
	MaxDatabaseConnections int  `json:"max_database_connections" gorm:"default:2"`
	MaxBackupSize        int64  `json:"max_backup_size" gorm:"default:5368709120"` // 5GB in bytes
	
	// Activity tracking
	LastLoginAt    *time.Time `json:"last_login_at"`
	LastLoginIP    *string    `json:"last_login_ip" gorm:"type:varchar(45)"`
	LoginAttempts  int        `json:"login_attempts" gorm:"default:0"`
	LockedUntil    *time.Time `json:"locked_until"`
	
	// Team relationships
	TeamMemberships []TeamMember `json:"team_memberships,omitempty" gorm:"foreignKey:UserID"`
	
	// Relationships
	DatabaseConnections []DatabaseConnection `json:"database_connections,omitempty" gorm:"foreignKey:UserID"`
	BackupJobs         []BackupJob          `json:"backup_jobs,omitempty" gorm:"foreignKey:UserID"`
	AuditLogs          []AuditLog           `json:"audit_logs,omitempty" gorm:"foreignKey:UserID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook to generate UID before creating user
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.UID == "" {
		u.UID = generateUID()
	}
	return nil
}

// IsLocked checks if the user account is currently locked
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// CanLogin checks if user can attempt to login
func (u *User) CanLogin() bool {
	return u.IsActive && !u.IsLocked()
}

// IncrementLoginAttempts increments the failed login attempts counter
func (u *User) IncrementLoginAttempts() {
	u.LoginAttempts++
	if u.LoginAttempts >= 5 {
		lockDuration := time.Hour
		lockUntil := time.Now().Add(lockDuration)
		u.LockedUntil = &lockUntil
	}
}

// ResetLoginAttempts resets the failed login attempts counter
func (u *User) ResetLoginAttempts() {
	u.LoginAttempts = 0
	u.LockedUntil = nil
}

// SetLastLogin updates the last login timestamp and IP
func (u *User) SetLastLogin(ip string) {
	now := time.Now()
	u.LastLoginAt = &now
	u.LastLoginIP = &ip
}

// GetFullName returns the user's full name
func (u *User) GetFullName() string {
	return u.FirstName + " " + u.LastName
}

// HasSubscription checks if user has a specific subscription tier
func (u *User) HasSubscription(tier string) bool {
	return u.SubscriptionTier == tier
}

// CanCreateDatabaseConnection checks if user can create more database connections
func (u *User) CanCreateDatabaseConnection() bool {
	return len(u.DatabaseConnections) < u.MaxDatabaseConnections
}

// GetRemainingConnections returns the number of database connections user can still create
func (u *User) GetRemainingConnections() int {
	remaining := u.MaxDatabaseConnections - len(u.DatabaseConnections)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CanCreateBackupOfSize checks if user can create a backup of the given size
func (u *User) CanCreateBackupOfSize(size int64) bool {
	return size <= u.MaxBackupSize
}

// IsEmailVerificationExpired checks if email verification token has expired
func (u *User) IsEmailVerificationExpired() bool {
	return u.EmailVerifyExpires != nil && u.EmailVerifyExpires.Before(time.Now())
}

// IsPasswordResetExpired checks if password reset token has expired
func (u *User) IsPasswordResetExpired() bool {
	return u.PasswordResetExpires != nil && u.PasswordResetExpires.Before(time.Now())
}

// ClearEmailVerification clears email verification fields
func (u *User) ClearEmailVerification() {
	u.EmailVerifyToken = nil
	u.EmailVerifyExpires = nil
	u.IsEmailVerified = true
}

// ClearPasswordReset clears password reset fields
func (u *User) ClearPasswordReset() {
	u.PasswordResetToken = nil
	u.PasswordResetExpires = nil
}

// Enable2FA enables two-factor authentication for the user
func (u *User) Enable2FA(secret, backupCode string) {
	u.TwoFactorEnabled = true
	u.TwoFactorSecret = &secret
	u.TwoFactorBackupCode = &backupCode
}

// Disable2FA disables two-factor authentication for the user
func (u *User) Disable2FA() {
	u.TwoFactorEnabled = false
	u.TwoFactorSecret = nil
	u.TwoFactorBackupCode = nil
}

// GetSubscriptionLimits returns the limits for the user's subscription tier
func (u *User) GetSubscriptionLimits() SubscriptionLimits {
	switch u.SubscriptionTier {
	case "pro":
		return SubscriptionLimits{
			MaxDatabaseConnections: 10,
			MaxBackupSize:         53687091200, // 50GB
			RetentionDays:         30,
			BackupFrequency:       "hourly",
		}
	case "business":
		return SubscriptionLimits{
			MaxDatabaseConnections: 50,
			MaxBackupSize:         536870912000, // 500GB
			RetentionDays:         90,
			BackupFrequency:       "realtime",
		}
	case "enterprise":
		return SubscriptionLimits{
			MaxDatabaseConnections: -1, // unlimited
			MaxBackupSize:         -1,  // unlimited
			RetentionDays:         365,
			BackupFrequency:       "realtime",
		}
	default: // free tier
		return SubscriptionLimits{
			MaxDatabaseConnections: 2,
			MaxBackupSize:         5368709120, // 5GB
			RetentionDays:         7,
			BackupFrequency:       "daily",
		}
	}
}

// SubscriptionLimits represents the limits for a subscription tier
type SubscriptionLimits struct {
	MaxDatabaseConnections int
	MaxBackupSize         int64
	RetentionDays         int
	BackupFrequency       string
}