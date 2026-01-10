// Package daemon provides the sync daemon for watching jj operations
// and synchronizing file changes to Turso cache database.
//
// The daemon watches jj's operation log for new operations that modify
// task/*.json or deps/*.json files, parses the changes, and updates
// the Turso cache accordingly.
package daemon

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// OpLogEntry represents a single operation from jj's operation log.
//
// Each entry captures metadata about a jj operation (snapshot, rebase, etc.)
// and can be used to determine which files were affected.
type OpLogEntry struct {
	// ID is the operation ID (64-character hex string)
	ID string

	// Description is the human-readable description of the operation
	// Examples: "snapshot working copy", "rebase", "new empty commit"
	Description string

	// Timestamp is when the operation started
	Timestamp time.Time

	// Duration is how long the operation took
	Duration time.Duration

	// User is the user who performed the operation (username@hostname)
	User string

	// Args are the command-line arguments that triggered the operation
	Args []string

	// AffectedFiles contains the list of task/dep files that were modified
	// These are detected by parsing the operation diff
	AffectedFiles []string
}

// OpLogWatcherConfig configures the operation log watcher.
type OpLogWatcherConfig struct {
	// RepoPath is the path to the jj repository
	RepoPath string

	// PollInterval is how often to check for new operations (default: 100ms)
	PollInterval time.Duration

	// TasksDir is the directory containing task files (relative to repo root)
	// Default: "tasks"
	TasksDir string

	// DepsDir is the directory containing dependency files (relative to repo root)
	// Default: "deps"
	DepsDir string

	// LastOpID is the operation ID to start watching from
	// If empty, starts from the most recent operation
	LastOpID string
}

// OpLogCallback is called when new operations are detected.
//
// The callback receives a slice of new operations in chronological order
// (oldest first). If the callback returns an error, watching continues
// but the error is logged.
type OpLogCallback func(entries []OpLogEntry) error

// ParseOpLog parses the output from `jj op log` command.
//
// The expected format is from:
//   jj op log --no-graph -T 'id ++ "\n" ++ description ++ "\n---\n"'
//
// This returns operations in reverse chronological order (newest first).
//
// Example input:
//   abc123...
//   snapshot working copy
//   ---
//   def456...
//   rebase
//   ---
func ParseOpLog(output []byte) ([]OpLogEntry, error) {
	var entries []OpLogEntry

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentID string
	var currentDesc string

	for scanner.Scan() {
		line := scanner.Text()

		// Separator line
		if line == "---" {
			if currentID != "" {
				entries = append(entries, OpLogEntry{
					ID:          currentID,
					Description: currentDesc,
				})
				currentID = ""
				currentDesc = ""
			}
			continue
		}

		// If we don't have an ID yet, this is the ID line
		if currentID == "" {
			currentID = strings.TrimSpace(line)
			continue
		}

		// If we have an ID but no description, this is the description
		if currentDesc == "" {
			currentDesc = strings.TrimSpace(line)
			continue
		}

		// Multi-line descriptions (shouldn't happen with our template, but handle it)
		currentDesc += " " + strings.TrimSpace(line)
	}

	// Handle final entry if no trailing separator
	if currentID != "" {
		entries = append(entries, OpLogEntry{
			ID:          currentID,
			Description: currentDesc,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan op log output: %w", err)
	}

	return entries, nil
}

// GetAffectedFiles determines which task/dep files were modified in an operation.
//
// This runs `jj op show {opID}` with diff output to see what files changed.
// It filters for files in the tasks/ and deps/ directories.
//
// Returns a list of file paths relative to the repository root.
func GetAffectedFiles(ctx context.Context, repoPath string, entry OpLogEntry, tasksDir, depsDir string) ([]string, error) {
	// Run: jj op show {opID} --op-diff --patch
	// This shows what files changed in this operation
	cmd := exec.CommandContext(ctx, "jj", "op", "show", entry.ID, "--op-diff", "--patch")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get op diff: %w (output: %s)", err, output)
	}

	return parseAffectedFiles(output, tasksDir, depsDir), nil
}

// parseAffectedFiles extracts task/dep file paths from jj op show output.
//
// It looks for lines like:
//   Added regular file tasks/bd-123.json:
//   Modified regular file deps/bd-abc--blocks--bd-xyz.json:
//   Removed regular file tasks/bd-456.json:
func parseAffectedFiles(diffOutput []byte, tasksDir, depsDir string) []string {
	var files []string
	seen := make(map[string]bool)

	// Regex to match file operation lines
	// Examples:
	//   Added regular file tasks/bd-123.json:
	//   Modified regular file deps/bd-abc--blocks--bd-xyz.json:
	//   Removed regular file tasks/bd-456.json:
	filePattern := regexp.MustCompile(`(?:Added|Modified|Removed) regular file (.+):`)

	scanner := bufio.NewScanner(bytes.NewReader(diffOutput))
	for scanner.Scan() {
		line := scanner.Text()

		matches := filePattern.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}

		filePath := matches[1]

		// Check if this is a task or dep file
		if !strings.HasPrefix(filePath, tasksDir+"/") &&
			!strings.HasPrefix(filePath, depsDir+"/") {
			continue
		}

		// Check if it's a JSON file
		if !strings.HasSuffix(filePath, ".json") {
			continue
		}

		// Deduplicate
		if seen[filePath] {
			continue
		}

		files = append(files, filePath)
		seen[filePath] = true
	}

	return files
}

