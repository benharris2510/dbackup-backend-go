package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dbackup/backend-go/internal/auth"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// WebSocketService manages WebSocket connections and messaging
type WebSocketService struct {
	upgrader    websocket.Upgrader
	connections map[string]*Connection
	hub         *Hub
	mutex       sync.RWMutex
	jwtManager  *auth.JWTManager
}

// Connection represents a WebSocket connection
type Connection struct {
	ID       string
	UserID   uint
	User     *models.User
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	LastPing time.Time
}

// Hub manages all WebSocket connections
type Hub struct {
	// Registered connections
	connections map[*Connection]bool

	// Inbound messages from connections
	broadcast chan []byte

	// Register requests from connections
	register chan *Connection

	// Unregister requests from connections
	unregister chan *Connection

	// User-specific channels for targeted messaging
	userChannels map[uint]map[*Connection]bool

	// Mutex for thread-safe operations
	mutex sync.RWMutex
}

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	UserID    *uint       `json:"user_id,omitempty"`
}

// BackupProgressMessage represents backup progress updates
type BackupProgressMessage struct {
	BackupJobUID    string  `json:"backup_job_uid"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	ProgressMessage string  `json:"progress_message"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
}

// NewWebSocketService creates a new WebSocket service
func NewWebSocketService(jwtManager *auth.JWTManager) *WebSocketService {
	hub := &Hub{
		connections:  make(map[*Connection]bool),
		broadcast:    make(chan []byte, 256), // Add buffer to prevent blocking
		register:     make(chan *Connection),
		unregister:   make(chan *Connection),
		userChannels: make(map[uint]map[*Connection]bool),
	}

	service := &WebSocketService{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		connections: make(map[string]*Connection),
		hub:         hub,
		jwtManager:  jwtManager,
	}

	// Start the hub
	go hub.run()

	return service
}

// HandleWebSocket handles WebSocket connections
func (ws *WebSocketService) HandleWebSocket(c echo.Context) error {
	// Authenticate the WebSocket connection using JWT token
	token := c.QueryParam("token")
	if token == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication token required")
	}

	// Validate the JWT token
	user, err := ws.authenticateToken(token)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid authentication token")
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := ws.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return err
	}

	// Create connection object
	connection := &Connection{
		ID:       generateConnectionID(),
		UserID:   user.ID,
		User:     user,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      ws.hub,
		LastPing: time.Now(),
	}

	// Register connection
	ws.mutex.Lock()
	ws.connections[connection.ID] = connection
	ws.mutex.Unlock()

	// Register with hub
	ws.hub.register <- connection

	// Start goroutines for reading and writing
	go connection.writePump()
	go connection.readPump()

	return nil
}

// authenticateToken validates JWT token and returns user
func (ws *WebSocketService) authenticateToken(tokenString string) (*models.User, error) {
	// Parse and validate JWT token using the JWT manager
	claims, err := ws.jwtManager.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Create user object from claims
	user := &models.User{
		ID:    claims.UserID,
		Email: claims.Email,
	}

	return user, nil
}

// BroadcastToUser sends a message to all connections for a specific user
func (ws *WebSocketService) BroadcastToUser(userID uint, message *Message) error {
	ws.hub.mutex.RLock()
	userConnections, exists := ws.hub.userChannels[userID]
	ws.hub.mutex.RUnlock()

	if !exists {
		return nil // No connections for this user
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	for conn := range userConnections {
		select {
		case conn.Send <- messageBytes:
		default:
			close(conn.Send)
			delete(userConnections, conn)
		}
	}

	return nil
}

// BroadcastBackupProgress sends backup progress updates to user
func (ws *WebSocketService) BroadcastBackupProgress(userID uint, progress *BackupProgressMessage) error {
	message := &Message{
		Type:      "backup_progress",
		Data:      progress,
		Timestamp: time.Now(),
		UserID:    &userID,
	}

	return ws.BroadcastToUser(userID, message)
}

// Broadcast sends a message to all connected clients
func (ws *WebSocketService) Broadcast(message *Message) error {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case ws.hub.broadcast <- messageBytes:
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "Broadcast channel full")
	}

	return nil
}

