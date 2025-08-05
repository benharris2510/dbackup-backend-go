package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test JWT manager
func createTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager("test-secret", 15*time.Minute, 7*24*time.Hour)
}

// Helper function to create a test token manager with mock revocation store
func createTestTokenManager() *auth.TokenManager {
	jm := createTestJWTManager()
	store := &MockRevokedTokenStore{
		revokedTokens: make(map[string]time.Time),
	}
	return auth.NewTokenManager(jm, store)
}

// Mock revoked token store
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

func TestJWT_DefaultMiddleware(t *testing.T) {
	jm := createTestJWTManager()
	middleware := JWT(jm)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test handler
	h := middleware(func(c echo.Context) error {
		user := GetUserFromContext(c)
		assert.NotNil(t, user)
		assert.Equal(t, uint(123), user.UserID)
		return c.String(http.StatusOK, "success")
	})

	t.Run("valid token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})
}

func TestJWTWithRevocation(t *testing.T) {
	tm := createTestTokenManager()
	middleware := JWTWithRevocation(tm)

	e := echo.New()
	h := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	t.Run("valid non-revoked token", func(t *testing.T) {
		token, err := tm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestJWTWithConfig_CustomConfig(t *testing.T) {
	jm := createTestJWTManager()
	
	t.Run("custom token lookup - query", func(t *testing.T) {
		config := AuthConfig{
			JWTManager:  jm,
			TokenLookup: "query:token",
			Skipper:     DefaultSkipper,
			ErrorHandler: DefaultAuthErrorHandler,
			SuccessHandler: DefaultAuthSuccessHandler,
		}
		middleware := JWTWithConfig(config)

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("custom token lookup - cookie", func(t *testing.T) {
		config := AuthConfig{
			JWTManager:  jm,
			TokenLookup: "cookie:auth",
			Skipper:     DefaultSkipper,
			ErrorHandler: DefaultAuthErrorHandler,
			SuccessHandler: DefaultAuthSuccessHandler,
		}
		middleware := JWTWithConfig(config)

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth", Value: token})
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("skip middleware", func(t *testing.T) {
		config := AuthConfig{
			JWTManager: jm,
			Skipper: func(c echo.Context) bool {
				return c.Request().URL.Path == "/public"
			},
			ErrorHandler: DefaultAuthErrorHandler,
			SuccessHandler: DefaultAuthSuccessHandler,
		}
		middleware := JWTWithConfig(config)

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("require access token - reject refresh token", func(t *testing.T) {
		config := AuthConfig{
			JWTManager:         jm,
			RequireAccessToken: true,
			Skipper:            DefaultSkipper,
			ErrorHandler:       DefaultAuthErrorHandler,
			SuccessHandler:     DefaultAuthSuccessHandler,
		}
		middleware := JWTWithConfig(config)

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		refreshToken, err := jm.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})

	t.Run("allow refresh token", func(t *testing.T) {
		config := AuthConfig{
			JWTManager:         jm,
			RequireAccessToken: false, // Allow both access and refresh tokens
			Skipper:            DefaultSkipper,
			ErrorHandler:       DefaultAuthErrorHandler,
			SuccessHandler:     DefaultAuthSuccessHandler,
		}
		middleware := JWTWithConfig(config)

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			user := GetUserFromContext(c)
			assert.Equal(t, auth.TokenTypeRefresh, user.TokenType)
			return c.String(http.StatusOK, "success")
		})

		refreshToken, err := jm.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestJWTWithConfig_Panic(t *testing.T) {
	assert.Panics(t, func() {
		JWTWithConfig(AuthConfig{}) // No JWT manager provided
	})
}

func TestGetUserFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("no user in context", func(t *testing.T) {
		user := GetUserFromContext(c)
		assert.Nil(t, user)
	})

	t.Run("valid user in context", func(t *testing.T) {
		claims := &auth.Claims{
			UserID: 123,
			Email:  "test@example.com",
		}
		c.Set("user", claims)

		user := GetUserFromContext(c)
		assert.NotNil(t, user)
		assert.Equal(t, uint(123), user.UserID)
		assert.Equal(t, "test@example.com", user.Email)
	})

	t.Run("invalid user type in context", func(t *testing.T) {
		c.Set("user", "invalid")
		user := GetUserFromContext(c)
		assert.Nil(t, user)
	})
}

func TestGetUserIDFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("no user ID in context", func(t *testing.T) {
		userID, ok := GetUserIDFromContext(c)
		assert.False(t, ok)
		assert.Equal(t, uint(0), userID)
	})

	t.Run("valid user ID in context", func(t *testing.T) {
		c.Set("user_id", uint(123))
		userID, ok := GetUserIDFromContext(c)
		assert.True(t, ok)
		assert.Equal(t, uint(123), userID)
	})

	t.Run("invalid user ID type in context", func(t *testing.T) {
		c.Set("user_id", "123")
		userID, ok := GetUserIDFromContext(c)
		assert.False(t, ok)
		assert.Equal(t, uint(0), userID)
	})
}

func TestGetTeamIDFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("no team ID in context", func(t *testing.T) {
		teamID, ok := GetTeamIDFromContext(c)
		assert.False(t, ok)
		assert.Equal(t, uint(0), teamID)
	})

	t.Run("valid team ID in context", func(t *testing.T) {
		c.Set("team_id", uint(456))
		teamID, ok := GetTeamIDFromContext(c)
		assert.True(t, ok)
		assert.Equal(t, uint(456), teamID)
	})
}

func TestRequireTeamMembership(t *testing.T) {
	middleware := RequireTeamMembership()

	e := echo.New()
	h := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	t.Run("with team membership", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("team_id", uint(456))

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("without team membership", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusForbidden, he.Code)
		}
	})
}

func TestRequireRole(t *testing.T) {
	t.Run("member role", func(t *testing.T) {
		middleware := RequireRole("member")

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", &auth.Claims{UserID: 123})

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("admin role - forbidden", func(t *testing.T) {
		middleware := RequireRole("admin")

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", &auth.Claims{UserID: 123})

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusForbidden, he.Code)
		}
	})

	t.Run("no user in context", func(t *testing.T) {
		middleware := RequireRole("member")

		e := echo.New()
		h := middleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})
}

func TestRefreshTokenOnly(t *testing.T) {
	middleware := RefreshTokenOnly()

	e := echo.New()
	h := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	t.Run("valid refresh token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", &auth.Claims{
			UserID:    123,
			TokenType: auth.TokenTypeRefresh,
		})

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("access token - rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user", &auth.Claims{
			UserID:    123,
			TokenType: auth.TokenTypeAccess,
		})

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})

	t.Run("no user in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})
}

func TestAllowRefreshToken(t *testing.T) {
	jm := createTestJWTManager()
	middleware := AllowRefreshToken(jm)

	e := echo.New()
	h := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	t.Run("access token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("refresh token", func(t *testing.T) {
		token, err := jm.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestOptionalAuth(t *testing.T) {
	jm := createTestJWTManager()
	middleware := OptionalAuth(jm)

	e := echo.New()
	h := middleware(func(c echo.Context) error {
		user := GetUserFromContext(c)
		if user != nil {
			return c.String(http.StatusOK, "authenticated")
		}
		return c.String(http.StatusOK, "anonymous")
	})

	t.Run("valid token", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "authenticated", rec.Body.String())
	})

	t.Run("no token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "anonymous", rec.Body.String())
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "anonymous", rec.Body.String())
	})
}

func TestIsAuthenticated(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	t.Run("authenticated", func(t *testing.T) {
		c.Set("user", &auth.Claims{UserID: 123})
		assert.True(t, IsAuthenticated(c))
	})

	t.Run("not authenticated", func(t *testing.T) {
		c.Set("user", nil) // Clear user
		assert.False(t, IsAuthenticated(c))
	})
}

func TestTokenExtractors(t *testing.T) {
	jm := createTestJWTManager()
	token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
	require.NoError(t, err)

	e := echo.New()

	t.Run("jwtFromQuery - valid", func(t *testing.T) {
		extractor := jwtFromQuery("token")
		req := httptest.NewRequest(http.MethodGet, "/?token="+token, nil)
		c := e.NewContext(req, nil)

		extracted, err := extractor(c)
		require.NoError(t, err)
		assert.Equal(t, token, extracted)
	})

	t.Run("jwtFromQuery - missing", func(t *testing.T) {
		extractor := jwtFromQuery("token")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := e.NewContext(req, nil)

		_, err := extractor(c)
		assert.Error(t, err)
	})

	t.Run("jwtFromCookie - valid", func(t *testing.T) {
		extractor := jwtFromCookie("auth")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth", Value: token})
		c := e.NewContext(req, nil)

		extracted, err := extractor(c)
		require.NoError(t, err)
		assert.Equal(t, token, extracted)
	})

	t.Run("jwtFromCookie - missing", func(t *testing.T) {
		extractor := jwtFromCookie("auth")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		c := e.NewContext(req, nil)

		_, err := extractor(c)
		assert.Error(t, err)
	})

	t.Run("jwtFromCookie - empty", func(t *testing.T) {
		extractor := jwtFromCookie("auth")
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "auth", Value: ""})
		c := e.NewContext(req, nil)

		_, err := extractor(c)
		assert.Error(t, err)
	})
}

func TestJWTMiddleware_TeamIDHandling(t *testing.T) {
	jm := createTestJWTManager()
	middleware := JWT(jm)

	e := echo.New()
	h := middleware(func(c echo.Context) error {
		teamID, hasTeam := GetTeamIDFromContext(c)
		if hasTeam {
			return c.JSON(http.StatusOK, map[string]interface{}{"team_id": teamID})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"team_id": nil})
	})

	t.Run("token with team ID", func(t *testing.T) {
		teamID := uint(456)
		token, err := jm.GenerateAccessToken(123, "test@example.com", &teamID)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"team_id":456`)
	})

	t.Run("token without team ID", func(t *testing.T) {
		token, err := jm.GenerateAccessToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"team_id":null`)
	})
}

