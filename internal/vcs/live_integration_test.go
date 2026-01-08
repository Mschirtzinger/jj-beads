package vcs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/beads/internal/vcs"
	// Import implementations to trigger auto-registration
	_ "github.com/steveyegge/beads/internal/vcs/git"
	_ "github.com/steveyegge/beads/internal/vcs/jj"
)

// TestLiveVCSDetection tests VCS detection in the actual beads repository.
// This test verifies that the factory can detect and create a VCS instance
// for the real working directory.
func TestLiveVCSDetection(t *testing.T) {
	// Get the repo root (2 levels up from internal/vcs/)
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	repoRoot := filepath.Join(testDir, "..", "..")

	// Test detection
	result, err := vcs.Detect(repoRoot)
	if err != nil {
		t.Fatalf("Failed to detect VCS: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil detection result")
	}

	// Verify we detected something
	if !result.HasGit && !result.HasJJ {
		t.Error("Expected to detect at least git or jj")
	}

	t.Logf("Detected VCS type: %s", result.Type)
	t.Logf("Repo root: %s", result.RepoRoot)
	t.Logf("Has git: %v", result.HasGit)
	t.Logf("Has jj: %v", result.HasJJ)
	t.Logf("Colocated: %v", result.Colocated)
}

// TestLiveVCSFactory tests the factory creation with the actual repository.
func TestLiveVCSFactory(t *testing.T) {
	// Get repo root (2 levels up from internal/vcs/)
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	repoRoot := filepath.Join(testDir, "..", "..")

	// Create factory and VCS instance
	factory := vcs.NewFactory()
	v, err := factory.Create(repoRoot)
	if err != nil {
		t.Fatalf("Failed to create VCS instance: %v", err)
	}

	if v == nil {
		t.Fatal("Expected non-nil VCS instance")
	}

	t.Logf("Created VCS: %s", v.Name())
}

// TestLiveVCSOperations tests basic VCS operations on the actual repository.
// These operations are read-only and won't modify the repository state.
func TestLiveVCSOperations(t *testing.T) {
	// Get repo root
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	repoRoot := filepath.Join(testDir, "..", "..")

	// Create VCS instance
	v, err := vcs.GetForPath(repoRoot)
	if err != nil {
		t.Fatalf("Failed to get VCS instance: %v", err)
	}

	// Test VCS type and version
	t.Run("Identity", func(t *testing.T) {
		vcsType := v.Name()
		if vcsType != vcs.TypeGit && vcsType != vcs.TypeJJ && vcsType != vcs.TypeColocate {
			t.Errorf("Unexpected VCS type: %s", vcsType)
		}

		version, err := v.Version()
		if err != nil {
			t.Errorf("Failed to get version: %v", err)
		} else {
			t.Logf("VCS version: %s", version)
		}
	})

	// Test repository information
	t.Run("RepositoryInfo", func(t *testing.T) {
		root, err := v.RepoRoot()
		if err != nil {
			t.Errorf("Failed to get repo root: %v", err)
		} else {
			t.Logf("Repo root: %s", root)
		}

		vcsDir, err := v.VCSDir()
		if err != nil {
			t.Errorf("Failed to get VCS dir: %v", err)
		} else {
			t.Logf("VCS dir: %s", vcsDir)
		}

		if !v.IsInVCS() {
			t.Error("Expected IsInVCS to return true")
		}
	})

	// Test reference operations (read-only)
	t.Run("References", func(t *testing.T) {
		currentRef, err := v.CurrentRef()
		if err != nil {
			t.Errorf("Failed to get current ref: %v", err)
		} else {
			t.Logf("Current ref: %s", currentRef)
		}

		refs, err := v.ListRefs()
		if err != nil {
			t.Errorf("Failed to list refs: %v", err)
		} else {
			t.Logf("Found %d refs", len(refs))
			if len(refs) > 0 {
				t.Logf("Sample ref: %s -> %s", refs[0].Name, refs[0].Hash)
			}
		}
	})

	// Test status operations
	t.Run("Status", func(t *testing.T) {
		hasChanges, err := v.HasChanges()
		if err != nil {
			t.Errorf("Failed to check for changes: %v", err)
		} else {
			t.Logf("Has changes: %v", hasChanges)
		}

		hasRemote := v.HasRemote()
		t.Logf("Has remote: %v", hasRemote)

		if hasRemote {
			remotes, err := v.GetRemotes()
			if err != nil {
				t.Errorf("Failed to get remotes: %v", err)
			} else {
				t.Logf("Found %d remotes", len(remotes))
				for _, remote := range remotes {
					t.Logf("  %s: %s", remote.Name, remote.URL)
				}
			}
		}
	})

	// Test file status
	t.Run("FileStatus", func(t *testing.T) {
		status, err := v.Status()
		if err != nil {
			t.Errorf("Failed to get status: %v", err)
		} else {
			t.Logf("Status returned %d files", len(status))
			if len(status) > 0 {
				t.Logf("Sample file: %s [%s]", status[0].Path, status[0].Status)
			}
		}
	})
}

