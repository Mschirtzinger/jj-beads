package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewFileWatcher verifies that creating a new FileWatcher succeeds.
func TestNewFileWatcher(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if fw == nil {
		t.Fatal("NewFileWatcher() returned nil")
	}

	if fw.IsRunning() {
		t.Error("Newly created watcher should not be running")
	}
}

// TestFileWatcher_StartStop verifies that the watcher can start and stop cleanly.
func TestFileWatcher_StartStop(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}

	// Start watching
	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !fw.IsRunning() {
		t.Error("Watcher should be running after Start()")
	}

	// Stop watching
	if err := fw.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	if fw.IsRunning() {
		t.Error("Watcher should not be running after Stop()")
	}
}

// TestFileWatcher_StartAlreadyRunning verifies that starting an already running watcher fails.
func TestFileWatcher_StartAlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	// Try to start again
	err = fw.Start(tasksDir, depsDir)
	if err == nil {
		t.Error("Second Start() should fail when watcher is already running")
	}
}

// TestFileWatcher_TaskFileCreated verifies that creating a task file triggers an event.
func TestFileWatcher_TaskFileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Create a task file
	taskPath := filepath.Join(tasksDir, "bd-test.json")
	if err := os.WriteFile(taskPath, []byte(`{"id":"bd-test"}`), 0644); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}

	// Wait for event
	select {
	case event := <-fw.Events():
		if event.Type != TypeTask {
			t.Errorf("Expected TypeTask, got %v", event.Type)
		}
		if event.Op != OpCreate {
			t.Errorf("Expected OpCreate, got %v", event.Op)
		}
		if filepath.Base(event.Path) != "bd-test.json" {
			t.Errorf("Expected bd-test.json, got %s", filepath.Base(event.Path))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for task create event")
	}
}

// TestFileWatcher_TaskFileModified verifies that modifying a task file triggers an event.
func TestFileWatcher_TaskFileModified(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	// Create a task file first
	taskPath := filepath.Join(tasksDir, "bd-test.json")
	if err := os.WriteFile(taskPath, []byte(`{"id":"bd-test"}`), 0644); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give watcher time to stabilize
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(taskPath, []byte(`{"id":"bd-test","status":"updated"}`), 0644); err != nil {
		t.Fatalf("Failed to update task file: %v", err)
	}

	// Wait for event
	select {
	case event := <-fw.Events():
		if event.Type != TypeTask {
			t.Errorf("Expected TypeTask, got %v", event.Type)
		}
		if event.Op != OpModify {
			t.Errorf("Expected OpModify, got %v", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for task modify event")
	}
}

// TestFileWatcher_TaskFileDeleted verifies that deleting a task file triggers an event.
func TestFileWatcher_TaskFileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	// Create a task file first
	taskPath := filepath.Join(tasksDir, "bd-test.json")
	if err := os.WriteFile(taskPath, []byte(`{"id":"bd-test"}`), 0644); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Give watcher time to stabilize
	time.Sleep(100 * time.Millisecond)

	// Delete the file
	if err := os.Remove(taskPath); err != nil {
		t.Fatalf("Failed to delete task file: %v", err)
	}

	// Wait for event
	select {
	case event := <-fw.Events():
		if event.Type != TypeTask {
			t.Errorf("Expected TypeTask, got %v", event.Type)
		}
		if event.Op != OpDelete {
			t.Errorf("Expected OpDelete, got %v", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for task delete event")
	}
}

// TestFileWatcher_DepFileCreated verifies that creating a dep file triggers an event.
func TestFileWatcher_DepFileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Create a dep file
	depPath := filepath.Join(depsDir, "bd-1--blocks--bd-2.json")
	depContent := `{"from":"bd-1","to":"bd-2","type":"blocks"}`
	if err := os.WriteFile(depPath, []byte(depContent), 0644); err != nil {
		t.Fatalf("Failed to write dep file: %v", err)
	}

	// Wait for event
	select {
	case event := <-fw.Events():
		if event.Type != TypeDep {
			t.Errorf("Expected TypeDep, got %v", event.Type)
		}
		if event.Op != OpCreate {
			t.Errorf("Expected OpCreate, got %v", event.Op)
		}
		if filepath.Base(event.Path) != "bd-1--blocks--bd-2.json" {
			t.Errorf("Expected bd-1--blocks--bd-2.json, got %s", filepath.Base(event.Path))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for dep create event")
	}
}

// TestFileWatcher_NonJSONFilesIgnored verifies that non-.json files are ignored.
func TestFileWatcher_NonJSONFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Create a non-.json file
	txtPath := filepath.Join(tasksDir, "readme.txt")
	if err := os.WriteFile(txtPath, []byte("This is a readme"), 0644); err != nil {
		t.Fatalf("Failed to write txt file: %v", err)
	}

	// Should not receive any event (or at least not timeout waiting)
	select {
	case event := <-fw.Events():
		t.Errorf("Should not receive event for non-.json file, got: %+v", event)
	case <-time.After(500 * time.Millisecond):
		// Expected - no event for non-.json file
	}
}

// TestFileWatcher_MultipleEvents verifies that multiple file operations are tracked.
func TestFileWatcher_MultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Create multiple files
	files := []struct {
		dir      string
		filename string
		fileType FileType
	}{
		{tasksDir, "bd-1.json", TypeTask},
		{tasksDir, "bd-2.json", TypeTask},
		{depsDir, "bd-1--blocks--bd-2.json", TypeDep},
	}

	for _, f := range files {
		path := filepath.Join(f.dir, f.filename)
		if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", f.filename, err)
		}
	}

	// Collect events (with timeout)
	var events []FileEvent
	timeout := time.After(3 * time.Second)
	for i := 0; i < len(files); i++ {
		select {
		case event := <-fw.Events():
			events = append(events, event)
		case <-timeout:
			t.Fatalf("Timeout waiting for events. Got %d/%d events", len(events), len(files))
		}
	}

	// Verify we got all events
	if len(events) != len(files) {
		t.Errorf("Expected %d events, got %d", len(files), len(events))
	}

	// Verify each event has correct type
	taskCount := 0
	depCount := 0
	for _, event := range events {
		if event.Op != OpCreate {
			t.Errorf("Expected OpCreate, got %v for %s", event.Op, event.Path)
		}
		if event.Type == TypeTask {
			taskCount++
		} else if event.Type == TypeDep {
			depCount++
		}
	}

	if taskCount != 2 {
		t.Errorf("Expected 2 task events, got %d", taskCount)
	}
	if depCount != 1 {
		t.Errorf("Expected 1 dep event, got %d", depCount)
	}
}

