package jj

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/beads/internal/vcs"
)

// ===================
// Workspace Operations
// ===================
// In jj, workspaces are implemented using the change model rather than
// separate working directories (like git worktrees). We create a temporary
// change for sync operations without affecting the user's working directory.

// jjWorkspace implements the Workspace interface using jj's change model.
type jjWorkspace struct {
	jj       *JJ
	ref      string // Bookmark for this workspace
	changeID string // The change ID for this workspace
	name     string // Workspace identifier
}

// CreateWorkspace creates an isolated workspace for sync operations.
//
// Unlike git worktrees which create separate directories, jj workspaces
// use the change model:
//  1. Create a new change with a descriptive message
//  2. Create a bookmark pointing to this change
//  3. Return workspace that manages this change
//
// The workspace allows operations without affecting the user's current work.
func (j *JJ) CreateWorkspace(opts vcs.WorkspaceOptions) (vcs.Workspace, error) {
	ctx := context.Background()

	// Check if bookmark already exists
	if j.RefExists(opts.Ref) {
		return nil, vcs.ErrWorkspaceExists
	}

	// Get current change ID to return to it later
	currentChange, err := j.getCurrentChangeID()
	if err != nil {
		return nil, fmt.Errorf("failed to get current change: %w", err)
	}

	// Create a new change for the workspace
	syncMsg := fmt.Sprintf("Sync: %s", time.Now().Format(time.RFC3339))
	if err := j.Commit(ctx, vcs.CommitOptions{
		Message:   syncMsg,
		CreateNew: true,
	}); err != nil {
		return nil, fmt.Errorf("failed to create workspace change: %w", err)
	}

	// Get the new change ID
	newChangeID, err := j.getCurrentChangeID()
	if err != nil {
		return nil, fmt.Errorf("failed to get new change ID: %w", err)
	}

	// Create bookmark at this change
	if err := j.CreateRef(opts.Ref, "@"); err != nil {
		// Try to return to original change
		_ = j.editChange(ctx, currentChange)
		return nil, fmt.Errorf("failed to create workspace bookmark: %w", err)
	}

	// Return to original change (user's work is undisturbed)
	if err := j.editChange(ctx, currentChange); err != nil {
		return nil, fmt.Errorf("failed to return to original change: %w", err)
	}

	return &jjWorkspace{
		jj:       j,
		ref:      opts.Ref,
		changeID: newChangeID,
		name:     opts.Name,
	}, nil
}

// getCurrentChangeID gets the current change ID.
func (j *JJ) getCurrentChangeID() (string, error) {
	ctx := context.Background()

	output, err := j.execWithOutput(ctx, "log", "-r", "@", "-n", "1", "--no-graph")
	if err != nil {
		return "", err
	}

	// Parse change ID from output
	// Format: changeID user@host timestamp [bookmark] commitID
	// Example: szvoputy setup@jj-dev.local 2026-01-08 00:09:30 test-bookmark e89402e0
	fields := strings.Fields(output)
	if len(fields) >= 1 {
		return fields[0], nil
	}

	return "", fmt.Errorf("could not parse change ID")
}

// editChange moves to a different change.
func (j *JJ) editChange(ctx context.Context, changeID string) error {
	_, err := j.Exec(ctx, "edit", changeID)
	return err
}

// ListWorkspaces returns information about existing workspaces.
// For jj, this lists bookmarks that match workspace naming pattern.
func (j *JJ) ListWorkspaces() ([]vcs.WorkspaceInfo, error) {
	refs, err := j.ListRefs()
	if err != nil {
		return nil, err
	}

	var workspaces []vcs.WorkspaceInfo

	// Filter for workspace bookmarks (typically named "beads-sync" or similar)
	for _, ref := range refs {
		// Only include local bookmarks
		if ref.IsRemote {
			continue
		}

		// Check if this looks like a workspace bookmark
		if strings.Contains(ref.Name, "sync") ||
			strings.Contains(ref.Name, "workspace") {
			workspaces = append(workspaces, vcs.WorkspaceInfo{
				Name:    ref.Name,
				Path:    j.repoRoot, // jj workspaces work in-place
				Ref:     ref.Name,
				IsValid: true,
			})
		}
	}

	return workspaces, nil
}

// ===================
// jjWorkspace Implementation
// ===================

// Path returns the filesystem path to the workspace.
// For jj, this is always the repository root (changes work in-place).
func (w *jjWorkspace) Path() string {
	return w.jj.repoRoot
}

