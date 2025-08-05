package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	jwtManager     *auth.JWTManager
	passwordHasher *auth.PasswordHasher
	totpManager    *auth.TOTPManager
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(jwtManager *auth.JWTManager, passwordHasher *auth.PasswordHasher, totpManager *auth.TOTPManager) *AuthHandler {
	return &AuthHandler{
		jwtManager:     jwtManager,
		passwordHasher: passwordHasher,
		totpManager:    totpManager,
	}
}

// RegisterRequest represents the user registration request
type RegisterRequest struct {
	Email                string `json:"email" validate:"required,email"`
	Password             string `json:"password" validate:"required,min=8,max=128"`
	PasswordConfirmation string `json:"password_confirmation" validate:"required"`
	FirstName            string `json:"first_name" validate:"required,min=1,max=50"`
	LastName             string `json:"last_name" validate:"required,min=1,max=50"`
	CompanyName          string `json:"company_name,omitempty" validate:"max=100"`
	AcceptTerms          bool   `json:"accept_terms"`
}

// RegisterResponse represents the user registration response
type RegisterResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	User    *UserPublic `json:"user,omitempty"`
	Tokens  *TokenPair  `json:"tokens,omitempty"`
}

// UserPublic represents the public user information
type UserPublic struct {
	ID          uint   `json:"id"`
	UID         string `json:"uid"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	CompanyName string `json:"company_name,omitempty"`
	IsVerified  bool   `json:"is_verified"`
	Has2FA      bool   `json:"has_2fa"`
	CreatedAt   string `json:"created_at"`
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// Register handles user registration
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Validate password confirmation
	if req.Password != req.PasswordConfirmation {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Validation failed",
			Errors: map[string]string{
				"password_confirmation": "Password confirmation does not match",
			},
		})
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Password does not meet requirements",
			Errors: map[string]string{
				"password": err.Error(),
			},
		})
	}

	// Validate terms acceptance
	if !req.AcceptTerms {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Terms and conditions must be accepted",
			Errors: map[string]string{
				"accept_terms": "You must accept the terms and conditions",
			},
		})
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	db := database.GetDB()
	var existingUser models.User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.JSON(http.StatusConflict, ErrorResponse{
			Success: false,
			Message: "User already exists",
			Errors: map[string]string{
				"email": "An account with this email address already exists",
			},
		})
	} else if err != gorm.ErrRecordNotFound {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Hash password
	hashedPassword, err := h.passwordHasher.HashPassword(req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process password")
	}

	// Create user
	user := models.User{
		Email:           req.Email,
		Password:        hashedPassword,
		FirstName:       strings.TrimSpace(req.FirstName),
		LastName:        strings.TrimSpace(req.LastName),
		IsActive:        true,
		IsEmailVerified: false, // Email verification required
	}

	// Set user before create hook will generate UID
	if err := user.BeforeCreate(db); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user")
	}

	// Save user to database
	if err := db.Create(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user account")
	}

	// Generate tokens
	accessToken, refreshToken, err := h.jwtManager.GenerateTokenPair(user.ID, user.Email, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate authentication tokens")
	}

	// Prepare response
	userPublic := &UserPublic{
		ID:          user.ID,
		UID:         user.UID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		CompanyName: req.CompanyName, // Use from request since not stored in user model
		IsVerified:  user.IsEmailVerified,
		Has2FA:      user.TwoFactorEnabled,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	tokens := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.jwtManager.GetTokenDuration(auth.TokenTypeAccess).Seconds()),
	}

	response := RegisterResponse{
		Success: true,
		Message: "Account created successfully. Please check your email to verify your account.",
		User:    userPublic,
		Tokens:  tokens,
	}

	return c.JSON(http.StatusCreated, response)
}

// LoginRequest represents the user login request
type LoginRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
	TOTPCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
	RememberMe bool   `json:"remember_me"`
}

// LoginResponse represents the user login response
type LoginResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	User      *UserPublic `json:"user,omitempty"`
	Tokens    *TokenPair  `json:"tokens,omitempty"`
	Requires2FA bool      `json:"requires_2fa,omitempty"`
}

// Login handles user authentication
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Find user by email
	db := database.GetDB()
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(http.StatusUnauthorized, ErrorResponse{
				Success: false,
				Message: "Invalid credentials",
			})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	// Check if account is active
	if !user.IsActive {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Account is disabled. Please contact support.",
		})
	}

	// Verify password
	valid, err := h.passwordHasher.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Authentication error")
	}

	if !valid {
		// Update failed login attempts
		user.LoginAttempts++
		
		// Lock account after 5 failed attempts
		if user.LoginAttempts >= 5 {
			user.IsActive = false
			lockUntil := time.Now().Add(30 * time.Minute)
			user.LockedUntil = &lockUntil
		}
		
		db.Save(&user)
		
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Invalid credentials",
		})
	}

	// Check 2FA if enabled
	if user.TwoFactorEnabled {
		if req.TOTPCode == "" && req.BackupCode == "" {
			return c.JSON(http.StatusOK, LoginResponse{
				Success:     false,
				Message:     "Two-factor authentication required",
				Requires2FA: true,
			})
		}

		// Verify TOTP code or backup code
		var totpValid bool
		if req.TOTPCode != "" && user.TwoFactorSecret != nil {
			totpValid = h.totpManager.ValidateCodeWithSkew(req.TOTPCode, *user.TwoFactorSecret, 1)
		} else if req.BackupCode != "" {
			// Check backup code (this would typically involve checking against stored backup codes)
			// For now, we'll use basic validation
			totpValid = h.totpManager.ValidateBackupCode(req.BackupCode)
		}

		if !totpValid {
			return c.JSON(http.StatusUnauthorized, ErrorResponse{
				Success: false,
				Message: "Invalid two-factor authentication code",
			})
		}
	}

	// Reset failed login attempts on successful login
	user.LoginAttempts = 0
	user.LockedUntil = nil
	user.LastLoginAt = func() *time.Time {
		now := time.Now()
		return &now
	}()
	realIP := c.RealIP()
	user.LastLoginIP = &realIP
	db.Save(&user)

	// Generate tokens (with extended duration if remember me is checked)
	var accessTokenDuration, refreshTokenDuration time.Duration
	if req.RememberMe {
		accessTokenDuration = 24 * time.Hour     // 24 hours
		refreshTokenDuration = 30 * 24 * time.Hour // 30 days
	} else {
		accessTokenDuration = h.jwtManager.GetTokenDuration(auth.TokenTypeAccess)
		refreshTokenDuration = h.jwtManager.GetTokenDuration(auth.TokenTypeRefresh)
	}

	// Create temporary JWT manager with custom durations if needed
	jm := h.jwtManager
	if req.RememberMe {
		jm = auth.NewJWTManager(
			string(h.jwtManager.GetSecretKey()), // We need to expose this method
			accessTokenDuration,
			refreshTokenDuration,
		)
	}

	accessToken, refreshToken, err := jm.GenerateTokenPair(user.ID, user.Email, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate authentication tokens")
	}

	// Prepare response
	userPublic := &UserPublic{
		ID:          user.ID,
		UID:         user.UID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		CompanyName: "", // Not stored in user model for this implementation
		IsVerified:  user.IsEmailVerified,
		Has2FA:      user.TwoFactorEnabled,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	tokens := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenDuration.Seconds()),
	}

	response := LoginResponse{
		Success: true,
		Message: "Login successful",
		User:    userPublic,
		Tokens:  tokens,
	}

	return c.JSON(http.StatusOK, response)
}

// RefreshRequest represents the token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// RefreshResponse represents the token refresh response
type RefreshResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Tokens  *TokenPair `json:"tokens,omitempty"`
}

// Refresh handles token refresh
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Validate refresh token
	claims, err := h.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Invalid or expired refresh token",
		})
	}

	// Check if user still exists and is active
	db := database.GetDB()
	var user models.User
	if err := db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "User not found",
		})
	}

	if !user.IsActive {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Account is disabled",
		})
	}

	// Generate new token pair
	accessToken, refreshToken, err := h.jwtManager.GenerateTokenPair(user.ID, user.Email, claims.TeamID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate authentication tokens")
	}

	tokens := &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.jwtManager.GetTokenDuration(auth.TokenTypeAccess).Seconds()),
	}

	response := RefreshResponse{
		Success: true,
		Message: "Token refreshed successfully",
		Tokens:  tokens,
	}

	return c.JSON(http.StatusOK, response)
}

// SessionResponse represents the session information response
type SessionResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	User    *UserPublic `json:"user,omitempty"`
}

// Session returns current user session information
func (h *AuthHandler) Session(c echo.Context) error {
	// Get user from context (set by auth middleware)
	userID := c.Get("user_id")
	if userID == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}

	// Get user from database
	db := database.GetDB()
	var user models.User
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	// Prepare response
	userPublic := &UserPublic{
		ID:          user.ID,
		UID:         user.UID,
		Email:       user.Email,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		CompanyName: "", // Not stored in user model for this implementation
		IsVerified:  user.IsEmailVerified,
		Has2FA:      user.TwoFactorEnabled,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	response := SessionResponse{
		Success: true,
		Message: "Session retrieved successfully",
		User:    userPublic,
	}

	return c.JSON(http.StatusOK, response)
}

// LogoutResponse represents the logout response
type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Logout handles user logout (token invalidation would be handled by token revocation)
func (h *AuthHandler) Logout(c echo.Context) error {
	// In a full implementation, this would revoke the current token
	// For now, we'll just return a success response
	response := LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	}

	return c.JSON(http.StatusOK, response)
}