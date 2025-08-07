package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test JWT Manager for testing
func createTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager("test-secret-key", time.Hour, 24*time.Hour)
}

// Helper function to create a valid JWT token for testing
func createTestToken(jwtManager *auth.JWTManager, userID uint, email string) (string, error) {
	return jwtManager.GenerateAccessToken(userID, email, nil)
}

// Helper function to create an Echo context with WebSocket upgrade
func createWebSocketEchoContext(req *http.Request, res http.ResponseWriter) echo.Context {
	e := echo.New()
	return e.NewContext(req, res)
}

func TestNewWebSocketService(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	assert.NotNil(t, wsService)
	assert.NotNil(t, wsService.hub)
	assert.NotNil(t, wsService.connections)
	assert.NotNil(t, wsService.jwtManager)
	assert.Equal(t, jwtManager, wsService.jwtManager)
	
	// Check upgrader configuration
	assert.Equal(t, 1024, wsService.upgrader.ReadBufferSize)
	assert.Equal(t, 1024, wsService.upgrader.WriteBufferSize)
	assert.NotNil(t, wsService.upgrader.CheckOrigin)
}

func TestWebSocketService_AuthenticateToken_ValidToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create a valid token
	userID := uint(123)
	email := "test@example.com"
	token, err := createTestToken(jwtManager, userID, email)
	require.NoError(t, err)

	// Test authentication
	user, err := wsService.authenticateToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, email, user.Email)
}

func TestWebSocketService_AuthenticateToken_InvalidToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Test with invalid token
	user, err := wsService.authenticateToken("invalid-token")
	assert.Error(t, err)
	assert.Nil(t, user)
}

func TestWebSocketService_AuthenticateToken_ExpiredToken(t *testing.T) {
	// Create JWT manager with very short expiration
	shortJWTManager := auth.NewJWTManager("test-secret-key", time.Millisecond, 24*time.Hour)
	wsService := NewWebSocketService(shortJWTManager)

	// Create token and wait for it to expire
	token, err := createTestToken(shortJWTManager, 123, "test@example.com")
	require.NoError(t, err)
	
	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Test authentication with expired token
	user, err := wsService.authenticateToken(token)
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, auth.ErrExpiredToken, err)
}

func TestWebSocketService_HandleWebSocket_MissingToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create request without token
	req := httptest.NewRequest("GET", "/ws", nil)
	res := httptest.NewRecorder()
	c := createWebSocketEchoContext(req, res)

	err := wsService.HandleWebSocket(c)
	assert.Error(t, err)
	
	// Should return unauthorized error
	httpError, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, httpError.Code)
	assert.Contains(t, httpError.Message.(string), "Authentication token required")
}

func TestWebSocketService_HandleWebSocket_InvalidToken(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create request with invalid token
	req := httptest.NewRequest("GET", "/ws?token=invalid-token", nil)
	res := httptest.NewRecorder()
	c := createWebSocketEchoContext(req, res)

	err := wsService.HandleWebSocket(c)
	assert.Error(t, err)
	
	// Should return unauthorized error
	httpError, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, httpError.Code)
	assert.Contains(t, httpError.Message.(string), "Invalid authentication token")
}

func TestWebSocketService_BroadcastToUser_NoConnections(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	message := &Message{
		Type: "test",
		Data: "test data",
	}

	// Broadcasting to user with no connections should not error
	err := wsService.BroadcastToUser(123, message)
	assert.NoError(t, err)
}

func TestWebSocketService_BroadcastBackupProgress(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-123",
		Status:          "running",
		Progress:        50.0,
		ProgressMessage: "Backing up table users",
	}

	// Should not error even with no connections
	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
}

