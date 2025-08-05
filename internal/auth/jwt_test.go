package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_NewJWTManager(t *testing.T) {
	secretKey := "test-secret-key"
	accessDuration := 15 * time.Minute
	refreshDuration := 7 * 24 * time.Hour

	jm := NewJWTManager(secretKey, accessDuration, refreshDuration)

	assert.NotNil(t, jm)
	assert.Equal(t, []byte(secretKey), jm.secretKey)
	assert.Equal(t, accessDuration, jm.accessTokenDuration)
	assert.Equal(t, refreshDuration, jm.refreshTokenDuration)
}

func TestJWTManager_GenerateAccessToken(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uint(123)
	email := "test@example.com"
	teamID := uint(456)

	token, err := jm.GenerateAccessToken(userID, email, &teamID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify token can be parsed
	claims, err := jm.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, TokenTypeAccess, claims.TokenType)
	assert.Equal(t, &teamID, claims.TeamID)
	assert.Equal(t, "dbackup-api", claims.Issuer)
	assert.Equal(t, "user:123", claims.Subject)
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uint(123)
	email := "test@example.com"

	token, err := jm.GenerateRefreshToken(userID, email, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify token can be parsed
	claims, err := jm.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, TokenTypeRefresh, claims.TokenType)
	assert.Nil(t, claims.TeamID)
}

func TestJWTManager_GenerateTokenPair(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	userID := uint(123)
	email := "test@example.com"
	teamID := uint(456)

	accessToken, refreshToken, err := jm.GenerateTokenPair(userID, email, &teamID)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// Verify both tokens
	accessClaims, err := jm.ValidateAccessToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, TokenTypeAccess, accessClaims.TokenType)

	refreshClaims, err := jm.ValidateRefreshToken(refreshToken)
	require.NoError(t, err)
	assert.Equal(t, TokenTypeRefresh, refreshClaims.TokenType)

	// Both should have same user info
	assert.Equal(t, accessClaims.UserID, refreshClaims.UserID)
	assert.Equal(t, accessClaims.Email, refreshClaims.Email)
	assert.Equal(t, accessClaims.TeamID, refreshClaims.TeamID)
}

func TestJWTManager_ValidateToken(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		claims, err := jm.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, uint(123), claims.UserID)
	})

	t.Run("invalid signature", func(t *testing.T) {
		wrongJM := NewJWTManager("wrong-secret", 15*time.Minute, 7*24*time.Hour)
		token, err := wrongJM.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		_, err = jm.ValidateToken(token)
		assert.ErrorIs(t, err, ErrInvalidSignature)
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := jm.ValidateToken("invalid.token.format")
		assert.ErrorIs(t, err, ErrTokenMalformed)
	})

	t.Run("expired token", func(t *testing.T) {
		shortJM := NewJWTManager("test-secret", -1*time.Hour, 7*24*time.Hour)
		token, err := shortJM.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		_, err = jm.ValidateToken(token)
		assert.ErrorIs(t, err, ErrExpiredToken)
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := jm.ValidateToken("")
		assert.ErrorIs(t, err, ErrTokenMalformed)
	})
}

func TestJWTManager_ValidateAccessToken(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid access token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		claims, err := jm.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, TokenTypeAccess, claims.TokenType)
	})

	t.Run("refresh token as access token", func(t *testing.T) {
		token, err := jm.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		_, err = jm.ValidateAccessToken(token)
		assert.ErrorIs(t, err, ErrInvalidTokenType)
	})
}

func TestJWTManager_ValidateRefreshToken(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid refresh token", func(t *testing.T) {
		token, err := jm.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		claims, err := jm.ValidateRefreshToken(token)
		require.NoError(t, err)
		assert.Equal(t, TokenTypeRefresh, claims.TokenType)
	})

	t.Run("access token as refresh token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		_, err = jm.ValidateRefreshToken(token)
		assert.ErrorIs(t, err, ErrInvalidTokenType)
	})
}

func TestJWTManager_RefreshAccessToken(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid refresh token", func(t *testing.T) {
		userID := uint(123)
		email := "test@example.com"
		teamID := uint(456)

		refreshToken, err := jm.GenerateRefreshToken(userID, email, &teamID)
		require.NoError(t, err)

		newAccessToken, err := jm.RefreshAccessToken(refreshToken)
		require.NoError(t, err)
		assert.NotEmpty(t, newAccessToken)

		// Verify new access token
		claims, err := jm.ValidateAccessToken(newAccessToken)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, &teamID, claims.TeamID)
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		accessToken, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		_, err = jm.RefreshAccessToken(accessToken)
		assert.ErrorIs(t, err, ErrInvalidTokenType)
	})

	t.Run("expired refresh token", func(t *testing.T) {
		shortJM := NewJWTManager("test-secret", 15*time.Minute, -1*time.Hour)
		refreshToken, err := shortJM.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		_, err = jm.RefreshAccessToken(refreshToken)
		assert.ErrorIs(t, err, ErrExpiredToken)
	})
}

