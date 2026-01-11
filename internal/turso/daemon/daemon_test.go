package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// TODO: Initialize schema when database layer is implemented
	// For now, just return the connection
	return db
}

// setupTestDirs creates temporary directories for tasks and deps.
func setupTestDirs(t *testing.T) (tasksDir, depsDir string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()
	tasksDir = filepath.Join(tmpDir, "tasks")
	depsDir = filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	cleanup = func() {
		// Cleanup is handled by t.TempDir()
	}

	return tasksDir, depsDir, cleanup
}

// writeTaskFile writes a task to disk for testing.
func writeTaskFile(t *testing.T, dir string, task *schema.TaskFile) {
	t.Helper()

	if err := schema.WriteTaskFile(dir, task); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}
}

// writeDepFile writes a dependency to disk for testing.
func writeDepFile(t *testing.T, dir string, dep *schema.DepFile) {
	t.Helper()

	if err := schema.WriteDepFile(dir, dep); err != nil {
		t.Fatalf("Failed to write dep file: %v", err)
	}
}

func TestNew(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	tests := []struct {
		name    string
		db      *sql.DB
		tasks   string
		deps    string
		wantErr bool
	}{
		{
			name:    "valid configuration",
			db:      db,
			tasks:   tasksDir,
			deps:    depsDir,
			wantErr: false,
		},
		{
			name:    "nil database",
			db:      nil,
			tasks:   tasksDir,
			deps:    depsDir,
			wantErr: true,
		},
		{
			name:    "empty tasks dir",
			db:      db,
			tasks:   "",
			deps:    depsDir,
			wantErr: true,
		},
		{
			name:    "empty deps dir",
			db:      db,
			tasks:   tasksDir,
			deps:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			daemon, err := New(tt.db, tt.tasks, tt.deps)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if daemon != nil {
				defer daemon.Stop()
			}
		})
	}
}