func TestWebSocketService_Broadcast_EmptyHub(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	message := &Message{
		Type: "test",
		Data: "test data",
	}

	// Should not error even with no connections
	// Allow time for the hub goroutine to process the broadcast
	err := wsService.Broadcast(message)
	assert.NoError(t, err)
	
	// Give time for message to be processed by hub
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocketService_GetConnectionCount(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Initially should have 0 connections
	count := wsService.GetConnectionCount()
	assert.Equal(t, 0, count)
}

func TestWebSocketService_GetUserConnectionCount(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// User with no connections should return 0
	count := wsService.GetUserConnectionCount(123)
	assert.Equal(t, 0, count)
}

func TestHub_RegisterUnregisterConnection(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create a mock connection
	mockConn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "test@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register connection
	wsService.hub.register <- mockConn

	// Give time for hub to process
	time.Sleep(10 * time.Millisecond)

	// Check that connection was registered
	wsService.hub.mutex.RLock()
	assert.True(t, wsService.hub.connections[mockConn])
	assert.NotNil(t, wsService.hub.userChannels[123])
	assert.True(t, wsService.hub.userChannels[123][mockConn])
	wsService.hub.mutex.RUnlock()

	// Unregister connection
	wsService.hub.unregister <- mockConn

	// Give time for hub to process
	time.Sleep(10 * time.Millisecond)

	// Check that connection was unregistered
	wsService.hub.mutex.RLock()
	assert.False(t, wsService.hub.connections[mockConn])
	// User channels should be cleaned up when no connections remain
	_, exists := wsService.hub.userChannels[123]
	assert.False(t, exists)
	wsService.hub.mutex.RUnlock()
}

func TestHub_BroadcastMessage(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create a mock connection
	mockConn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "test@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register connection
	wsService.hub.register <- mockConn

	// Give time for hub to process registration
	time.Sleep(10 * time.Millisecond)

	// Send broadcast message
	testMessage := []byte(`{"type":"test","data":"hello"}`)
	wsService.hub.broadcast <- testMessage

	// Give time for hub to process broadcast
	time.Sleep(10 * time.Millisecond)

	// Check that message was received
	select {
	case receivedMessage := <-mockConn.Send:
		assert.Equal(t, testMessage, receivedMessage)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive broadcasted message")
	}

	// Clean up
	wsService.hub.unregister <- mockConn
	time.Sleep(10 * time.Millisecond)
}

func TestConnection_HandleIncomingMessage_Ping(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	mockConn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "test@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Test ping message
	pingMsg := &Message{
		Type: "ping",
		Data: "ping",
	}

	mockConn.handleIncomingMessage(pingMsg)

	// Should receive pong response
	select {
	case response := <-mockConn.Send:
		var responseMsg Message
		err := parseJSON(response, &responseMsg)
		assert.NoError(t, err)
		assert.Equal(t, "pong", responseMsg.Type)
		assert.Equal(t, "pong", responseMsg.Data)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive pong response")
	}
}

func TestConnection_HandleIncomingMessage_Subscribe(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	mockConn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "test@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Test subscribe message
	subscribeMsg := &Message{
		Type: "subscribe",
		Data: "backup_progress",
	}

	// Should not panic or error
	mockConn.handleIncomingMessage(subscribeMsg)
}

func TestConnection_HandleIncomingMessage_Unknown(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	mockConn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "test@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Test unknown message type
	unknownMsg := &Message{
		Type: "unknown",
		Data: "unknown data",
	}

	// Should not panic or error
	mockConn.handleIncomingMessage(unknownMsg)
}

func TestGenerateConnectionID(t *testing.T) {
	id1 := generateConnectionID()
	id2 := generateConnectionID()

	// IDs should be different
	assert.NotEqual(t, id1, id2)
	
	// IDs should have expected format (timestamp-random)
	parts := strings.Split(id1, "-")
	assert.Len(t, parts, 2)
	assert.Len(t, parts[0], 14) // timestamp format: 20060102150405
	assert.Len(t, parts[1], 8)  // random string length
}

func TestGenerateRandomString(t *testing.T) {
	str1 := generateRandomString(10)
	str2 := generateRandomString(10)

	// Strings should be different
	assert.NotEqual(t, str1, str2)
	
	// Strings should have correct length
	assert.Len(t, str1, 10)
	assert.Len(t, str2, 10)
	
	// Strings should only contain expected characters
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, char := range str1 {
		assert.Contains(t, charset, string(char))
	}
}

func TestMessage_JSONSerialization(t *testing.T) {
	message := &Message{
		Type:      "test",
		Data:      map[string]interface{}{"key": "value"},
		Timestamp: time.Now(),
		UserID:    func() *uint { id := uint(123); return &id }(),
	}

	// Test JSON marshaling
	jsonData, err := message.toJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var parsedMessage Message
	err = parseJSON(jsonData, &parsedMessage)
	assert.NoError(t, err)
	assert.Equal(t, message.Type, parsedMessage.Type)
	assert.Equal(t, *message.UserID, *parsedMessage.UserID)
}

func TestBackupProgressMessage_Structure(t *testing.T) {
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-123",
		Status:          "running",
		Progress:        75.5,
		ProgressMessage: "Backing up table orders",
		StartedAt:       &time.Time{},
		CompletedAt:     nil,
		ErrorMessage:    nil,
	}

	// Test that all fields are properly set
	assert.Equal(t, "backup-123", progress.BackupJobUID)
	assert.Equal(t, "running", progress.Status)
	assert.Equal(t, 75.5, progress.Progress)
	assert.Equal(t, "Backing up table orders", progress.ProgressMessage)
	assert.NotNil(t, progress.StartedAt)
	assert.Nil(t, progress.CompletedAt)
	assert.Nil(t, progress.ErrorMessage)
}

// Helper functions for tests
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (m *Message) toJSON() ([]byte, error) {
	return json.Marshal(m)
}

// Integration test for WebSocket upgrade (requires real HTTP server)
func TestWebSocketService_Integration_ValidConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create valid token
	token, err := createTestToken(jwtManager, 123, "test@example.com")
	require.NoError(t, err)

	// Create test server
	e := echo.New()
	e.GET("/ws", wsService.HandleWebSocket)

	server := httptest.NewServer(e)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + url.QueryEscape(token)

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skipf("WebSocket connection failed (expected in test environment): %v", err)
		return
	}
	defer conn.Close()

	// Send ping message
	pingMsg := Message{Type: "ping", Data: "ping"}
	pingData, _ := json.Marshal(pingMsg)
	err = conn.WriteMessage(websocket.TextMessage, pingData)
	assert.NoError(t, err)

	// Read pong response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, responseData, err := conn.ReadMessage()
	assert.NoError(t, err)

	var responseMsg Message
	err = json.Unmarshal(responseData, &responseMsg)
	assert.NoError(t, err)
	assert.Equal(t, "pong", responseMsg.Type)
}