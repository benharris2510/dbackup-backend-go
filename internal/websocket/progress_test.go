package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupProgressMessage_Serialization(t *testing.T) {
	startTime := time.Now()
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-12345",
		Status:          "running",
		Progress:        75.5,
		ProgressMessage: "Backing up table users",
		StartedAt:       &startTime,
		CompletedAt:     nil,
		ErrorMessage:    nil,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(progress)
	require.NoError(t, err)

	// Test JSON deserialization
	var parsed BackupProgressMessage
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Equal(t, progress.BackupJobUID, parsed.BackupJobUID)
	assert.Equal(t, progress.Status, parsed.Status)
	assert.Equal(t, progress.Progress, parsed.Progress)
	assert.Equal(t, progress.ProgressMessage, parsed.ProgressMessage)
	assert.NotNil(t, parsed.StartedAt)
	assert.Nil(t, parsed.CompletedAt)
	assert.Nil(t, parsed.ErrorMessage)
}

func TestBackupProgressMessage_AllFields(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Minute)
	errorMsg := "Connection timeout"

	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-67890",
		Status:          "failed",
		Progress:        45.0,
		ProgressMessage: "Failed at table orders",
		StartedAt:       &startTime,
		CompletedAt:     &endTime,
		ErrorMessage:    &errorMsg,
	}

	jsonData, err := json.Marshal(progress)
	require.NoError(t, err)

	var parsed BackupProgressMessage
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Equal(t, progress.BackupJobUID, parsed.BackupJobUID)
	assert.Equal(t, progress.Status, parsed.Status)
	assert.Equal(t, progress.Progress, parsed.Progress)
	assert.Equal(t, progress.ProgressMessage, parsed.ProgressMessage)
	assert.NotNil(t, parsed.StartedAt)
	assert.NotNil(t, parsed.CompletedAt)
	assert.NotNil(t, parsed.ErrorMessage)
	assert.Equal(t, errorMsg, *parsed.ErrorMessage)
}

func TestWebSocketService_BroadcastBackupProgress_Basic(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-test-123",
		Status:          "running",
		Progress:        50.0,
		ProgressMessage: "Processing backup...",
		StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}

	// Should not error even with no connections
	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
}

func TestWebSocketService_BroadcastBackupProgress_WithConnections(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)

	// Create and register a connection
	conn := &Connection{
		ID:     "test-progress-conn",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond) // Allow registration

	// Send backup progress
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-test-456",
		Status:          "running",
		Progress:        75.0,
		ProgressMessage: "Backing up table products",
		StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}

	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond) // Allow message processing

	// Verify message was received
	select {
	case msgBytes := <-conn.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		
		assert.Equal(t, "backup_progress", message.Type)
		assert.Equal(t, userID, *message.UserID)
		
		// Check progress data
		progressData, ok := message.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "backup-test-456", progressData["backup_job_uid"])
		assert.Equal(t, "running", progressData["status"])
		assert.Equal(t, 75.0, progressData["progress"])
		assert.Equal(t, "Backing up table products", progressData["progress_message"])
		
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive backup progress message")
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}

