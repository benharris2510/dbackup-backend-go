package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestHub_NewHub(t *testing.T) {
	hub := &Hub{
		connections:  make(map[*Connection]bool),
		broadcast:    make(chan []byte),
		register:     make(chan *Connection),
		unregister:   make(chan *Connection),
		userChannels: make(map[uint]map[*Connection]bool),
	}

	assert.NotNil(t, hub.connections)
	assert.NotNil(t, hub.broadcast)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.NotNil(t, hub.userChannels)
	assert.Equal(t, 0, len(hub.connections))
	assert.Equal(t, 0, len(hub.userChannels))
}

func TestHub_RegisterSingleConnection(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create a connection
	conn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register the connection
	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond) // Allow processing

	// Verify registration
	wsService.hub.mutex.RLock()
	defer wsService.hub.mutex.RUnlock()

	assert.True(t, wsService.hub.connections[conn])
	assert.Contains(t, wsService.hub.userChannels, uint(123))
	assert.True(t, wsService.hub.userChannels[123][conn])
	assert.Equal(t, 1, len(wsService.hub.connections))
	assert.Equal(t, 1, len(wsService.hub.userChannels))
	assert.Equal(t, 1, len(wsService.hub.userChannels[123]))
}

func TestHub_RegisterMultipleConnectionsSameUser(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)

	// Create multiple connections for the same user
	conn1 := &Connection{
		ID:     "test-conn-1",
		UserID: userID,
		User: &models.User{
			ID:    userID,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	conn2 := &Connection{
		ID:     "test-conn-2",
		UserID: userID,
		User: &models.User{
			ID:    userID,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register both connections
	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond) // Allow processing

	// Verify both connections are registered
	wsService.hub.mutex.RLock()
	defer wsService.hub.mutex.RUnlock()

	assert.True(t, wsService.hub.connections[conn1])
	assert.True(t, wsService.hub.connections[conn2])
	assert.Equal(t, 2, len(wsService.hub.connections))
	
	// Both connections should be in the same user channel
	assert.Contains(t, wsService.hub.userChannels, userID)
	assert.True(t, wsService.hub.userChannels[userID][conn1])
	assert.True(t, wsService.hub.userChannels[userID][conn2])
	assert.Equal(t, 2, len(wsService.hub.userChannels[userID]))
}

func TestHub_RegisterMultipleConnectionsDifferentUsers(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create connections for different users
	conn1 := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	conn2 := &Connection{
		ID:     "test-conn-2",
		UserID: 456,
		User: &models.User{
			ID:    456,
			Email: "user456@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register both connections
	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond) // Allow processing

	// Verify both connections are registered
	wsService.hub.mutex.RLock()
	defer wsService.hub.mutex.RUnlock()

	assert.True(t, wsService.hub.connections[conn1])
	assert.True(t, wsService.hub.connections[conn2])
	assert.Equal(t, 2, len(wsService.hub.connections))
	
	// Should have separate user channels
	assert.Contains(t, wsService.hub.userChannels, uint(123))
	assert.Contains(t, wsService.hub.userChannels, uint(456))
	assert.True(t, wsService.hub.userChannels[123][conn1])
	assert.True(t, wsService.hub.userChannels[456][conn2])
	assert.Equal(t, 1, len(wsService.hub.userChannels[123]))
	assert.Equal(t, 1, len(wsService.hub.userChannels[456]))
}

func TestHub_UnregisterConnection(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	conn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		User: &models.User{
			ID:    123,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register and then unregister
	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)

	// Verify unregistration
	wsService.hub.mutex.RLock()
	defer wsService.hub.mutex.RUnlock()

	assert.False(t, wsService.hub.connections[conn])
	assert.Equal(t, 0, len(wsService.hub.connections))
	
	// User channel should be cleaned up
	_, exists := wsService.hub.userChannels[123]
	assert.False(t, exists)
}

func TestHub_UnregisterOneOfMultipleConnections(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)

	conn1 := &Connection{
		ID:     "test-conn-1",
		UserID: userID,
		User: &models.User{
			ID:    userID,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	conn2 := &Connection{
		ID:     "test-conn-2",
		UserID: userID,
		User: &models.User{
			ID:    userID,
			Email: "user123@example.com",
		},
		Send: make(chan []byte, 256),
		Hub:  wsService.hub,
	}

	// Register both connections
	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond)

	// Unregister one connection
	wsService.hub.unregister <- conn1
	time.Sleep(10 * time.Millisecond)

	// Verify partial unregistration
	wsService.hub.mutex.RLock()
	defer wsService.hub.mutex.RUnlock()

	assert.False(t, wsService.hub.connections[conn1])
	assert.True(t, wsService.hub.connections[conn2])
	assert.Equal(t, 1, len(wsService.hub.connections))
	
	// User channel should still exist with remaining connection
	assert.Contains(t, wsService.hub.userChannels, userID)
	assert.False(t, wsService.hub.userChannels[userID][conn1])
	assert.True(t, wsService.hub.userChannels[userID][conn2])
	assert.Equal(t, 1, len(wsService.hub.userChannels[userID]))
}

func TestHub_BroadcastToAllConnections(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create connections for different users
	conn1 := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	conn2 := &Connection{
		ID:     "test-conn-2",
		UserID: 456,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	// Register connections
	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond)

	// Broadcast message
	testMessage := []byte(`{"type":"broadcast","data":"hello all"}`)
	wsService.hub.broadcast <- testMessage
	time.Sleep(10 * time.Millisecond)

	// Both connections should receive the message
	select {
	case msg := <-conn1.Send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Connection 1 did not receive broadcast message")
	}

	select {
	case msg := <-conn2.Send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Connection 2 did not receive broadcast message")
	}

	// Clean up
	wsService.hub.unregister <- conn1
	wsService.hub.unregister <- conn2
	time.Sleep(10 * time.Millisecond)
}

func TestHub_BroadcastToSpecificUser(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create connections for different users
	conn1 := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	conn2 := &Connection{
		ID:     "test-conn-2",
		UserID: 456,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	// Register connections
	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond)

	// Send message to specific user
	message := &Message{
		Type: "user_specific",
		Data: "hello user 123",
	}

	err := wsService.BroadcastToUser(123, message)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Only user 123 should receive the message
	select {
	case msg := <-conn1.Send:
		var receivedMsg Message
		err := json.Unmarshal(msg, &receivedMsg)
		assert.NoError(t, err)
		assert.Equal(t, "user_specific", receivedMsg.Type)
		assert.Equal(t, "hello user 123", receivedMsg.Data)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("User 123 did not receive targeted message")
	}

	// User 456 should not receive the message
	select {
	case <-conn2.Send:
		t.Fatal("User 456 should not have received the message")
	case <-time.After(50 * time.Millisecond):
		// This is expected - no message for user 456
	}

	// Clean up
	wsService.hub.unregister <- conn1
	wsService.hub.unregister <- conn2
	time.Sleep(10 * time.Millisecond)
}

func TestHub_ConcurrentOperations(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	numUsers := 10
	numConnectionsPerUser := 3
	var wg sync.WaitGroup
	var connectionsMutex sync.Mutex
	var connections []*Connection

	// Concurrently register multiple connections
	for userID := 1; userID <= numUsers; userID++ {
		for connNum := 1; connNum <= numConnectionsPerUser; connNum++ {
			wg.Add(1)
			go func(uid, cnum int) {
				defer wg.Done()
				
				conn := &Connection{
					ID:     generateConnectionID(),
					UserID: uint(uid),
					User: &models.User{
						ID:    uint(uid),
						Email: "user@example.com",
					},
					Send: make(chan []byte, 256),
					Hub:  wsService.hub,
				}

				connectionsMutex.Lock()
				connections = append(connections, conn)
				connectionsMutex.Unlock()
				
				wsService.hub.register <- conn
			}(userID, connNum)
		}
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Allow all registrations to process

	// Verify all connections are registered
	expectedConnections := numUsers * numConnectionsPerUser
	actualConnections := wsService.GetConnectionCount()
	assert.Equal(t, expectedConnections, actualConnections)

	// Verify user channels
	wsService.hub.mutex.RLock()
	assert.Equal(t, numUsers, len(wsService.hub.userChannels))
	for userID := 1; userID <= numUsers; userID++ {
		userConns, exists := wsService.hub.userChannels[uint(userID)]
		assert.True(t, exists)
		assert.Equal(t, numConnectionsPerUser, len(userConns))
	}
	wsService.hub.mutex.RUnlock()

	// Concurrently send messages to different users
	for userID := 1; userID <= numUsers; userID++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			
			message := &Message{
				Type: "concurrent_test",
				Data: map[string]interface{}{"user_id": uid, "message": "test"},
			}
			
			err := wsService.BroadcastToUser(uint(uid), message)
			assert.NoError(t, err)
		}(userID)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	// Clean up all connections concurrently
	connectionsMutex.Lock()
	for _, conn := range connections {
		wg.Add(1)
		go func(c *Connection) {
			defer wg.Done()
			wsService.hub.unregister <- c
		}(conn)
	}
	connectionsMutex.Unlock()

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Verify cleanup
	assert.Equal(t, 0, wsService.GetConnectionCount())
	
	wsService.hub.mutex.RLock()
	assert.Equal(t, 0, len(wsService.hub.userChannels))
	wsService.hub.mutex.RUnlock()
}

func TestHub_HandleDisconnectedConnection(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	conn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		Send:   make(chan []byte, 1), // Small buffer to test full channel
		Hub:    wsService.hub,
	}

	// Register connection
	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	// Fill the send channel
	conn.Send <- []byte("message1")

	// Try to broadcast when channel is full (simulates slow/disconnected client)
	testMessage := []byte(`{"type":"test","data":"should be dropped"}`)
	wsService.hub.broadcast <- testMessage
	time.Sleep(10 * time.Millisecond)

	// The connection should be automatically unregistered due to full channel
	wsService.hub.mutex.RLock()
	isConnected := wsService.hub.connections[conn]
	wsService.hub.mutex.RUnlock()

	// The connection should be removed when its channel is full
	assert.False(t, isConnected)
}

func TestHub_MessageDeliveryOrder(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	conn := &Connection{
		ID:     "test-conn-1",
		UserID: 123,
		Send:   make(chan []byte, 10),
		Hub:    wsService.hub,
	}

	// Register connection
	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	// Send multiple messages in sequence
	messages := []string{
		`{"type":"msg1","data":"first"}`,
		`{"type":"msg2","data":"second"}`,
		`{"type":"msg3","data":"third"}`,
	}

	for _, msg := range messages {
		wsService.hub.broadcast <- []byte(msg)
	}
	time.Sleep(20 * time.Millisecond)

	// Verify messages are received in order
	for i, expectedMsg := range messages {
		select {
		case receivedMsg := <-conn.Send:
			assert.Equal(t, expectedMsg, string(receivedMsg), "Message %d out of order", i+1)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Did not receive message %d", i+1)
		}
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}

func TestHub_UserChannelIsolation(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	// Create connections for two different users
	user1Conn1 := &Connection{
		ID:     "user1-conn1",
		UserID: 100,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	user1Conn2 := &Connection{
		ID:     "user1-conn2",
		UserID: 100,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	user2Conn := &Connection{
		ID:     "user2-conn1",
		UserID: 200,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	// Register all connections
	wsService.hub.register <- user1Conn1
	wsService.hub.register <- user1Conn2
	wsService.hub.register <- user2Conn
	time.Sleep(30 * time.Millisecond)

	// Send message to user 100 only
	message := &Message{
		Type: "isolation_test",
		Data: "message for user 100",
	}

	err := wsService.BroadcastToUser(100, message)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Both user 100 connections should receive the message
	messageCount := 0
	
	select {
	case <-user1Conn1.Send:
		messageCount++
	case <-time.After(50 * time.Millisecond):
		t.Error("User 100 connection 1 did not receive message")
	}

	select {
	case <-user1Conn2.Send:
		messageCount++
	case <-time.After(50 * time.Millisecond):
		t.Error("User 100 connection 2 did not receive message")
	}

	// User 200 connection should not receive the message
	select {
	case <-user2Conn.Send:
		t.Error("User 200 should not have received the message")
	case <-time.After(50 * time.Millisecond):
		// This is expected - user 200 should not receive the message
	}

	assert.Equal(t, 2, messageCount, "Expected exactly 2 connections to receive the message")

	// Clean up
	wsService.hub.unregister <- user1Conn1
	wsService.hub.unregister <- user1Conn2
	wsService.hub.unregister <- user2Conn
	time.Sleep(30 * time.Millisecond)
}

func TestHub_BackupProgressBroadcast(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	conn := &Connection{
		ID:     "backup-test-conn",
		UserID: 123,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	// Register connection
	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	// Send backup progress update
	progress := &BackupProgressMessage{
		BackupJobUID:    "job-456",
		Status:          "running",
		Progress:        75.0,
		ProgressMessage: "Backing up table users",
		StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}

	err := wsService.BroadcastBackupProgress(123, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Verify backup progress message was received
	select {
	case msgBytes := <-conn.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		assert.NoError(t, err)
		assert.Equal(t, "backup_progress", message.Type)
		
		// Verify progress data
		progressData := message.Data.(map[string]interface{})
		assert.Equal(t, "job-456", progressData["backup_job_uid"])
		assert.Equal(t, "running", progressData["status"])
		assert.Equal(t, 75.0, progressData["progress"])
		assert.Equal(t, "Backing up table users", progressData["progress_message"])
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive backup progress message")
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}