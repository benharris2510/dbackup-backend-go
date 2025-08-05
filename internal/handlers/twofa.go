package handlers

import (
	"net/http"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/labstack/echo/v4"
)

// TwoFAHandler handles two-factor authentication operations
type TwoFAHandler struct {
	totpManager    *auth.TOTPManager
	passwordHasher *auth.PasswordHasher
}

// NewTwoFAHandler creates a new two-factor authentication handler
func NewTwoFAHandler(totpManager *auth.TOTPManager, passwordHasher *auth.PasswordHasher) *TwoFAHandler {
	return &TwoFAHandler{
		totpManager:    totpManager,
		passwordHasher: passwordHasher,
	}
}

// SetupRequest represents the 2FA setup initiation request
type SetupRequest struct {
	Password string `json:"password" validate:"required"`
}

// SetupResponse represents the 2FA setup response
type SetupResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Secret    string `json:"secret,omitempty"`
	QRCodeURL string `json:"qr_code_url,omitempty"`
}

// Setup initiates 2FA setup by generating a secret and QR code
func (h *TwoFAHandler) Setup(c echo.Context) error {
	var req SetupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

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

	// Verify current password
	valid, err := h.passwordHasher.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Authentication error")
	}

	if !valid {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Invalid password",
		})
	}

	// Check if 2FA is already enabled
	if user.TwoFactorEnabled {
		return c.JSON(http.StatusConflict, ErrorResponse{
			Success: false,
			Message: "Two-factor authentication is already enabled",
		})
	}

	// Generate new secret
	key, err := h.totpManager.GenerateSecret(user.Email)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate 2FA secret")
	}

	secret := key.Secret()
	qrCodeURL := h.totpManager.GetQRCodeURL(key)

	response := SetupResponse{
		Success:   true,
		Message:   "2FA setup initiated. Please scan the QR code with your authenticator app.",
		Secret:    secret,
		QRCodeURL: qrCodeURL,
	}

	return c.JSON(http.StatusOK, response)
}

// EnableRequest represents the 2FA enable request
type EnableRequest struct {
	Password string `json:"password" validate:"required"`
	Secret   string `json:"secret" validate:"required"`
	Code     string `json:"code" validate:"required,len=6"`
}

// EnableResponse represents the 2FA enable response
type EnableResponse struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	BackupCodes []string `json:"backup_codes,omitempty"`
}

// Enable enables 2FA after verifying the TOTP code
func (h *TwoFAHandler) Enable(c echo.Context) error {
	var req EnableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

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

	// Verify current password
	valid, err := h.passwordHasher.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Authentication error")
	}

	if !valid {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Invalid password",
		})
	}

	// Check if 2FA is already enabled
	if user.TwoFactorEnabled {
		return c.JSON(http.StatusConflict, ErrorResponse{
			Success: false,
			Message: "Two-factor authentication is already enabled",
		})
	}

	// Verify TOTP code
	if !h.totpManager.ValidateCodeWithSkew(req.Code, req.Secret, 1) {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid verification code",
		})
	}

	// Generate backup codes
	backupCodes, err := h.totpManager.GenerateBackupCodes(8)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate backup codes")
	}

	// Hash backup codes for storage
	hashedBackupCode, err := h.passwordHasher.HashPassword(backupCodes[0]) // Store first as example
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process backup codes")
	}

	// Enable 2FA
	user.TwoFactorEnabled = true
	user.TwoFactorSecret = &req.Secret
	user.TwoFactorBackupCode = &hashedBackupCode
	user.UpdatedAt = time.Now()

	if err := db.Save(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enable 2FA")
	}

	response := EnableResponse{
		Success:     true,
		Message:     "Two-factor authentication has been enabled successfully",
		BackupCodes: backupCodes,
	}

	return c.JSON(http.StatusOK, response)
}

// DisableRequest represents the 2FA disable request
type DisableRequest struct {
	Password string `json:"password" validate:"required"`
	Code     string `json:"code,omitempty"`
}

// DisableResponse represents the 2FA disable response
type DisableResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Disable disables 2FA after verifying password and optionally TOTP code
func (h *TwoFAHandler) Disable(c echo.Context) error {
	var req DisableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

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

	// Verify current password
	valid, err := h.passwordHasher.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Authentication error")
	}

	if !valid {
		return c.JSON(http.StatusUnauthorized, ErrorResponse{
			Success: false,
			Message: "Invalid password",
		})
	}

	// Check if 2FA is currently enabled
	if !user.TwoFactorEnabled {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Two-factor authentication is not enabled",
		})
	}

	// If TOTP code is provided, verify it
	if req.Code != "" && user.TwoFactorSecret != nil {
		if !h.totpManager.ValidateCodeWithSkew(req.Code, *user.TwoFactorSecret, 1) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{
				Success: false,
				Message: "Invalid verification code",
			})
		}
	}

	// Disable 2FA
	user.TwoFactorEnabled = false
	user.TwoFactorSecret = nil
	user.TwoFactorBackupCode = nil
	user.UpdatedAt = time.Now()

	if err := db.Save(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to disable 2FA")
	}

	response := DisableResponse{
		Success: true,
		Message: "Two-factor authentication has been disabled successfully",
	}

	return c.JSON(http.StatusOK, response)
}

// VerifyRequest represents the 2FA verification request
type VerifyRequest struct {
	Code string `json:"code" validate:"required,len=6"`
}

// VerifyResponse represents the 2FA verification response
type VerifyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Valid   bool   `json:"valid"`
}

// Verify verifies a TOTP code for an authenticated user
func (h *TwoFAHandler) Verify(c echo.Context) error {
	var req VerifyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

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

	// Check if 2FA is enabled
	if !user.TwoFactorEnabled || user.TwoFactorSecret == nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Two-factor authentication is not enabled",
		})
	}

	// Verify TOTP code
	valid := h.totpManager.ValidateCodeWithSkew(req.Code, *user.TwoFactorSecret, 1)

	response := VerifyResponse{
		Success: true,
		Message: "Code verification completed",
		Valid:   valid,
	}

	return c.JSON(http.StatusOK, response)
}

// StatusResponse represents the 2FA status response
type StatusResponse struct {
	Success bool `json:"success"`
	Message string `json:"message"`
	Enabled bool `json:"enabled"`
}

// Status returns the current 2FA status for the authenticated user
func (h *TwoFAHandler) Status(c echo.Context) error {
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

	response := StatusResponse{
		Success: true,
		Message: "2FA status retrieved successfully",
		Enabled: user.TwoFactorEnabled,
	}

	return c.JSON(http.StatusOK, response)
}