func TestExtractTokenFromHeader(t *testing.T) {
	testCases := []struct {
		name        string
		header      string
		expectedErr string
		expectedOk  bool
	}{
		{
			name:       "valid bearer token",
			header:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectedOk: true,
		},
		{
			name:        "empty header",
			header:      "",
			expectedErr: "authorization header is required",
		},
		{
			name:        "invalid format - no Bearer",
			header:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectedErr: "authorization header must start with 'Bearer '",
		},
		{
			name:        "invalid format - wrong prefix",
			header:      "Basic dGVzdDp0ZXN0",
			expectedErr: "authorization header must start with 'Bearer '",
		},
		{
			name:        "Bearer with no token",
			header:      "Bearer ",
			expectedErr: "token is required",
		},
		{
			name:        "Bearer with spaces only",
			header:      "Bearer    ",
			expectedErr: "token is required",
		},
		{
			name:        "too short header",
			header:      "Bear",
			expectedErr: "invalid authorization header format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := ExtractTokenFromHeader(tc.header)

			if tc.expectedOk {
				require.NoError(t, err)
				assert.NotEmpty(t, token)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				assert.Empty(t, token)
			}
		})
	}
}

func TestJWTManager_GetTokenDuration(t *testing.T) {
	accessDuration := 15 * time.Minute
	refreshDuration := 7 * 24 * time.Hour
	jm := NewJWTManager("test-secret", accessDuration, refreshDuration)

	assert.Equal(t, accessDuration, jm.GetTokenDuration(TokenTypeAccess))
	assert.Equal(t, refreshDuration, jm.GetTokenDuration(TokenTypeRefresh))
	assert.Equal(t, time.Duration(0), jm.GetTokenDuration(TokenType("invalid")))
}

func TestIsTokenExpired(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid token not expired", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		assert.False(t, IsTokenExpired(token))
	})

	t.Run("expired token", func(t *testing.T) {
		expiredJM := NewJWTManager("test-secret", -1*time.Hour, 7*24*time.Hour)
		token, err := expiredJM.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		assert.True(t, IsTokenExpired(token))
	})

	t.Run("invalid token", func(t *testing.T) {
		assert.True(t, IsTokenExpired("invalid.token"))
	})
}

func TestGetTokenClaims(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid token", func(t *testing.T) {
		userID := uint(123)
		email := "test@example.com"
		token, err := jm.GenerateAccessToken(userID, email, nil)
		require.NoError(t, err)

		claims, err := GetTokenClaims(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, TokenTypeAccess, claims.TokenType)
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := GetTokenClaims("invalid.token")
		assert.Error(t, err)
	})
}

func TestJWTManager_GetTokenInfo(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	userID := uint(123)
	email := "test@example.com"
	teamID := uint(456)
	token, err := jm.GenerateAccessToken(userID, email, &teamID)
	require.NoError(t, err)

	info, err := jm.GetTokenInfo(token)
	require.NoError(t, err)

	assert.Equal(t, userID, info.UserID)
	assert.Equal(t, email, info.Email)
	assert.Equal(t, TokenTypeAccess, info.TokenType)
	assert.Equal(t, &teamID, info.TeamID)
	assert.Equal(t, "user:123", info.Subject)
	assert.Equal(t, "dbackup-api", info.Issuer)
	assert.False(t, info.IsExpired)
	assert.True(t, info.IssuedAt.Before(time.Now()))
	assert.True(t, info.ExpiresAt.After(time.Now()))
}

func TestClaims_RegisteredClaims(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
	require.NoError(t, err)

	claims, err := jm.ValidateToken(token)
	require.NoError(t, err)

	// Test RegisteredClaims fields
	assert.NotNil(t, claims.IssuedAt)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.NotBefore)
	assert.Equal(t, "dbackup-api", claims.Issuer)
	assert.Equal(t, "user:123", claims.Subject)

	// Test timing
	now := time.Now()
	assert.True(t, claims.IssuedAt.Time.Before(now.Add(time.Second)))
	assert.True(t, claims.ExpiresAt.Time.After(now))
	assert.True(t, claims.NotBefore.Time.Before(now.Add(time.Second)))
}

