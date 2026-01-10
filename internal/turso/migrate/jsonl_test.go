package migrate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

func TestFromJSONL(t *testing.T) {
	// Create temp JSONL file
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")

	// Write test data
	issue1 := types.Issue{
		ID:          "bd-123",
		Title:       "Test Issue",
		Description: "Test description",
		Status:      types.StatusOpen,
		Priority:    1,
		IssueType:   types.TypeTask,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	issue2 := types.Issue{
		ID:          "bd-456",
		Title:       "Another Issue",
		Description: "Another description",
		Status:      types.StatusClosed,
		Priority:    2,
		IssueType:   types.TypeBug,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		ClosedAt:    ptrTime(time.Now()),
	}

	file, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(issue1); err != nil {
		t.Fatalf("failed to encode issue1: %v", err)
	}
	if err := encoder.Encode(issue2); err != nil {
		t.Fatalf("failed to encode issue2: %v", err)
	}
	file.Close()

	// Test FromJSONL
	issues, err := FromJSONL(jsonlPath)
	if err != nil {
		t.Fatalf("FromJSONL failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}

	if issues[0].ID != "bd-123" {
		t.Errorf("expected first issue ID bd-123, got %s", issues[0].ID)
	}

	if issues[1].ID != "bd-456" {
		t.Errorf("expected second issue ID bd-456, got %s", issues[1].ID)
	}
}

func TestFromJSONL_InvalidFile(t *testing.T) {
	_, err := FromJSONL("/nonexistent/path.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFromJSONL_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "invalid.jsonl")

	// Write invalid JSON
	if err := os.WriteFile(jsonlPath, []byte("{invalid json}\n"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := FromJSONL(jsonlPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestIssueToTaskFile(t *testing.T) {
	now := time.Now()
	dueDate := now.Add(24 * time.Hour)

	issue := &types.Issue{
		ID:          "bd-789",
		Title:       "Test Task",
		Description: "Task description",
		Status:      types.StatusInProgress,
		Priority:    1,
		IssueType:   types.TypeTask,
		Assignee:    "agent-42",
		Labels:      []string{"backend", "api"},
		CreatedAt:   now,
		UpdatedAt:   now,
		DueAt:       &dueDate,
	}

	task := IssueToTaskFile(issue)

	if task.ID != "bd-789" {
		t.Errorf("expected ID bd-789, got %s", task.ID)
	}

	if task.AssignedAgent != "agent-42" {
		t.Errorf("expected assigned_agent agent-42, got %s", task.AssignedAgent)
	}

	if len(task.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(task.Tags))
	}

	if task.Tags[0] != "backend" {
		t.Errorf("expected first tag backend, got %s", task.Tags[0])
	}

	if task.DueAt == nil {
		t.Error("expected due_at to be set")
	}
}

func TestDependencyToDepFile(t *testing.T) {
	now := time.Now()

	dep := &types.Dependency{
		IssueID:     "bd-123",
		DependsOnID: "bd-456",
		Type:        types.DepBlocks,
		CreatedAt:   now,
	}

	depFile := DependencyToDepFile(dep)

	if depFile.From != "bd-123" {
		t.Errorf("expected from bd-123, got %s", depFile.From)
	}

	if depFile.To != "bd-456" {
		t.Errorf("expected to bd-456, got %s", depFile.To)
	}

	if depFile.Type != "blocks" {
		t.Errorf("expected type blocks, got %s", depFile.Type)
	}
}

func TestDepFileName(t *testing.T) {
	filename := DepFileName("bd-123", "blocks", "bd-456")
	expected := "bd-123--blocks--bd-456.json"

	if filename != expected {
		t.Errorf("expected %s, got %s", expected, filename)
	}
}

func TestWriteTaskFile(t *testing.T) {
	tmpDir := t.TempDir()

	task := &TaskFile{
		ID:          "bd-test",
		Title:       "Test Task",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Description: "Test description",
	}

	err := WriteTaskFile(task, tmpDir)
	if err != nil {
		t.Fatalf("WriteTaskFile failed: %v", err)
	}

	// Verify file was created
	taskPath := filepath.Join(tmpDir, "tasks", "bd-test.json")
	if _, err := os.Stat(taskPath); err != nil {
		t.Errorf("task file was not created: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("failed to read task file: %v", err)
	}

	var readTask TaskFile
	if err := json.Unmarshal(data, &readTask); err != nil {
		t.Fatalf("failed to parse task file: %v", err)
	}

	if readTask.ID != "bd-test" {
		t.Errorf("expected ID bd-test, got %s", readTask.ID)
	}
}

func TestWriteDepFile(t *testing.T) {
	tmpDir := t.TempDir()

	dep := &DepFile{
		From:      "bd-123",
		To:        "bd-456",
		Type:      "blocks",
		CreatedAt: time.Now(),
	}

	err := WriteDepFile(dep, tmpDir)
	if err != nil {
		t.Fatalf("WriteDepFile failed: %v", err)
	}

	// Verify file was created
	depPath := filepath.Join(tmpDir, "deps", "bd-123--blocks--bd-456.json")
	if _, err := os.Stat(depPath); err != nil {
		t.Errorf("dep file was not created: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(depPath)
	if err != nil {
		t.Fatalf("failed to read dep file: %v", err)
	}

	var readDep DepFile
	if err := json.Unmarshal(data, &readDep); err != nil {
		t.Fatalf("failed to parse dep file: %v", err)
	}

	if readDep.From != "bd-123" {
		t.Errorf("expected from bd-123, got %s", readDep.From)
	}
}

func TestMigrate_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")
	outputDir := filepath.Join(tmpDir, "output")

	// Create test JSONL
	issue := types.Issue{
		ID:          "bd-dry",
		Title:       "Dry Run Test",
		Description: "Test description",
		Status:      types.StatusOpen,
		Priority:    1,
		IssueType:   types.TypeTask,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Dependencies: []*types.Dependency{
			{
				IssueID:     "bd-dry",
				DependsOnID: "bd-other",
				Type:        types.DepBlocks,
				CreatedAt:   time.Now(),
			},
		},
	}

	file, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(issue); err != nil {
		t.Fatalf("failed to encode issue: %v", err)
	}
	file.Close()

	// Run migration in dry-run mode
	opts := MigrateOptions{
		FromJSONL: jsonlPath,
		ToFiles:   outputDir,
		DryRun:    true,
		Backup:    false,
	}

	result, err := Migrate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify statistics
	if result.TasksConverted != 1 {
		t.Errorf("expected 1 task converted, got %d", result.TasksConverted)
	}

	if result.DepsCreated != 1 {
		t.Errorf("expected 1 dep created, got %d", result.DepsCreated)
	}

	if result.FilesWritten != 0 {
		t.Errorf("expected 0 files written in dry-run, got %d", result.FilesWritten)
	}

	// Verify no files were created
	if _, err := os.Stat(filepath.Join(outputDir, "tasks")); !os.IsNotExist(err) {
		t.Error("tasks directory should not exist in dry-run mode")
	}
}

func TestMigrate_WithBackup(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")
	outputDir := filepath.Join(tmpDir, "output")

	// Create test JSONL
	issue := types.Issue{
		ID:          "bd-backup",
		Title:       "Backup Test",
		Description: "Test description",
		Status:      types.StatusOpen,
		Priority:    1,
		IssueType:   types.TypeTask,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	file, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(issue); err != nil {
		t.Fatalf("failed to encode issue: %v", err)
	}
	file.Close()

	// Run migration with backup
	opts := MigrateOptions{
		FromJSONL: jsonlPath,
		ToFiles:   outputDir,
		DryRun:    false,
		Backup:    true,
	}

	result, err := Migrate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify backup was created
	if result.BackupCreated == "" {
		t.Error("backup should have been created")
	}

	if _, err := os.Stat(result.BackupCreated); err != nil {
		t.Errorf("backup file does not exist: %v", err)
	}
}

func TestMigrate_SkipTombstones(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "test.jsonl")
	outputDir := filepath.Join(tmpDir, "output")

	// Create test JSONL with a tombstone
	deletedAt := time.Now()
	issue := types.Issue{
		ID:           "bd-tomb",
		Title:        "Tombstone Test",
		Description:  "Test description",
		Status:       types.StatusTombstone,
		Priority:     1,
		IssueType:    types.TypeTask,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		DeletedAt:    &deletedAt,
		DeletedBy:    "test",
		DeleteReason: "testing",
	}

	file, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(issue); err != nil {
		t.Fatalf("failed to encode issue: %v", err)
	}
	file.Close()

	// Run migration
	opts := MigrateOptions{
		FromJSONL: jsonlPath,
		ToFiles:   outputDir,
		DryRun:    false,
		Backup:    false,
	}

	result, err := Migrate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify tombstone was skipped
	if result.TasksConverted != 0 {
		t.Errorf("expected 0 tasks converted (tombstone skipped), got %d", result.TasksConverted)
	}
}

func TestCleanupMigration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		t.Fatalf("failed to create deps dir: %v", err)
	}

	// Create dummy files
	if err := os.WriteFile(filepath.Join(tasksDir, "test.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Run cleanup
	if err := CleanupMigration(tmpDir); err != nil {
		t.Fatalf("CleanupMigration failed: %v", err)
	}

	// Verify directories were removed
	if _, err := os.Stat(tasksDir); !os.IsNotExist(err) {
		t.Error("tasks directory should have been removed")
	}
	if _, err := os.Stat(depsDir); !os.IsNotExist(err) {
		t.Error("deps directory should have been removed")
	}
}

// Helper function
func ptrTime(t time.Time) *time.Time {
	return &t
}
