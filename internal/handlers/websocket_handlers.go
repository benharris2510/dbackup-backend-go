package handlers

import (
	"net/http"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/websocket"
	"github.com/labstack/echo/v4"
)

// WebSocketHandler handles WebSocket-related HTTP requests
type WebSocketHandler struct {
	wsService *websocket.WebSocketService
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(wsService *websocket.WebSocketService) *WebSocketHandler {
	return &WebSocketHandler{
		wsService: wsService,
	}
}

// HandleWebSocketConnection handles WebSocket upgrade requests
func (h *WebSocketHandler) HandleWebSocketConnection(c echo.Context) error {
	return h.wsService.HandleWebSocket(c)
}

// GetConnectionStats returns WebSocket connection statistics
func (h *WebSocketHandler) GetConnectionStats(c echo.Context) error {
	user := c.Get("user")
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}

	stats := map[string]interface{}{
		"total_connections": h.wsService.GetConnectionCount(),
		"timestamp":         "now",
	}

	// If user is authenticated, also return their specific connection count
	if userModel, ok := user.(*models.User); ok {
		stats["user_connections"] = h.wsService.GetUserConnectionCount(userModel.ID)
	}

	return c.JSON(http.StatusOK, stats)
}

// SendTestMessage sends a test message to the authenticated user (for testing)
func (h *WebSocketHandler) SendTestMessage(c echo.Context) error {
	user := c.Get("user")
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}

	userModel, ok := user.(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "Invalid user context")
	}

	// Create test message
	message := &websocket.Message{
		Type: "test_message",
		Data: map[string]interface{}{
			"message": "Hello from WebSocket service!",
			"user_id": userModel.ID,
		},
	}

	err := h.wsService.BroadcastToUser(userModel.ID, message)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to send message")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "sent",
		"message": "Test message sent to user WebSocket connections",
	})
}

// RegisterWebSocketRoutes registers WebSocket-related routes
func (h *WebSocketHandler) RegisterRoutes(g *echo.Group) {
	// WebSocket upgrade endpoint
	g.GET("/ws", h.HandleWebSocketConnection)
	
	// WebSocket management endpoints (require authentication)
	g.GET("/ws/stats", h.GetConnectionStats)
	g.POST("/ws/test", h.SendTestMessage)
}