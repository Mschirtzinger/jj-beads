package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/steveyegge/beads/internal/vcs"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "git-vcs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestNew(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if g.Name() != vcs.TypeGit {
		t.Errorf("Name() = %v, want %v", g.Name(), vcs.TypeGit)
	}

	if !g.IsInVCS() {
		t.Error("IsInVCS() = false, want true")
	}
}

func TestVersion(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	version, err := g.Version()
	if err != nil {
		t.Fatalf("Version() failed: %v", err)
	}

	if version == "" {
		t.Error("Version() returned empty string")
	}
}

func TestRepoRoot(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	root, err := g.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() failed: %v", err)
	}

	// Should be the temp directory we created
	// Use EvalSymlinks to handle /var -> /private/var on macOS
	absRepoPath, _ := filepath.Abs(repoPath)
	absRepoPath, _ = filepath.EvalSymlinks(absRepoPath)
	rootResolved, _ := filepath.EvalSymlinks(root)
	if rootResolved != absRepoPath {
		t.Errorf("RepoRoot() = %v, want %v", root, absRepoPath)
	}
}

func TestCurrentRef(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create initial commit so we're not in unborn state
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	exec.Command("git", "-C", repoPath, "add", "test.txt").Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "initial").Run()

	ref, err := g.CurrentRef()
	if err != nil {
		t.Fatalf("CurrentRef() failed: %v", err)
	}

	// Default branch should be main or master
	if ref != "main" && ref != "master" {
		t.Errorf("CurrentRef() = %v, want main or master", ref)
	}
}

func TestRefOperations(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", repoPath, "add", "test.txt").Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "initial").Run()

	// Test CreateRef
	if err := g.CreateRef("feature", ""); err != nil {
		t.Errorf("CreateRef() failed: %v", err)
	}

	// Test RefExists
	if !g.RefExists("feature") {
		t.Error("RefExists(feature) = false, want true")
	}

	// Test ListRefs
	refs, err := g.ListRefs()
	if err != nil {
		t.Fatalf("ListRefs() failed: %v", err)
	}

	foundFeature := false
	for _, ref := range refs {
		if ref.Name == "feature" && !ref.IsRemote {
			foundFeature = true
			break
		}
	}
	if !foundFeature {
		t.Error("ListRefs() did not include feature branch")
	}

	// Test DeleteRef
	if err := g.DeleteRef("feature"); err != nil {
		t.Errorf("DeleteRef() failed: %v", err)
	}

	if g.RefExists("feature") {
		t.Error("RefExists(feature) = true after delete, want false")
	}
}

func TestHasChanges(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// No changes initially
	hasChanges, err := g.HasChanges()
	if err != nil {
		t.Fatalf("HasChanges() failed: %v", err)
	}
	if hasChanges {
		t.Error("HasChanges() = true for empty repo, want false")
	}

	// Create a file
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Should have changes now
	hasChanges, err = g.HasChanges()
	if err != nil {
		t.Fatalf("HasChanges() failed: %v", err)
	}
	if !hasChanges {
		t.Error("HasChanges() = false after creating file, want true")
	}
}

func TestCommit(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create a file
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Commit
	ctx := context.Background()
	opts := vcs.CommitOptions{
		Message: "test commit",
		Paths:   []string{"test.txt"},
	}

	if err := g.Commit(ctx, opts); err != nil {
		t.Errorf("Commit() failed: %v", err)
	}

	// Should have no changes after commit
	hasChanges, _ := g.HasChanges()
	if hasChanges {
		t.Error("HasChanges() = true after commit, want false")
	}
}

func TestStatus(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create a file
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Get status
	statuses, err := g.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("Status() returned %d files, want 1", len(statuses))
	}

	status := statuses[0]
	if status.Path != "test.txt" {
		t.Errorf("Status()[0].Path = %v, want test.txt", status.Path)
	}

	if status.Status != vcs.StatusUntracked {
		t.Errorf("Status()[0].Status = %v, want %v", status.Status, vcs.StatusUntracked)
	}
}

func TestWorkspace(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(repoPath, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", repoPath, "add", "test.txt").Run()
	exec.Command("git", "-C", repoPath, "commit", "-m", "initial").Run()

	// Create workspace
	opts := vcs.WorkspaceOptions{
		Name: "test-workspace",
		Ref:  "test-branch",
	}

	ws, err := g.CreateWorkspace(opts)
	if err != nil {
		t.Fatalf("CreateWorkspace() failed: %v", err)
	}
	defer ws.Cleanup()

	// Verify workspace path exists
	if _, err := os.Stat(ws.Path()); err != nil {
		t.Errorf("workspace path does not exist: %v", err)
	}

	// Verify ref
	if ws.Ref() != "test-branch" {
		t.Errorf("Ref() = %v, want test-branch", ws.Ref())
	}

	// Test IsHealthy
	if err := ws.IsHealthy(); err != nil {
		t.Errorf("IsHealthy() failed: %v", err)
	}

	// Test ListWorkspaces
	workspaces, err := g.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces() failed: %v", err)
	}

	// Should have at least 2 (main repo + our workspace)
	if len(workspaces) < 2 {
		t.Errorf("ListWorkspaces() returned %d workspaces, want at least 2", len(workspaces))
	}
}

func TestHasRemote(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// No remote initially
	if g.HasRemote() {
		t.Error("HasRemote() = true for local repo, want false")
	}

	// Add a remote
	exec.Command("git", "-C", repoPath, "remote", "add", "origin", "https://example.com/repo.git").Run()

	// Should have remote now
	if !g.HasRemote() {
		t.Error("HasRemote() = false after adding remote, want true")
	}

	// Test GetRemotes
	remotes, err := g.GetRemotes()
	if err != nil {
		t.Fatalf("GetRemotes() failed: %v", err)
	}

	if len(remotes) != 1 {
		t.Fatalf("GetRemotes() returned %d remotes, want 1", len(remotes))
	}

	if remotes[0].Name != "origin" {
		t.Errorf("GetRemotes()[0].Name = %v, want origin", remotes[0].Name)
	}
}

func TestConflicts(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	g, err := New(repoPath)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// No conflicts initially
	hasConflicts, err := g.HasConflicts()
	if err != nil {
		t.Fatalf("HasConflicts() failed: %v", err)
	}
	if hasConflicts {
		t.Error("HasConflicts() = true for clean repo, want false")
	}

	// No unmerged paths
	hasUnmerged, err := g.HasUnmergedPaths()
	if err != nil {
		t.Fatalf("HasUnmergedPaths() failed: %v", err)
	}
	if hasUnmerged {
		t.Error("HasUnmergedPaths() = true for clean repo, want false")
	}

	// Not in rebase/merge
	if g.IsInRebaseOrMerge() {
		t.Error("IsInRebaseOrMerge() = true for clean repo, want false")
	}
}
