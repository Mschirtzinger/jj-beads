package jj

import (
	"context"
	"fmt"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// ===================
// File Operations
// ===================

// Add is a no-op in jj (files are auto-tracked).
// This method exists for VCS interface compatibility.
func (j *JJ) Add(paths []string) error {
	// jj has no staging area - changes are automatically tracked
	return nil
}

// Status returns the status of files in the working directory.
func (j *JJ) Status(paths ...string) ([]vcs.FileStatus, error) {
	ctx := context.Background()

	args := []string{"status"}
	args = append(args, paths...)

	output, err := j.execWithOutput(ctx, args...)
	if err != nil {
		return nil, err
	}

	return parseStatus(output), nil
}

// parseStatus parses the output of `jj status`.
// Format:
//   Working copy changes:
//   M file1.go
//   A file2.go
//   D file3.go
func parseStatus(output string) []vcs.FileStatus {
	var statuses []vcs.FileStatus
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}

		// Parse status line: "M file.go"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		statusCode := fields[0]
		path := strings.Join(fields[1:], " ")

		// Map jj status codes to VCS status codes
		var code vcs.StatusCode
		switch statusCode {
		case "M":
			code = vcs.StatusModified
		case "A":
			code = vcs.StatusAdded
		case "D":
			code = vcs.StatusDeleted
		case "R":
			code = vcs.StatusRenamed
		case "C":
			code = vcs.StatusCopied
		default:
			code = vcs.StatusUnmodified
		}

		statuses = append(statuses, vcs.FileStatus{
			Path:       path,
			Status:     code,
			StagedCode: vcs.StatusUnmodified, // jj has no staging area
		})
	}

	return statuses
}

// HasChanges returns true if there are uncommitted changes.
func (j *JJ) HasChanges(paths ...string) (bool, error) {
	statuses, err := j.Status(paths...)
	if err != nil {
		return false, err
	}
	return len(statuses) > 0, nil
}

// ===================
// Commit Operations
// ===================

// Commit creates a commit with the specified options.
//
// In jj, this uses `jj describe` to update the current change description,
// and optionally `jj new` to create a new change if CreateNew is true.
func (j *JJ) Commit(ctx context.Context, opts vcs.CommitOptions) error {
	// In jj, changes are automatically tracked, so we just need to describe them
	args := []string{"describe", "-m", opts.Message}

	// Handle author override
	if opts.Author != "" {
		// jj doesn't have direct --author flag like git
		// Would need to set via config or environment
		// For now, we'll skip this feature
	}

	// Execute describe command
	_, err := j.Exec(ctx, args...)
	if err != nil {
		return err
	}

	// If CreateNew is true, create a new change after describing
	if opts.CreateNew {
		_, err = j.Exec(ctx, "new")
		if err != nil {
			return fmt.Errorf("failed to create new change: %w", err)
		}
	}

	return nil
}

// GetCommitHash returns the commit hash for the given reference.
// For jj, this returns the commit ID (not change ID).
func (j *JJ) GetCommitHash(ref string) (string, error) {
	ctx := context.Background()

	// Use log to get the commit ID for a revision
	output, err := j.execWithOutput(ctx, "log", "-r", ref, "-n", "1", "--no-graph")
	if err != nil {
		return "", err
	}

	// Parse the commit ID from output
	// Format: @ changeID user@host timestamp commitID
	fields := strings.Fields(output)
	if len(fields) >= 5 {
		// Commit ID is typically the 5th field
		return fields[4], nil
	}

	return "", fmt.Errorf("could not parse commit ID from output")
}

// ===================
// Status Operations
// ===================

// HasUnmergedPaths returns true if there are unmerged paths.
// In jj, this checks for conflicted files.
func (j *JJ) HasUnmergedPaths() (bool, error) {
	return j.HasConflicts()
}

// IsInRebaseOrMerge returns false (jj doesn't have these states).
// Jj handles conflicts differently - they're stored in commits.
func (j *JJ) IsInRebaseOrMerge() bool {
	return false
}

// ===================
// Conflict Detection
// ===================

// HasConflicts returns true if there are unresolved conflicts.
// In jj, conflicts are first-class objects that can exist in commits.
func (j *JJ) HasConflicts() (bool, error) {
	conflicts, err := j.GetConflictedFiles()
	if err != nil {
		return false, err
	}
	return len(conflicts) > 0, nil
}

// GetConflictedFiles returns the list of files with conflicts.
func (j *JJ) GetConflictedFiles() ([]string, error) {
	ctx := context.Background()

	output, err := j.execWithOutput(ctx, "resolve", "--list")
	if err != nil {
		// If resolve --list fails, there might be no conflicts
		if strings.Contains(err.Error(), "No conflicts") {
			return []string{}, nil
		}
		return nil, err
	}

	var conflicts []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse conflict line
		// Format varies, but typically starts with file path
		fields := strings.Fields(line)
		if len(fields) > 0 {
			conflicts = append(conflicts, fields[0])
		}
	}

	return conflicts, nil
}
