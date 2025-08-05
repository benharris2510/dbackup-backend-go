package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/config"
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

// setupTestAuthServer creates a test server with auth routes
func setupTestAuthServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Debug = true

	// Set up validator
	e.Validator = validation.NewValidator()

	// Setup middleware
	e.Use(echoMiddleware.LoggerWithConfig(echoMiddleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339}","method":"${method}","uri":"${uri}","status":${status},"error":"${error}","latency_human":"${latency_human}","bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02T15:04:05.000Z07:00",
		Output: os.Stdout,
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

	// Setup auth routes
	routes.SetupAuthRoutes(e, jwtManager, passwordHasher, totpManager)

	return e
}

// setupTestDatabase initializes a test database connection using SQLite
func setupTestDatabase(t *testing.T) func() {
	// Set test database environment variables for SQLite
	os.Setenv("DATABASE_URL", "sqlite://test.db?_foreign_keys=on")
	os.Setenv("ENVIRONMENT", "test")
	
	// Load test configuration
	cfg, err := config.Load()
	require.NoError(t, err)

	// Initialize database
	err = database.Initialize(cfg)
	require.NoError(t, err)

	// Auto-migrate models for testing
	db := database.GetDB()
	err = db.AutoMigrate(&models.User{}, &models.Team{}, &models.TeamMember{})
	require.NoError(t, err)

	// Return cleanup function
	return func() {
		// Clean up test data
		db.Exec("DELETE FROM users")
		db.Exec("DELETE FROM teams")
		db.Exec("DELETE FROM team_members")
		database.Close()
		os.Remove("test.db") // Remove SQLite file
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("ENVIRONMENT")
	}
}