func TestDaemon_PerformFullSync(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// Create test tasks
	now := time.Now()
	task1 := &schema.TaskFile{
		ID:          "bd-test1",
		Title:       "Test task 1",
		Description: "First test task",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		Tags:        []string{"test"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	writeTaskFile(t, tasksDir, task1)

	task2 := &schema.TaskFile{
		ID:          "bd-test2",
		Title:       "Test task 2",
		Description: "Second test task",
		Type:        "bug",
		Status:      "in_progress",
		Priority:    0,
		Tags:        []string{"urgent"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	writeTaskFile(t, tasksDir, task2)

	// Create test dependency
	dep := &schema.DepFile{
		From:      "bd-test1",
		To:        "bd-test2",
		Type:      "blocks",
		CreatedAt: now,
	}
	writeDepFile(t, depsDir, dep)

	// Create daemon with silent logger
	config := DefaultConfig()
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	// Perform full sync
	if err := daemon.PerformFullSync(); err != nil {
		t.Fatalf("PerformFullSync() error = %v", err)
	}

	// TODO: Verify database contents when schema is implemented
}

func TestDaemon_FileWatching(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// Create daemon with short intervals for testing
	config := DefaultConfig()
	config.DebounceInterval = 50 * time.Millisecond
	config.BlockedCacheRefreshInterval = 100 * time.Millisecond
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	// Start daemon in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	// Wait for daemon to initialize
	time.Sleep(100 * time.Millisecond)

	// Create a new task file
	now := time.Now()
	task := &schema.TaskFile{
		ID:          "bd-watch1",
		Title:       "Watched task",
		Description: "This task was created after daemon started",
		Type:        "feature",
		Status:      "open",
		Priority:    2,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	writeTaskFile(t, tasksDir, task)

	// Wait for debounce and processing
	time.Sleep(200 * time.Millisecond)

	// Modify the task
	task.Status = "in_progress"
	task.UpdatedAt = time.Now()
	writeTaskFile(t, tasksDir, task)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Delete the task file
	taskPath := filepath.Join(tasksDir, task.Filename())
	if err := os.Remove(taskPath); err != nil {
		t.Fatalf("Failed to delete task file: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Wait for daemon to finish
	<-ctx.Done()
	if err := <-errCh; err != nil {
		t.Errorf("Daemon error: %v", err)
	}

	// TODO: Verify database state when schema is implemented
}

func TestDaemon_DebounceMultipleChanges(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// Create daemon with longer debounce interval
	config := DefaultConfig()
	config.DebounceInterval = 200 * time.Millisecond
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	// Start daemon
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	// Wait for initialization
	time.Sleep(100 * time.Millisecond)

	// Create a task and rapidly modify it
	now := time.Now()
	task := &schema.TaskFile{
		ID:          "bd-debounce1",
		Title:       "Debounce test",
		Description: "Initial description",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	writeTaskFile(t, tasksDir, task)

	// Make rapid changes (faster than debounce interval)
	for i := 0; i < 5; i++ {
		task.Description = time.Now().String()
		task.UpdatedAt = time.Now()
		writeTaskFile(t, tasksDir, task)
		time.Sleep(30 * time.Millisecond)
	}

	// Wait for debounce to settle
	time.Sleep(500 * time.Millisecond)

	// TODO: Verify only one sync happened (or reduced number)
	// This would require exposing metrics from the daemon

	<-ctx.Done()
	if err := <-errCh; err != nil {
		t.Errorf("Daemon error: %v", err)
	}
}

func TestDaemon_GracefulShutdown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	config := DefaultConfig()
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Signal shutdown
	cancel()

	// Wait for graceful shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Daemon shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Daemon did not shut down within timeout")
	}
}

func TestDaemon_InvalidFiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// Create an invalid task file (missing required fields)
	invalidTask := map[string]interface{}{
		"id": "bd-invalid",
		// Missing title, type, status, etc.
	}
	data, _ := json.MarshalIndent(invalidTask, "", "  ")
	invalidPath := filepath.Join(tasksDir, "bd-invalid.json")
	if err := os.WriteFile(invalidPath, data, 0644); err != nil {
		t.Fatalf("Failed to write invalid file: %v", err)
	}

	config := DefaultConfig()
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	// Full sync should complete despite invalid file
	// (it should log warning and continue)
	if err := daemon.PerformFullSync(); err != nil {
		t.Errorf("PerformFullSync() should handle invalid files gracefully, got error: %v", err)
	}
}

func TestDaemon_NonJsonFiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// Create non-JSON files that should be ignored
	txtFile := filepath.Join(tasksDir, "README.txt")
	if err := os.WriteFile(txtFile, []byte("This is not a task file"), 0644); err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}

	config := DefaultConfig()
	config.DebounceInterval = 50 * time.Millisecond
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	// Wait for initialization
	time.Sleep(100 * time.Millisecond)

	// Modify the .txt file - should be ignored
	if err := os.WriteFile(txtFile, []byte("Updated text"), 0644); err != nil {
		t.Fatalf("Failed to update txt file: %v", err)
	}

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Should not cause any errors
	<-ctx.Done()
	if err := <-errCh; err != nil {
		t.Errorf("Daemon error: %v", err)
	}
}

func TestDaemon_EmptyDirectories(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	config := DefaultConfig()
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	// Full sync on empty directories should work
	if err := daemon.PerformFullSync(); err != nil {
		t.Errorf("PerformFullSync() on empty dirs error = %v", err)
	}
}

func TestDaemon_ConcurrentFileChanges(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tasksDir, depsDir, cleanup := setupTestDirs(t)
	defer cleanup()

	config := DefaultConfig()
	config.DebounceInterval = 100 * time.Millisecond
	config.Logger = log.New(io.Discard, "", 0)

	daemon, err := NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		t.Fatalf("Failed to create daemon: %v", err)
	}
	defer daemon.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Simulate multiple agents writing concurrently
	now := time.Now()
	done := make(chan bool, 3)

	// Agent 1 - creates tasks
	go func() {
		for i := 0; i < 5; i++ {
			task := &schema.TaskFile{
				ID:        "bd-agent1-" + string(rune('a'+i)),
				Title:     "Agent 1 task",
				Type:      "task",
				Status:    "open",
				Priority:  1,
				CreatedAt: now,
				UpdatedAt: time.Now(),
			}
			writeTaskFile(t, tasksDir, task)
			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	// Agent 2 - creates different tasks
	go func() {
		for i := 0; i < 5; i++ {
			task := &schema.TaskFile{
				ID:        "bd-agent2-" + string(rune('a'+i)),
				Title:     "Agent 2 task",
				Type:      "bug",
				Status:    "open",
				Priority:  0,
				CreatedAt: now,
				UpdatedAt: time.Now(),
			}
			writeTaskFile(t, tasksDir, task)
			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	// Agent 3 - creates dependencies
	go func() {
		time.Sleep(100 * time.Millisecond) // Wait for some tasks to exist
		for i := 0; i < 3; i++ {
			dep := &schema.DepFile{
				From:      "bd-agent1-a",
				To:        "bd-agent2-a",
				Type:      "blocks",
				CreatedAt: time.Now(),
			}
			writeDepFile(t, depsDir, dep)
			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all agents
	for i := 0; i < 3; i++ {
		<-done
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	<-ctx.Done()
	if err := <-errCh; err != nil {
		t.Errorf("Daemon error: %v", err)
	}

	// TODO: Verify all changes were synced correctly
}
