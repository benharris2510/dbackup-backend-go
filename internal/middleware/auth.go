package middleware

import (
	"net/http"
	"strings"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/labstack/echo/v4"
)

// AuthConfig holds configuration for the auth middleware
type AuthConfig struct {
	// JWTManager is the JWT manager instance
	JWTManager *auth.JWTManager

	// TokenManager is the token manager with revocation support (optional)
	TokenManager *auth.TokenManager

	// Skipper defines a function to skip middleware
	Skipper Skipper

	// ErrorHandler defines a function which is executed for an invalid token
	ErrorHandler AuthErrorHandler

	// SuccessHandler defines a function which is executed for a valid token
	SuccessHandler AuthSuccessHandler

	// TokenLookup is a string in the form of "<source>:<name>" that is used
	// to extract token from the request.
	// Optional. Default value "header:Authorization".
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "cookie:<name>"
	TokenLookup string

	// AuthScheme to be used in the Authorization header.
	// Optional. Default value "Bearer".
	AuthScheme string

	// RequireAccessToken when true, only accepts access tokens
	// When false, accepts both access and refresh tokens
	RequireAccessToken bool
}

// Skipper defines a function to skip middleware
type Skipper func(c echo.Context) bool

// AuthErrorHandler defines a function which is executed for an invalid token
type AuthErrorHandler func(error) error

// AuthSuccessHandler defines a function which is executed for a valid token
type AuthSuccessHandler func(c echo.Context)

// DefaultSkipper returns false which processes the middleware
func DefaultSkipper(echo.Context) bool {
	return false
}

// DefaultAuthErrorHandler is the default error handler
func DefaultAuthErrorHandler(err error) error {
	return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
}

// DefaultAuthSuccessHandler is the default success handler (no-op)
func DefaultAuthSuccessHandler(echo.Context) {}

// DefaultAuthConfig is the default auth middleware config
var DefaultAuthConfig = AuthConfig{
	Skipper:            DefaultSkipper,
	ErrorHandler:       DefaultAuthErrorHandler,
	SuccessHandler:     DefaultAuthSuccessHandler,
	TokenLookup:        "header:Authorization",
	AuthScheme:         "Bearer",
	RequireAccessToken: true,
}

// JWT returns a JWT auth middleware
func JWT(jwtManager *auth.JWTManager) echo.MiddlewareFunc {
	c := DefaultAuthConfig
	c.JWTManager = jwtManager
	return JWTWithConfig(c)
}

// JWTWithRevocation returns a JWT auth middleware with token revocation support
func JWTWithRevocation(tokenManager *auth.TokenManager) echo.MiddlewareFunc {
	c := DefaultAuthConfig
	c.TokenManager = tokenManager
	c.JWTManager = tokenManager.JWTManager
	return JWTWithConfig(c)
}

// JWTWithConfig returns a JWT auth middleware with config
func JWTWithConfig(config AuthConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultAuthConfig.Skipper
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultAuthConfig.ErrorHandler
	}
	if config.SuccessHandler == nil {
		config.SuccessHandler = DefaultAuthConfig.SuccessHandler
	}
	if config.TokenLookup == "" {
		config.TokenLookup = DefaultAuthConfig.TokenLookup
	}
	if config.AuthScheme == "" {
		config.AuthScheme = DefaultAuthConfig.AuthScheme
	}
	if config.JWTManager == nil {
		panic("JWT manager is required")
	}

	// Initialize
	parts := strings.Split(config.TokenLookup, ":")
	extractor := jwtFromHeader(parts[1], config.AuthScheme)
	switch parts[0] {
	case "query":
		extractor = jwtFromQuery(parts[1])
	case "cookie":
		extractor = jwtFromCookie(parts[1])
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			token, err := extractor(c)
			if err != nil {
				return config.ErrorHandler(err)
			}

			var claims *auth.Claims
			if config.TokenManager != nil {
				// Use token manager for revocation checking
				claims, err = config.TokenManager.ValidateTokenWithRevocation(token)
			} else {
				// Use basic JWT validation
				claims, err = config.JWTManager.ValidateToken(token)
			}

			if err != nil {
				return config.ErrorHandler(err)
			}

			// Check token type if required
			if config.RequireAccessToken && claims.TokenType != auth.TokenTypeAccess {
				return config.ErrorHandler(auth.ErrInvalidTokenType)
			}

			// Set user info in context
			c.Set("user", claims)
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("token_type", claims.TokenType)
			if claims.TeamID != nil {
				c.Set("team_id", *claims.TeamID)
			}

			config.SuccessHandler(c)

			return next(c)
		}
	}
}

// jwtFromHeader returns a function that extracts token from the request header
func jwtFromHeader(header, authScheme string) func(echo.Context) (string, error) {
	return func(c echo.Context) (string, error) {
		authHeader := c.Request().Header.Get(header)
		if authHeader == "" {
			return "", echo.NewHTTPError(http.StatusUnauthorized, "missing or empty authorization header")
		}
		return auth.ExtractTokenFromHeader(authHeader)
	}
}

