package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/handlers"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/routes"
	"github.com/dbackup/backend-go/internal/validation"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestTwoFAServer creates a test server with 2FA routes
func setupTestTwoFAServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Debug = true

	// Set up validator
	e.Validator = validation.NewValidator()

	// Setup middleware
	e.Use(echoMiddleware.LoggerWithConfig(echoMiddleware.LoggerConfig{
		Format:           `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"error":"${error}","latency_human":"${latency_human}","bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02T15:04:05.000Z07:00",
		Output:           os.Stdout,
	}))
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.RequestID())
	e.Use(echoMiddleware.TimeoutWithConfig(echoMiddleware.TimeoutConfig{
		Timeout: 30 * time.Second,
	}))
	e.Use(middleware.CORS())
	e.Use(middleware.SecurityHeaders())

	// Initialize auth components
	jwtManager := auth.NewJWTManager(
		"test-secret-key-for-testing-purposes-only",
		15*time.Minute, // access token
		24*time.Hour,   // refresh token
	)
	passwordHasher := auth.NewPasswordHasher()
	totpManager := auth.NewTOTPManager("dbackup-test")

	// Setup auth routes (includes 2FA routes)
	routes.SetupAuthRoutes(e, jwtManager, passwordHasher, totpManager)

	return e
}

func TestTwoFAStatus(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()
	server := setupTestTwoFAServer()

	// Helper to create a user and get tokens
	createUserAndGetTokens := func(email, password string) (*handlers.RegisterResponse, error) {
		payload := map[string]interface{}{
			"email":                 email,
			"password":              password,
			"password_confirmation": password,
			"first_name":            "Test",
			"last_name":             "User",
			"accept_terms":          true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			return nil, fmt.Errorf("failed to create user: %d", rec.Code)
		}
		var response handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}

	t.Run("Get 2FA status - disabled", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("status@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Get 2FA status
		req := httptest.NewRequest(http.MethodGet, "/api/auth/2fa/status", nil)
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.StatusResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "2FA status retrieved successfully", response.Message)
		assert.False(t, response.Enabled)
	})

	t.Run("Get 2FA status - no auth token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/2fa/status", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Get 2FA status - invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/2fa/status", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestTwoFASetup(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()
	server := setupTestTwoFAServer()

	// Helper to create a user and get tokens
	createUserAndGetTokens := func(email, password string) (*handlers.RegisterResponse, error) {
		payload := map[string]interface{}{
			"email":                 email,
			"password":              password,
			"password_confirmation": password,
			"first_name":            "Test",
			"last_name":             "User",
			"accept_terms":          true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			return nil, fmt.Errorf("failed to create user: %d", rec.Code)
		}
		var response handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}

	t.Run("Setup 2FA successfully", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("setup@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Setup 2FA
		payload := map[string]interface{}{
			"password": "TestPassword123!",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.SetupResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "2FA setup initiated. Please scan the QR code with your authenticator app.", response.Message)
		assert.NotEmpty(t, response.Secret)
		assert.NotEmpty(t, response.QRCodeURL)
		assert.Contains(t, response.QRCodeURL, "otpauth://totp/")
		assert.Contains(t, response.QRCodeURL, "setup@example.com")
		assert.Contains(t, response.QRCodeURL, "dbackup-test")
	})

	t.Run("Setup 2FA with wrong password", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("setup-wrong@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Setup 2FA with wrong password
		payload := map[string]interface{}{
			"password": "WrongPassword123!",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid password", response.Message)
	})

	t.Run("Setup 2FA without password", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("setup-nopass@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Setup 2FA without password
		payload := map[string]interface{}{}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Setup 2FA without authorization", func(t *testing.T) {
		payload := map[string]interface{}{
			"password": "TestPassword123!",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestTwoFAEnable(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()
	server := setupTestTwoFAServer()

	// Helper to create a user and get tokens
	createUserAndGetTokens := func(email, password string) (*handlers.RegisterResponse, error) {
		payload := map[string]interface{}{
			"email":                 email,
			"password":              password,
			"password_confirmation": password,
			"first_name":            "Test",
			"last_name":             "User",
			"accept_terms":          true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			return nil, fmt.Errorf("failed to create user: %d", rec.Code)
		}
		var response handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}

	// Helper to setup 2FA and get secret
	setup2FA := func(accessToken, password string) (string, error) {
		payload := map[string]interface{}{
			"password": password,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			return "", fmt.Errorf("failed to setup 2FA: %d", rec.Code)
		}
		var response handlers.SetupResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		if err != nil {
			return "", err
		}
		return response.Secret, nil
	}

	t.Run("Enable 2FA successfully", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("enable@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Setup 2FA and get secret
		secret, err := setup2FA(registerResp.Tokens.AccessToken, "TestPassword123!")
		require.NoError(t, err)
		require.NotEmpty(t, secret)

		// Generate TOTP code
		totpManager := auth.NewTOTPManager("dbackup-test")
		code, err := totpManager.GenerateCode(secret)
		require.NoError(t, err)

		// Enable 2FA
		payload := map[string]interface{}{
			"password": "TestPassword123!",
			"secret":   secret,
			"code":     code,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.EnableResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Two-factor authentication has been enabled successfully", response.Message)
		assert.NotEmpty(t, response.BackupCodes)
		assert.Len(t, response.BackupCodes, 8)

		// Verify 2FA is enabled in database
		db := database.GetDB()
		var user models.User
		err = db.Where("email = ?", "enable@example.com").First(&user).Error
		require.NoError(t, err)
		assert.True(t, user.TwoFactorEnabled)
		assert.NotNil(t, user.TwoFactorSecret)
		assert.Equal(t, secret, *user.TwoFactorSecret)
	})

	t.Run("Enable 2FA with invalid code", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("enable-invalid@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Setup 2FA and get secret
		secret, err := setup2FA(registerResp.Tokens.AccessToken, "TestPassword123!")
		require.NoError(t, err)
		require.NotEmpty(t, secret)

		// Enable 2FA with invalid code
		payload := map[string]interface{}{
			"password": "TestPassword123!",
			"secret":   secret,
			"code":     "123456", // Invalid code
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid verification code", response.Message)
	})

	t.Run("Enable 2FA with wrong password", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("enable-wrongpass@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Setup 2FA and get secret
		secret, err := setup2FA(registerResp.Tokens.AccessToken, "TestPassword123!")
		require.NoError(t, err)
		require.NotEmpty(t, secret)

		// Generate TOTP code
		totpManager := auth.NewTOTPManager("dbackup-test")
		code, err := totpManager.GenerateCode(secret)
		require.NoError(t, err)

		// Enable 2FA with wrong password
		payload := map[string]interface{}{
			"password": "WrongPassword123!",
			"secret":   secret,
			"code":     code,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid password", response.Message)
	})
}

func TestTwoFAVerify(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()
	server := setupTestTwoFAServer()

	// Helper to create a user with 2FA enabled and get tokens
	createUserWith2FA := func(email, password string) (*handlers.RegisterResponse, string, error) {
		// Register user
		payload := map[string]interface{}{
			"email":                 email,
			"password":              password,
			"password_confirmation": password,
			"first_name":            "Test",
			"last_name":             "User",
			"accept_terms":          true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			return nil, "", fmt.Errorf("failed to create user: %d", rec.Code)
		}
		var registerResponse handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &registerResponse)
		if err != nil {
			return nil, "", err
		}

		// Setup 2FA
		setupPayload := map[string]interface{}{
			"password": password,
		}
		setupBody, _ := json.Marshal(setupPayload)
		setupReq := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(setupBody))
		setupReq.Header.Set("Content-Type", "application/json")
		setupReq.Header.Set("Authorization", "Bearer "+registerResponse.Tokens.AccessToken)
		setupRec := httptest.NewRecorder()
		server.ServeHTTP(setupRec, setupReq)
		if setupRec.Code != http.StatusOK {
			return nil, "", fmt.Errorf("failed to setup 2FA: %d", setupRec.Code)
		}
		var setupResponse handlers.SetupResponse
		err = json.Unmarshal(setupRec.Body.Bytes(), &setupResponse)
		if err != nil {
			return nil, "", err
		}

		// Enable 2FA
		totpManager := auth.NewTOTPManager("dbackup-test")
		code, err := totpManager.GenerateCode(setupResponse.Secret)
		if err != nil {
			return nil, "", err
		}

		enablePayload := map[string]interface{}{
			"password": password,
			"secret":   setupResponse.Secret,
			"code":     code,
		}
		enableBody, _ := json.Marshal(enablePayload)
		enableReq := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(enableBody))
		enableReq.Header.Set("Content-Type", "application/json")
		enableReq.Header.Set("Authorization", "Bearer "+registerResponse.Tokens.AccessToken)
		enableRec := httptest.NewRecorder()
		server.ServeHTTP(enableRec, enableReq)
		if enableRec.Code != http.StatusOK {
			return nil, "", fmt.Errorf("failed to enable 2FA: %d", enableRec.Code)
		}

		return &registerResponse, setupResponse.Secret, nil
	}

	t.Run("Verify valid TOTP code", func(t *testing.T) {
		// Create user with 2FA enabled
		registerResp, secret, err := createUserWith2FA("verify@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)
		require.NotEmpty(t, secret)

		// Generate TOTP code
		totpManager := auth.NewTOTPManager("dbackup-test")
		code, err := totpManager.GenerateCode(secret)
		require.NoError(t, err)

		// Verify code
		payload := map[string]interface{}{
			"code": code,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.VerifyResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Code verification completed", response.Message)
		assert.True(t, response.Valid)
	})

	t.Run("Verify invalid TOTP code", func(t *testing.T) {
		// Create user with 2FA enabled
		registerResp, _, err := createUserWith2FA("verify-invalid@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Verify invalid code
		payload := map[string]interface{}{
			"code": "123456", // Invalid code
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.VerifyResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Code verification completed", response.Message)
		assert.False(t, response.Valid)
	})
}

func TestTwoFADisable(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()
	server := setupTestTwoFAServer()

	// Helper to create a user with 2FA enabled and get tokens
	createUserWith2FA := func(email, password string) (*handlers.RegisterResponse, string, error) {
		// Register user
		payload := map[string]interface{}{
			"email":                 email,
			"password":              password,
			"password_confirmation": password,
			"first_name":            "Test",
			"last_name":             "User",
			"accept_terms":          true,
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			return nil, "", fmt.Errorf("failed to create user: %d", rec.Code)
		}
		var registerResponse handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &registerResponse)
		if err != nil {
			return nil, "", err
		}

		// Setup and enable 2FA (same as previous test)
		setupPayload := map[string]interface{}{
			"password": password,
		}
		setupBody, _ := json.Marshal(setupPayload)
		setupReq := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/setup", bytes.NewReader(setupBody))
		setupReq.Header.Set("Content-Type", "application/json")
		setupReq.Header.Set("Authorization", "Bearer "+registerResponse.Tokens.AccessToken)
		setupRec := httptest.NewRecorder()
		server.ServeHTTP(setupRec, setupReq)
		if setupRec.Code != http.StatusOK {
			return nil, "", fmt.Errorf("failed to setup 2FA: %d", setupRec.Code)
		}
		var setupResponse handlers.SetupResponse
		err = json.Unmarshal(setupRec.Body.Bytes(), &setupResponse)
		if err != nil {
			return nil, "", err
		}

		// Enable 2FA
		totpManager := auth.NewTOTPManager("dbackup-test")
		code, err := totpManager.GenerateCode(setupResponse.Secret)
		if err != nil {
			return nil, "", err
		}

		enablePayload := map[string]interface{}{
			"password": password,
			"secret":   setupResponse.Secret,
			"code":     code,
		}
		enableBody, _ := json.Marshal(enablePayload)
		enableReq := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/enable", bytes.NewReader(enableBody))
		enableReq.Header.Set("Content-Type", "application/json")
		enableReq.Header.Set("Authorization", "Bearer "+registerResponse.Tokens.AccessToken)
		enableRec := httptest.NewRecorder()
		server.ServeHTTP(enableRec, enableReq)
		if enableRec.Code != http.StatusOK {
			return nil, "", fmt.Errorf("failed to enable 2FA: %d", enableRec.Code)
		}

		return &registerResponse, setupResponse.Secret, nil
	}

	t.Run("Disable 2FA with password only", func(t *testing.T) {
		// Create user with 2FA enabled
		registerResp, _, err := createUserWith2FA("disable@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Disable 2FA
		payload := map[string]interface{}{
			"password": "TestPassword123!",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/disable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.DisableResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Two-factor authentication has been disabled successfully", response.Message)

		// Verify 2FA is disabled in database
		db := database.GetDB()
		var user models.User
		err = db.Where("email = ?", "disable@example.com").First(&user).Error
		require.NoError(t, err)
		assert.False(t, user.TwoFactorEnabled)
		assert.Nil(t, user.TwoFactorSecret)
		assert.Nil(t, user.TwoFactorBackupCode)
	})

	t.Run("Disable 2FA with wrong password", func(t *testing.T) {
		// Create user with 2FA enabled
		registerResp, _, err := createUserWith2FA("disable-wrongpass@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Disable 2FA with wrong password
		payload := map[string]interface{}{
			"password": "WrongPassword123!",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/2fa/disable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid password", response.Message)
	})
}