// TestFileWatcher_StopClosesChannels verifies that Stop() closes the event channels.
func TestFileWatcher_StopClosesChannels(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	events := fw.Events()
	errors := fw.Errors()

	if err := fw.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}

	// Verify channels are closed
	select {
	case _, ok := <-events:
		if ok {
			t.Error("Events channel should be closed after Stop()")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout verifying events channel closure")
	}

	select {
	case _, ok := <-errors:
		if ok {
			t.Error("Errors channel should be closed after Stop()")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout verifying errors channel closure")
	}
}

// TestEventOp_String verifies the String() method for EventOp.
func TestEventOp_String(t *testing.T) {
	tests := []struct {
		op       EventOp
		expected string
	}{
		{OpCreate, "create"},
		{OpModify, "modify"},
		{OpDelete, "delete"},
		{EventOp(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.expected {
			t.Errorf("EventOp(%d).String() = %q, want %q", tt.op, got, tt.expected)
		}
	}
}

// TestFileType_String verifies the String() method for FileType.
func TestFileType_String(t *testing.T) {
	tests := []struct {
		ft       FileType
		expected string
	}{
		{TypeTask, "task"},
		{TypeDep, "dep"},
		{FileType(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.ft.String(); got != tt.expected {
			t.Errorf("FileType(%d).String() = %q, want %q", tt.ft, got, tt.expected)
		}
	}
}

// TestFileWatcher_StartNonexistentDirectory verifies that starting with nonexistent directories fails.
func TestFileWatcher_StartNonexistentDirectory(t *testing.T) {
	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	// Try to start with nonexistent directories
	err = fw.Start("/nonexistent/tasks", "/nonexistent/deps")
	if err == nil {
		t.Error("Start() should fail with nonexistent directories")
	}
}

// TestFileWatcher_ConcurrentAccess verifies thread safety of watcher operations.
func TestFileWatcher_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("Failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("Failed to create deps dir: %v", err)
	}

	fw, err := NewFileWatcher()
	if err != nil {
		t.Fatalf("NewFileWatcher() failed: %v", err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Concurrent IsRunning() calls should be safe
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = fw.IsRunning()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
