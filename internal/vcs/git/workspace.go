package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// GitWorkspace implements the Workspace interface using git worktrees
type GitWorkspace struct {
	git  *Git
	path string
	ref  string
}

// CreateWorkspace creates an isolated workspace for sync operations using git worktrees
func (g *Git) CreateWorkspace(opts vcs.WorkspaceOptions) (vcs.Workspace, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("workspace name is required")
	}

	if opts.Ref == "" {
		return nil, fmt.Errorf("workspace ref is required")
	}

	// Default path if not specified
	path := opts.Path
	if path == "" {
		path = filepath.Join(g.mainRepoRoot, ".git", "beads-worktrees", opts.Name)
	}

	// Check if worktree already exists
	if exists, err := g.worktreeExists(path); err != nil {
		return nil, err
	} else if exists {
		// Verify health
		ws := &GitWorkspace{git: g, path: path, ref: opts.Ref}
		if err := ws.IsHealthy(); err == nil {
			return ws, nil // Already exists and healthy
		}
		// Unhealthy, remove and recreate
		if err := g.removeWorktree(path); err != nil {
			return nil, fmt.Errorf("failed to remove unhealthy worktree: %w", err)
		}
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create the reference if it doesn't exist
	if !g.RefExists(opts.Ref) {
		if err := g.CreateRef(opts.Ref, ""); err != nil {
			return nil, fmt.Errorf("failed to create ref: %w", err)
		}
	}

	// Create worktree
	args := []string{"worktree", "add", "-f", "--no-checkout", path, opts.Ref}
	cmd := exec.Command("git", args...)
	cmd.Dir = g.mainRepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w\n%s", err, string(output))
	}

	ws := &GitWorkspace{git: g, path: path, ref: opts.Ref}

	// Configure sparse checkout if requested
	if opts.Sparse {
		if err := ws.configureSparseCheckout(opts.SparsePaths); err != nil {
			_ = g.removeWorktree(path) // Cleanup on failure
			return nil, fmt.Errorf("failed to configure sparse checkout: %w", err)
		}
	}

	// Checkout the branch
	checkoutCmd := exec.Command("git", "checkout", opts.Ref)
	checkoutCmd.Dir = path
	output, err = checkoutCmd.CombinedOutput()
	if err != nil {
		_ = g.removeWorktree(path)
		return nil, fmt.Errorf("failed to checkout branch: %w\n%s", err, string(output))
	}

	// Disable sparse checkout on main repo (GH#886)
	disableCmd := exec.Command("git", "config", "core.sparseCheckout", "false")
	disableCmd.Dir = g.mainRepoRoot
	_ = disableCmd.Run() // Best effort

	return ws, nil
}

