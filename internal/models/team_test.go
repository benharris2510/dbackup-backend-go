package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeam_BeforeCreate(t *testing.T) {
	team := &Team{
		Name: "Test Team",
		Slug: "test-team",
	}

	err := team.BeforeCreate(nil)
	require.NoError(t, err)
	assert.NotEmpty(t, team.UID)
	assert.Equal(t, 36, len(team.UID))
}

func TestTeam_GetMemberCount(t *testing.T) {
	team := &Team{
		Members: []TeamMember{
			{UserID: 1},
			{UserID: 2},
			{UserID: 3},
		},
	}
	assert.Equal(t, 3, team.GetMemberCount())
}

func TestTeam_CanAddMember(t *testing.T) {
	t.Run("can add member", func(t *testing.T) {
		team := &Team{
			MaxUsers: 5,
			Members: []TeamMember{
				{UserID: 1},
				{UserID: 2},
			},
		}
		assert.True(t, team.CanAddMember())
	})

	t.Run("cannot add member - at limit", func(t *testing.T) {
		team := &Team{
			MaxUsers: 2,
			Members: []TeamMember{
				{UserID: 1},
				{UserID: 2},
			},
		}
		assert.False(t, team.CanAddMember())
	})
}

func TestTeam_GetRemainingMemberSlots(t *testing.T) {
	t.Run("has remaining slots", func(t *testing.T) {
		team := &Team{
			MaxUsers: 5,
			Members: []TeamMember{
				{UserID: 1},
				{UserID: 2},
			},
		}
		assert.Equal(t, 3, team.GetRemainingMemberSlots())
	})

	t.Run("no remaining slots", func(t *testing.T) {
		team := &Team{
			MaxUsers: 2,
			Members: []TeamMember{
				{UserID: 1},
				{UserID: 2},
			},
		}
		assert.Equal(t, 0, team.GetRemainingMemberSlots())
	})

	t.Run("over limit returns zero", func(t *testing.T) {
		team := &Team{
			MaxUsers: 2,
			Members: []TeamMember{
				{UserID: 1},
				{UserID: 2},
				{UserID: 3},
			},
		}
		assert.Equal(t, 0, team.GetRemainingMemberSlots())
	})
}

func TestTeam_TrialMethods(t *testing.T) {
	t.Run("trial active", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		team := &Team{TrialEndsAt: &future}
		assert.True(t, team.IsTrialActive())
		assert.False(t, team.IsTrialExpired())
	})

	t.Run("trial expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		team := &Team{TrialEndsAt: &past}
		assert.False(t, team.IsTrialActive())
		assert.True(t, team.IsTrialExpired())
	})

	t.Run("no trial", func(t *testing.T) {
		team := &Team{}
		assert.False(t, team.IsTrialActive())
		assert.False(t, team.IsTrialExpired())
	})
}

func TestTeam_GetSubscriptionLimits(t *testing.T) {
	testCases := []struct {
		tier                   string
		expectedUsers          int
		expectedConnections    int
		expectedStorageGB      int
		expectedRetentionDays  int
		expectedAPIRateLimit   int
	}{
		{"free", 5, 10, 100, 7, 100},
		{"startup", 10, 25, 250, 30, 1000},
		{"business", 50, 100, 1000, 90, 5000},
		{"enterprise", -1, -1, -1, 365, 10000},
		{"unknown", 5, 10, 100, 7, 100}, // defaults to free
	}

	for _, tc := range testCases {
		t.Run(tc.tier, func(t *testing.T) {
			team := &Team{SubscriptionTier: tc.tier}
			limits := team.GetSubscriptionLimits()
			
			assert.Equal(t, tc.expectedUsers, limits.MaxUsers)
			assert.Equal(t, tc.expectedConnections, limits.MaxDatabaseConnections)
			assert.Equal(t, tc.expectedStorageGB, limits.MaxStorageGB)
			assert.Equal(t, tc.expectedRetentionDays, limits.BackupRetentionDays)
			assert.Equal(t, tc.expectedAPIRateLimit, limits.APIRateLimit)
		})
	}
}

