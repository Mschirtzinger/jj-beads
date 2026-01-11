package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// setupTestDB creates a temporary database for testing.
func setupTestDB(t *testing.T) (*db.DB, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	database, err := db.Open("file:" + dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := database.InitSchema(); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	return database, tmpDir
}

// createTestTask creates a test task file.
func createTestTask(t *testing.T, dir, id, title string) string {
	t.Helper()

	task := &schema.TaskFile{
		ID:          id,
		Title:       title,
		Description: "Test task",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		Tags:        []string{"test"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := schema.WriteTaskFile(dir, task); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}

	return filepath.Join(dir, task.Filename())
}

// createTestDep creates a test dependency file.
func createTestDep(t *testing.T, dir, from, to, typ string) string {
	t.Helper()

	dep := &schema.DepFile{
		From:      from,
		To:        to,
		Type:      typ,
		CreatedAt: time.Now(),
	}

	if err := schema.WriteDepFile(dir, dep); err != nil {
		t.Fatalf("failed to create test dep: %v", err)
	}

	return filepath.Join(dir, dep.ToFileName())
}

func TestSyncTask(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer database.Close()

	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	syncer := New(database, log.New(os.Stderr, "[test] ", 0))
	taskPath := createTestTask(t, tasksDir, "bd-test", "Test Task")

	if err := syncer.SyncTask(taskPath); err != nil {
		t.Fatalf("SyncTask failed: %v", err)
	}

	count, err := database.GetTaskCount()
	if err != nil {
		t.Fatalf("failed to get task count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 task, got %d", count)
	}
}

func TestSyncDep(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer database.Close()

	// Need to create tasks first (foreign key constraint)
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("failed to create deps dir: %v", err)
	}

	syncer := New(database, log.New(os.Stderr, "[test] ", 0))

	// Create tasks first
	task1Path := createTestTask(t, tasksDir, "bd-1", "Task 1")
	task2Path := createTestTask(t, tasksDir, "bd-2", "Task 2")
	if err := syncer.SyncTask(task1Path); err != nil {
		t.Fatalf("SyncTask 1 failed: %v", err)
	}
	if err := syncer.SyncTask(task2Path); err != nil {
		t.Fatalf("SyncTask 2 failed: %v", err)
	}

	// Now create dependency
	createTestDep(t, depsDir, "bd-1", "bd-2", "blocks")
	depPath := filepath.Join(depsDir, "bd-1--blocks--bd-2.json")

	if err := syncer.SyncDep(depPath); err != nil {
		t.Fatalf("SyncDep failed: %v", err)
	}

	count, err := database.GetDepCount()
	if err != nil {
		t.Fatalf("failed to get dep count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 dep, got %d", count)
	}
}

func TestDeleteTask(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer database.Close()

	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	syncer := New(database, log.New(os.Stderr, "[test] ", 0))
	taskPath := createTestTask(t, tasksDir, "bd-test", "Test Task")
	if err := syncer.SyncTask(taskPath); err != nil {
		t.Fatalf("SyncTask failed: %v", err)
	}

	if err := syncer.DeleteTask("bd-test"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	count, err := database.GetTaskCount()
	if err != nil {
		t.Fatalf("failed to get task count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", count)
	}
}

func TestFullSync(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer database.Close()

	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("failed to create deps dir: %v", err)
	}

	createTestTask(t, tasksDir, "bd-1", "Task 1")
	createTestTask(t, tasksDir, "bd-2", "Task 2")
	createTestTask(t, tasksDir, "bd-3", "Task 3")
	createTestDep(t, depsDir, "bd-1", "bd-2", "blocks")
	createTestDep(t, depsDir, "bd-2", "bd-3", "related")

	syncer := New(database, log.New(os.Stderr, "[test] ", 0))
	if err := syncer.FullSync(tasksDir, depsDir); err != nil {
		t.Fatalf("FullSync failed: %v", err)
	}

	taskCount, err := database.GetTaskCount()
	if err != nil {
		t.Fatalf("failed to get task count: %v", err)
	}
	if taskCount != 3 {
		t.Errorf("expected 3 tasks, got %d", taskCount)
	}

	depCount, err := database.GetDepCount()
	if err != nil {
		t.Fatalf("failed to get dep count: %v", err)
	}
	if depCount != 2 {
		t.Errorf("expected 2 deps, got %d", depCount)
	}
}

func TestFullSync_ConcurrentWrites(t *testing.T) {
	database, tmpDir := setupTestDB(t)
	defer database.Close()

	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	for i := 1; i <= 10; i++ {
		createTestTask(t, tasksDir, fmt.Sprintf("bd-%d", i), fmt.Sprintf("Task %d", i))
	}

	syncer1 := New(database, log.New(os.Stderr, "[test1] ", 0))
	syncer2 := New(database, log.New(os.Stderr, "[test2] ", 0))

	errChan := make(chan error, 2)
	go func() {
		errChan <- syncer1.FullSync(tasksDir, "")
	}()
	go func() {
		errChan <- syncer2.FullSync(tasksDir, "")
	}()

	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent sync %d failed: %v", i+1, err)
		}
	}

	count, err := database.GetTaskCount()
	if err != nil {
		t.Fatalf("failed to get task count: %v", err)
	}
	if count != 10 {
		t.Errorf("expected 10 tasks (upserted, not duplicated), got %d", count)
	}
}