// TestLiveVCSAvailability tests the binary availability checks.
func TestLiveVCSAvailability(t *testing.T) {
	gitAvailable := vcs.IsGitAvailable()
	jjAvailable := vcs.IsJJAvailable()

	t.Logf("Git available: %v", gitAvailable)
	t.Logf("JJ available: %v", jjAvailable)

	if !gitAvailable && !jjAvailable {
		t.Error("Expected at least git or jj to be available")
	}
}

// TestLiveVCSWorkspaceList tests workspace listing (read-only).
func TestLiveVCSWorkspaceList(t *testing.T) {
	// Get repo root
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	repoRoot := filepath.Join(testDir, "..", "..")

	// Create VCS instance
	v, err := vcs.GetForPath(repoRoot)
	if err != nil {
		t.Fatalf("Failed to get VCS instance: %v", err)
	}

	// List workspaces (should work for both git worktrees and jj)
	workspaces, err := v.ListWorkspaces()
	if err != nil {
		// ListWorkspaces might not be implemented yet, so just log
		t.Logf("ListWorkspaces returned error (might not be implemented): %v", err)
		return
	}

	t.Logf("Found %d workspaces", len(workspaces))
	for _, ws := range workspaces {
		t.Logf("  %s: %s (ref: %s)", ws.Name, ws.Path, ws.Ref)
	}
}

// TestLiveVCSConvenience tests the convenience functions.
func TestLiveVCSConvenience(t *testing.T) {
	// Save and restore working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Change to repo root (2 levels up from internal/vcs/)
	repoRoot := filepath.Join(originalWd, "..", "..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	// Test Get() - should detect from current directory
	v, err := vcs.Get()
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if v == nil {
		t.Fatal("Expected non-nil VCS from Get()")
	}

	t.Logf("Get() returned: %s", v.Name())

	// Test GetForPath()
	v2, err := vcs.GetForPath(".")
	if err != nil {
		t.Fatalf("GetForPath() failed: %v", err)
	}

	if v2 == nil {
		t.Fatal("Expected non-nil VCS from GetForPath()")
	}

	// Types should match
	if v.Name() != v2.Name() {
		t.Errorf("VCS type mismatch: %s vs %s", v.Name(), v2.Name())
	}
}

// TestLiveVCSExec tests raw command execution.
func TestLiveVCSExec(t *testing.T) {
	// Get repo root
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	repoRoot := filepath.Join(testDir, "..", "..")

	// Create VCS instance
	v, err := vcs.GetForPath(repoRoot)
	if err != nil {
		t.Fatalf("Failed to get VCS instance: %v", err)
	}

	ctx := context.Background()

	// Test different commands based on VCS type
	switch v.Name() {
	case vcs.TypeGit:
		output, err := v.Exec(ctx, "status", "--short")
		if err != nil {
			t.Errorf("Exec failed: %v", err)
		} else {
			t.Logf("git status --short output length: %d bytes", len(output))
		}

	case vcs.TypeJJ:
		output, err := v.Exec(ctx, "status")
		if err != nil {
			t.Errorf("Exec failed: %v", err)
		} else {
			t.Logf("jj status output length: %d bytes", len(output))
		}

	case vcs.TypeColocate:
		// For colocated, we don't know which implementation, so just log
		t.Logf("Colocated repo detected, skipping Exec test")
	}
}