// ListWorkspaces returns information about existing workspaces
func (g *Git) ListWorkspaces() ([]vcs.WorkspaceInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = g.mainRepoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var workspaces []vcs.WorkspaceInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	var current *vcs.WorkspaceInfo
	for _, line := range lines {
		if line == "" {
			if current != nil {
				workspaces = append(workspaces, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			current = &vcs.WorkspaceInfo{
				Path:    path,
				IsValid: true,
			}
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Ref = strings.TrimPrefix(ref, "refs/heads/")
			current.Name = filepath.Base(current.Path)
		}
	}

	// Add last workspace if exists
	if current != nil {
		workspaces = append(workspaces, *current)
	}

	return workspaces, nil
}

// worktreeExists checks if a worktree exists at the given path
func (g *Git) worktreeExists(path string) (bool, error) {
	workspaces, err := g.ListWorkspaces()
	if err != nil {
		return false, err
	}

	absPath, _ := filepath.Abs(path)
	for _, ws := range workspaces {
		wsAbsPath, _ := filepath.Abs(ws.Path)
		if wsAbsPath == absPath {
			return true, nil
		}
	}

	return false, nil
}

// removeWorktree removes a git worktree
func (g *Git) removeWorktree(path string) error {
	cmd := exec.Command("git", "worktree", "remove", path, "--force")
	cmd.Dir = g.mainRepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try manual cleanup
		if removeErr := os.RemoveAll(path); removeErr != nil {
			return fmt.Errorf("failed to remove worktree: %w (git error: %v, output: %s)",
				removeErr, err, string(output))
		}

		// Prune stale entries
		pruneCmd := exec.Command("git", "worktree", "prune")
		pruneCmd.Dir = g.mainRepoRoot
		_ = pruneCmd.Run()
	}

	return nil
}

// Workspace implementation

// Path returns the filesystem path to the workspace
func (w *GitWorkspace) Path() string {
	return w.path
}

// Ref returns the reference this workspace is based on
func (w *GitWorkspace) Ref() string {
	return w.ref
}

// SyncToWorkspace copies a file from the main repo to the workspace
func (w *GitWorkspace) SyncToWorkspace(srcPath, dstRelPath string) error {
	dstPath := filepath.Join(w.path, dstRelPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// SyncFromWorkspace copies a file from the workspace to the main repo
func (w *GitWorkspace) SyncFromWorkspace(srcRelPath, dstPath string) error {
	srcPath := filepath.Join(w.path, srcRelPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// HasChanges returns true if there are uncommitted changes in the workspace
func (w *GitWorkspace) HasChanges(paths ...string) (bool, error) {
	args := []string{"status", "--porcelain"}
	args = append(args, paths...)

	cmd := exec.Command("git", args...)
	cmd.Dir = w.path

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// Commit commits changes in the workspace
func (w *GitWorkspace) Commit(ctx context.Context, message string, paths []string) error {
	// Stage files
	if len(paths) > 0 {
		args := append([]string{"add"}, paths...)
		addCmd := exec.CommandContext(ctx, "git", args...)
		addCmd.Dir = w.path

		if output, err := addCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git add failed: %w\n%s", err, string(output))
		}
	}

	// Commit
	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	if len(paths) > 0 {
		commitCmd.Args = append(commitCmd.Args, "--")
		commitCmd.Args = append(commitCmd.Args, paths...)
	}
	commitCmd.Dir = w.path

	output, err := commitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %w\n%s", err, string(output))
	}

	return nil
}

// Push pushes the workspace's reference to the remote
func (w *GitWorkspace) Push(ctx context.Context, remote string) error {
	if remote == "" {
		remote = "origin"
	}

	cmd := exec.CommandContext(ctx, "git", "push", remote, w.ref)
	cmd.Dir = w.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %w\n%s", err, string(output))
	}

	return nil
}

// Pull pulls changes from the remote into the workspace
func (w *GitWorkspace) Pull(ctx context.Context, remote string) error {
	if remote == "" {
		remote = "origin"
	}

	cmd := exec.CommandContext(ctx, "git", "pull", remote, w.ref)
	cmd.Dir = w.path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %w\n%s", err, string(output))
	}

	return nil
}

// Cleanup removes the workspace
func (w *GitWorkspace) Cleanup() error {
	return w.git.removeWorktree(w.path)
}

// IsHealthy verifies the workspace is in a good state
func (w *GitWorkspace) IsHealthy() error {
	// Check if path exists
	if _, err := os.Stat(w.path); os.IsNotExist(err) {
		return fmt.Errorf("workspace path does not exist")
	}

	// Check if .git file exists
	gitFile := filepath.Join(w.path, ".git")
	if _, err := os.Stat(gitFile); err != nil {
		return fmt.Errorf("workspace .git file missing")
	}

	// Verify it's in the worktree list
	exists, err := w.git.worktreeExists(w.path)
	if err != nil {
		return fmt.Errorf("failed to check worktree list: %w", err)
	}
	if !exists {
		return fmt.Errorf("workspace not in git worktree list")
	}

	return nil
}

// configureSparseCheckout sets up sparse checkout for the workspace
func (w *GitWorkspace) configureSparseCheckout(paths []string) error {
	// Initialize sparse checkout in non-cone mode
	initCmd := exec.Command("git", "sparse-checkout", "init", "--no-cone")
	initCmd.Dir = w.path
	output, err := initCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to init sparse checkout: %w\n%s", err, string(output))
	}

	// Set sparse checkout patterns
	args := append([]string{"sparse-checkout", "set"}, paths...)
	setCmd := exec.Command("git", args...)
	setCmd.Dir = w.path
	output, err = setCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set sparse checkout patterns: %w\n%s", err, string(output))
	}

	return nil
}
