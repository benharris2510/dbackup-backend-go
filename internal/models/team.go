package models

import (
	"time"

	"gorm.io/gorm"
)

// TeamRole represents different roles within a team
type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
	TeamRoleGuest  TeamRole = "guest"
)

// Team represents a team/organization in the system
type Team struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	UID  string `json:"uid" gorm:"type:varchar(36);uniqueIndex;not null"`
	Name string `json:"name" gorm:"type:varchar(255);not null"`
	Slug string `json:"slug" gorm:"type:varchar(100);uniqueIndex;not null"`
	
	// Contact information
	Email   *string `json:"email" gorm:"type:varchar(255)"`
	Website *string `json:"website" gorm:"type:varchar(255)"`
	Phone   *string `json:"phone" gorm:"type:varchar(50)"`
	
	// Address
	Address    *string `json:"address" gorm:"type:text"`
	City       *string `json:"city" gorm:"type:varchar(100)"`
	State      *string `json:"state" gorm:"type:varchar(100)"`
	PostalCode *string `json:"postal_code" gorm:"type:varchar(20)"`
	Country    *string `json:"country" gorm:"type:varchar(100)"`
	
	// Subscription and billing
	SubscriptionTier   string     `json:"subscription_tier" gorm:"type:varchar(50);default:'free'"`
	SubscriptionStatus string     `json:"subscription_status" gorm:"type:varchar(50);default:'active'"`
	BillingEmail       *string    `json:"billing_email" gorm:"type:varchar(255)"`
	TrialEndsAt        *time.Time `json:"trial_ends_at"`
	
	// Settings
	IsActive           bool `json:"is_active" gorm:"default:true"`
	MaxUsers           int  `json:"max_users" gorm:"default:5"`
	MaxDatabaseConnections int `json:"max_database_connections" gorm:"default:10"`
	MaxStorageGB       int  `json:"max_storage_gb" gorm:"default:100"`
	
	// Relationships
	Members             []TeamMember         `json:"members,omitempty" gorm:"foreignKey:TeamID"`
	DatabaseConnections []DatabaseConnection `json:"database_connections,omitempty" gorm:"foreignKey:TeamID"`
	StorageConfigurations []StorageConfiguration `json:"storage_configurations,omitempty" gorm:"foreignKey:TeamID"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	ID     uint     `json:"id" gorm:"primaryKey"`
	Role   TeamRole `json:"role" gorm:"type:varchar(20);not null;default:'member'"`
	
	// Permissions
	CanInviteMembers     bool `json:"can_invite_members" gorm:"default:false"`
	CanManageBackups     bool `json:"can_manage_backups" gorm:"default:true"`
	CanManageConnections bool `json:"can_manage_connections" gorm:"default:false"`
	CanViewAuditLogs     bool `json:"can_view_audit_logs" gorm:"default:false"`
	CanManageBilling     bool `json:"can_manage_billing" gorm:"default:false"`
	
	// Status
	IsActive   bool       `json:"is_active" gorm:"default:true"`
	InvitedAt  *time.Time `json:"invited_at"`
	JoinedAt   *time.Time `json:"joined_at"`
	InvitedBy  *uint      `json:"invited_by,omitempty" gorm:"index"`
	
	// Relationships
	TeamID   uint `json:"team_id" gorm:"not null;index"`
	Team     Team `json:"team,omitempty" gorm:"foreignKey:TeamID"`
	UserID   uint `json:"user_id" gorm:"not null;index"`
	User     User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Inviter  *User `json:"inviter,omitempty" gorm:"foreignKey:InvitedBy"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the Team model
func (Team) TableName() string {
	return "teams"
}

// TableName returns the table name for the TeamMember model
func (TeamMember) TableName() string {
	return "team_members"
}

// BeforeCreate hook to generate UID before creating team
func (t *Team) BeforeCreate(tx *gorm.DB) error {
	if t.UID == "" {
		t.UID = generateUID()
	}
	return nil
}

// GetMemberCount returns the number of members in the team
func (t *Team) GetMemberCount() int {
	return len(t.Members)
}

// CanAddMember checks if the team can add more members
func (t *Team) CanAddMember() bool {
	return t.GetMemberCount() < t.MaxUsers
}