func TestWebSocketService_BroadcastBackupProgress_MultipleUsers(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	user1ID := uint(123)
	user2ID := uint(456)

	// Create connections for both users
	conn1 := &Connection{
		ID:     "user1-conn",
		UserID: user1ID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	conn2 := &Connection{
		ID:     "user2-conn",
		UserID: user2ID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond)

	// Send progress to user1 only
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-user1-789",
		Status:          "completed",
		Progress:        100.0,
		ProgressMessage: "Backup completed successfully",
		StartedAt:       func() *time.Time { t := time.Now().Add(-5 * time.Minute); return &t }(),
		CompletedAt:     func() *time.Time { t := time.Now(); return &t }(),
	}

	err := wsService.BroadcastBackupProgress(user1ID, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// User1 should receive the message
	select {
	case msgBytes := <-conn1.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		assert.Equal(t, "backup_progress", message.Type)
		assert.Equal(t, user1ID, *message.UserID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("User 1 did not receive backup progress message")
	}

	// User2 should not receive the message
	select {
	case <-conn2.Send:
		t.Fatal("User 2 should not have received the message")
	case <-time.After(50 * time.Millisecond):
		// This is expected - user 2 should not receive the message
	}

	// Clean up
	wsService.hub.unregister <- conn1
	wsService.hub.unregister <- conn2
	time.Sleep(20 * time.Millisecond)
}

func TestWebSocketService_BroadcastBackupProgress_MultipleConnectionsSameUser(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)

	// Create multiple connections for the same user
	conn1 := &Connection{
		ID:     "user-conn-1",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	conn2 := &Connection{
		ID:     "user-conn-2",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn1
	wsService.hub.register <- conn2
	time.Sleep(20 * time.Millisecond)

	// Send backup progress
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-multi-conn",
		Status:          "running",
		Progress:        25.0,
		ProgressMessage: "Starting backup process",
		StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}

	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Both connections should receive the message
	messagesReceived := 0

	select {
	case msgBytes := <-conn1.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		assert.Equal(t, "backup_progress", message.Type)
		messagesReceived++
	case <-time.After(100 * time.Millisecond):
		t.Error("Connection 1 did not receive message")
	}

	select {
	case msgBytes := <-conn2.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		assert.Equal(t, "backup_progress", message.Type)
		messagesReceived++
	case <-time.After(100 * time.Millisecond):
		t.Error("Connection 2 did not receive message")
	}

	assert.Equal(t, 2, messagesReceived, "Both connections should receive the message")

	// Clean up
	wsService.hub.unregister <- conn1
	wsService.hub.unregister <- conn2
	time.Sleep(20 * time.Millisecond)
}

func TestBackupProgressMessage_DifferentStatuses(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	conn := &Connection{
		ID:     "status-test-conn",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	statuses := []struct {
		status   string
		progress float64
		message  string
	}{
		{"pending", 0.0, "Backup job queued"},
		{"running", 10.0, "Starting backup process"},
		{"running", 50.0, "Backing up table users"},
		{"running", 80.0, "Backing up table orders"},
		{"completed", 100.0, "Backup completed successfully"},
	}

	for _, s := range statuses {
		progress := &BackupProgressMessage{
			BackupJobUID:    "backup-status-test",
			Status:          s.status,
			Progress:        s.progress,
			ProgressMessage: s.message,
			StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
		}

		if s.status == "completed" {
			progress.CompletedAt = func() *time.Time { t := time.Now(); return &t }()
		}

		err := wsService.BroadcastBackupProgress(userID, progress)
		assert.NoError(t, err)
		time.Sleep(10 * time.Millisecond)

		// Verify message was received with correct status
		select {
		case msgBytes := <-conn.Send:
			var message Message
			err := json.Unmarshal(msgBytes, &message)
			require.NoError(t, err)
			
			progressData, ok := message.Data.(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, s.status, progressData["status"])
			assert.Equal(t, s.progress, progressData["progress"])
			assert.Equal(t, s.message, progressData["progress_message"])
			
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("Did not receive message for status %s", s.status)
		}
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}

func TestBackupProgressMessage_WithError(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	conn := &Connection{
		ID:     "error-test-conn",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	errorMsg := "Database connection failed"
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-error-test",
		Status:          "failed",
		Progress:        35.0,
		ProgressMessage: "Backup failed during table export",
		StartedAt:       func() *time.Time { t := time.Now().Add(-2 * time.Minute); return &t }(),
		CompletedAt:     func() *time.Time { t := time.Now(); return &t }(),
		ErrorMessage:    &errorMsg,
	}

	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	select {
	case msgBytes := <-conn.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		
		progressData, ok := message.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "failed", progressData["status"])
		assert.Equal(t, 35.0, progressData["progress"])
		assert.Equal(t, errorMsg, progressData["error_message"])
		
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive error message")
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}

func TestBackupProgressMessage_ConcurrentBroadcasts(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	numUsers := 5
	numMessages := 10
	var connections []*Connection

	// Create connections for multiple users
	for i := 0; i < numUsers; i++ {
		conn := &Connection{
			ID:     "concurrent-conn-" + string(rune(i)),
			UserID: uint(i + 1),
			Send:   make(chan []byte, 256),
			Hub:    wsService.hub,
		}
		connections = append(connections, conn)
		wsService.hub.register <- conn
	}
	time.Sleep(50 * time.Millisecond) // Allow all registrations

	// Send messages concurrently to all users
	var wg sync.WaitGroup
	for userID := 1; userID <= numUsers; userID++ {
		for msgNum := 1; msgNum <= numMessages; msgNum++ {
			wg.Add(1)
			go func(uid, num int) {
				defer wg.Done()
				
				progress := &BackupProgressMessage{
					BackupJobUID:    "concurrent-backup-" + string(rune(uid)) + "-" + string(rune(num)),
					Status:          "running",
					Progress:        float64(num * 10),
					ProgressMessage: "Processing message " + string(rune(num)),
					StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
				}

				err := wsService.BroadcastBackupProgress(uint(uid), progress)
				assert.NoError(t, err)
			}(userID, msgNum)
		}
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Allow all messages to be processed

	// Verify each user received exactly numMessages messages
	for i, conn := range connections {
		messagesReceived := 0
		timeout := time.After(1 * time.Second)
		
		for messagesReceived < numMessages {
			select {
			case msgBytes := <-conn.Send:
				var message Message
				err := json.Unmarshal(msgBytes, &message)
				require.NoError(t, err)
				assert.Equal(t, "backup_progress", message.Type)
				assert.Equal(t, uint(i+1), *message.UserID)
				messagesReceived++
			case <-timeout:
				t.Fatalf("User %d only received %d/%d messages", i+1, messagesReceived, numMessages)
			}
		}
		
		assert.Equal(t, numMessages, messagesReceived)
	}

	// Clean up
	for _, conn := range connections {
		wsService.hub.unregister <- conn
	}
	time.Sleep(50 * time.Millisecond)
}

func TestBackupProgressMessage_LargePayload(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	conn := &Connection{
		ID:     "large-payload-conn",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	// Create a large progress message
	largeMessage := make([]byte, 1000)
	for i := range largeMessage {
		largeMessage[i] = 'A' + byte(i%26)
	}

	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-large-payload-test-with-very-long-uid-" + string(largeMessage[:100]),
		Status:          "running",
		Progress:        50.0,
		ProgressMessage: "Processing large backup with extensive details: " + string(largeMessage[:500]),
		StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}

	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Verify large message was received correctly
	select {
	case msgBytes := <-conn.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		
		progressData, ok := message.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "running", progressData["status"])
		assert.Contains(t, progressData["backup_job_uid"].(string), "backup-large-payload-test")
		assert.Contains(t, progressData["progress_message"].(string), "Processing large backup")
		
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Did not receive large payload message")
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}

func TestBackupProgressMessage_MessageTimestamp(t *testing.T) {
	jwtManager := createTestJWTManager()
	wsService := NewWebSocketService(jwtManager)

	userID := uint(123)
	conn := &Connection{
		ID:     "timestamp-test-conn",
		UserID: userID,
		Send:   make(chan []byte, 256),
		Hub:    wsService.hub,
	}

	wsService.hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	beforeSend := time.Now()
	
	progress := &BackupProgressMessage{
		BackupJobUID:    "backup-timestamp-test",
		Status:          "running",
		Progress:        60.0,
		ProgressMessage: "Testing timestamp",
		StartedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}

	err := wsService.BroadcastBackupProgress(userID, progress)
	assert.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	afterSend := time.Now()

	select {
	case msgBytes := <-conn.Send:
		var message Message
		err := json.Unmarshal(msgBytes, &message)
		require.NoError(t, err)
		
		// Verify message timestamp is within expected range
		assert.True(t, message.Timestamp.After(beforeSend) || message.Timestamp.Equal(beforeSend))
		assert.True(t, message.Timestamp.Before(afterSend) || message.Timestamp.Equal(afterSend))
		
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive timestamp test message")
	}

	// Clean up
	wsService.hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)
}