func TestTeam_MembershipMethods(t *testing.T) {
	team := &Team{
		Members: []TeamMember{
			{UserID: 1, Role: TeamRoleOwner},
			{UserID: 2, Role: TeamRoleAdmin},
			{UserID: 3, Role: TeamRoleMember},
			{UserID: 4, Role: TeamRoleGuest},
		},
	}

	t.Run("get member by user ID", func(t *testing.T) {
		member := team.GetMemberByUserID(2)
		assert.NotNil(t, member)
		assert.Equal(t, uint(2), member.UserID)
		assert.Equal(t, TeamRoleAdmin, member.Role)

		nonMember := team.GetMemberByUserID(999)
		assert.Nil(t, nonMember)
	})

	t.Run("has member", func(t *testing.T) {
		assert.True(t, team.HasMember(1))
		assert.True(t, team.HasMember(3))
		assert.False(t, team.HasMember(999))
	})

	t.Run("get owners", func(t *testing.T) {
		owners := team.GetOwners()
		assert.Len(t, owners, 1)
		assert.Equal(t, uint(1), owners[0].UserID)
	})

	t.Run("get admins", func(t *testing.T) {
		admins := team.GetAdmins()
		assert.Len(t, admins, 2) // owner + admin
		
		var userIDs []uint
		for _, admin := range admins {
			userIDs = append(userIDs, admin.UserID)
		}
		assert.Contains(t, userIDs, uint(1)) // owner
		assert.Contains(t, userIDs, uint(2)) // admin
	})

	t.Run("is owner", func(t *testing.T) {
		assert.True(t, team.IsOwner(1))
		assert.False(t, team.IsOwner(2))
		assert.False(t, team.IsOwner(999))
	})

	t.Run("is admin", func(t *testing.T) {
		assert.True(t, team.IsAdmin(1))  // owner
		assert.True(t, team.IsAdmin(2))  // admin
		assert.False(t, team.IsAdmin(3)) // member
		assert.False(t, team.IsAdmin(999))
	})

	t.Run("can manage team", func(t *testing.T) {
		assert.True(t, team.CanManageTeam(1))  // owner
		assert.True(t, team.CanManageTeam(2))  // admin
		assert.False(t, team.CanManageTeam(3)) // member
	})
}

func TestTeamMember_SetPermissionsByRole(t *testing.T) {
	testCases := []struct {
		role                    TeamRole
		expectedInviteMembers   bool
		expectedManageBackups   bool
		expectedManageConnections bool
		expectedViewAuditLogs   bool
		expectedManageBilling   bool
	}{
		{TeamRoleOwner, true, true, true, true, true},
		{TeamRoleAdmin, true, true, true, true, false},
		{TeamRoleMember, false, true, false, false, false},
		{TeamRoleGuest, false, false, false, false, false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.role), func(t *testing.T) {
			member := &TeamMember{Role: tc.role}
			member.SetPermissionsByRole()
			
			assert.Equal(t, tc.expectedInviteMembers, member.CanInviteMembers)
			assert.Equal(t, tc.expectedManageBackups, member.CanManageBackups)
			assert.Equal(t, tc.expectedManageConnections, member.CanManageConnections)
			assert.Equal(t, tc.expectedViewAuditLogs, member.CanViewAuditLogs)
			assert.Equal(t, tc.expectedManageBilling, member.CanManageBilling)
		})
	}
}

func TestTeamMember_RoleMethods(t *testing.T) {
	t.Run("is owner", func(t *testing.T) {
		owner := &TeamMember{Role: TeamRoleOwner}
		admin := &TeamMember{Role: TeamRoleAdmin}
		
		assert.True(t, owner.IsOwner())
		assert.False(t, admin.IsOwner())
	})

	t.Run("is admin", func(t *testing.T) {
		owner := &TeamMember{Role: TeamRoleOwner}
		admin := &TeamMember{Role: TeamRoleAdmin}
		member := &TeamMember{Role: TeamRoleMember}
		
		assert.True(t, owner.IsAdmin())
		assert.True(t, admin.IsAdmin())
		assert.False(t, member.IsAdmin())
	})
}

