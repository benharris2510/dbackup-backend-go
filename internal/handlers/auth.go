package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/responses"
	"github.com/dbackup/backend-go/internal/utils"
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
	Email          string `json:"email" validate:"required,email"`
	Password       string `json:"password" validate:"required,min=8,max=128"`
	FullName       string `json:"full_name" validate:"required,min=1,max=100"`
	CompanyName    string `json:"company_name,omitempty" validate:"max=100"`
	AcceptTerms    bool   `json:"accept_terms"`
	RecaptchaToken string `json:"recaptcha_token,omitempty"`
}

// Temporary structs to fix compilation (will remove these with better approach)
type RefreshResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Tokens  *TokenPair `json:"tokens,omitempty"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type ErrorResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// Register handles user registration
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Parse full name into first and last name
	names := strings.Fields(strings.TrimSpace(req.FullName))
	var firstName, lastName string
	if len(names) == 0 {
		return responses.ValidationError(c, "Full name is required", map[string]string{
			"full_name": "Full name cannot be empty",
		})
	} else if len(names) == 1 {
		firstName = names[0]
		lastName = ""
	} else {
		firstName = names[0]
		lastName = strings.Join(names[1:], " ")
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		return responses.ValidationError(c, "Password does not meet requirements", map[string]string{
			"password": err.Error(),
		})
	}

	// Validate terms acceptance
	if !req.AcceptTerms {
		return responses.ValidationError(c, "Terms and conditions must be accepted", map[string]string{
			"accept_terms": "You must accept the terms and conditions",
		})
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	db := database.GetDB()
	var existingUser models.User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return responses.ValidationError(c, "User already exists", map[string]string{
			"email": "An account with this email address already exists",
		})
	} else if err != gorm.ErrRecordNotFound {
		return responses.InternalError(c, "Database error")
	}

	// Hash password
	hashedPassword, err := h.passwordHasher.HashPassword(req.Password)
	if err != nil {
		return responses.InternalError(c, "Failed to process password")
	}

	// Create user
	user := models.User{
		Email:           req.Email,
		Password:        hashedPassword,
		FirstName:       firstName,
		LastName:        lastName,
		IsActive:        true,
		IsEmailVerified: false, // Email verification required
	}

	// Set user before create hook will generate UID
	if err := user.BeforeCreate(db); err != nil {
		return responses.InternalError(c, "Failed to create user")
	}

	// Save user to database
	if err := db.Create(&user).Error; err != nil {
		return responses.InternalError(c, "Failed to create user account")
	}

	// Generate tokens
	accessToken, refreshToken, err := h.jwtManager.GenerateTokenPair(user.ID, user.Email, nil)
	if err != nil {
		return responses.InternalError(c, "Failed to generate authentication tokens")
	}

	// Set tokens as httpOnly cookies
	utils.SetTokenCookies(c, accessToken, refreshToken)

	return responses.Created(c, "Account created successfully. Please check your email to verify your account.", &user)
}

// LoginRequest represents the user login request
type LoginRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Password   string `json:"password" validate:"required"`
	TOTPCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
	RememberMe bool   `json:"remember_me"`
}

// Login handles user authentication
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Find user by email
	db := database.GetDB()
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.Unauthorized(c, "Invalid credentials")
		}
		return responses.InternalError(c, "Database error")
	}

	// Check if account is active
	if !user.IsActive {
		return responses.Unauthorized(c, "Account is disabled. Please contact support.")
	}

	// Verify password
	valid, err := h.passwordHasher.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return responses.InternalError(c, "Authentication error")
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

		return responses.Unauthorized(c, "Invalid credentials")
	}

	// Check 2FA if enabled
	if user.TwoFactorEnabled {
		if req.TOTPCode == "" && req.BackupCode == "" {
			return responses.ErrorWithData(c, http.StatusUnauthorized, "Two-factor authentication required", map[string]interface{}{
				"requires_2fa": true,
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
			return responses.Unauthorized(c, "Invalid two-factor authentication code")
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
		accessTokenDuration = 24 * time.Hour       // 24 hours
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
		return responses.InternalError(c, "Failed to generate authentication tokens")
	}

	// Set tokens as httpOnly cookies
	utils.SetTokenCookies(c, accessToken, refreshToken)

	return responses.Success(c, "Login successful", &user)
}

// RefreshRequest represents the token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Refresh handles token refresh
func (h *AuthHandler) Refresh(c echo.Context) error {
	var req RefreshRequest
	if err := c.Bind(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Validate refresh token
	claims, err := h.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return responses.Unauthorized(c, "Invalid or expired refresh token")
	}

	// Check if user still exists and is active
	db := database.GetDB()
	var user models.User
	if err := db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return responses.Unauthorized(c, "User not found")
	}

	if !user.IsActive {
		return responses.Unauthorized(c, "Account is disabled")
	}

	// Generate new token pair
	accessToken, refreshToken, err := h.jwtManager.GenerateTokenPair(user.ID, user.Email, claims.TeamID)
	if err != nil {
		return responses.InternalError(c, "Failed to generate authentication tokens")
	}

	tokens := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int64(h.jwtManager.GetTokenDuration(auth.TokenTypeAccess).Seconds()),
	}

	return responses.Success(c, "Token refreshed successfully", tokens)
}

// Session returns current user session information
func (h *AuthHandler) Session(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	return responses.Success(c, "Session retrieved successfully", user)
}

// Logout handles user logout (token invalidation would be handled by token revocation)
func (h *AuthHandler) Logout(c echo.Context) error {
	// Clear token cookies
	utils.ClearTokenCookies(c)

	return responses.Success(c, "Logged out successfully", nil)
}

// GetUsers returns a list of users (demo endpoint to show array serialization)
func (h *AuthHandler) GetUsers(c echo.Context) error {
	db := database.GetDB()
	var users []models.User

	if err := db.Limit(10).Find(&users).Error; err != nil {
		return responses.InternalError(c, "Failed to fetch users")
	}

	// Just return the slice of models - automatic serialization handles the rest!
	return responses.Success(c, "Users retrieved successfully", users)
}