func TestAuthRegistration(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()

	server := setupTestAuthServer()

	t.Run("Successful registration", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "test@example.com",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "John",
			"last_name":             "Doe",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Contains(t, response.Message, "Account created successfully")
		assert.NotNil(t, response.User)
		assert.NotNil(t, response.Tokens)

		// Verify user data
		assert.Equal(t, "test@example.com", response.User.Email)
		assert.Equal(t, "John", response.User.FirstName)
		assert.Equal(t, "Doe", response.User.LastName)
		assert.False(t, response.User.IsVerified) // Should be false initially
		assert.False(t, response.User.Has2FA)    // Should be false initially
		assert.NotEmpty(t, response.User.UID)

		// Verify tokens
		assert.NotEmpty(t, response.Tokens.AccessToken)
		assert.NotEmpty(t, response.Tokens.RefreshToken)
		assert.Equal(t, "Bearer", response.Tokens.TokenType)
		assert.Greater(t, response.Tokens.ExpiresIn, int64(0))

		// Verify user exists in database
		db := database.GetDB()
		var user models.User
		err = db.Where("email = ?", "test@example.com").First(&user).Error
		require.NoError(t, err)
		assert.Equal(t, "John", user.FirstName)
		assert.Equal(t, "Doe", user.LastName)
		assert.True(t, user.IsActive)
		assert.False(t, user.IsEmailVerified)
	})

	t.Run("Duplicate email registration", func(t *testing.T) {
		// First registration
		payload := map[string]interface{}{
			"email":                 "duplicate@example.com",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "Jane",
			"last_name":             "Smith",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Second registration with same email
		body, _ = json.Marshal(payload)
		req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "User already exists", response.Message)
		assert.Contains(t, response.Errors["email"], "already exists")
	})

	t.Run("Invalid email format", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "invalid-email",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "John",
			"last_name":             "Doe",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Password confirmation mismatch", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "mismatch@example.com",
			"password":              "TestPassword123!",
			"password_confirmation": "DifferentPassword123!",
			"first_name":            "John",
			"last_name":             "Doe",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Validation failed", response.Message)
		assert.Contains(t, response.Errors["password_confirmation"], "does not match")
	})

	t.Run("Weak password", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "weak@example.com",
			"password":              "weakpass", // 8 chars but no numbers/special chars
			"password_confirmation": "weakpass",
			"first_name":            "John",
			"last_name":             "Doe",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Password does not meet requirements")
		assert.NotEmpty(t, response.Errors["password"])
	})

	t.Run("Terms not accepted", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "terms@example.com",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "John",
			"last_name":             "Doe",
			"accept_terms":          false,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Terms and conditions must be accepted")
		assert.Contains(t, response.Errors["accept_terms"], "must accept")
	})

	t.Run("Missing required fields", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload map[string]interface{}
		}{
			{
				name: "Missing email",
				payload: map[string]interface{}{
					"password":              "TestPassword123!",
					"password_confirmation": "TestPassword123!",
					"first_name":            "John",
					"last_name":             "Doe",
					"accept_terms":          true,
				},
			},
			{
				name: "Missing password",
				payload: map[string]interface{}{
					"email":                 "missing@example.com",
					"password_confirmation": "TestPassword123!",
					"first_name":            "John",
					"last_name":             "Doe",
					"accept_terms":          true,
				},
			},
			{
				name: "Missing first name",
				payload: map[string]interface{}{
					"email":                 "missing@example.com",
					"password":              "TestPassword123!",
					"password_confirmation": "TestPassword123!",
					"last_name":             "Doe",
					"accept_terms":          true,
				},
			},
			{
				name: "Missing last name",
				payload: map[string]interface{}{
					"email":                 "missing@example.com",
					"password":              "TestPassword123!",
					"password_confirmation": "TestPassword123!",
					"first_name":            "John",
					"accept_terms":          true,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, _ := json.Marshal(tc.payload)
				req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				server.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	})

	t.Run("Empty string fields", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "",
			"last_name":             "",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Empty request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Field length validation", func(t *testing.T) {
		testCases := []struct {
			name       string
			field      string
			value      string
			shouldFail bool
		}{
			{
				name:       "First name too long",
				field:      "first_name",
				value:      strings.Repeat("a", 51), // max is 50
				shouldFail: true,
			},
			{
				name:       "Last name too long",
				field:      "last_name",
				value:      strings.Repeat("b", 51), // max is 50
				shouldFail: true,
			},
			{
				name:       "Password too long",
				field:      "password",
				value:      strings.Repeat("A1!", 43), // max is 128, this is 129
				shouldFail: true,
			},
			{
				name:       "Valid length fields",
				field:      "first_name",
				value:      strings.Repeat("a", 50), // exactly max
				shouldFail: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				payload := map[string]interface{}{
					"email":                 fmt.Sprintf("length-test-%d@example.com", time.Now().UnixNano()),
					"password":              "TestPassword123!",
					"password_confirmation": "TestPassword123!",
					"first_name":            "John",
					"last_name":             "Doe",
					"accept_terms":          true,
				}

				// Override the specific field being tested
				payload[tc.field] = tc.value
				if tc.field == "password" {
					payload["password_confirmation"] = tc.value
				}

				body, _ := json.Marshal(payload)
				req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				server.ServeHTTP(rec, req)

				if tc.shouldFail {
					assert.Equal(t, http.StatusBadRequest, rec.Code)
				} else {
					assert.Equal(t, http.StatusCreated, rec.Code)
				}
			})
		}
	})

	t.Run("Case insensitive email handling", func(t *testing.T) {
		// Register with uppercase email
		payload := map[string]interface{}{
			"email":                 "CASE@EXAMPLE.COM",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "Case",
			"last_name":             "Test",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		var response handlers.RegisterResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Email should be stored as lowercase
		assert.Equal(t, "case@example.com", response.User.Email)

		// Try to register again with lowercase - should fail due to duplicate
		payload["email"] = "case@example.com"
		body, _ = json.Marshal(payload)
		req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()

		server.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestAuthRegistrationWithDatabaseErrors(t *testing.T) {
	// Test database connection errors by not setting up database
	server := setupTestAuthServer()

	t.Run("Database connection error", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":                 "dbtest@example.com",
			"password":              "TestPassword123!",
			"password_confirmation": "TestPassword123!",
			"first_name":            "DB",
			"last_name":             "Test",
			"accept_terms":          true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		// Should return 500 due to database connection error
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestAuthLogin(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()

	server := setupTestAuthServer()

	// Create a test user first
	createTestUser := func(email, password string, enable2FA bool, secret *string) {
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
		require.Equal(t, http.StatusCreated, rec.Code)

		// If 2FA should be enabled, update the user record
		if enable2FA {
			db := database.GetDB()
			var user models.User
			err := db.Where("email = ?", email).First(&user).Error
			require.NoError(t, err)

			user.TwoFactorEnabled = true
			if secret != nil {
				user.TwoFactorSecret = secret
			} else {
				defaultSecret := "JBSWY3DPEHPK3PXP"
				user.TwoFactorSecret = &defaultSecret
			}
			err = db.Save(&user).Error
			require.NoError(t, err)
		}
	}

	t.Run("Successful login without 2FA", func(t *testing.T) {
		email := "login@example.com"
		password := "TestPassword123!"
		createTestUser(email, password, false, nil)

		payload := map[string]interface{}{
			"email":    email,
			"password": password,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.LoginResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Login successful", response.Message)
		assert.NotNil(t, response.User)
		assert.NotNil(t, response.Tokens)
		assert.False(t, response.Requires2FA)

		// Verify user data
		assert.Equal(t, email, response.User.Email)
		assert.Equal(t, "Test", response.User.FirstName)
		assert.Equal(t, "User", response.User.LastName)
		assert.NotEmpty(t, response.User.UID)

		// Verify tokens
		assert.NotEmpty(t, response.Tokens.AccessToken)
		assert.NotEmpty(t, response.Tokens.RefreshToken)
		assert.Equal(t, "Bearer", response.Tokens.TokenType)
		assert.Greater(t, response.Tokens.ExpiresIn, int64(0))

		// Verify login tracking was updated in database
		db := database.GetDB()
		var user models.User
		err = db.Where("email = ?", email).First(&user).Error
		require.NoError(t, err)
		assert.Equal(t, 0, user.LoginAttempts)
		assert.NotNil(t, user.LastLoginAt)
		assert.NotNil(t, user.LastLoginIP)
	})

	t.Run("Login with 2FA required", func(t *testing.T) {
		email := "login2fa@example.com"
		password := "TestPassword123!"
		createTestUser(email, password, true, nil)

		payload := map[string]interface{}{
			"email":    email,
			"password": password,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.LoginResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Two-factor authentication required", response.Message)
		assert.Nil(t, response.User)
		assert.Nil(t, response.Tokens)
		assert.True(t, response.Requires2FA)
	})

	t.Run("Invalid credentials", func(t *testing.T) {
		email := "wrong@example.com"
		createTestUser(email, "CorrectPassword123!", false, nil)

		payload := map[string]interface{}{
			"email":    email,
			"password": "WrongPassword123!",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid credentials", response.Message)

		// Verify login attempts were incremented
		db := database.GetDB()
		var user models.User
		err = db.Where("email = ?", email).First(&user).Error
		require.NoError(t, err)
		assert.Equal(t, 1, user.LoginAttempts)
	})

	t.Run("User not found", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    "nonexistent@example.com",
			"password": "TestPassword123!",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid credentials", response.Message)
	})

	t.Run("Inactive account", func(t *testing.T) {
		email := "inactive@example.com"
		password := "TestPassword123!"
		createTestUser(email, password, false, nil)

		// Deactivate the user
		db := database.GetDB()
		var user models.User
		err := db.Where("email = ?", email).First(&user).Error
		require.NoError(t, err)
		user.IsActive = false
		err = db.Save(&user).Error
		require.NoError(t, err)

		payload := map[string]interface{}{
			"email":    email,
			"password": password,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Account is disabled")
	})

	t.Run("Remember me functionality", func(t *testing.T) {
		email := "remember@example.com"
		password := "TestPassword123!"
		createTestUser(email, password, false, nil)

		payload := map[string]interface{}{
			"email":       email,
			"password":    password,
			"remember_me": true,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.LoginResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.NotNil(t, response.Tokens)

		// With remember me, tokens should have extended expiry (24 hours = 86400 seconds)
		assert.Equal(t, int64(86400), response.Tokens.ExpiresIn)
	})

	t.Run("Multiple failed login attempts account lockout", func(t *testing.T) {
		email := "lockout@example.com"
		password := "TestPassword123!"
		createTestUser(email, password, false, nil)

		// Perform 5 failed login attempts
		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{
				"email":    email,
				"password": "WrongPassword!",
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		}

		// Verify account is locked
		db := database.GetDB()
		var user models.User
		err := db.Where("email = ?", email).First(&user).Error
		require.NoError(t, err)
		assert.Equal(t, 5, user.LoginAttempts)
		assert.False(t, user.IsActive) // Should be deactivated after 5 attempts
		assert.NotNil(t, user.LockedUntil)
	})

	t.Run("Missing required fields", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload map[string]interface{}
		}{
			{
				name: "Missing email",
				payload: map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
			{
				name: "Missing password",
				payload: map[string]interface{}{
					"email": "test@example.com",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, _ := json.Marshal(tc.payload)
				req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				server.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Case insensitive email", func(t *testing.T) {
		email := "case-login@example.com"
		password := "TestPassword123!"
		createTestUser(email, password, false, nil)

		// Try to login with uppercase email
		payload := map[string]interface{}{
			"email":    "CASE-LOGIN@EXAMPLE.COM",
			"password": password,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.LoginResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, email, response.User.Email) // Should be stored as lowercase
	})
}

func TestAuthRefresh(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()

	server := setupTestAuthServer()

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

	t.Run("Successful token refresh", func(t *testing.T) {
		// Create user and get initial tokens
		registerResp, err := createUserAndGetTokens("refresh@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Use refresh token to get new tokens
		payload := map[string]interface{}{
			"refresh_token": registerResp.Tokens.RefreshToken,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.RefreshResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Token refreshed successfully", response.Message)
		assert.NotNil(t, response.Tokens)

		// Verify new tokens are different from original
		assert.NotEqual(t, registerResp.Tokens.AccessToken, response.Tokens.AccessToken)
		assert.NotEqual(t, registerResp.Tokens.RefreshToken, response.Tokens.RefreshToken)

		// Verify token format
		assert.NotEmpty(t, response.Tokens.AccessToken)
		assert.NotEmpty(t, response.Tokens.RefreshToken)
		assert.Equal(t, "Bearer", response.Tokens.TokenType)
		assert.Greater(t, response.Tokens.ExpiresIn, int64(0))
	})

	t.Run("Invalid refresh token", func(t *testing.T) {
		payload := map[string]interface{}{
			"refresh_token": "invalid.jwt.token",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Invalid or expired refresh token")
	})

	t.Run("Expired refresh token", func(t *testing.T) {
		// Create a JWT manager with very short refresh token duration
		shortJWTManager := auth.NewJWTManager(
			"test-secret-key-for-testing-purposes-only",
			15*time.Minute,
			1*time.Nanosecond, // Very short refresh token duration
		)

		// Generate an expired refresh token
		expiredRefreshToken, _, err := shortJWTManager.GenerateTokenPair(1, "test@example.com", nil)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		payload := map[string]interface{}{
			"refresh_token": expiredRefreshToken, // This will be expired
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Invalid or expired refresh token")
	})

	t.Run("Missing refresh token", func(t *testing.T) {
		payload := map[string]interface{}{}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Empty refresh token", func(t *testing.T) {
		payload := map[string]interface{}{
			"refresh_token": "",
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")  
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("User no longer exists", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("deleted@example.com", "TestPassword123!")
		require.NoError(t, err)

		// Delete the user from database
		db := database.GetDB()
		err = db.Where("email = ?", "deleted@example.com").Delete(&models.User{}).Error
		require.NoError(t, err)

		// Try to refresh token
		payload := map[string]interface{}{
			"refresh_token": registerResp.Tokens.RefreshToken,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "User not found", response.Message)
	})

	t.Run("Inactive user", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("inactive-refresh@example.com", "TestPassword123!")
		require.NoError(t, err)

		// Deactivate the user
		db := database.GetDB()
		var user models.User
		err = db.Where("email = ?", "inactive-refresh@example.com").First(&user).Error
		require.NoError(t, err)
		user.IsActive = false
		err = db.Save(&user).Error
		require.NoError(t, err)

		// Try to refresh token
		payload := map[string]interface{}{
			"refresh_token": registerResp.Tokens.RefreshToken,
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Equal(t, "Account is disabled", response.Message)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Using access token as refresh token", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("wrong-token@example.com", "TestPassword123!")
		require.NoError(t, err)

		// Try to use access token as refresh token
		payload := map[string]interface{}{
			"refresh_token": registerResp.Tokens.AccessToken, // Wrong token type
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var response handlers.ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Invalid or expired refresh token")
	})
}

func TestAuthSession(t *testing.T) {
	cleanup := setupTestDatabase(t)
	defer cleanup()
	server := setupTestAuthServer()

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

	t.Run("Successful session retrieval", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("session@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Get session with valid token
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response handlers.SessionResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, "Session retrieved successfully", response.Message)
		assert.NotNil(t, response.User)
		assert.Equal(t, registerResp.User.Email, response.User.Email)
		assert.Equal(t, registerResp.User.FirstName, response.User.FirstName)
		assert.Equal(t, registerResp.User.LastName, response.User.LastName)
		assert.Equal(t, registerResp.User.IsVerified, response.User.IsVerified)
		assert.Equal(t, registerResp.User.Has2FA, response.User.Has2FA)
	})

	t.Run("Missing Authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Invalid token format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		req.Header.Set("Authorization", "InvalidToken")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Invalid Bearer token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Expired token", func(t *testing.T) {
		// Create a JWT manager with very short expiration
		shortJWTManager := auth.NewJWTManager(
			"test-secret-key-for-testing-purposes-only",
			1*time.Nanosecond, // Very short access token duration
			15*time.Minute,
		)

		// Generate an expired access token
		expiredAccessToken, _, err := shortJWTManager.GenerateTokenPair(1, "test@example.com", nil)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		req.Header.Set("Authorization", "Bearer "+expiredAccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("User no longer exists", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("deleted-session@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Delete the user (soft delete)
		db := database.GetDB()
		db.Where("email = ?", "deleted-session@example.com").Delete(&models.User{})

		// Try to get session with valid token but deleted user
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.AccessToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("Using refresh token as access token", func(t *testing.T) {
		// Create user and get tokens
		registerResp, err := createUserAndGetTokens("wrong-token-session@example.com", "TestPassword123!")
		require.NoError(t, err)
		require.NotNil(t, registerResp.Tokens)

		// Try to use refresh token as access token
		req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
		req.Header.Set("Authorization", "Bearer "+registerResp.Tokens.RefreshToken)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}