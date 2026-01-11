// Package jj implements the VCS interface for Jujutsu (jj).
//
// Jujutsu is a Git-compatible version control system with powerful features
// including automatic change tracking, operation log with undo, first-class
// conflicts, and stable change IDs.
//
// This implementation wraps the jj CLI using os/exec to provide a Go interface
// for beads issue tracker integration.
package jj

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// JJ implements the VCS interface for Jujutsu.
//
// This struct wraps the jj command-line tool and provides
// a unified interface for version control operations.
type JJ struct {
	// repoRoot is the repository root directory
	repoRoot string

	// jjDir is the .jj directory path
	jjDir string

	// isColocated indicates if this is a colocated repo (.jj + .git)
	isColocated bool
}

// New creates a new JJ instance for the given repository root.
//
// The repository must already be initialized with jj (have a .jj directory).
// Use Init() to create a new jj repository.
func New(repoRoot string) (*JJ, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repository root: %w", err)
	}

	jjDir := filepath.Join(absRoot, ".jj")
	if _, err := os.Stat(jjDir); err != nil {
		return nil, vcs.ErrNotInVCS
	}

	// Check if colocated (both .jj and .git exist)
	gitPath := filepath.Join(absRoot, ".git")
	_, gitErr := os.Stat(gitPath)
	isColocated := gitErr == nil

	return &JJ{
		repoRoot:    absRoot,
		jjDir:       jjDir,
		isColocated: isColocated,
	}, nil
}

// Init initializes a new jj repository in the given path.
// If colocate is true, initializes in colocated mode (jj + git together).
func Init(path string, colocate bool) (*JJ, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	args := []string{"git", "init"}
	if colocate {
		args = append(args, "--colocate")
	}

	cmd := exec.Command("jj", args...)
	cmd.Dir = absPath

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to initialize jj repository: %w", err)
	}

	return New(absPath)
}

// ===================
// Identity
// ===================

// Name returns the VCS type.
// Returns "jj" for non-colocated repos, "colocate" for colocated repos.
func (j *JJ) Name() vcs.Type {
	if j.isColocated {
		return vcs.TypeColocate
	}
	return vcs.TypeJJ
}

// Version returns the jj binary version string.
func (j *JJ) Version() (string, error) {
	cmd := exec.Command("jj", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get jj version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	// Parse "jj 0.32.0" to "0.32.0"
	parts := strings.Fields(version)
	if len(parts) >= 2 {
		return parts[1], nil
	}

	return version, nil
}

// ===================
// Repository Information
// ===================

// RepoRoot returns the repository root directory path.
func (j *JJ) RepoRoot() (string, error) {
	return j.repoRoot, nil
}

// VCSDir returns the .jj directory path.
func (j *JJ) VCSDir() (string, error) {
	return j.jjDir, nil
}

// IsInVCS returns true if we're inside a jj repository.
func (j *JJ) IsInVCS() bool {
	return j.jjDir != ""
}

// ===================
// Raw Command Execution
// ===================

// Exec executes a raw jj command.
// This is the internal command runner used by all other methods.
func (j *JJ) Exec(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "jj", args...)
	cmd.Dir = j.repoRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check for common error patterns
		stderrStr := stderr.String()

		// Detect specific error conditions
		if strings.Contains(stderrStr, "No workspace configured") {
			return nil, vcs.ErrNotInVCS
		}
		if strings.Contains(stderrStr, "No remote configured") {
			return nil, vcs.ErrNoRemote
		}
		if strings.Contains(stderrStr, "conflict") {
			return nil, vcs.ErrConflicts
		}

		return nil, fmt.Errorf("jj %s failed: %w: %s",
			strings.Join(args, " "), err, stderrStr)
	}

	return stdout.Bytes(), nil
}

// execWithOutput is a helper that runs a command and returns stdout as string.
func (j *JJ) execWithOutput(ctx context.Context, args ...string) (string, error) {
	output, err := j.Exec(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ===================
// Undo/Recovery
// ===================

// CanUndo returns true (jj always supports undo via operation log).
func (j *JJ) CanUndo() bool {
	return true
}

// Undo undoes the last operation using jj's operation log.
func (j *JJ) Undo(ctx context.Context) error {
	_, err := j.Exec(ctx, "op", "undo")
	return err
}

// GetOperationLog returns recent VCS operations from jj's operation log.
func (j *JJ) GetOperationLog(limit int) ([]vcs.OperationInfo, error) {
	ctx := context.Background()

	args := []string{"op", "log"}
	if limit > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", limit))
	}

	output, err := j.execWithOutput(ctx, args...)
	if err != nil {
		return nil, err
	}

	// Parse operation log output
	// Format is complex, so this is a basic implementation
	// Real implementation would need more robust parsing
	return parseOperationLog(output), nil
}

// parseOperationLog parses the output of `jj op log`.
// This is a simplified parser - real implementation would be more robust.
func parseOperationLog(output string) []vcs.OperationInfo {
	var ops []vcs.OperationInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Very basic parsing - just capture the description
		// Real implementation would parse timestamp, user, etc.
		ops = append(ops, vcs.OperationInfo{
			Description: line,
		})
	}

	return ops
}
