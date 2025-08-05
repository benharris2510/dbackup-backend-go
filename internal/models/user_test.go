package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser_BeforeCreate(t *testing.T) {
	user := &User{
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
		Password:  "hashedpassword",
	}

	// Mock GORM DB - in practice this would be called by GORM
	err := user.BeforeCreate(nil)
	require.NoError(t, err)
	assert.NotEmpty(t, user.UID)
	assert.Equal(t, 36, len(user.UID)) // UUID length with hyphens
}

func TestUser_IsLocked(t *testing.T) {
	t.Run("not locked", func(t *testing.T) {
		user := &User{}
		assert.False(t, user.IsLocked())
	})

	t.Run("locked in future", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		user := &User{LockedUntil: &future}
		assert.True(t, user.IsLocked())
	})

	t.Run("lock expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		user := &User{LockedUntil: &past}
		assert.False(t, user.IsLocked())
	})
}

func TestUser_CanLogin(t *testing.T) {
	t.Run("active user can login", func(t *testing.T) {
		user := &User{IsActive: true}
		assert.True(t, user.CanLogin())
	})

	t.Run("inactive user cannot login", func(t *testing.T) {
		user := &User{IsActive: false}
		assert.False(t, user.CanLogin())
	})

	t.Run("locked user cannot login", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		user := &User{
			IsActive:    true,
			LockedUntil: &future,
		}
		assert.False(t, user.CanLogin())
	})
}

func TestUser_IncrementLoginAttempts(t *testing.T) {
	t.Run("normal increment", func(t *testing.T) {
		user := &User{LoginAttempts: 0}
		user.IncrementLoginAttempts()
		assert.Equal(t, 1, user.LoginAttempts)
		assert.Nil(t, user.LockedUntil)
	})

	t.Run("lock after 5 attempts", func(t *testing.T) {
		user := &User{LoginAttempts: 4}
		user.IncrementLoginAttempts()
		assert.Equal(t, 5, user.LoginAttempts)
		assert.NotNil(t, user.LockedUntil)
		assert.True(t, user.LockedUntil.After(time.Now()))
	})
}

func TestUser_ResetLoginAttempts(t *testing.T) {
	future := time.Now().Add(time.Hour)
	user := &User{
		LoginAttempts: 5,
		LockedUntil:   &future,
	}

	user.ResetLoginAttempts()
	assert.Equal(t, 0, user.LoginAttempts)
	assert.Nil(t, user.LockedUntil)
}

func TestUser_SetLastLogin(t *testing.T) {
	user := &User{}
	ip := "192.168.1.1"

	user.SetLastLogin(ip)
	assert.NotNil(t, user.LastLoginAt)
	assert.NotNil(t, user.LastLoginIP)
	assert.Equal(t, ip, *user.LastLoginIP)
	assert.WithinDuration(t, time.Now(), *user.LastLoginAt, time.Second)
}

func TestUser_GetFullName(t *testing.T) {
	user := &User{
		FirstName: "John",
		LastName:  "Doe",
	}
	assert.Equal(t, "John Doe", user.GetFullName())
}

func TestUser_HasSubscription(t *testing.T) {
	user := &User{SubscriptionTier: "pro"}
	assert.True(t, user.HasSubscription("pro"))
	assert.False(t, user.HasSubscription("business"))
}

func TestUser_CanCreateDatabaseConnection(t *testing.T) {
	t.Run("can create connection", func(t *testing.T) {
		user := &User{
			MaxDatabaseConnections: 5,
			DatabaseConnections:    make([]DatabaseConnection, 3),
		}
		assert.True(t, user.CanCreateDatabaseConnection())
	})

	t.Run("cannot create connection - at limit", func(t *testing.T) {
		user := &User{
			MaxDatabaseConnections: 5,
			DatabaseConnections:    make([]DatabaseConnection, 5),
		}
		assert.False(t, user.CanCreateDatabaseConnection())
	})

	t.Run("cannot create connection - over limit", func(t *testing.T) {
		user := &User{
			MaxDatabaseConnections: 5,
			DatabaseConnections:    make([]DatabaseConnection, 6),
		}
		assert.False(t, user.CanCreateDatabaseConnection())
	})
}

