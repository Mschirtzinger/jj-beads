// Package schema provides data structures for jj-turso task files.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

// TaskFile represents a task stored as individual JSON file in tasks/*.json.
// This structure is CRDT-friendly with flat fields and last-write-wins semantics.
// Each field can be updated independently, and timestamps help resolve conflicts.
type TaskFile struct {
	// ===== Core Identification =====
	ID string `json:"id"`

	// ===== Task Content =====
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // bug, feature, task, epic, chore
	Status      string `json:"status"` // open, in_progress, blocked, closed, etc.

	// ===== Priority & Scheduling =====
	Priority int `json:"priority"` // 0-4 (P0=critical, P4=backlog)

	// ===== Assignment & Ownership =====
	AssignedAgent string `json:"assigned_agent,omitempty"` // Agent ID for ownership

	// ===== Tags & Classification =====
	Tags []string `json:"tags,omitempty"` // Labels/tags for categorization

	// ===== Timestamps (CRDT conflict resolution) =====
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// ===== Time-Based Scheduling =====
	DueAt      *time.Time `json:"due_at,omitempty"`      // When this should be completed
	DeferUntil *time.Time `json:"defer_until,omitempty"` // Hide from ready work until this time
}

// Validate checks if the TaskFile has valid field values.
func (t *TaskFile) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("id is required")
	}
	if t.Title == "" {
		return fmt.Errorf("title is required")
	}
	if len(t.Title) > 500 {
		return fmt.Errorf("title must be 500 characters or less (got %d)", len(t.Title))
	}
	if t.Priority < 0 || t.Priority > 4 {
		return fmt.Errorf("priority must be between 0 and 4 (got %d)", t.Priority)
	}
	if t.Type == "" {
		return fmt.Errorf("type is required")
	}
	if t.Status == "" {
		return fmt.Errorf("status is required")
	}
	if t.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	if t.UpdatedAt.IsZero() {
		return fmt.Errorf("updated_at is required")
	}
	return nil
}

// Filename returns the canonical filename for this task: {id}.json
func (t *TaskFile) Filename() string {
	return fmt.Sprintf("%s.json", t.ID)
}

// ToIssue converts TaskFile to the existing types.Issue structure.
// This allows interoperability with the current storage layer.
func (t *TaskFile) ToIssue() *types.Issue {
	issue := &types.Issue{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		IssueType:   types.IssueType(t.Type),
		Status:      types.Status(t.Status),
		Priority:    t.Priority,
		Assignee:    t.AssignedAgent,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		DueAt:       t.DueAt,
		DeferUntil:  t.DeferUntil,
		Labels:      t.Tags,
	}
	return issue
}

// FromIssue converts a types.Issue to TaskFile format.
// This is the inverse of ToIssue() for migration and compatibility.
func FromIssue(issue *types.Issue) *TaskFile {
	task := &TaskFile{
		ID:            issue.ID,
		Title:         issue.Title,
		Description:   issue.Description,
		Type:          string(issue.IssueType),
		Status:        string(issue.Status),
		Priority:      issue.Priority,
		AssignedAgent: issue.Assignee,
		CreatedAt:     issue.CreatedAt,
		UpdatedAt:     issue.UpdatedAt,
		DueAt:         issue.DueAt,
		DeferUntil:    issue.DeferUntil,
		Tags:          issue.Labels,
	}
	return task
}

// ReadTaskFile reads and parses a task JSON file from the given path.
// Returns the parsed TaskFile or an error if reading/parsing fails.
func ReadTaskFile(path string) (*TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file %s: %w", path, err)
	}

	var task TaskFile
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to parse task file %s: %w", path, err)
	}

	if err := task.Validate(); err != nil {
		return nil, fmt.Errorf("invalid task file %s: %w", path, err)
	}

	return &task, nil
}

// WriteTaskFile writes a TaskFile to disk as JSON.
// The file is written to tasksDir/{id}.json with pretty-printed formatting.
func WriteTaskFile(tasksDir string, task *TaskFile) error {
	if err := task.Validate(); err != nil {
		return fmt.Errorf("cannot write invalid task: %w", err)
	}

	// Ensure tasks directory exists
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	// Marshal to pretty JSON
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task %s: %w", task.ID, err)
	}

	// Write to file
	path := filepath.Join(tasksDir, task.Filename())
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file %s: %w", path, err)
	}

	return nil
}

// ReadAllTaskFiles reads all task files from the given directory.
// Returns a slice of TaskFile pointers or an error if reading fails.
// Invalid files are skipped with a warning to stderr.
func ReadAllTaskFiles(tasksDir string) ([]*TaskFile, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TaskFile{}, nil // Empty directory is valid
		}
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var tasks []*TaskFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name())
		task, err := ReadTaskFile(path)
		if err != nil {
			// Log warning but continue processing other files
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid task file %s: %v\n", entry.Name(), err)
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// SetDefaults applies default values for optional fields.
// This ensures consistent behavior when fields are omitted.
func (t *TaskFile) SetDefaults() {
	if t.Status == "" {
		t.Status = "open"
	}
	if t.Type == "" {
		t.Type = "task"
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = time.Now()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
}

// UpdateTimestamp sets UpdatedAt to current time.
// This should be called whenever any field is modified.
func (t *TaskFile) UpdateTimestamp() {
	t.UpdatedAt = time.Now()
}

// IsBlocked returns true if this task has blocking dependencies.
// Note: This requires checking the deps/ directory separately,
// as dependency information is stored in separate files.
func (t *TaskFile) IsBlocked() bool {
	// This is a placeholder - actual implementation requires
	// reading dependency files from deps/ directory
	return false
}
