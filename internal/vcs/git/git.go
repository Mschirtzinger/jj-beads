// Package git provides a Git implementation of the VCS interface.
//
// This package wraps Git commands to provide the operations needed by beads,
// including repository discovery, reference management, commits, and worktree-based
// workspaces for sync branch functionality.
package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// Git implements the VCS interface for git repositories.
type Git struct {
	// repoRoot is the repository root directory path
	repoRoot string

	// vcsDir is the .git directory path (may be a file for worktrees)
	vcsDir string

	// isWorktree indicates if this is a git worktree
	isWorktree bool

	// mainRepoRoot is the main repository root (for worktrees)
	mainRepoRoot string
}

// New creates a new Git VCS instance for the given repository.
// The path should be somewhere within a git repository.
func New(path string) (*Git, error) {
	g := &Git{}

	// Detect repository information
	if err := g.detect(path); err != nil {
		return nil, err
	}

	return g, nil
}

// Name returns the VCS type (git)
func (g *Git) Name() vcs.Type {
	return vcs.TypeGit
}

// Version returns the git version string
func (g *Git) Version() (string, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git version: %w", err)
	}

	// Output format: "git version 2.39.0"
	version := strings.TrimSpace(string(output))
	if strings.HasPrefix(version, "git version ") {
		version = strings.TrimPrefix(version, "git version ")
	}

	return version, nil
}

// RepoRoot returns the repository root directory path
func (g *Git) RepoRoot() (string, error) {
	if g.repoRoot == "" {
		return "", vcs.ErrNotInVCS
	}
	return g.repoRoot, nil
}

// VCSDir returns the .git directory path
func (g *Git) VCSDir() (string, error) {
	if g.vcsDir == "" {
		return "", vcs.ErrNotInVCS
	}
	return g.vcsDir, nil
}

// IsInVCS returns true if inside a git repository
func (g *Git) IsInVCS() bool {
	return g.repoRoot != ""
}

// Exec executes a raw git command
func (g *Git) Exec(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git %s failed: %w\n%s",
			strings.Join(args, " "), err, string(output))
	}

	return output, nil
}

// CanUndo returns true if undo is supported
// Git has limited undo support via reflog
func (g *Git) CanUndo() bool {
	// Git has reflog but it's limited compared to jj's operation log
	// Return false to indicate this is not a first-class feature
	return false
}

// Undo attempts to undo the last operation using reflog
func (g *Git) Undo(ctx context.Context) error {
	// Check if reflog has entries
	checkCmd := exec.CommandContext(ctx, "git", "reflog", "show", "-1")
	checkCmd.Dir = g.repoRoot
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("no reflog entries available: %w", err)
	}

	// Reset to previous reflog entry
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD@{1}")
	cmd.Dir = g.repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %w\n%s", err, string(output))
	}

	return nil
}

// GetOperationLog returns recent git operations from reflog
func (g *Git) GetOperationLog(limit int) ([]vcs.OperationInfo, error) {
	if limit <= 0 {
		limit = 10
	}

	cmd := exec.Command("git", "reflog", "-n", fmt.Sprintf("%d", limit),
		"--format=%H %gD %gs")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git reflog failed: %w", err)
	}

	var ops []vcs.OperationInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}

		ops = append(ops, vcs.OperationInfo{
			ID:          parts[0],
			Description: parts[2],
			// Git reflog doesn't have structured timestamps/user/args
		})
	}

	return ops, nil
}