func TestUser_GetRemainingConnections(t *testing.T) {
	t.Run("has remaining connections", func(t *testing.T) {
		user := &User{
			MaxDatabaseConnections: 5,
			DatabaseConnections:    make([]DatabaseConnection, 2),
		}
		assert.Equal(t, 3, user.GetRemainingConnections())
	})

	t.Run("no remaining connections", func(t *testing.T) {
		user := &User{
			MaxDatabaseConnections: 5,
			DatabaseConnections:    make([]DatabaseConnection, 5),
		}
		assert.Equal(t, 0, user.GetRemainingConnections())
	})

	t.Run("over limit returns zero", func(t *testing.T) {
		user := &User{
			MaxDatabaseConnections: 5,
			DatabaseConnections:    make([]DatabaseConnection, 7),
		}
		assert.Equal(t, 0, user.GetRemainingConnections())
	})
}

func TestUser_CanCreateBackupOfSize(t *testing.T) {
	user := &User{MaxBackupSize: 1024 * 1024} // 1MB

	assert.True(t, user.CanCreateBackupOfSize(512*1024))  // 512KB
	assert.True(t, user.CanCreateBackupOfSize(1024*1024)) // 1MB
	assert.False(t, user.CanCreateBackupOfSize(2*1024*1024)) // 2MB
}

func TestUser_EmailVerificationMethods(t *testing.T) {
	t.Run("email verification expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		user := &User{EmailVerifyExpires: &past}
		assert.True(t, user.IsEmailVerificationExpired())
	})

	t.Run("email verification not expired", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		user := &User{EmailVerifyExpires: &future}
		assert.False(t, user.IsEmailVerificationExpired())
	})

	t.Run("clear email verification", func(t *testing.T) {
		token := "test-token"
		expires := time.Now().Add(time.Hour)
		user := &User{
			EmailVerifyToken:   &token,
			EmailVerifyExpires: &expires,
			IsEmailVerified:    false,
		}

		user.ClearEmailVerification()
		assert.Nil(t, user.EmailVerifyToken)
		assert.Nil(t, user.EmailVerifyExpires)
		assert.True(t, user.IsEmailVerified)
	})
}

func TestUser_PasswordResetMethods(t *testing.T) {
	t.Run("password reset expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		user := &User{PasswordResetExpires: &past}
		assert.True(t, user.IsPasswordResetExpired())
	})

	t.Run("password reset not expired", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		user := &User{PasswordResetExpires: &future}
		assert.False(t, user.IsPasswordResetExpired())
	})

	t.Run("clear password reset", func(t *testing.T) {
		token := "reset-token"
		expires := time.Now().Add(time.Hour)
		user := &User{
			PasswordResetToken:   &token,
			PasswordResetExpires: &expires,
		}

		user.ClearPasswordReset()
		assert.Nil(t, user.PasswordResetToken)
		assert.Nil(t, user.PasswordResetExpires)
	})
}

func TestUser_TwoFactorMethods(t *testing.T) {
	t.Run("enable 2FA", func(t *testing.T) {
		user := &User{}
		secret := "JBSWY3DPEHPK3PXP"
		backupCode := "ABCD-1234-EFGH-5678"

		user.Enable2FA(secret, backupCode)
		assert.True(t, user.TwoFactorEnabled)
		assert.NotNil(t, user.TwoFactorSecret)
		assert.Equal(t, secret, *user.TwoFactorSecret)
		assert.NotNil(t, user.TwoFactorBackupCode)
		assert.Equal(t, backupCode, *user.TwoFactorBackupCode)
	})

	t.Run("disable 2FA", func(t *testing.T) {
		secret := "JBSWY3DPEHPK3PXP"
		backupCode := "ABCD-1234-EFGH-5678"
		user := &User{
			TwoFactorEnabled:    true,
			TwoFactorSecret:     &secret,
			TwoFactorBackupCode: &backupCode,
		}

		user.Disable2FA()
		assert.False(t, user.TwoFactorEnabled)
		assert.Nil(t, user.TwoFactorSecret)
		assert.Nil(t, user.TwoFactorBackupCode)
	})
}

