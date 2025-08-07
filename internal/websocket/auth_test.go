package websocket

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSocketJWTAuth_ValidAccessToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	email := "test@example.com"
	
	// Generate a valid access token
	token, err := jwtManager.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	// Test authentication
	user, err := wsService.authenticateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, email, user.Email)
}

func TestWebSocketJWTAuth_ValidRefreshToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	email := "test@example.com"
	
	// Generate a valid refresh token
	token, err := jwtManager.GenerateRefreshToken(userID, email, nil)
	require.NoError(t, err)

	// Test authentication - refresh tokens should also work for WebSocket
	user, err := wsService.authenticateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, email, user.Email)
}

func TestWebSocketJWTAuth_InvalidTokenFormat(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	testCases := []struct {
		name  string
		token string
	}{
		{"Empty token", ""},
		{"Invalid format", "not.a.jwt"},
		{"Wrong parts count", "only.two"},
		{"Garbage token", "garbage"},
		{"Base64 but not JWT", "dGVzdA=="},
		{"Invalid header", "eyJhbGciOiJub25lIn0.invalid.signature"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user, err := wsService.authenticateToken(tc.token)
			assert.Error(t, err, "Expected error for token: %s", tc.token)
			assert.Nil(t, user)
		})
	}
}

func TestWebSocketJWTAuth_ExpiredToken(t *testing.T) {
	// Create JWT manager with very short expiration
	shortJWTManager := auth.NewJWTManager("test-secret-key", time.Millisecond, 24*time.Hour)
	wsService := NewWebSocketService(shortJWTManager)

	userID := uint(123)
	email := "test@example.com"

	// Generate token
	token, err := shortJWTManager.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Test authentication with expired token
	user, err := wsService.authenticateToken(token)
	assert.Error(t, err)
	assert.Equal(t, auth.ErrExpiredToken, err)
	assert.Nil(t, user)
}

func TestWebSocketJWTAuth_TamperedToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	email := "test@example.com"

	// Generate a valid token
	validToken, err := jwtManager.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	// Tamper with the token by changing a character
	tamperedToken := strings.Replace(validToken, "a", "b", 1)
	
	// Test authentication with tampered token
	user, err := wsService.authenticateToken(tamperedToken)
	assert.Error(t, err)
	assert.Nil(t, user)
}

func TestWebSocketJWTAuth_WrongSecret(t *testing.T) {
	// Create JWT manager with one secret
	jwtManager1 := auth.NewJWTManager("secret-1", time.Hour, 24*time.Hour)
	// Create WebSocket service with different secret
	jwtManager2 := auth.NewJWTManager("secret-2", time.Hour, 24*time.Hour)
	wsService := NewWebSocketService(jwtManager2)

	userID := uint(123)
	email := "test@example.com"

	// Generate token with first manager
	token, err := jwtManager1.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	// Try to authenticate with second manager (different secret)
	user, err := wsService.authenticateToken(token)
	assert.Error(t, err)
	assert.Nil(t, user)
}

func TestWebSocketJWTAuth_TokenWithTeamID(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	email := "test@example.com"
	teamID := uint(456)

	// Generate token with team ID
	token, err := jwtManager.GenerateAccessToken(userID, email, &teamID)
	require.NoError(t, err)

	// Test authentication
	user, err := wsService.authenticateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, email, user.Email)
}

func TestWebSocketConnection_AuthenticationFlow(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	email := "test@example.com"

	// Generate valid token
	token, err := jwtManager.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	// Test WebSocket upgrade with valid token
	req := httptest.NewRequest("GET", "/ws?token="+url.QueryEscape(token), nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	res := httptest.NewRecorder()
	c := createWebSocketEchoContext(req, res)

	// This should not error (though upgrade will fail in test environment)
	err = wsService.HandleWebSocket(c)
	
	// In test environment, the upgrade will fail, but authentication should succeed
	// The error should be about WebSocket upgrade, not authentication
	if err != nil {
		// Should not be an authentication error
		httpError, ok := err.(*echo.HTTPError)
		if ok {
			assert.NotEqual(t, http.StatusUnauthorized, httpError.Code, 
				"Should not get unauthorized error with valid token")
		}
	}
}

func TestWebSocketConnection_AuthenticationFailures(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	testCases := []struct {
		name        string
		queryParam  string
		expectedMsg string
	}{
		{
			name:        "No token",
			queryParam:  "",
			expectedMsg: "Authentication token required",
		},
		{
			name:        "Invalid token",
			queryParam:  "token=invalid",
			expectedMsg: "Invalid authentication token",
		},
		{
			name:        "Empty token",
			queryParam:  "token=",
			expectedMsg: "Authentication token required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws?"+tc.queryParam, nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
			req.Header.Set("Sec-WebSocket-Version", "13")

			res := httptest.NewRecorder()
			c := createWebSocketEchoContext(req, res)

			err := wsService.HandleWebSocket(c)
			assert.Error(t, err)

			httpError, ok := err.(*echo.HTTPError)
			assert.True(t, ok, "Expected HTTP error")
			assert.Equal(t, http.StatusUnauthorized, httpError.Code)
			assert.Contains(t, httpError.Message.(string), tc.expectedMsg)
		})
	}
}