// Ref returns the reference (bookmark) this workspace is based on.
func (w *jjWorkspace) Ref() string {
	return w.ref
}

// SyncToWorkspace copies a file from the main repo to the workspace.
// For jj, this is a simple file copy since we work in-place.
func (w *jjWorkspace) SyncToWorkspace(srcPath, dstRelPath string) error {
	dstPath := filepath.Join(w.jj.repoRoot, dstRelPath)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Copy file
	return copyFile(srcPath, dstPath)
}

// SyncFromWorkspace copies a file from the workspace to the main repo.
// For jj, this is a simple file copy since we work in-place.
func (w *jjWorkspace) SyncFromWorkspace(srcRelPath, dstPath string) error {
	srcPath := filepath.Join(w.jj.repoRoot, srcRelPath)
	return copyFile(srcPath, dstPath)
}

// HasChanges returns true if there are uncommitted changes in the workspace.
func (w *jjWorkspace) HasChanges(paths ...string) (bool, error) {
	ctx := context.Background()

	// Switch to workspace change
	currentChange, err := w.jj.getCurrentChangeID()
	if err != nil {
		return false, err
	}
	defer func() { _ = w.jj.editChange(ctx, currentChange) }()

	if err := w.jj.editChange(ctx, w.changeID); err != nil {
		return false, err
	}

	// Check for changes
	return w.jj.HasChanges(paths...)
}

// Commit commits changes in this workspace with the given message.
func (w *jjWorkspace) Commit(ctx context.Context, message string, paths []string) error {
	// Switch to workspace change
	currentChange, err := w.jj.getCurrentChangeID()
	if err != nil {
		return err
	}
	defer func() { _ = w.jj.editChange(ctx, currentChange) }()

	if err := w.jj.editChange(ctx, w.changeID); err != nil {
		return err
	}

	// Commit changes
	return w.jj.Commit(ctx, vcs.CommitOptions{
		Message:   message,
		Paths:     paths,
		CreateNew: false, // Don't create new change - update current
	})
}

// Push pushes the workspace's reference to the remote.
func (w *jjWorkspace) Push(ctx context.Context, remote string) error {
	// Push the workspace bookmark
	return w.jj.Push(ctx, vcs.PushOptions{
		Remote: remote,
		Ref:    w.ref,
	})
}

// Pull pulls changes from the remote into the workspace.
func (w *jjWorkspace) Pull(ctx context.Context, remote string) error {
	// Fetch changes
	if err := w.jj.Fetch(ctx, remote, w.ref); err != nil {
		return err
	}

	// Update workspace bookmark to track remote
	remoteName := remote
	if remoteName == "" {
		remoteName = "origin"
	}

	remoteRef := fmt.Sprintf("%s/%s", remoteName, w.ref)
	if err := w.jj.MoveRef(w.ref, remoteRef); err != nil {
		return err
	}

	return nil
}

// Cleanup removes the workspace and cleans up resources.
func (w *jjWorkspace) Cleanup() error {
	ctx := context.Background()

	// Check if workspace has uncommitted changes
	hasChanges, err := w.HasChanges()
	if err != nil {
		return err
	}

	// If workspace is empty, abandon the change
	if !hasChanges {
		// Switch to a different change first
		currentChange, err := w.jj.getCurrentChangeID()
		if err == nil && currentChange != w.changeID {
			// Abandon the workspace change
			_, _ = w.jj.Exec(ctx, "abandon", w.changeID)
		}
	}

	// Delete the bookmark
	if err := w.jj.DeleteRef(w.ref); err != nil {
		// Don't fail cleanup if bookmark deletion fails
		return nil
	}

	return nil
}

// IsHealthy verifies the workspace is in a good state.
func (w *jjWorkspace) IsHealthy() error {
	// Check if bookmark still exists
	if !w.jj.RefExists(w.ref) {
		return fmt.Errorf("workspace bookmark %s no longer exists", w.ref)
	}

	// Check if change still exists
	ctx := context.Background()
	_, err := w.jj.execWithOutput(ctx, "log", "-r", w.changeID, "-n", "1")
	if err != nil {
		return fmt.Errorf("workspace change %s no longer exists: %w", w.changeID, err)
	}

	return nil
}

// ===================
// Helper Functions
// ===================

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Sync to ensure data is written
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}