func TestTeamMember_CanPerformAction(t *testing.T) {
	member := &TeamMember{
		Role:                 TeamRoleMember,
		IsActive:            true,
		CanManageBackups:    true,
		CanInviteMembers:    false,
		CanManageConnections: false,
	}

	t.Run("can perform allowed actions", func(t *testing.T) {
		assert.True(t, member.CanPerformAction("manage_backups"))
	})

	t.Run("cannot perform disallowed actions", func(t *testing.T) {
		assert.False(t, member.CanPerformAction("invite_members"))
		assert.False(t, member.CanPerformAction("manage_connections"))
	})

	t.Run("inactive member cannot perform any action", func(t *testing.T) {
		member.IsActive = false
		assert.False(t, member.CanPerformAction("manage_backups"))
	})

	t.Run("unknown action returns false", func(t *testing.T) {
		member.IsActive = true
		assert.False(t, member.CanPerformAction("unknown_action"))
	})
}

func TestTeamMember_Accept(t *testing.T) {
	member := &TeamMember{
		IsActive: false,
		JoinedAt: nil,
	}

	member.Accept()
	assert.True(t, member.IsActive)
	assert.NotNil(t, member.JoinedAt)
	assert.WithinDuration(t, time.Now(), *member.JoinedAt, time.Second)
}

func TestTeamMember_GetRoleDisplayName(t *testing.T) {
	testCases := []struct {
		role     TeamRole
		expected string
	}{
		{TeamRoleOwner, "Owner"},
		{TeamRoleAdmin, "Administrator"},
		{TeamRoleMember, "Member"},
		{TeamRoleGuest, "Guest"},
		{TeamRole("custom"), "custom"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.role), func(t *testing.T) {
			member := &TeamMember{Role: tc.role}
			assert.Equal(t, tc.expected, member.GetRoleDisplayName())
		})
	}
}

func TestTeamMember_GetPermissionSummary(t *testing.T) {
	member := &TeamMember{
		CanInviteMembers:     true,
		CanManageBackups:     true,
		CanManageConnections: false,
		CanViewAuditLogs:     false,
		CanManageBilling:     false,
	}

	permissions := member.GetPermissionSummary()
	assert.Len(t, permissions, 2)
	assert.Contains(t, permissions, "Invite Members")
	assert.Contains(t, permissions, "Manage Backups")
}

func TestTeamMember_IsInvitePending(t *testing.T) {
	now := time.Now()
	
	t.Run("invite pending", func(t *testing.T) {
		member := &TeamMember{
			InvitedAt: &now,
			JoinedAt:  nil,
		}
		assert.True(t, member.IsInvitePending())
	})

	t.Run("invite accepted", func(t *testing.T) {
		member := &TeamMember{
			InvitedAt: &now,
			JoinedAt:  &now,
		}
		assert.False(t, member.IsInvitePending())
	})

	t.Run("no invite", func(t *testing.T) {
		member := &TeamMember{}
		assert.False(t, member.IsInvitePending())
	})
}

func TestTeamMember_GetMembershipDuration(t *testing.T) {
	t.Run("has membership duration", func(t *testing.T) {
		joinTime := time.Now().Add(-24 * time.Hour)
		member := &TeamMember{JoinedAt: &joinTime}
		
		duration := member.GetMembershipDuration()
		assert.NotNil(t, duration)
		assert.True(t, *duration > 23*time.Hour)
		assert.True(t, *duration < 25*time.Hour)
	})

	t.Run("no membership duration", func(t *testing.T) {
		member := &TeamMember{}
		duration := member.GetMembershipDuration()
		assert.Nil(t, duration)
	})
}