func TestUser_GetSubscriptionLimits(t *testing.T) {
	testCases := []struct {
		tier                   string
		expectedConnections    int
		expectedSize           int64
		expectedRetentionDays  int
		expectedFrequency      string
	}{
		{"free", 2, 5368709120, 7, "daily"},
		{"pro", 10, 53687091200, 30, "hourly"},
		{"business", 50, 536870912000, 90, "realtime"},
		{"enterprise", -1, -1, 365, "realtime"},
		{"unknown", 2, 5368709120, 7, "daily"}, // defaults to free
	}

	for _, tc := range testCases {
		t.Run(tc.tier, func(t *testing.T) {
			user := &User{SubscriptionTier: tc.tier}
			limits := user.GetSubscriptionLimits()
			
			assert.Equal(t, tc.expectedConnections, limits.MaxDatabaseConnections)
			assert.Equal(t, tc.expectedSize, limits.MaxBackupSize)
			assert.Equal(t, tc.expectedRetentionDays, limits.RetentionDays)
			assert.Equal(t, tc.expectedFrequency, limits.BackupFrequency)
		})
	}
}

func TestUser_TableName(t *testing.T) {
	user := User{}
	assert.Equal(t, "users", user.TableName())
}

func TestGenerateUID(t *testing.T) {
	uid1 := generateUID()
	uid2 := generateUID()
	
	assert.NotEmpty(t, uid1)
	assert.NotEmpty(t, uid2)
	assert.NotEqual(t, uid1, uid2)
	assert.Equal(t, 36, len(uid1)) // UUID length with hyphens
	assert.Equal(t, 36, len(uid2))
}

func TestGenerateSecureToken(t *testing.T) {
	t.Run("generate 32 byte token", func(t *testing.T) {
		token, err := generateSecureToken(32)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Equal(t, 64, len(token)) // 32 bytes = 64 hex characters
	})

	t.Run("generate different tokens", func(t *testing.T) {
		token1, err1 := generateSecureToken(16)
		token2, err2 := generateSecureToken(16)
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, token1, token2)
	})
}

func TestGenerateBackupCode(t *testing.T) {
	t.Run("generate backup code", func(t *testing.T) {
		code, err := generateBackupCode()
		require.NoError(t, err)
		assert.NotEmpty(t, code)
		
		// Should be 8 groups of 4 characters separated by hyphens
		// Format: XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX
		parts := len(code)
		assert.Equal(t, 39, parts) // 8*4 + 7 hyphens = 39 characters
	})

	t.Run("generate different codes", func(t *testing.T) {
		code1, err1 := generateBackupCode()
		code2, err2 := generateBackupCode()
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, code1, code2)
	})
}

func TestValidateEmail(t *testing.T) {
	testCases := []struct {
		email    string
		expected bool
	}{
		{"valid@example.com", true},
		{"user+tag@domain.co.uk", true},
		{"test@test.org", true},
		{"invalid-email", false},
		{"@example.com", false},
		{"user@", false},
		{"", false},
		{"a@b.c", true}, // Minimal valid email
		{"user@domain", false}, // No TLD
	}

	for _, tc := range testCases {
		t.Run(tc.email, func(t *testing.T) {
			result := validateEmail(tc.email)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Test@Example.COM", "test@example.com"},
		{"  user@domain.org  ", "user@domain.org"},
		{"USER+TAG@DOMAIN.NET", "user+tag@domain.net"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeEmail(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}