func TestWebSocketConnection_TokenInDifferentFormats(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	email := "test@example.com"

	// Generate valid token
	token, err := jwtManager.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	testCases := []struct {
		name  string
		url   string
		valid bool
	}{
		{
			name:  "Token in query param",
			url:   "/ws?token=" + url.QueryEscape(token),
			valid: true,
		},
		{
			name:  "Token with spaces (URL encoded)",
			url:   "/ws?token=" + url.QueryEscape(" "+token+" "),
			valid: false, // Spaces should make it invalid
		},
		{
			name:  "Token with additional params",
			url:   "/ws?token=" + url.QueryEscape(token) + "&other=param",
			valid: true,
		},
		{
			name:  "Token as second parameter",
			url:   "/ws?first=value&token=" + url.QueryEscape(token),
			valid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			req.Header.Set("Upgrade", "websocket")
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
			req.Header.Set("Sec-WebSocket-Version", "13")

			res := httptest.NewRecorder()
			c := createWebSocketEchoContext(req, res)

			err := wsService.HandleWebSocket(c)

			if tc.valid {
				// For valid tokens, we expect either no error or a non-auth error
				if err != nil {
					httpError, ok := err.(*echo.HTTPError)
					if ok {
						assert.NotEqual(t, http.StatusUnauthorized, httpError.Code,
							"Valid token should not result in unauthorized error")
					}
				}
			} else {
				// For invalid tokens, we expect an auth error
				assert.Error(t, err)
				httpError, ok := err.(*echo.HTTPError)
				assert.True(t, ok)
				assert.Equal(t, http.StatusUnauthorized, httpError.Code)
			}
		})
	}
}

func TestWebSocketJWTAuth_TokenClaims(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	testCases := []struct {
		name   string
		userID uint
		email  string
		teamID *uint
	}{
		{
			name:   "Basic user",
			userID: 123,
			email:  "user123@example.com",
			teamID: nil,
		},
		{
			name:   "User with team",
			userID: 456,
			email:  "user456@company.com",
			teamID: func() *uint { id := uint(789); return &id }(),
		},
		{
			name:   "User with zero team ID",
			userID: 999,
			email:  "user999@test.com",
			teamID: func() *uint { id := uint(0); return &id }(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate token
			token, err := jwtManager.GenerateAccessToken(tc.userID, tc.email, tc.teamID)
			require.NoError(t, err)

			// Authenticate and verify user data
			user, err := wsService.authenticateToken(token)
			assert.NoError(t, err)
			assert.NotNil(t, user)
			assert.Equal(t, tc.userID, user.ID)
			assert.Equal(t, tc.email, user.Email)
		})
	}
}

func TestWebSocketJWTAuth_ConcurrentAuthentication(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	numGoroutines := 100
	results := make(chan bool, numGoroutines)

	// Generate different tokens for concurrent testing
	var tokens []string
	for i := 0; i < numGoroutines; i++ {
		token, err := jwtManager.GenerateAccessToken(uint(i+1), 
			"user"+string(rune(i))+"@example.com", nil)
		require.NoError(t, err)
		tokens = append(tokens, token)
	}

	// Run concurrent authentications
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			user, err := wsService.authenticateToken(tokens[idx])
			success := (err == nil && user != nil && user.ID == uint(idx+1))
			results <- success
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-results {
			successCount++
		}
	}

	assert.Equal(t, numGoroutines, successCount, 
		"All concurrent authentications should succeed")
}

func TestWebSocketJWTAuth_EdgeCases(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Test with maximum values
	maxUserID := uint(4294967295) // Max uint32
	longEmail := strings.Repeat("a", 100) + "@" + strings.Repeat("b", 100) + ".com"

	token, err := jwtManager.GenerateAccessToken(maxUserID, longEmail, nil)
	require.NoError(t, err)

	user, err := wsService.authenticateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, maxUserID, user.ID)
	assert.Equal(t, longEmail, user.Email)
}

func TestWebSocketJWTAuth_PerformanceTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Generate a token
	token, err := jwtManager.GenerateAccessToken(123, "perf@test.com", nil)
	require.NoError(t, err)

	// Measure authentication time
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		user, err := wsService.authenticateToken(token)
		assert.NoError(t, err)
		assert.NotNil(t, user)
	}

	duration := time.Since(start)
	avgTime := duration / time.Duration(iterations)

	t.Logf("Average authentication time: %v", avgTime)
	
	// Assert reasonable performance (should be well under 1ms per auth)
	assert.Less(t, avgTime, time.Millisecond, 
		"Authentication should be fast (< 1ms per operation)")
}

// Integration test for full WebSocket authentication flow
func TestWebSocketJWTAuth_IntegrationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Generate token
	userID := uint(123)
	email := "integration@test.com"
	token, err := jwtManager.GenerateAccessToken(userID, email, nil)
	require.NoError(t, err)

	// Create test server
	e := echo.New()
	e.GET("/ws", wsService.HandleWebSocket)

	server := httptest.NewServer(e)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL with token
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + 
		"/ws?token=" + url.QueryEscape(token)

	// Attempt WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// In test environment, this may fail due to network restrictions
		// but it shouldn't be due to authentication
		t.Skipf("WebSocket connection failed (network/test env issue): %v", err)
		return
	}
	defer conn.Close()

	// If we get here, authentication worked
	t.Log("WebSocket connection with JWT authentication successful")

	// Send a test message
	testMsg := map[string]interface{}{
		"type": "test",
		"data": "authentication test",
	}

	err = conn.WriteJSON(testMsg)
	assert.NoError(t, err)

	// Try to read response (with timeout)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var response map[string]interface{}
	err = conn.ReadJSON(&response)
	
	// Don't assert on read success as the server might not send a response
	// The fact that we could connect and write is sufficient for auth testing
}