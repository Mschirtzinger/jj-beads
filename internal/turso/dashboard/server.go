// Package dashboard provides real-time WebSocket server for agent coordination.
//
// The dashboard broadcasts task state changes, dependency updates, and sync events
// to connected WebSocket clients, enabling real-time monitoring of agent activity.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// MessageType defines the type of dashboard message
type MessageType string

const (
	// MessageTypeTaskUpdate indicates a task was created, updated, or deleted
	MessageTypeTaskUpdate MessageType = "task_update"

	// MessageTypeDepUpdate indicates a dependency was added or removed
	MessageTypeDepUpdate MessageType = "dep_update"

	// MessageTypeSyncComplete indicates a full sync completed
	MessageTypeSyncComplete MessageType = "sync_complete"

	// MessageTypeStats indicates updated task statistics
	MessageTypeStats MessageType = "stats"

	// MessageTypeBlockedCache indicates blocked cache was refreshed
	MessageTypeBlockedCache MessageType = "blocked_cache"
)

// Message represents a dashboard broadcast message
type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// TaskUpdateData contains task change information
type TaskUpdateData struct {
	TaskID    string `json:"task_id"`
	Action    string `json:"action"` // created, updated, deleted
	Status    string `json:"status,omitempty"`
	Title     string `json:"title,omitempty"`
	Priority  int    `json:"priority,omitempty"`
	Assignee  string `json:"assignee,omitempty"`
}

// DepUpdateData contains dependency change information
type DepUpdateData struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Type   string `json:"type"`
	Action string `json:"action"` // added, removed
}

// StatsData contains task statistics
type StatsData struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"by_status"`
	Blocked    int            `json:"blocked"`
	Ready      int            `json:"ready"`
	InProgress int            `json:"in_progress"`
}

// SyncCompleteData contains sync completion information
type SyncCompleteData struct {
	TasksProcessed int           `json:"tasks_processed"`
	DepsProcessed  int           `json:"deps_processed"`
	Duration       time.Duration `json:"duration"`
}

// BlockedCacheData contains blocked cache refresh information
type BlockedCacheData struct {
	BlockedCount int `json:"blocked_count"`
	ReadyCount   int `json:"ready_count"`
}

// Server manages WebSocket connections and broadcasts dashboard messages
type Server struct {
	addr     string
	listener net.Listener
	server   *http.Server

	// WebSocket client management
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex

	// Message broadcasting
	broadcast chan Message

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Logging
	logger *log.Logger
}

// Config holds server configuration
type Config struct {
	// Port to listen on (default: 8080)
	Port int

	// Logger for server activity (default: stderr logger)
	Logger *log.Logger
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Port:   8080,
		Logger: log.Default(),
	}
}

// NewServer creates a new dashboard WebSocket server
func NewServer(config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		addr:      fmt.Sprintf(":%d", config.Port),
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan Message, 100),
		ctx:       ctx,
		cancel:    cancel,
		logger:    config.Logger,
	}
}

// Start begins the HTTP server and WebSocket handler
func (s *Server) Start() error {
	// Create listener
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	s.listener = ln

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start broadcast handler
	s.wg.Add(1)
	go s.broadcastLoop()

	// Start HTTP server
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Printf("Dashboard server listening on %s", s.addr)
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("Server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop() error {
	s.logger.Println("Stopping dashboard server")

	// Signal shutdown
	s.cancel()

	// Close all WebSocket connections
	s.clientsMu.Lock()
	for conn := range s.clients {
		_ = conn.Close(websocket.StatusGoingAway, "Server shutting down")
		delete(s.clients, conn)
	}
	s.clientsMu.Unlock()

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Wait for goroutines
	s.wg.Wait()

	s.logger.Println("Dashboard server stopped")
	return nil
}

// Broadcast sends a message to all connected clients
func (s *Server) Broadcast(msg Message) {
	select {
	case s.broadcast <- msg:
	case <-s.ctx.Done():
		return
	default:
		s.logger.Println("Warning: broadcast channel full, dropping message")
	}
}

// broadcastLoop handles message broadcasting to all clients
func (s *Server) broadcastLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return

		case msg := <-s.broadcast:
			// Add timestamp if not set
			if msg.Timestamp.IsZero() {
				msg.Timestamp = time.Now()
			}

			// Marshal message to JSON
			data, err := json.Marshal(msg)
			if err != nil {
				s.logger.Printf("Failed to marshal message: %v", err)
				continue
			}

			// Send to all connected clients
			s.clientsMu.RLock()
			clients := make([]*websocket.Conn, 0, len(s.clients))
			for conn := range s.clients {
				clients = append(clients, conn)
			}
			s.clientsMu.RUnlock()

			// Send to clients (outside read lock to avoid blocking broadcasts)
			for _, conn := range clients {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := conn.Write(ctx, websocket.MessageText, data)
				cancel()

				if err != nil {
					s.logger.Printf("Failed to send to client: %v", err)
					s.removeClient(conn)
				}
			}
		}
	}
}

// handleWebSocket upgrades HTTP connections to WebSocket
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Allow all origins for development
	})
	if err != nil {
		s.logger.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Add client
	s.clientsMu.Lock()
	s.clients[conn] = true
	clientCount := len(s.clients)
	s.clientsMu.Unlock()

	s.logger.Printf("Client connected (total: %d)", clientCount)

	// Send initial welcome message
	welcome := Message{
		Type:      MessageTypeStats,
		Timestamp: time.Now(),
	}
	welcomeData, _ := json.Marshal(welcome)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = conn.Write(ctx, websocket.MessageText, welcomeData)
	cancel()

	// Keep connection alive (read loop)
	go s.readLoop(conn)
}

// readLoop keeps the WebSocket connection alive and handles client disconnects
func (s *Server) readLoop(conn *websocket.Conn) {
	defer s.removeClient(conn)

	for {
		_, _, err := conn.Read(s.ctx)
		if err != nil {
			return
		}
		// We don't process client messages, just keep connection alive
	}
}

// removeClient safely removes a client connection
func (s *Server) removeClient(conn *websocket.Conn) {
	s.clientsMu.Lock()
	if _, exists := s.clients[conn]; exists {
		delete(s.clients, conn)
		clientCount := len(s.clients)
		s.clientsMu.Unlock()

		_ = conn.Close(websocket.StatusNormalClosure, "")
		s.logger.Printf("Client disconnected (total: %d)", clientCount)
	} else {
		s.clientsMu.Unlock()
	}
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.clientsMu.RLock()
	clientCount := len(s.clients)
	s.clientsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"clients": clientCount,
	})
}

// handleRoot returns basic server information
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Beads Dashboard</title>
</head>
<body>
    <h1>Beads Dashboard Server</h1>
    <p>WebSocket endpoint: <code>ws://%s/ws</code></p>
    <p>Health check: <a href="/health">/health</a></p>
    <p>Connect a WebSocket client to receive real-time task updates.</p>
</body>
</html>`, r.Host)
}

// GetAddr returns the server's listening address
func (s *Server) GetAddr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

// ClientCount returns the current number of connected clients
func (s *Server) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}
