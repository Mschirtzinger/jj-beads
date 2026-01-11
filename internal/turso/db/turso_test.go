package db

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/turso/schema"
)

// testDBPath returns a temporary path for test databases
func testDBPath(t *testing.T) string {
	tmpDir := t.TempDir()
	return filepath.Join(tmpDir, "test.db")
}

// TestOpen_Success tests successful database creation and initialization
func TestOpen_Success(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("Open() returned nil database")
	}

	if db.path != path {
		t.Errorf("path = %q, want %q", db.path, path)
	}
}

// TestInitSchema_Success tests schema creation
func TestInitSchema_Success(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Check that all tables exist
	tables := []string{"tasks", "deps", "blocked_cache"}
	for _, table := range tables {
		var count int
		query := `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`
		err := db.conn.QueryRow(query, table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s does not exist", table)
		}
	}
}

// TestInitSchema_Idempotent tests that schema initialization is idempotent
func TestInitSchema_Idempotent(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	// Initialize schema twice
	if err := db.InitSchema(); err != nil {
		t.Fatalf("First InitSchema() failed: %v", err)
	}

	if err := db.InitSchema(); err != nil {
		t.Errorf("Second InitSchema() failed: %v", err)
	}
}

// TestUpsertTask_Insert tests inserting a new task
func TestUpsertTask_Insert(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()
	task := &schema.TaskFile{
		ID:          "test-1",
		Title:       "Test Task",
		Description: "Test description",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		Tags:        []string{"test", "tag"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := db.UpsertTask(task); err != nil {
		t.Fatalf("UpsertTask() failed: %v", err)
	}

	// Verify task was inserted
	var id, title string
	query := `SELECT id, title FROM tasks WHERE id = ?`
	err = db.conn.QueryRow(query, "test-1").Scan(&id, &title)
	if err != nil {
		t.Fatalf("Failed to query task: %v", err)
	}

	if id != "test-1" {
		t.Errorf("ID = %q, want 'test-1'", id)
	}
	if title != "Test Task" {
		t.Errorf("Title = %q, want 'Test Task'", title)
	}
}

// TestUpsertTask_Update tests updating an existing task
func TestUpsertTask_Update(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()
	task := &schema.TaskFile{
		ID:        "test-1",
		Title:     "Original Title",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Insert
	if err := db.UpsertTask(task); err != nil {
		t.Fatalf("First UpsertTask() failed: %v", err)
	}

	// Update
	task.Title = "Updated Title"
	task.Status = "in_progress"
	task.UpdatedAt = now.Add(time.Hour)

	if err := db.UpsertTask(task); err != nil {
		t.Fatalf("Second UpsertTask() failed: %v", err)
	}

	// Verify update
	var title, status string
	query := `SELECT title, status FROM tasks WHERE id = ?`
	err = db.conn.QueryRow(query, "test-1").Scan(&title, &status)
	if err != nil {
		t.Fatalf("Failed to query task: %v", err)
	}

	if title != "Updated Title" {
		t.Errorf("Title = %q, want 'Updated Title'", title)
	}
	if status != "in_progress" {
		t.Errorf("Status = %q, want 'in_progress'", status)
	}
}

// TestDeleteTask tests deleting a task
func TestDeleteTask(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Insert task
	now := time.Now().UTC()
	task := &schema.TaskFile{
		ID:        "test-1",
		Title:     "Test Task",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.UpsertTask(task); err != nil {
		t.Fatalf("UpsertTask() failed: %v", err)
	}

	// Delete task
	if err := db.DeleteTask("test-1"); err != nil {
		t.Fatalf("DeleteTask() failed: %v", err)
	}

	// Verify deletion
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE id = ?", "test-1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tasks: %v", err)
	}
	if count != 0 {
		t.Errorf("Task count = %d, want 0", count)
	}
}

// TestUpsertDep tests inserting a dependency
func TestUpsertDep(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Insert tasks first (required for foreign keys)
	now := time.Now().UTC()
	task1 := &schema.TaskFile{
		ID:        "test-1",
		Title:     "Task 1",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	task2 := &schema.TaskFile{
		ID:        "test-2",
		Title:     "Task 2",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}

	// Insert dependency
	dep := &schema.DepFile{
		From:      "test-1",
		To:        "test-2",
		Type:      "blocks",
		CreatedAt: now,
	}

	if err := db.UpsertDep(dep); err != nil {
		t.Fatalf("UpsertDep() failed: %v", err)
	}

	// Verify dependency
	var fromID, toID, depType string
	query := `SELECT from_id, to_id, type FROM deps WHERE from_id = ? AND to_id = ?`
	err = db.conn.QueryRow(query, "test-1", "test-2").Scan(&fromID, &toID, &depType)
	if err != nil {
		t.Fatalf("Failed to query dependency: %v", err)
	}

	if fromID != "test-1" {
		t.Errorf("FromID = %q, want 'test-1'", fromID)
	}
	if toID != "test-2" {
		t.Errorf("ToID = %q, want 'test-2'", toID)
	}
	if depType != "blocks" {
		t.Errorf("Type = %q, want 'blocks'", depType)
	}
}

// TestDeleteDep tests deleting a dependency
func TestDeleteDep(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Insert tasks and dependency
	now := time.Now().UTC()
	task1 := &schema.TaskFile{
		ID: "test-1", Title: "Task 1", Type: "task", Status: "open",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}
	task2 := &schema.TaskFile{
		ID: "test-2", Title: "Task 2", Type: "task", Status: "open",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}

	dep := &schema.DepFile{
		From:      "test-1",
		To:        "test-2",
		Type:      "blocks",
		CreatedAt: now,
	}

	if err := db.UpsertDep(dep); err != nil {
		t.Fatalf("UpsertDep() failed: %v", err)
	}

	// Delete dependency
	if err := db.DeleteDep("test-1", "test-2", "blocks"); err != nil {
		t.Fatalf("DeleteDep() failed: %v", err)
	}

	// Verify deletion
	var count int
	query := `SELECT COUNT(*) FROM deps WHERE from_id = ? AND to_id = ? AND type = ?`
	err = db.conn.QueryRow(query, "test-1", "test-2", "blocks").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count deps: %v", err)
	}
	if count != 0 {
		t.Errorf("Dep count = %d, want 0", count)
	}
}

// TestRefreshBlockedCache tests the blocked cache computation
func TestRefreshBlockedCache(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Create tasks: task1 blocks task2
	now := time.Now().UTC()
	task1 := &schema.TaskFile{
		ID: "task-1", Title: "Blocker", Type: "task", Status: "open",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}
	task2 := &schema.TaskFile{
		ID: "task-2", Title: "Blocked", Type: "task", Status: "open",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}

	dep := &schema.DepFile{
		From:      "task-1",
		To:        "task-2",
		Type:      "blocks",
		CreatedAt: now,
	}

	if err := db.UpsertDep(dep); err != nil {
		t.Fatalf("UpsertDep() failed: %v", err)
	}

	// Refresh blocked cache
	if err := db.RefreshBlockedCache(); err != nil {
		t.Fatalf("RefreshBlockedCache() failed: %v", err)
	}

	// Check is_blocked flag
	var isBlocked1, isBlocked2 int
	query := `SELECT is_blocked FROM tasks WHERE id = ?`

	if err := db.conn.QueryRow(query, "task-1").Scan(&isBlocked1); err != nil {
		t.Fatalf("Failed to query task-1: %v", err)
	}
	if err := db.conn.QueryRow(query, "task-2").Scan(&isBlocked2); err != nil {
		t.Fatalf("Failed to query task-2: %v", err)
	}

	if isBlocked1 != 0 {
		t.Errorf("task-1 is_blocked = %d, want 0 (not blocked)", isBlocked1)
	}
	if isBlocked2 != 1 {
		t.Errorf("task-2 is_blocked = %d, want 1 (blocked)", isBlocked2)
	}
}

// TestGetReadyTasks tests the ready work query
func TestGetReadyTasks(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Ready task
	task1 := &schema.TaskFile{
		ID: "ready-1", Title: "Ready Task", Type: "task", Status: "open",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}

	// Blocked task
	task2 := &schema.TaskFile{
		ID: "blocked-1", Title: "Blocked Task", Type: "task", Status: "open",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}

	// Closed task
	task3 := &schema.TaskFile{
		ID: "closed-1", Title: "Closed Task", Type: "task", Status: "closed",
		Priority: 1, CreatedAt: now, UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}
	if err := db.UpsertTask(task3); err != nil {
		t.Fatalf("UpsertTask(task3) failed: %v", err)
	}

	// Block task2
	dep := &schema.DepFile{
		From:      "blocked-1",
		To:        "ready-1",
		Type:      "blocks",
		CreatedAt: now,
	}
	if err := db.UpsertDep(dep); err != nil {
		t.Fatalf("UpsertDep() failed: %v", err)
	}

	if err := db.RefreshBlockedCache(); err != nil {
		t.Fatalf("RefreshBlockedCache() failed: %v", err)
	}

	// Query ready tasks
	ctx := context.Background()
	tasks, err := db.GetReadyTasks(ctx, ReadyTasksOptions{})
	if err != nil {
		t.Fatalf("GetReadyTasks() failed: %v", err)
	}

	// Should only return task2 (ready, not blocked, not closed)
	if len(tasks) != 1 {
		t.Fatalf("GetReadyTasks() returned %d tasks, want 1", len(tasks))
	}

	if tasks[0].ID != "blocked-1" {
		t.Errorf("Ready task ID = %q, want 'blocked-1'", tasks[0].ID)
	}
}

// TestGetTaskCount tests task counting
func TestGetTaskCount(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Insert tasks
	now := time.Now().UTC()
	for i := 1; i <= 5; i++ {
		task := &schema.TaskFile{
			ID: fmt.Sprintf("task-%d", i),
			Title: fmt.Sprintf("Task %d", i),
			Type: "task",
			Status: "open",
			Priority: 1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.UpsertTask(task); err != nil {
			t.Fatalf("UpsertTask() failed: %v", err)
		}
	}

	count, err := db.GetTaskCount()
	if err != nil {
		t.Fatalf("GetTaskCount() failed: %v", err)
	}

	if count != 5 {
		t.Errorf("GetTaskCount() = %d, want 5", count)
	}
}

// TestGetDepCount tests dependency counting
func TestGetDepCount(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Insert tasks and dependencies
	now := time.Now().UTC()
	for i := 1; i <= 3; i++ {
		task := &schema.TaskFile{
			ID: fmt.Sprintf("task-%d", i),
			Title: fmt.Sprintf("Task %d", i),
			Type: "task",
			Status: "open",
			Priority: 1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.UpsertTask(task); err != nil {
			t.Fatalf("UpsertTask() failed: %v", err)
		}
	}

	// Add dependencies
	deps := []*schema.DepFile{
		{From: "task-1", To: "task-2", Type: "blocks", CreatedAt: now},
		{From: "task-2", To: "task-3", Type: "blocks", CreatedAt: now},
	}

	for _, dep := range deps {
		if err := db.UpsertDep(dep); err != nil {
			t.Fatalf("UpsertDep() failed: %v", err)
		}
	}

	count, err := db.GetDepCount()
	if err != nil {
		t.Fatalf("GetDepCount() failed: %v", err)
	}

	if count != 2 {
		t.Errorf("GetDepCount() = %d, want 2", count)
	}
}

// TestClose tests database cleanup
func TestClose(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	// Close database
	if err := db.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Calling Close() again should be safe
	if err := db.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

// TestForeignKeyConstraint tests that foreign key constraints are enforced
func TestForeignKeyConstraint(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Try to insert dependency with non-existent task
	now := time.Now().UTC()
	dep := &schema.DepFile{
		From:      "nonexistent-1",
		To:        "nonexistent-2",
		Type:      "blocks",
		CreatedAt: now,
	}

	err = db.UpsertDep(dep)
	if err == nil {
		t.Error("Expected foreign key constraint error, got nil")
	}
}

// TestGetBlockingTasks tests the transitive closure of blocking dependencies
func TestGetBlockingTasks(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Create a chain: task1 blocks task2 blocks task3
	task1 := &schema.TaskFile{
		ID:        "test-blocking-1",
		Title:     "Blocker 1",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	task2 := &schema.TaskFile{
		ID:        "test-blocking-2",
		Title:     "Blocker 2",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	task3 := &schema.TaskFile{
		ID:        "test-blocking-3",
		Title:     "Blocked Task",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}
	if err := db.UpsertTask(task3); err != nil {
		t.Fatalf("UpsertTask(task3) failed: %v", err)
	}

	// Create dependencies
	dep1 := &schema.DepFile{
		From:      "test-blocking-1",
		To:        "test-blocking-2",
		Type:      "blocks",
		CreatedAt: now,
	}
	dep2 := &schema.DepFile{
		From:      "test-blocking-2",
		To:        "test-blocking-3",
		Type:      "blocks",
		CreatedAt: now,
	}

	if err := db.UpsertDep(dep1); err != nil {
		t.Fatalf("UpsertDep(dep1) failed: %v", err)
	}
	if err := db.UpsertDep(dep2); err != nil {
		t.Fatalf("UpsertDep(dep2) failed: %v", err)
	}

	// Get blocking tasks for task3 (should include both task1 and task2 transitively)
	blocking, err := db.GetBlockingTasks("test-blocking-3")
	if err != nil {
		t.Fatalf("GetBlockingTasks() failed: %v", err)
	}

	if len(blocking) != 2 {
		t.Fatalf("expected 2 blocking tasks, got %d", len(blocking))
	}

	// Verify both blockers are present
	found := make(map[string]bool)
	for _, task := range blocking {
		found[task.ID] = true
	}

	if !found["test-blocking-1"] {
		t.Error("expected test-blocking-1 in blocking tasks")
	}
	if !found["test-blocking-2"] {
		t.Error("expected test-blocking-2 in blocking tasks")
	}
}

// TestGetBlockingTasks_ClosedTasksExcluded tests that closed tasks don't appear in blocking list
func TestGetBlockingTasks_ClosedTasksExcluded(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Create task1 (closed) and task2 (open) blocking task3
	task1 := &schema.TaskFile{
		ID:        "test-closed-blocker",
		Title:     "Closed Blocker",
		Type:      "task",
		Status:    "closed",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	task2 := &schema.TaskFile{
		ID:        "test-open-blocker",
		Title:     "Open Blocker",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	task3 := &schema.TaskFile{
		ID:        "test-blocked",
		Title:     "Blocked Task",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}
	if err := db.UpsertTask(task3); err != nil {
		t.Fatalf("UpsertTask(task3) failed: %v", err)
	}

	// Create dependencies
	dep1 := &schema.DepFile{
		From:      "test-closed-blocker",
		To:        "test-blocked",
		Type:      "blocks",
		CreatedAt: now,
	}
	dep2 := &schema.DepFile{
		From:      "test-open-blocker",
		To:        "test-blocked",
		Type:      "blocks",
		CreatedAt: now,
	}

	if err := db.UpsertDep(dep1); err != nil {
		t.Fatalf("UpsertDep(dep1) failed: %v", err)
	}
	if err := db.UpsertDep(dep2); err != nil {
		t.Fatalf("UpsertDep(dep2) failed: %v", err)
	}

	// Get blocking tasks - should only include open blocker
	blocking, err := db.GetBlockingTasks("test-blocked")
	if err != nil {
		t.Fatalf("GetBlockingTasks() failed: %v", err)
	}

	if len(blocking) != 1 {
		t.Fatalf("expected 1 blocking task, got %d", len(blocking))
	}

	if blocking[0].ID != "test-open-blocker" {
		t.Errorf("expected test-open-blocker, got %s", blocking[0].ID)
	}
}

// TestGetTaskByID tests single task retrieval
func TestGetTaskByID(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	dueAt := now.Add(24 * time.Hour)
	deferUntil := now.Add(1 * time.Hour)

	task := &schema.TaskFile{
		ID:            "test-get-by-id",
		Title:         "Test Get By ID",
		Description:   "Test description",
		Type:          "task",
		Status:        "open",
		Priority:      2,
		AssignedAgent: "agent-123",
		Tags:          []string{"test", "unit"},
		CreatedAt:     now,
		UpdatedAt:     now,
		DueAt:         &dueAt,
		DeferUntil:    &deferUntil,
	}

	if err := db.UpsertTask(task); err != nil {
		t.Fatalf("UpsertTask() failed: %v", err)
	}

	// Retrieve the task
	retrieved, err := db.GetTaskByID("test-get-by-id")
	if err != nil {
		t.Fatalf("GetTaskByID() failed: %v", err)
	}

	// Verify all fields
	if retrieved.ID != task.ID {
		t.Errorf("ID = %q, want %q", retrieved.ID, task.ID)
	}
	if retrieved.Title != task.Title {
		t.Errorf("Title = %q, want %q", retrieved.Title, task.Title)
	}
	if retrieved.Description != task.Description {
		t.Errorf("Description = %q, want %q", retrieved.Description, task.Description)
	}
	if retrieved.AssignedAgent != task.AssignedAgent {
		t.Errorf("AssignedAgent = %q, want %q", retrieved.AssignedAgent, task.AssignedAgent)
	}
	if len(retrieved.Tags) != len(task.Tags) {
		t.Errorf("Tags length = %d, want %d", len(retrieved.Tags), len(task.Tags))
	}
	if retrieved.DueAt == nil || !retrieved.DueAt.Equal(*task.DueAt) {
		t.Errorf("DueAt = %v, want %v", retrieved.DueAt, task.DueAt)
	}
	if retrieved.DeferUntil == nil || !retrieved.DeferUntil.Equal(*task.DeferUntil) {
		t.Errorf("DeferUntil = %v, want %v", retrieved.DeferUntil, task.DeferUntil)
	}
}

// TestGetTaskByID_NotFound tests error handling for missing task
func TestGetTaskByID_NotFound(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	// Try to get non-existent task
	_, err = db.GetTaskByID("non-existent")
	if err == nil {
		t.Error("expected error for non-existent task, got nil")
	}
}

// TestListTasks tests filtering and pagination
func TestListTasks(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Create diverse test tasks
	tasks := []*schema.TaskFile{
		{
			ID:            "list-1",
			Title:         "Bug Task",
			Type:          "bug",
			Status:        "open",
			Priority:      0,
			AssignedAgent: "agent-1",
			Tags:          []string{"urgent", "backend"},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "list-2",
			Title:         "Feature Task",
			Type:          "feature",
			Status:        "in_progress",
			Priority:      1,
			AssignedAgent: "agent-2",
			Tags:          []string{"frontend"},
			CreatedAt:     now.Add(1 * time.Minute),
			UpdatedAt:     now.Add(1 * time.Minute),
		},
		{
			ID:        "list-3",
			Title:     "Task Task",
			Type:      "task",
			Status:    "open",
			Priority:  2,
			Tags:      []string{"backend", "database"},
			CreatedAt: now.Add(2 * time.Minute),
			UpdatedAt: now.Add(2 * time.Minute),
		},
	}

	for _, task := range tasks {
		if err := db.UpsertTask(task); err != nil {
			t.Fatalf("UpsertTask(%s) failed: %v", task.ID, err)
		}
	}

	// Test filter by status (Priority=-1 means "all priorities")
	t.Run("FilterByStatus", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{Status: "open", Priority: -1})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 open tasks, got %d", len(result))
		}
	})

	// Test filter by type
	t.Run("FilterByType", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{Type: "bug", Priority: -1})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 bug task, got %d", len(result))
		}
	})

	// Test filter by priority
	t.Run("FilterByPriority", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{Priority: 0})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 P0 task, got %d", len(result))
		}
	})

	// Test filter by assigned agent
	t.Run("FilterByAgent", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{AssignedAgent: "agent-1", Priority: -1})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 task for agent-1, got %d", len(result))
		}
	})

	// Test filter by tag
	t.Run("FilterByTag", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{Tag: "backend", Priority: -1})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 backend tasks, got %d", len(result))
		}
	})

	// Test limit
	t.Run("Limit", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{Limit: 2, Priority: -1})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 tasks with limit, got %d", len(result))
		}
	})

	// Test offset
	t.Run("Offset", func(t *testing.T) {
		result, err := db.ListTasks(ListTasksFilter{Limit: 2, Offset: 1, Priority: -1})
		if err != nil {
			t.Fatalf("ListTasks() failed: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 tasks with offset, got %d", len(result))
		}
	})
}