// jwtFromQuery returns a function that extracts token from the query string
func jwtFromQuery(param string) func(echo.Context) (string, error) {
	return func(c echo.Context) (string, error) {
		token := c.QueryParam(param)
		if token == "" {
			return "", echo.NewHTTPError(http.StatusUnauthorized, "missing token in query parameter")
		}
		return token, nil
	}
}

// jwtFromCookie returns a function that extracts token from a cookie
func jwtFromCookie(name string) func(echo.Context) (string, error) {
	return func(c echo.Context) (string, error) {
		cookie, err := c.Cookie(name)
		if err != nil {
			return "", echo.NewHTTPError(http.StatusUnauthorized, "missing token in cookie")
		}
		if cookie.Value == "" {
			return "", echo.NewHTTPError(http.StatusUnauthorized, "empty token in cookie")
		}
		return cookie.Value, nil
	}
}

// GetUserFromContext extracts user claims from the echo context
func GetUserFromContext(c echo.Context) *auth.Claims {
	user := c.Get("user")
	if user == nil {
		return nil
	}

	if claims, ok := user.(*auth.Claims); ok {
		return claims
	}

	return nil
}

// GetUserIDFromContext extracts user ID from the echo context
func GetUserIDFromContext(c echo.Context) (uint, bool) {
	userID := c.Get("user_id")
	if userID == nil {
		return 0, false
	}

	if id, ok := userID.(uint); ok {
		return id, true
	}

	return 0, false
}

// GetTeamIDFromContext extracts team ID from the echo context
func GetTeamIDFromContext(c echo.Context) (uint, bool) {
	teamID := c.Get("team_id")
	if teamID == nil {
		return 0, false
	}

	if id, ok := teamID.(uint); ok {
		return id, true
	}

	return 0, false
}

// RequireTeamMembership returns middleware that ensures user is a member of a team
func RequireTeamMembership() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			_, hasTeam := GetTeamIDFromContext(c)
			if !hasTeam {
				return echo.NewHTTPError(http.StatusForbidden, "team membership required")
			}
			return next(c)
		}
	}
}

// RequireRole returns middleware that ensures user has specific role permissions
func RequireRole(permission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUserFromContext(c)
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			// For now, we'll implement a basic permission check
			// This would typically integrate with a more sophisticated RBAC system
			switch permission {
			case "admin":
				// Check if user has admin privileges
				// This would typically be stored in the JWT claims or fetched from database
				return echo.NewHTTPError(http.StatusForbidden, "admin privileges required")
			case "member":
				// Basic membership is granted if user is authenticated
				return next(c)
			default:
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}
		}
	}
}

// RefreshTokenOnly returns middleware that only accepts refresh tokens
func RefreshTokenOnly() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUserFromContext(c)
			if user == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}

			if user.TokenType != auth.TokenTypeRefresh {
				return echo.NewHTTPError(http.StatusUnauthorized, "refresh token required")
			}

			return next(c)
		}
	}
}

// AllowRefreshToken returns middleware config that accepts both access and refresh tokens
func AllowRefreshToken(jwtManager *auth.JWTManager) echo.MiddlewareFunc {
	config := DefaultAuthConfig
	config.JWTManager = jwtManager
	config.RequireAccessToken = false
	return JWTWithConfig(config)
}

// CookieJWT returns a JWT middleware that extracts tokens from cookies (default for protected routes)
func CookieJWT(jwtManager *auth.JWTManager) echo.MiddlewareFunc {
	config := DefaultAuthConfig
	config.JWTManager = jwtManager
	config.TokenLookup = "cookie:access_token"
	config.RequireAccessToken = true
	config.SuccessHandler = func(c echo.Context) {
		// Get user ID from claims and fetch full user model
		userID := c.Get("user_id")
		if userID != nil {
			// Import database package to get user
			db := database.GetDB()
			var user models.User
			if err := db.Where("id = ?", userID).First(&user).Error; err == nil {
				c.Set("user_model", &user)
			}
		}
	}
	return JWTWithConfig(config)
}

// OptionalAuth returns middleware that tries to authenticate but doesn't fail if no token
func OptionalAuth(jwtManager *auth.JWTManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Try to get token
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				// No token provided, continue without authentication
				return next(c)
			}

			token, err := auth.ExtractTokenFromHeader(authHeader)
			if err != nil {
				// Invalid token format, continue without authentication
				return next(c)
			}

			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				// Invalid token, continue without authentication
				return next(c)
			}

			// Set user info in context
			c.Set("user", claims)
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("token_type", claims.TokenType)
			if claims.TeamID != nil {
				c.Set("team_id", *claims.TeamID)
			}

			return next(c)
		}
	}
}

// IsAuthenticated checks if the current request is authenticated
func IsAuthenticated(c echo.Context) bool {
	return GetUserFromContext(c) != nil
}

// GetUserModel extracts the full user model from context (set by cookie JWT middleware)
func GetUserModel(c echo.Context) *models.User {
	user, ok := c.Get("user_model").(*models.User)
	if !ok {
		return nil
	}
	return user
}