func TestTeamMember_CanBeRemovedBy(t *testing.T) {
	testCases := []struct {
		memberRole  TeamRole
		removerRole TeamRole
		expected    bool
	}{
		{TeamRoleOwner, TeamRoleOwner, false},   // Owner cannot remove owner
		{TeamRoleAdmin, TeamRoleOwner, true},    // Owner can remove admin
		{TeamRoleMember, TeamRoleOwner, true},   // Owner can remove member
		{TeamRoleGuest, TeamRoleOwner, true},    // Owner can remove guest
		
		{TeamRoleOwner, TeamRoleAdmin, false},   // Admin cannot remove owner
		{TeamRoleAdmin, TeamRoleAdmin, false},   // Admin cannot remove admin
		{TeamRoleMember, TeamRoleAdmin, true},   // Admin can remove member
		{TeamRoleGuest, TeamRoleAdmin, true},    // Admin can remove guest
		
		{TeamRoleOwner, TeamRoleMember, false},  // Member cannot remove owner
		{TeamRoleAdmin, TeamRoleMember, false},  // Member cannot remove admin
		{TeamRoleMember, TeamRoleMember, false}, // Member cannot remove member
		{TeamRoleGuest, TeamRoleMember, false},  // Member cannot remove guest
	}

	for _, tc := range testCases {
		t.Run(string(tc.memberRole)+"_by_"+string(tc.removerRole), func(t *testing.T) {
			member := &TeamMember{Role: tc.memberRole}
			result := member.CanBeRemovedBy(tc.removerRole)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTeamMember_CanChangeRoleTo(t *testing.T) {
	testCases := []struct {
		currentRole TeamRole
		targetRole  TeamRole
		changerRole TeamRole
		expected    bool
	}{
		// Owner role changes - only owners can do this
		{TeamRoleMember, TeamRoleOwner, TeamRoleOwner, true},
		{TeamRoleMember, TeamRoleOwner, TeamRoleAdmin, false},
		{TeamRoleOwner, TeamRoleMember, TeamRoleOwner, true},
		{TeamRoleOwner, TeamRoleMember, TeamRoleAdmin, false},
		
		// Admin role changes - admins can change non-owner roles
		{TeamRoleMember, TeamRoleAdmin, TeamRoleAdmin, true},
		{TeamRoleMember, TeamRoleGuest, TeamRoleAdmin, true},
		{TeamRoleGuest, TeamRoleMember, TeamRoleAdmin, true},
		
		// Members cannot change roles
		{TeamRoleMember, TeamRoleGuest, TeamRoleMember, false},
	}

	for _, tc := range testCases {
		name := string(tc.currentRole) + "_to_" + string(tc.targetRole) + "_by_" + string(tc.changerRole)
		t.Run(name, func(t *testing.T) {
			member := &TeamMember{Role: tc.currentRole}
			result := member.CanChangeRoleTo(tc.targetRole, tc.changerRole)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTeam_TableName(t *testing.T) {
	team := Team{}
	assert.Equal(t, "teams", team.TableName())
}

func TestTeamMember_TableName(t *testing.T) {
	member := TeamMember{}
	assert.Equal(t, "team_members", member.TableName())
}

func TestTeam_CanInviteMembers(t *testing.T) {
	team := &Team{
		Members: []TeamMember{
			{UserID: 1, Role: TeamRoleOwner, CanInviteMembers: true},
			{UserID: 2, Role: TeamRoleMember, CanInviteMembers: true},
			{UserID: 3, Role: TeamRoleMember, CanInviteMembers: false},
		},
	}

	t.Run("owner can invite", func(t *testing.T) {
		assert.True(t, team.CanInviteMembers(1))
	})

	t.Run("member with permission can invite", func(t *testing.T) {
		assert.True(t, team.CanInviteMembers(2))
	})

	t.Run("member without permission cannot invite", func(t *testing.T) {
		assert.False(t, team.CanInviteMembers(3))
	})

	t.Run("non-member cannot invite", func(t *testing.T) {
		assert.False(t, team.CanInviteMembers(999))
	})
}