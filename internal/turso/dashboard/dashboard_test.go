package dashboard

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/steveyegge/beads/internal/turso/schema"
)

func TestServerStartStop(t *testing.T) {
	config := &Config{
		Port:   0, // Use random available port
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Check that server is listening
	addr := server.GetAddr()
	if addr == "" {
		t.Fatal("Server address is empty")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}

func TestWebSocketConnection(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Connect WebSocket client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Verify client count
	if count := server.ClientCount(); count != 1 {
		t.Errorf("Expected 1 client, got %d", count)
	}

	// Read welcome message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != MessageTypeStats {
		t.Errorf("Expected welcome message type %s, got %s", MessageTypeStats, msg.Type)
	}
}

func TestMultipleClients(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"

	// Connect multiple clients
	numClients := 3
	clients := make([]*websocket.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		clients[i] = conn

		// Read welcome message
		_, _, err = conn.Read(ctx)
		if err != nil {
			t.Fatalf("Failed to read welcome message for client %d: %v", i, err)
		}
	}

	// Verify client count
	if count := server.ClientCount(); count != numClients {
		t.Errorf("Expected %d clients, got %d", numClients, count)
	}
}

func TestMessageBroadcast(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"

	// Connect client
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	// Broadcast a test message
	testData := TaskUpdateData{
		TaskID:   "bd-test",
		Action:   "created",
		Status:   "open",
		Title:    "Test Task",
		Priority: 1,
	}

	dataJSON, _ := json.Marshal(testData)
	testMsg := Message{
		Type:      MessageTypeTaskUpdate,
		Timestamp: time.Now(),
		Data:      dataJSON,
	}

	server.Broadcast(testMsg)

	// Read broadcasted message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read broadcast message: %v", err)
	}

	var received Message
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if received.Type != MessageTypeTaskUpdate {
		t.Errorf("Expected message type %s, got %s", MessageTypeTaskUpdate, received.Type)
	}

	var receivedData TaskUpdateData
	if err := json.Unmarshal(received.Data, &receivedData); err != nil {
		t.Fatalf("Failed to unmarshal task data: %v", err)
	}

	if receivedData.TaskID != testData.TaskID {
		t.Errorf("Expected task ID %s, got %s", testData.TaskID, receivedData.TaskID)
	}
}

func TestHandlerTaskEvents(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	handler := NewHandler(server, log.New(os.Stderr, "[test-handler] ", log.LstdFlags))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"

	// Connect client
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	// Test OnTaskCreated
	task := &schema.TaskFile{
		ID:          "bd-test1",
		Title:       "Test Task 1",
		Description: "Test description",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	handler.OnTaskCreated(task)

	// Read task update message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read task update: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != MessageTypeTaskUpdate {
		t.Errorf("Expected message type %s, got %s", MessageTypeTaskUpdate, msg.Type)
	}

	// Read stats message
	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read stats update: %v", err)
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal stats message: %v", err)
	}

	if msg.Type != MessageTypeStats {
		t.Errorf("Expected message type %s, got %s", MessageTypeStats, msg.Type)
	}
}

func TestHandlerDepEvents(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	handler := NewHandler(server, log.New(os.Stderr, "[test-handler] ", log.LstdFlags))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"

	// Connect client
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	// Test OnDepAdded
	dep := &schema.DepFile{
		From:      "bd-test1",
		To:        "bd-test2",
		Type:      "blocks",
		CreatedAt: time.Now(),
	}

	handler.OnDepAdded(dep)

	// Read dep update message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read dep update: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != MessageTypeDepUpdate {
		t.Errorf("Expected message type %s, got %s", MessageTypeDepUpdate, msg.Type)
	}

	var depData DepUpdateData
	if err := json.Unmarshal(msg.Data, &depData); err != nil {
		t.Fatalf("Failed to unmarshal dep data: %v", err)
	}

	if depData.From != dep.From || depData.To != dep.To || depData.Type != dep.Type {
		t.Errorf("Dep data mismatch: got %+v, want from=%s to=%s type=%s",
			depData, dep.From, dep.To, dep.Type)
	}

	if depData.Action != "added" {
		t.Errorf("Expected action 'added', got %s", depData.Action)
	}
}

func TestHandlerSyncComplete(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	handler := NewHandler(server, log.New(os.Stderr, "[test-handler] ", log.LstdFlags))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"

	// Connect client
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	// Test OnSyncComplete
	handler.OnSyncComplete(100, 50, 2*time.Second)

	// Read sync complete message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read sync complete: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != MessageTypeSyncComplete {
		t.Errorf("Expected message type %s, got %s", MessageTypeSyncComplete, msg.Type)
	}

	var syncData SyncCompleteData
	if err := json.Unmarshal(msg.Data, &syncData); err != nil {
		t.Fatalf("Failed to unmarshal sync data: %v", err)
	}

	if syncData.TasksProcessed != 100 {
		t.Errorf("Expected 100 tasks processed, got %d", syncData.TasksProcessed)
	}

	if syncData.DepsProcessed != 50 {
		t.Errorf("Expected 50 deps processed, got %d", syncData.DepsProcessed)
	}
}

func TestHandlerBlockedCache(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	handler := NewHandler(server, log.New(os.Stderr, "[test-handler] ", log.LstdFlags))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws://" + server.GetAddr() + "/ws"

	// Connect client
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	// Test OnBlockedCacheRefresh
	handler.OnBlockedCacheRefresh(25, 75)

	// Read blocked cache message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read blocked cache: %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if msg.Type != MessageTypeBlockedCache {
		t.Errorf("Expected message type %s, got %s", MessageTypeBlockedCache, msg.Type)
	}

	var cacheData BlockedCacheData
	if err := json.Unmarshal(msg.Data, &cacheData); err != nil {
		t.Fatalf("Failed to unmarshal cache data: %v", err)
	}

	if cacheData.BlockedCount != 25 {
		t.Errorf("Expected 25 blocked, got %d", cacheData.BlockedCount)
	}

	if cacheData.ReadyCount != 75 {
		t.Errorf("Expected 75 ready, got %d", cacheData.ReadyCount)
	}

	// Read stats message
	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read stats update: %v", err)
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Failed to unmarshal stats message: %v", err)
	}

	if msg.Type != MessageTypeStats {
		t.Errorf("Expected message type %s, got %s", MessageTypeStats, msg.Type)
	}
}

func TestHealthEndpoint(t *testing.T) {
	config := &Config{
		Port:   0,
		Logger: log.New(os.Stderr, "[test] ", log.LstdFlags),
	}

	server := NewServer(config)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Note: Full HTTP testing would require net/http/httptest
	// This is a placeholder to show the test structure
	// In a real implementation, you would make an HTTP GET to /health
	// and verify the JSON response
	t.Log("Health endpoint test placeholder - full implementation would use httptest")
}
