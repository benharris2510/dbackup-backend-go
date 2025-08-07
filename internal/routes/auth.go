package routes

import (
	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/handlers"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/labstack/echo/v4"
)

// SetupAuthRoutes sets up authentication routes
func SetupAuthRoutes(e *echo.Echo, jm *auth.JWTManager, ph *auth.PasswordHasher, tm *auth.TOTPManager) {
	// Create auth handler
	authHandler := handlers.NewAuthHandler(jm, ph, tm)
	
	// Create 2FA handler
	twoFAHandler := handlers.NewTwoFAHandler(tm, ph)

	// Use centralized cookie-based JWT middleware
	cookieJWTMiddleware := middleware.CookieJWT(jm)

	// Auth routes group
	authGroup := e.Group("/api/auth")

	// Public routes (no authentication required)
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	
	// Demo route to show automatic array serialization (public for testing)
	authGroup.GET("/demo/users", authHandler.GetUsers)

	// Protected routes (authentication required) - use cookie-based auth
	authGroup.GET("/session", authHandler.Session, cookieJWTMiddleware)
	authGroup.POST("/logout", authHandler.Logout, cookieJWTMiddleware)
	
	// Demo route to show automatic array serialization
	authGroup.GET("/users", authHandler.GetUsers, cookieJWTMiddleware)

	// 2FA routes (authentication required) - use cookie-based auth
	twoFAGroup := authGroup.Group("/2fa", cookieJWTMiddleware)
	twoFAGroup.GET("/status", twoFAHandler.Status)
	twoFAGroup.POST("/setup", twoFAHandler.Setup)
	twoFAGroup.POST("/enable", twoFAHandler.Enable)
	twoFAGroup.POST("/disable", twoFAHandler.Disable)
	twoFAGroup.POST("/verify", twoFAHandler.Verify)
}