// TestGetDepsForTask tests dependency retrieval
func TestGetDepsForTask(t *testing.T) {
	path := testDBPath(t)
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Create tasks
	task1 := &schema.TaskFile{
		ID:        "deps-1",
		Title:     "Task 1",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	task2 := &schema.TaskFile{
		ID:        "deps-2",
		Title:     "Task 2",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	task3 := &schema.TaskFile{
		ID:        "deps-3",
		Title:     "Task 3",
		Type:      "task",
		Status:    "open",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := db.UpsertTask(task1); err != nil {
		t.Fatalf("UpsertTask(task1) failed: %v", err)
	}
	if err := db.UpsertTask(task2); err != nil {
		t.Fatalf("UpsertTask(task2) failed: %v", err)
	}
	if err := db.UpsertTask(task3); err != nil {
		t.Fatalf("UpsertTask(task3) failed: %v", err)
	}

	// Create dependencies:
	// - task1 blocks task2
	// - task2 blocks task3
	// - task1 relates-to task3
	deps := []*schema.DepFile{
		{From: "deps-1", To: "deps-2", Type: "blocks", CreatedAt: now},
		{From: "deps-2", To: "deps-3", Type: "blocks", CreatedAt: now},
		{From: "deps-1", To: "deps-3", Type: "related", CreatedAt: now},
	}

	for _, dep := range deps {
		if err := db.UpsertDep(dep); err != nil {
			t.Fatalf("UpsertDep(%s -> %s) failed: %v", dep.From, dep.To, err)
		}
	}

	// Get all deps for task2 (should have 2: one incoming, one outgoing)
	result, err := db.GetDepsForTask("deps-2")
	if err != nil {
		t.Fatalf("GetDepsForTask() failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 deps for task2, got %d", len(result))
	}

	// Verify we have both incoming and outgoing deps
	hasIncoming := false
	hasOutgoing := false
	for _, dep := range result {
		if dep.From == "deps-1" && dep.To == "deps-2" {
			hasIncoming = true
		}
		if dep.From == "deps-2" && dep.To == "deps-3" {
			hasOutgoing = true
		}
	}

	if !hasIncoming {
		t.Error("expected incoming dependency (deps-1 -> deps-2)")
	}
	if !hasOutgoing {
		t.Error("expected outgoing dependency (deps-2 -> deps-3)")
	}
}

// BenchmarkGetBlockingTasks benchmarks the transitive closure query
func BenchmarkGetBlockingTasks(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.db")
	db, err := Open(path)
	if err != nil {
		b.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		b.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Create a chain of 10 blocking dependencies
	for i := 0; i < 10; i++ {
		task := &schema.TaskFile{
			ID:        fmt.Sprintf("bench-block-%d", i),
			Title:     fmt.Sprintf("Benchmark Task %d", i),
			Type:      "task",
			Status:    "open",
			Priority:  1,
			Tags:      []string{},
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := db.UpsertTask(task); err != nil {
			b.Fatalf("UpsertTask() failed: %v", err)
		}

		if i > 0 {
			dep := &schema.DepFile{
				From:      fmt.Sprintf("bench-block-%d", i-1),
				To:        fmt.Sprintf("bench-block-%d", i),
				Type:      "blocks",
				CreatedAt: now,
			}
			if err := db.UpsertDep(dep); err != nil {
				b.Fatalf("UpsertDep() failed: %v", err)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.GetBlockingTasks("bench-block-9")
		if err != nil {
			b.Fatalf("GetBlockingTasks() failed: %v", err)
		}
	}
}

// BenchmarkListTasks benchmarks filtered task listing
func BenchmarkListTasks(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.db")
	db, err := Open(path)
	if err != nil {
		b.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		b.Fatalf("InitSchema() failed: %v", err)
	}

	now := time.Now().UTC()

	// Create 100 test tasks
	for i := 0; i < 100; i++ {
		task := &schema.TaskFile{
			ID:        fmt.Sprintf("bench-list-%d", i),
			Title:     fmt.Sprintf("Benchmark Task %d", i),
			Type:      "task",
			Status:    "open",
			Priority:  i % 5,
			Tags:      []string{"benchmark", fmt.Sprintf("tag-%d", i%10)},
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := db.UpsertTask(task); err != nil {
			b.Fatalf("UpsertTask() failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.ListTasks(ListTasksFilter{Status: "open", Limit: 20})
		if err != nil {
			b.Fatalf("ListTasks() failed: %v", err)
		}
	}
}