// GetRemainingMemberSlots returns the number of member slots remaining
func (t *Team) GetRemainingMemberSlots() int {
	remaining := t.MaxUsers - t.GetMemberCount()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsTrialActive checks if the team is in trial period
func (t *Team) IsTrialActive() bool {
	return t.TrialEndsAt != nil && t.TrialEndsAt.After(time.Now())
}

// IsTrialExpired checks if the trial has expired
func (t *Team) IsTrialExpired() bool {
	return t.TrialEndsAt != nil && t.TrialEndsAt.Before(time.Now())
}

// GetSubscriptionLimits returns the limits for the team's subscription
func (t *Team) GetSubscriptionLimits() TeamLimits {
	switch t.SubscriptionTier {
	case "startup":
		return TeamLimits{
			MaxUsers:               10,
			MaxDatabaseConnections: 25,
			MaxStorageGB:          250,
			BackupRetentionDays:   30,
			APIRateLimit:          1000,
		}
	case "business":
		return TeamLimits{
			MaxUsers:               50,
			MaxDatabaseConnections: 100,
			MaxStorageGB:          1000,
			BackupRetentionDays:   90,
			APIRateLimit:          5000,
		}
	case "enterprise":
		return TeamLimits{
			MaxUsers:               -1, // unlimited
			MaxDatabaseConnections: -1, // unlimited
			MaxStorageGB:          -1,  // unlimited
			BackupRetentionDays:   365,
			APIRateLimit:          10000,
		}
	default: // free tier
		return TeamLimits{
			MaxUsers:               5,
			MaxDatabaseConnections: 10,
			MaxStorageGB:          100,
			BackupRetentionDays:   7,
			APIRateLimit:          100,
		}
	}
}

// GetMemberByUserID returns a team member by user ID
func (t *Team) GetMemberByUserID(userID uint) *TeamMember {
	for _, member := range t.Members {
		if member.UserID == userID {
			return &member
		}
	}
	return nil
}

// HasMember checks if a user is a member of the team
func (t *Team) HasMember(userID uint) bool {
	return t.GetMemberByUserID(userID) != nil
}

// GetOwners returns all team owners
func (t *Team) GetOwners() []TeamMember {
	var owners []TeamMember
	for _, member := range t.Members {
		if member.Role == TeamRoleOwner {
			owners = append(owners, member)
		}
	}
	return owners
}

// GetAdmins returns all team admins (including owners)
func (t *Team) GetAdmins() []TeamMember {
	var admins []TeamMember
	for _, member := range t.Members {
		if member.Role == TeamRoleOwner || member.Role == TeamRoleAdmin {
			admins = append(admins, member)
		}
	}
	return admins
}

// IsOwner checks if a user is an owner of the team
func (t *Team) IsOwner(userID uint) bool {
	member := t.GetMemberByUserID(userID)
	return member != nil && member.Role == TeamRoleOwner
}

// IsAdmin checks if a user is an admin or owner of the team
func (t *Team) IsAdmin(userID uint) bool {
	member := t.GetMemberByUserID(userID)
	return member != nil && (member.Role == TeamRoleOwner || member.Role == TeamRoleAdmin)
}

// CanManageTeam checks if a user can manage team settings
func (t *Team) CanManageTeam(userID uint) bool {
	return t.IsAdmin(userID)
}

// CanInviteMembers checks if a user can invite new members
func (t *Team) CanInviteMembers(userID uint) bool {
	member := t.GetMemberByUserID(userID)
	return member != nil && (t.IsAdmin(userID) || member.CanInviteMembers)
}

// TeamLimits represents the limits for a team's subscription
type TeamLimits struct {
	MaxUsers               int
	MaxDatabaseConnections int
	MaxStorageGB          int
	BackupRetentionDays   int
	APIRateLimit          int
}

// SetPermissionsByRole sets permissions based on the team role
func (tm *TeamMember) SetPermissionsByRole() {
	switch tm.Role {
	case TeamRoleOwner:
		tm.CanInviteMembers = true
		tm.CanManageBackups = true
		tm.CanManageConnections = true
		tm.CanViewAuditLogs = true
		tm.CanManageBilling = true
	case TeamRoleAdmin:
		tm.CanInviteMembers = true
		tm.CanManageBackups = true
		tm.CanManageConnections = true
		tm.CanViewAuditLogs = true
		tm.CanManageBilling = false
	case TeamRoleMember:
		tm.CanInviteMembers = false
		tm.CanManageBackups = true
		tm.CanManageConnections = false
		tm.CanViewAuditLogs = false
		tm.CanManageBilling = false
	case TeamRoleGuest:
		tm.CanInviteMembers = false
		tm.CanManageBackups = false
		tm.CanManageConnections = false
		tm.CanViewAuditLogs = false
		tm.CanManageBilling = false
	}
}

// IsOwner checks if this member is an owner
func (tm *TeamMember) IsOwner() bool {
	return tm.Role == TeamRoleOwner
}

// IsAdmin checks if this member is an admin or owner
func (tm *TeamMember) IsAdmin() bool {
	return tm.Role == TeamRoleOwner || tm.Role == TeamRoleAdmin
}

// CanPerformAction checks if this member can perform a specific action
func (tm *TeamMember) CanPerformAction(action string) bool {
	if !tm.IsActive {
		return false
	}
	
	switch action {
	case "invite_members":
		return tm.CanInviteMembers
	case "manage_backups":
		return tm.CanManageBackups
	case "manage_connections":
		return tm.CanManageConnections
	case "view_audit_logs":
		return tm.CanViewAuditLogs
	case "manage_billing":
		return tm.CanManageBilling
	default:
		return false
	}
}

// Accept marks the team invitation as accepted
func (tm *TeamMember) Accept() {
	now := time.Now()
	tm.JoinedAt = &now
	tm.IsActive = true
}

// GetRoleDisplayName returns a human-readable role name
func (tm *TeamMember) GetRoleDisplayName() string {
	switch tm.Role {
	case TeamRoleOwner:
		return "Owner"
	case TeamRoleAdmin:
		return "Administrator"
	case TeamRoleMember:
		return "Member"
	case TeamRoleGuest:
		return "Guest"
	default:
		return string(tm.Role)
	}
}

// GetPermissionSummary returns a summary of the member's permissions
func (tm *TeamMember) GetPermissionSummary() []string {
	var permissions []string
	
	if tm.CanInviteMembers {
		permissions = append(permissions, "Invite Members")
	}
	if tm.CanManageBackups {
		permissions = append(permissions, "Manage Backups")
	}
	if tm.CanManageConnections {
		permissions = append(permissions, "Manage Connections")
	}
	if tm.CanViewAuditLogs {
		permissions = append(permissions, "View Audit Logs")
	}
	if tm.CanManageBilling {
		permissions = append(permissions, "Manage Billing")
	}
	
	return permissions
}

// IsInvitePending checks if the member invitation is still pending
func (tm *TeamMember) IsInvitePending() bool {
	return tm.InvitedAt != nil && tm.JoinedAt == nil
}

// GetMembershipDuration returns how long the user has been a member
func (tm *TeamMember) GetMembershipDuration() *time.Duration {
	if tm.JoinedAt == nil {
		return nil
	}
	
	duration := time.Since(*tm.JoinedAt)
	return &duration
}

// CanBeRemovedBy checks if this member can be removed by another member
func (tm *TeamMember) CanBeRemovedBy(removerRole TeamRole) bool {
	// Owners can remove anyone except other owners
	if removerRole == TeamRoleOwner {
		return tm.Role != TeamRoleOwner
	}
	
	// Admins can remove members and guests
	if removerRole == TeamRoleAdmin {
		return tm.Role == TeamRoleMember || tm.Role == TeamRoleGuest
	}
	
	// Members and guests cannot remove anyone
	return false
}

// CanChangeRoleTo checks if this member's role can be changed to the target role
func (tm *TeamMember) CanChangeRoleTo(targetRole TeamRole, changerRole TeamRole) bool {
	// Only owners can change roles to/from owner
	if targetRole == TeamRoleOwner || tm.Role == TeamRoleOwner {
		return changerRole == TeamRoleOwner
	}
	
	// Admins can change member/guest roles
	if changerRole == TeamRoleAdmin {
		return targetRole == TeamRoleMember || targetRole == TeamRoleGuest || targetRole == TeamRoleAdmin
	}
	
	return false
}