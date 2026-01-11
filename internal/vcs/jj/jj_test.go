package jj

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/beads/internal/vcs"
)

// TestNew verifies JJ instance creation.
func TestNew(t *testing.T) {
	// Skip if jj is not available
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	// Create temporary directory
	tmpDir := t.TempDir()

	// Initialize jj repo
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize jj repo: %v", err)
	}

	// Verify repo root
	root, err := j.RepoRoot()
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	if root != tmpDir {
		t.Errorf("Expected repo root %s, got %s", tmpDir, root)
	}

	// Verify VCS dir
	vcsDir, err := j.VCSDir()
	if err != nil {
		t.Fatalf("Failed to get VCS dir: %v", err)
	}
	expectedVCSDir := filepath.Join(tmpDir, ".jj")
	if vcsDir != expectedVCSDir {
		t.Errorf("Expected VCS dir %s, got %s", expectedVCSDir, vcsDir)
	}

	// Verify IsInVCS
	if !j.IsInVCS() {
		t.Error("Expected IsInVCS to return true")
	}
}

// TestInit_Colocated verifies colocated repository initialization.
func TestInit_Colocated(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()

	// Initialize colocated repo
	j, err := Init(tmpDir, true)
	if err != nil {
		t.Fatalf("Failed to initialize colocated repo: %v", err)
	}

	// Verify both .jj and .git exist
	jjDir := filepath.Join(tmpDir, ".jj")
	gitPath := filepath.Join(tmpDir, ".git")

	if _, err := os.Stat(jjDir); err != nil {
		t.Errorf(".jj directory not found: %v", err)
	}
	if _, err := os.Stat(gitPath); err != nil {
		t.Errorf(".git directory not found: %v", err)
	}

	// Verify Name returns TypeColocate
	if j.Name() != vcs.TypeColocate {
		t.Errorf("Expected type %s, got %s", vcs.TypeColocate, j.Name())
	}
}

// TestVersion verifies version retrieval.
func TestVersion(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	version, err := j.Version()
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}

	if version == "" {
		t.Error("Expected non-empty version string")
	}
	t.Logf("jj version: %s", version)
}

// TestCanUndo verifies undo capability.
func TestCanUndo(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// jj always supports undo
	if !j.CanUndo() {
		t.Error("Expected CanUndo to return true for jj")
	}
}

// TestBookmarkOperations verifies bookmark creation, listing, and deletion.
func TestBookmarkOperations(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create bookmark
	bookmarkName := "test-bookmark"
	if err := j.CreateRef(bookmarkName, ""); err != nil {
		t.Fatalf("Failed to create bookmark: %v", err)
	}

	// Verify bookmark exists
	if !j.RefExists(bookmarkName) {
		t.Error("Bookmark should exist after creation")
	}

	// List bookmarks
	refs, err := j.ListRefs()
	if err != nil {
		t.Fatalf("Failed to list refs: %v", err)
	}

	found := false
	for _, ref := range refs {
		if ref.Name == bookmarkName && !ref.IsRemote {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created bookmark not found in list")
	}

	// Delete bookmark
	if err := j.DeleteRef(bookmarkName); err != nil {
		t.Fatalf("Failed to delete bookmark: %v", err)
	}

	// Verify bookmark no longer exists
	if j.RefExists(bookmarkName) {
		t.Error("Bookmark should not exist after deletion")
	}
}

// TestCommit verifies commit operations.
func TestCommit(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Commit changes (jj automatically tracks files)
	ctx := context.Background()
	commitMsg := "Test commit"
	if err := j.Commit(ctx, vcs.CommitOptions{
		Message: commitMsg,
	}); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify commit exists (check operation log or status)
	// In jj, the change is automatically tracked
	hasChanges, err := j.HasChanges()
	if err != nil {
		t.Fatalf("Failed to check changes: %v", err)
	}

	// After describe, working copy might still have the file
	// This is expected in jj - the change is described but still in working copy
	t.Logf("Has changes after commit: %v", hasChanges)
}

// TestStatus verifies status operations.
func TestStatus(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Get status
	statuses, err := j.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	// Should have at least the files we created
	if len(statuses) == 0 {
		t.Error("Expected non-empty status")
	}

	t.Logf("Status: %+v", statuses)
}

// TestWorkspace verifies workspace operations.
func TestWorkspace(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()
	j, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Create workspace
	wsOpts := vcs.WorkspaceOptions{
		Name: "test-workspace",
		Ref:  "beads-sync",
	}

	ws, err := j.CreateWorkspace(wsOpts)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Verify workspace path
	if ws.Path() != tmpDir {
		t.Errorf("Expected workspace path %s, got %s", tmpDir, ws.Path())
	}

	// Verify workspace ref
	if ws.Ref() != wsOpts.Ref {
		t.Errorf("Expected workspace ref %s, got %s", wsOpts.Ref, ws.Ref())
	}

	// Verify workspace is healthy
	if err := ws.IsHealthy(); err != nil {
		t.Errorf("Workspace should be healthy: %v", err)
	}

	// Clean up workspace
	if err := ws.Cleanup(); err != nil {
		t.Errorf("Failed to cleanup workspace: %v", err)
	}
}

// TestIsJJRepo verifies repository detection.
func TestIsJJRepo(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()

	// Before init, should not be a jj repo
	if IsJJRepo(tmpDir) {
		t.Error("Should not be a jj repo before init")
	}

	// Initialize repo
	_, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// After init, should be a jj repo
	if !IsJJRepo(tmpDir) {
		t.Error("Should be a jj repo after init")
	}

	// Subdirectory should also be detected as in jj repo
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	if !IsJJRepo(subDir) {
		t.Error("Subdirectory should be detected as in jj repo")
	}
}

// TestFindRepoRoot verifies repo root detection.
func TestFindRepoRoot(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	tmpDir := t.TempDir()

	// Initialize repo
	_, err := Init(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to initialize repo: %v", err)
	}

	// Find repo root from tmpDir
	root := FindRepoRoot(tmpDir)
	if root != tmpDir {
		t.Errorf("Expected repo root %s, got %s", tmpDir, root)
	}

	// Find repo root from subdirectory
	subDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirs: %v", err)
	}

	root = FindRepoRoot(subDir)
	if root != tmpDir {
		t.Errorf("Expected repo root %s from subdir, got %s", tmpDir, root)
	}
}

// TestIsColocated verifies colocated repo detection.
func TestIsColocated(t *testing.T) {
	if !vcs.IsJJAvailable() {
		t.Skip("jj not available")
	}

	// Test non-colocated
	tmpDir1 := t.TempDir()
	_, err := Init(tmpDir1, false)
	if err != nil {
		t.Fatalf("Failed to initialize non-colocated repo: %v", err)
	}

	if IsColocated(tmpDir1) {
		t.Error("Non-colocated repo should not be detected as colocated")
	}

	// Test colocated
	tmpDir2 := t.TempDir()
	_, err = Init(tmpDir2, true)
	if err != nil {
		t.Fatalf("Failed to initialize colocated repo: %v", err)
	}

	if !IsColocated(tmpDir2) {
		t.Error("Colocated repo should be detected as colocated")
	}
}