// GetConnectionCount returns the number of active connections
func (ws *WebSocketService) GetConnectionCount() int {
	ws.hub.mutex.RLock()
	defer ws.hub.mutex.RUnlock()
	return len(ws.hub.connections)
}

// GetUserConnectionCount returns the number of connections for a specific user
func (ws *WebSocketService) GetUserConnectionCount(userID uint) int {
	ws.hub.mutex.RLock()
	defer ws.hub.mutex.RUnlock()

	if userConnections, exists := ws.hub.userChannels[userID]; exists {
		return len(userConnections)
	}
	return 0
}

// Hub methods

// run starts the hub's main loop
func (h *Hub) run() {
	for {
		select {
		case connection := <-h.register:
			h.mutex.Lock()
			h.connections[connection] = true
			
			// Add to user-specific channel
			if h.userChannels[connection.UserID] == nil {
				h.userChannels[connection.UserID] = make(map[*Connection]bool)
			}
			h.userChannels[connection.UserID][connection] = true
			h.mutex.Unlock()

			log.Printf("WebSocket connection registered for user %d (total: %d)", 
				connection.UserID, len(h.connections))

		case connection := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.connections[connection]; ok {
				delete(h.connections, connection)
				close(connection.Send)

				// Remove from user-specific channel
				if userConns, exists := h.userChannels[connection.UserID]; exists {
					delete(userConns, connection)
					if len(userConns) == 0 {
						delete(h.userChannels, connection.UserID)
					}
				}
			}
			h.mutex.Unlock()

			log.Printf("WebSocket connection unregistered for user %d (total: %d)", 
				connection.UserID, len(h.connections))

		case message := <-h.broadcast:
			h.mutex.RLock()
			for connection := range h.connections {
				select {
				case connection.Send <- message:
				default:
					close(connection.Send)
					delete(h.connections, connection)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// Connection methods

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// readPump pumps messages from the WebSocket connection to the hub
func (c *Connection) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.LastPing = time.Now()
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages from client
		var msg Message
		if err := json.Unmarshal(message, &msg); err == nil {
			c.handleIncomingMessage(&msg)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleIncomingMessage processes messages received from the client
func (c *Connection) handleIncomingMessage(msg *Message) {
	switch msg.Type {
	case "ping":
		// Respond with pong
		response := &Message{
			Type:      "pong",
			Data:      "pong",
			Timestamp: time.Now(),
		}
		if responseBytes, err := json.Marshal(response); err == nil {
			select {
			case c.Send <- responseBytes:
			default:
				// Channel full, skip
			}
		}
	case "subscribe":
		// Handle subscription requests (for future use)
		log.Printf("User %d subscribed to updates", c.UserID)
	default:
		log.Printf("Unknown message type from user %d: %s", c.UserID, msg.Type)
	}
}

// generateConnectionID generates a unique connection ID
func generateConnectionID() string {
	return time.Now().Format("20060102150405") + "-" + generateRandomString(8)
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Shutdown gracefully closes all WebSocket connections
func (h *Hub) Shutdown(ctx context.Context) error {
	log.Println("Shutting down WebSocket hub...")

	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Send close message to all connections
	closeMessage := &Message{
		Type:      "server_shutdown",
		Data:      "Server is shutting down",
		Timestamp: time.Now(),
	}

	closeBytes, err := json.Marshal(closeMessage)
	if err != nil {
		log.Printf("Error marshaling close message: %v", err)
	}

	// Notify all connections about shutdown
	for conn := range h.connections {
		select {
		case conn.Send <- closeBytes:
		default:
			// Channel might be full, skip
		}
		
		// Close the connection gracefully
		conn.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server shutdown"))
		conn.Conn.Close()
		close(conn.Send)
	}

	// Clear all connections
	h.connections = make(map[*Connection]bool)
	h.userChannels = make(map[uint]map[*Connection]bool)

	log.Println("WebSocket hub shutdown completed")
	return nil
}