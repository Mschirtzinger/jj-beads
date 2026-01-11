package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

// TaskFile represents the individual JSON file format for tasks/*.json
type TaskFile struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Priority     int        `json:"priority"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	AssignedAgent string    `json:"assigned_agent,omitempty"`
	Description  string     `json:"description,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	DueAt        *time.Time `json:"due_at,omitempty"`
	DeferUntil   *time.Time `json:"defer_until,omitempty"`
}

// DepFile represents the individual JSON file format for deps/*.json
type DepFile struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// MigrateOptions contains configuration for the migration
type MigrateOptions struct {
	FromJSONL string // Input JSONL file path
	ToFiles   string // Output directory for task files
	DryRun    bool   // Preview without writing
	Backup    bool   // Create backup of original
}

// MigrateResult contains statistics about the migration
type MigrateResult struct {
	TasksConverted int
	DepsCreated    int
	FilesWritten   int
	BackupCreated  string
	Errors         []string
}

// FromJSONL reads a JSONL file and returns parsed issues
func FromJSONL(jsonlPath string) ([]*types.Issue, error) {
	// #nosec G304 - controlled path from CLI
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSONL file: %w", err)
	}
	defer file.Close()

	var issues []*types.Issue
	decoder := json.NewDecoder(file)
	lineNum := 0

	for {
		var issue types.Issue
		if err := decoder.Decode(&issue); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("invalid JSON at line %d: %w", lineNum+1, err)
		}
		lineNum++

		// Apply defaults for missing fields
		issue.SetDefaults()

		issues = append(issues, &issue)
	}

	return issues, nil
}

// IssueToTaskFile converts an Issue to TaskFile format
func IssueToTaskFile(issue *types.Issue) *TaskFile {
	task := &TaskFile{
		ID:          issue.ID,
		Title:       issue.Title,
		Type:        string(issue.IssueType),
		Status:      string(issue.Status),
		Priority:    issue.Priority,
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
		Description: issue.Description,
		DueAt:       issue.DueAt,
		DeferUntil:  issue.DeferUntil,
	}

	// Convert assignee to assigned_agent
	if issue.Assignee != "" {
		task.AssignedAgent = issue.Assignee
	}

	// Convert labels to tags
	if len(issue.Labels) > 0 {
		task.Tags = issue.Labels
	}

	return task
}

// DependencyToDepFile converts a Dependency to DepFile format
func DependencyToDepFile(dep *types.Dependency) *DepFile {
	return &DepFile{
		From:      dep.IssueID,
		To:        dep.DependsOnID,
		Type:      string(dep.Type),
		CreatedAt: dep.CreatedAt,
	}
}

// DepFileName generates the filename for a dependency
// Format: {from}--{type}--{to}.json
func DepFileName(from, typ, to string) string {
	return fmt.Sprintf("%s--%s--%s.json", from, typ, to)
}

// WriteTaskFile writes a TaskFile to disk
func WriteTaskFile(task *TaskFile, outputDir string) error {
	taskPath := filepath.Join(outputDir, "tasks", task.ID+".json")

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(taskPath), 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Write atomically via temp file
	tmpPath := taskPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, taskPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// WriteDepFile writes a DepFile to disk
func WriteDepFile(dep *DepFile, outputDir string) error {
	depPath := filepath.Join(outputDir, "deps", DepFileName(dep.From, dep.Type, dep.To))

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(depPath), 0755); err != nil {
		return fmt.Errorf("failed to create deps directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(dep, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal dependency: %w", err)
	}

	// Write atomically via temp file
	tmpPath := depPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, depPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Migrate performs the JSONL to file-based format migration
func Migrate(ctx context.Context, opts MigrateOptions) (*MigrateResult, error) {
	result := &MigrateResult{}

	// Validate input file exists
	if _, err := os.Stat(opts.FromJSONL); err != nil {
		return nil, fmt.Errorf("input file does not exist: %w", err)
	}

	// Create backup if requested
	if opts.Backup && !opts.DryRun {
		backupPath := opts.FromJSONL + ".backup." + time.Now().Format("20060102-150405")
		input, err := os.ReadFile(opts.FromJSONL)
		if err != nil {
			return nil, fmt.Errorf("failed to read input for backup: %w", err)
		}
		if err := os.WriteFile(backupPath, input, 0600); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
		result.BackupCreated = backupPath
	}

	// Read and parse JSONL
	issues, err := FromJSONL(opts.FromJSONL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSONL: %w", err)
	}

	// Track already written dependencies to avoid duplicates
	writtenDeps := make(map[string]bool)

	// Convert and write each issue
	for _, issue := range issues {
		// Skip tombstones - they shouldn't be migrated
		if issue.IsTombstone() {
			continue
		}

		// Convert to TaskFile
		task := IssueToTaskFile(issue)

		if !opts.DryRun {
			if err := WriteTaskFile(task, opts.ToFiles); err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("failed to write task %s: %v", task.ID, err))
				continue
			}
			result.FilesWritten++
		}
		result.TasksConverted++

		// Convert and write dependencies
		for _, dep := range issue.Dependencies {
			// Generate unique key for this dependency
			depKey := fmt.Sprintf("%s|%s|%s", dep.IssueID, dep.Type, dep.DependsOnID)

			// Skip if already written
			if writtenDeps[depKey] {
				continue
			}
			writtenDeps[depKey] = true

			depFile := DependencyToDepFile(dep)

			if !opts.DryRun {
				if err := WriteDepFile(depFile, opts.ToFiles); err != nil {
					result.Errors = append(result.Errors,
						fmt.Sprintf("failed to write dep %s: %v", depKey, err))
					continue
				}
				result.FilesWritten++
			}
			result.DepsCreated++
		}
	}

	return result, nil
}

// CleanupMigration removes generated files (for rollback)
func CleanupMigration(outputDir string) error {
	tasksDir := filepath.Join(outputDir, "tasks")
	depsDir := filepath.Join(outputDir, "deps")

	// Remove directories if they exist
	for _, dir := range []string{tasksDir, depsDir} {
		if _, err := os.Stat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("failed to remove %s: %w", dir, err)
			}
		}
	}

	return nil
}