func TestTokenManager_WithRevocation(t *testing.T) {
	// Mock revoked token store
	mockStore := &MockRevokedTokenStore{
		revokedTokens: make(map[string]time.Time),
	}

	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
	tm := NewTokenManager(jm, mockStore)

	userID := uint(123)
	email := "test@example.com"

	t.Run("validate non-revoked token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(userID, email, nil)
		require.NoError(t, err)

		claims, err := tm.ValidateTokenWithRevocation(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
	})

	t.Run("validate revoked token", func(t *testing.T) {
		// Create token with JTI claim for revocation testing
		now := time.Now()
		claims := &Claims{
			UserID:    userID,
			Email:     email,
			TokenType: TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "test-jti-123",
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
				NotBefore: jwt.NewNumericDate(now),
				Issuer:    "dbackup-api",
				Subject:   "user:123",
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jm.secretKey)
		require.NoError(t, err)

		// Token should validate initially
		validClaims, err := tm.ValidateTokenWithRevocation(tokenString)
		require.NoError(t, err)
		assert.Equal(t, "test-jti-123", validClaims.ID)

		// Revoke the token
		mockStore.RevokeToken("test-jti-123", now.Add(15*time.Minute))

		// Token should now be invalid
		_, err = tm.ValidateTokenWithRevocation(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "revoked")
	})

	t.Run("revoke token", func(t *testing.T) {
		// Create token with JTI for revocation
		now := time.Now()
		claims := &Claims{
			UserID:    userID,
			Email:     email,
			TokenType: TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "revoke-test-123",
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
				Subject:   "user:123",
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jm.secretKey)
		require.NoError(t, err)

		// Revoke the token
		err = tm.RevokeToken(tokenString)
		require.NoError(t, err)

		// Verify it's in the store
		assert.True(t, mockStore.IsTokenRevoked("revoke-test-123"))
	})

	t.Run("cleanup expired tokens", func(t *testing.T) {
		err := tm.CleanupExpiredTokens()
		assert.NoError(t, err)
	})
}

// Mock implementation of RevokedTokenStore for testing
type MockRevokedTokenStore struct {
	revokedTokens map[string]time.Time
}

func (m *MockRevokedTokenStore) RevokeToken(tokenID string, expiresAt time.Time) error {
	m.revokedTokens[tokenID] = expiresAt
	return nil
}

func (m *MockRevokedTokenStore) IsTokenRevoked(tokenID string) bool {
	_, exists := m.revokedTokens[tokenID]
	return exists
}

func (m *MockRevokedTokenStore) CleanupExpiredTokens() error {
	now := time.Now()
	for tokenID, expiresAt := range m.revokedTokens {
		if expiresAt.Before(now) {
			delete(m.revokedTokens, tokenID)
		}
	}
	return nil
}

func TestTokenTypes(t *testing.T) {
	assert.Equal(t, "access", string(TokenTypeAccess))
	assert.Equal(t, "refresh", string(TokenTypeRefresh))
}

func TestTokenWithTeamID(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("with team ID", func(t *testing.T) {
		teamID := uint(789)
		token, err := jm.GenerateAccessToken(123, "test@example.com", &teamID)
		require.NoError(t, err)

		claims, err := jm.ValidateToken(token)
		require.NoError(t, err)
		assert.NotNil(t, claims.TeamID)
		assert.Equal(t, teamID, *claims.TeamID)
	})

	t.Run("without team ID", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		claims, err := jm.ValidateToken(token)
		require.NoError(t, err)
		assert.Nil(t, claims.TeamID)
	})
}

func TestJWTManager_EdgeCases(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("zero user ID", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(0, "test@example.com", nil)
		require.NoError(t, err)

		claims, err := jm.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, uint(0), claims.UserID)
		assert.Equal(t, "user:0", claims.Subject)
	})

	t.Run("empty email", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "", nil)
		require.NoError(t, err)

		claims, err := jm.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "", claims.Email)
	})

	t.Run("very long email", func(t *testing.T) {
		longEmail := strings.Repeat("a", 1000) + "@example.com"

		token, err := jm.GenerateAccessToken(123, longEmail, nil)
		require.NoError(t, err)

		claims, err := jm.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, longEmail, claims.Email)
	})
}

func TestJWTManager_SigningMethodValidation(t *testing.T) {
	jm := NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)

	// Test with a manually crafted token that would use wrong signing method
	// This tests the signing method validation in the ValidateToken function
	_, err := jm.ValidateToken("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.invalid.token")
	assert.Error(t, err)
}