// WatchOpLog polls the jj operation log for new operations and calls the callback.
//
// This function blocks until the context is cancelled. It polls at the configured
// interval and calls the callback whenever new operations are detected.
//
// Operations are delivered to the callback in chronological order (oldest first).
//
// Example:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	config := OpLogWatcherConfig{
//	    RepoPath: "/path/to/repo",
//	    PollInterval: 100 * time.Millisecond,
//	    TasksDir: "tasks",
//	    DepsDir: "deps",
//	}
//
//	err := WatchOpLog(ctx, config, func(entries []OpLogEntry) error {
//	    for _, entry := range entries {
//	        log.Printf("New operation: %s - %s", entry.ID[:12], entry.Description)
//	        for _, file := range entry.AffectedFiles {
//	            log.Printf("  Changed: %s", file)
//	        }
//	    }
//	    return nil
//	})
func WatchOpLog(ctx context.Context, config OpLogWatcherConfig, callback OpLogCallback) error {
	// Set defaults
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond
	}
	if config.TasksDir == "" {
		config.TasksDir = "tasks"
	}
	if config.DepsDir == "" {
		config.DepsDir = "deps"
	}

	// Get absolute repo path
	absRepoPath, err := filepath.Abs(config.RepoPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute repo path: %w", err)
	}

	lastSeenID := config.LastOpID
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			// Get recent operations
			cmd := exec.CommandContext(ctx, "jj", "op", "log", "--no-graph",
				"-T", `id ++ "\n" ++ description ++ "\n---\n"`,
				"-n", "50") // Check last 50 operations
			cmd.Dir = absRepoPath

			output, err := cmd.CombinedOutput()
			if err != nil {
				// Log error but continue watching
				fmt.Printf("Warning: failed to get op log: %v\n", err)
				continue
			}

			entries, err := ParseOpLog(output)
			if err != nil {
				fmt.Printf("Warning: failed to parse op log: %v\n", err)
				continue
			}

			// Find new operations
			newOps := findNewOperations(entries, lastSeenID)
			if len(newOps) == 0 {
				continue
			}

			// Reverse to get chronological order (oldest first)
			reverseSlice(newOps)

			// Get affected files for each operation
			for i := range newOps {
				files, err := GetAffectedFiles(ctx, absRepoPath, newOps[i], config.TasksDir, config.DepsDir)
				if err != nil {
					// Log error but continue with other operations
					fmt.Printf("Warning: failed to get affected files for %s: %v\n", newOps[i].ID[:12], err)
					continue
				}
				newOps[i].AffectedFiles = files
			}

			// Update last seen ID (most recent in the original list)
			lastSeenID = entries[0].ID

			// Call callback with new operations
			if err := callback(newOps); err != nil {
				fmt.Printf("Warning: callback error: %v\n", err)
				// Continue watching despite callback error
			}
		}
	}
}

// findNewOperations returns operations that are newer than lastSeenID.
//
// The input entries are assumed to be in reverse chronological order (newest first).
// Returns new operations in reverse chronological order.
func findNewOperations(entries []OpLogEntry, lastSeenID string) []OpLogEntry {
	if lastSeenID == "" {
		// First run - return only the most recent operation
		if len(entries) > 0 {
			return entries[:1]
		}
		return nil
	}

	// Find where the last seen ID appears
	for i, entry := range entries {
		if entry.ID == lastSeenID {
			// Return everything before this index (newer operations)
			if i == 0 {
				return nil // No new operations
			}
			return entries[:i]
		}
	}

	// lastSeenID not found - it might have been garbage collected
	// Return only the most recent operation to avoid processing too much history
	if len(entries) > 0 {
		return entries[:1]
	}
	return nil
}

// reverseSlice reverses a slice of OpLogEntry in place.
func reverseSlice(entries []OpLogEntry) {
	for i := 0; i < len(entries)/2; i++ {
		j := len(entries) - 1 - i
		entries[i], entries[j] = entries[j], entries[i]
	}
}

// GetLatestOperationID returns the ID of the most recent operation.
//
// This is useful for initializing the watcher's LastOpID.
func GetLatestOperationID(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "jj", "op", "log", "--no-graph",
		"-T", `id ++ "\n" ++ description ++ "\n---\n"`,
		"-n", "1")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get latest operation: %w (output: %s)", err, output)
	}

	entries, err := ParseOpLog(output)
	if err != nil {
		return "", fmt.Errorf("failed to parse op log: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no operations found")
	}

	return entries[0].ID, nil
}