func TestAuthMiddleware_IntegrationScenarios(t *testing.T) {
	jm := createTestJWTManager()

	t.Run("multiple middleware chain", func(t *testing.T) {
		// Create a chain: JWT + Team Membership + Member Role
		jwtMiddleware := JWT(jm)
		teamMiddleware := RequireTeamMembership()
		roleMiddleware := RequireRole("member")

		e := echo.New()
		
		// Chain middlewares
		h := jwtMiddleware(teamMiddleware(roleMiddleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})))

		// Generate token with team
		teamID := uint(456)
		token, err := jm.GenerateAccessToken(123, "test@example.com", &teamID)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("refresh endpoint workflow", func(t *testing.T) {
		// Allow both access and refresh tokens, then require refresh token only
		allowBothMiddleware := AllowRefreshToken(jm)
		refreshOnlyMiddleware := RefreshTokenOnly()

		e := echo.New()
		h := allowBothMiddleware(refreshOnlyMiddleware(func(c echo.Context) error {
			return c.String(http.StatusOK, "refreshed")
		}))

		// Generate refresh token
		refreshToken, err := jm.GenerateRefreshToken(123, "test@example.com", nil)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = h(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestDefaultHandlers(t *testing.T) {
	t.Run("DefaultSkipper", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		result := DefaultSkipper(c)
		assert.False(t, result)
	})

	t.Run("DefaultAuthErrorHandler", func(t *testing.T) {
		err := DefaultAuthErrorHandler(auth.ErrInvalidToken)
		assert.Error(t, err)
		if he, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusUnauthorized, he.Code)
		}
	})

	t.Run("DefaultAuthSuccessHandler", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Should not panic
		assert.NotPanics(t, func() {
			DefaultAuthSuccessHandler(c)
		})
	})
}