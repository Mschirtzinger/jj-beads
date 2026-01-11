package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

func TestTaskFile_Validate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		task    TaskFile
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid task",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Implement feature X",
				Type:      "task",
				Status:    "in_progress",
				Priority:  1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: false,
		},
		{
			name: "missing id",
			task: TaskFile{
				Title:     "Test",
				Type:      "task",
				Status:    "open",
				Priority:  2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "id is required",
		},
		{
			name: "missing title",
			task: TaskFile{
				ID:        "bd-xyz",
				Type:      "task",
				Status:    "open",
				Priority:  2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "title too long",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     string(make([]byte, 501)), // 501 characters
				Type:      "task",
				Status:    "open",
				Priority:  2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "title must be 500 characters or less",
		},
		{
			name: "invalid priority - negative",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Test",
				Type:      "task",
				Status:    "open",
				Priority:  -1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "priority must be between 0 and 4",
		},
		{
			name: "invalid priority - too high",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Test",
				Type:      "task",
				Status:    "open",
				Priority:  5,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "priority must be between 0 and 4",
		},
		{
			name: "missing type",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Test",
				Status:    "open",
				Priority:  2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name: "missing status",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Test",
				Type:      "task",
				Priority:  2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "status is required",
		},
		{
			name: "missing created_at",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Test",
				Type:      "task",
				Status:    "open",
				Priority:  2,
				UpdatedAt: now,
			},
			wantErr: true,
			errMsg:  "created_at is required",
		},
		{
			name: "missing updated_at",
			task: TaskFile{
				ID:        "bd-xyz",
				Title:     "Test",
				Type:      "task",
				Status:    "open",
				Priority:  2,
				CreatedAt: now,
			},
			wantErr: true,
			errMsg:  "updated_at is required",
		},
		{
			name: "valid with optional fields",
			task: TaskFile{
				ID:            "bd-abc",
				Title:         "Complete task",
				Description:   "Detailed description",
				Type:          "bug",
				Status:        "closed",
				Priority:      0,
				AssignedAgent: "agent-47",
				Tags:          []string{"backend", "api"},
				CreatedAt:     now,
				UpdatedAt:     now,
				DueAt:         &now,
				DeferUntil:    &now,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					// Allow partial match for dynamic error messages
					if len(tt.errMsg) > 0 && len(err.Error()) >= len(tt.errMsg) {
						if err.Error()[:len(tt.errMsg)] != tt.errMsg {
							t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTaskFile_Filename(t *testing.T) {
	task := TaskFile{ID: "bd-xyz"}
	want := "bd-xyz.json"
	if got := task.Filename(); got != want {
		t.Errorf("Filename() = %v, want %v", got, want)
	}
}

func TestTaskFile_SetDefaults(t *testing.T) {
	task := TaskFile{
		ID:    "bd-test",
		Title: "Test task",
	}

	task.SetDefaults()

	if task.Status != "open" {
		t.Errorf("SetDefaults() status = %v, want 'open'", task.Status)
	}
	if task.Type != "task" {
		t.Errorf("SetDefaults() type = %v, want 'task'", task.Type)
	}
	if task.Tags == nil {
		t.Errorf("SetDefaults() tags is nil, want empty slice")
	}
	if task.CreatedAt.IsZero() {
		t.Errorf("SetDefaults() created_at is zero, want current time")
	}
	if task.UpdatedAt.IsZero() {
		t.Errorf("SetDefaults() updated_at is zero, want current time")
	}
}

func TestTaskFile_UpdateTimestamp(t *testing.T) {
	now := time.Now()
	task := TaskFile{
		ID:        "bd-test",
		Title:     "Test",
		UpdatedAt: now.Add(-1 * time.Hour), // 1 hour ago
	}

	before := task.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamp
	task.UpdateTimestamp()

	if !task.UpdatedAt.After(before) {
		t.Errorf("UpdateTimestamp() did not update timestamp: before=%v, after=%v", before, task.UpdatedAt)
	}
}

func TestTaskFile_ToIssue(t *testing.T) {
	now := time.Now()
	dueAt := now.Add(24 * time.Hour)
	deferUntil := now.Add(1 * time.Hour)

	task := TaskFile{
		ID:            "bd-xyz",
		Title:         "Test task",
		Description:   "Test description",
		Type:          "bug",
		Status:        "in_progress",
		Priority:      1,
		AssignedAgent: "agent-47",
		Tags:          []string{"backend", "api"},
		CreatedAt:     now,
		UpdatedAt:     now,
		DueAt:         &dueAt,
		DeferUntil:    &deferUntil,
	}

	issue := task.ToIssue()

	if issue.ID != task.ID {
		t.Errorf("ToIssue() ID = %v, want %v", issue.ID, task.ID)
	}
	if issue.Title != task.Title {
		t.Errorf("ToIssue() Title = %v, want %v", issue.Title, task.Title)
	}
	if issue.Description != task.Description {
		t.Errorf("ToIssue() Description = %v, want %v", issue.Description, task.Description)
	}
	if string(issue.IssueType) != task.Type {
		t.Errorf("ToIssue() IssueType = %v, want %v", issue.IssueType, task.Type)
	}
	if string(issue.Status) != task.Status {
		t.Errorf("ToIssue() Status = %v, want %v", issue.Status, task.Status)
	}
	if issue.Priority != task.Priority {
		t.Errorf("ToIssue() Priority = %v, want %v", issue.Priority, task.Priority)
	}
	if issue.Assignee != task.AssignedAgent {
		t.Errorf("ToIssue() Assignee = %v, want %v", issue.Assignee, task.AssignedAgent)
	}
	if !issue.CreatedAt.Equal(task.CreatedAt) {
		t.Errorf("ToIssue() CreatedAt = %v, want %v", issue.CreatedAt, task.CreatedAt)
	}
	if !issue.UpdatedAt.Equal(task.UpdatedAt) {
		t.Errorf("ToIssue() UpdatedAt = %v, want %v", issue.UpdatedAt, task.UpdatedAt)
	}
	if issue.DueAt == nil || !issue.DueAt.Equal(*task.DueAt) {
		t.Errorf("ToIssue() DueAt = %v, want %v", issue.DueAt, task.DueAt)
	}
	if issue.DeferUntil == nil || !issue.DeferUntil.Equal(*task.DeferUntil) {
		t.Errorf("ToIssue() DeferUntil = %v, want %v", issue.DeferUntil, task.DeferUntil)
	}
	if len(issue.Labels) != len(task.Tags) {
		t.Errorf("ToIssue() Labels length = %v, want %v", len(issue.Labels), len(task.Tags))
	}
}

func TestFromIssue(t *testing.T) {
	now := time.Now()
	dueAt := now.Add(24 * time.Hour)
	deferUntil := now.Add(1 * time.Hour)

	issue := &types.Issue{
		ID:          "bd-xyz",
		Title:       "Test issue",
		Description: "Test description",
		IssueType:   types.TypeBug,
		Status:      types.StatusInProgress,
		Priority:    1,
		Assignee:    "agent-47",
		Labels:      []string{"backend", "api"},
		CreatedAt:   now,
		UpdatedAt:   now,
		DueAt:       &dueAt,
		DeferUntil:  &deferUntil,
	}

	task := FromIssue(issue)

	if task.ID != issue.ID {
		t.Errorf("FromIssue() ID = %v, want %v", task.ID, issue.ID)
	}
	if task.Title != issue.Title {
		t.Errorf("FromIssue() Title = %v, want %v", task.Title, issue.Title)
	}
	if task.Description != issue.Description {
		t.Errorf("FromIssue() Description = %v, want %v", task.Description, issue.Description)
	}
	if task.Type != string(issue.IssueType) {
		t.Errorf("FromIssue() Type = %v, want %v", task.Type, issue.IssueType)
	}
	if task.Status != string(issue.Status) {
		t.Errorf("FromIssue() Status = %v, want %v", task.Status, issue.Status)
	}
	if task.Priority != issue.Priority {
		t.Errorf("FromIssue() Priority = %v, want %v", task.Priority, issue.Priority)
	}
	if task.AssignedAgent != issue.Assignee {
		t.Errorf("FromIssue() AssignedAgent = %v, want %v", task.AssignedAgent, issue.Assignee)
	}
	if !task.CreatedAt.Equal(issue.CreatedAt) {
		t.Errorf("FromIssue() CreatedAt = %v, want %v", task.CreatedAt, issue.CreatedAt)
	}
	if !task.UpdatedAt.Equal(issue.UpdatedAt) {
		t.Errorf("FromIssue() UpdatedAt = %v, want %v", task.UpdatedAt, issue.UpdatedAt)
	}
	if task.DueAt == nil || !task.DueAt.Equal(*issue.DueAt) {
		t.Errorf("FromIssue() DueAt = %v, want %v", task.DueAt, issue.DueAt)
	}
	if task.DeferUntil == nil || !task.DeferUntil.Equal(*issue.DeferUntil) {
		t.Errorf("FromIssue() DeferUntil = %v, want %v", task.DeferUntil, issue.DeferUntil)
	}
	if len(task.Tags) != len(issue.Labels) {
		t.Errorf("FromIssue() Tags length = %v, want %v", len(task.Tags), len(issue.Labels))
	}
}

func TestWriteTaskFile(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")

	now := time.Now()
	task := &TaskFile{
		ID:            "bd-test",
		Title:         "Test task",
		Description:   "Test description",
		Type:          "task",
		Status:        "open",
		Priority:      2,
		AssignedAgent: "agent-1",
		Tags:          []string{"test"},
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	err := WriteTaskFile(tasksDir, task)
	if err != nil {
		t.Fatalf("WriteTaskFile() error = %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tasksDir, "bd-test.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("WriteTaskFile() did not create file at %s", expectedPath)
	}

	// Verify content is valid JSON
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	var parsed TaskFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("WriteTaskFile() created invalid JSON: %v", err)
	}

	// Verify content matches original
	if parsed.ID != task.ID {
		t.Errorf("Written file ID = %v, want %v", parsed.ID, task.ID)
	}
	if parsed.Title != task.Title {
		t.Errorf("Written file Title = %v, want %v", parsed.Title, task.Title)
	}
}

func TestReadTaskFile(t *testing.T) {
	// Create temp directory and write test file
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	now := time.Now()
	expected := &TaskFile{
		ID:          "bd-read",
		Title:       "Read test",
		Description: "Testing read",
		Type:        "task",
		Status:      "open",
		Priority:    2,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Write file first
	if err := WriteTaskFile(tasksDir, expected); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Now read it back
	path := filepath.Join(tasksDir, expected.Filename())
	task, err := ReadTaskFile(path)
	if err != nil {
		t.Fatalf("ReadTaskFile() error = %v", err)
	}

	if task.ID != expected.ID {
		t.Errorf("ReadTaskFile() ID = %v, want %v", task.ID, expected.ID)
	}
	if task.Title != expected.Title {
		t.Errorf("ReadTaskFile() Title = %v, want %v", task.Title, expected.Title)
	}
}

func TestReadTaskFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	// Write invalid JSON
	invalidPath := filepath.Join(tasksDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	_, err := ReadTaskFile(invalidPath)
	if err == nil {
		t.Errorf("ReadTaskFile() expected error for invalid JSON, got nil")
	}
}

func TestReadAllTaskFiles(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	now := time.Now()

	// Write multiple task files
	tasks := []*TaskFile{
		{
			ID:        "bd-1",
			Title:     "Task 1",
			Type:      "task",
			Status:    "open",
			Priority:  1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "bd-2",
			Title:     "Task 2",
			Type:      "bug",
			Status:    "in_progress",
			Priority:  0,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "bd-3",
			Title:     "Task 3",
			Type:      "feature",
			Status:    "closed",
			Priority:  2,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, task := range tasks {
		if err := WriteTaskFile(tasksDir, task); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	// Read all tasks
	readTasks, err := ReadAllTaskFiles(tasksDir)
	if err != nil {
		t.Fatalf("ReadAllTaskFiles() error = %v", err)
	}

	if len(readTasks) != len(tasks) {
		t.Errorf("ReadAllTaskFiles() returned %d tasks, want %d", len(readTasks), len(tasks))
	}

	// Verify all task IDs are present
	idMap := make(map[string]bool)
	for _, task := range readTasks {
		idMap[task.ID] = true
	}

	for _, expected := range tasks {
		if !idMap[expected.ID] {
			t.Errorf("ReadAllTaskFiles() missing task %s", expected.ID)
		}
	}
}

func TestReadAllTaskFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "tasks")
	os.MkdirAll(tasksDir, 0755)

	tasks, err := ReadAllTaskFiles(tasksDir)
	if err != nil {
		t.Errorf("ReadAllTaskFiles() error = %v, want nil for empty directory", err)
	}
	if len(tasks) != 0 {
		t.Errorf("ReadAllTaskFiles() returned %d tasks, want 0 for empty directory", len(tasks))
	}
}

func TestReadAllTaskFiles_NonexistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, "nonexistent")

	tasks, err := ReadAllTaskFiles(tasksDir)
	if err != nil {
		t.Errorf("ReadAllTaskFiles() error = %v, want nil for nonexistent directory", err)
	}
	if len(tasks) != 0 {
		t.Errorf("ReadAllTaskFiles() returned %d tasks, want 0 for nonexistent directory", len(tasks))
	}
}

func TestJSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for JSON round-trip comparison
	dueAt := now.Add(24 * time.Hour)

	original := &TaskFile{
		ID:            "bd-roundtrip",
		Title:         "Roundtrip test",
		Description:   "Testing JSON round-trip",
		Type:          "task",
		Status:        "in_progress",
		Priority:      1,
		AssignedAgent: "agent-99",
		Tags:          []string{"test", "roundtrip"},
		CreatedAt:     now,
		UpdatedAt:     now,
		DueAt:         &dueAt,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var parsed TaskFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Compare
	if parsed.ID != original.ID {
		t.Errorf("Round-trip ID = %v, want %v", parsed.ID, original.ID)
	}
	if parsed.Title != original.Title {
		t.Errorf("Round-trip Title = %v, want %v", parsed.Title, original.Title)
	}
	if parsed.Description != original.Description {
		t.Errorf("Round-trip Description = %v, want %v", parsed.Description, original.Description)
	}
	if parsed.Type != original.Type {
		t.Errorf("Round-trip Type = %v, want %v", parsed.Type, original.Type)
	}
	if parsed.Status != original.Status {
		t.Errorf("Round-trip Status = %v, want %v", parsed.Status, original.Status)
	}
	if parsed.Priority != original.Priority {
		t.Errorf("Round-trip Priority = %v, want %v", parsed.Priority, original.Priority)
	}
	if parsed.AssignedAgent != original.AssignedAgent {
		t.Errorf("Round-trip AssignedAgent = %v, want %v", parsed.AssignedAgent, original.AssignedAgent)
	}
	if len(parsed.Tags) != len(original.Tags) {
		t.Errorf("Round-trip Tags length = %v, want %v", len(parsed.Tags), len(original.Tags))
	}
	if !parsed.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("Round-trip CreatedAt = %v, want %v", parsed.CreatedAt, original.CreatedAt)
	}
	if !parsed.UpdatedAt.Equal(original.UpdatedAt) {
		t.Errorf("Round-trip UpdatedAt = %v, want %v", parsed.UpdatedAt, original.UpdatedAt)
	}
	if parsed.DueAt == nil || !parsed.DueAt.Equal(*original.DueAt) {
		t.Errorf("Round-trip DueAt = %v, want %v", parsed.DueAt, original.DueAt)
	}
}
