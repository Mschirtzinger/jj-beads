package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// detect populates git repository information
func (g *Git) detect(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Use git rev-parse to get all info in one call
	cmd := exec.Command("git", "rev-parse", "--git-dir", "--git-common-dir", "--show-toplevel")
	cmd.Dir = absPath

	output, err := cmd.Output()
	if err != nil {
		return vcs.ErrNotInVCS
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 3 {
		return fmt.Errorf("unexpected git rev-parse output: got %d lines, expected 3", len(lines))
	}

	gitDir := strings.TrimSpace(lines[0])
	commonDir := strings.TrimSpace(lines[1])
	repoRoot := strings.TrimSpace(lines[2])

	// Convert to absolute paths
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(absPath, gitDir)
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(absPath, commonDir)
	}

	g.vcsDir = gitDir
	g.repoRoot = normalizeRepoRoot(repoRoot)

	// Detect worktree by comparing git-dir and common-dir
	absGitDir, _ := filepath.Abs(gitDir)
	absCommonDir, _ := filepath.Abs(commonDir)
	g.isWorktree = absGitDir != absCommonDir

	// Set main repo root
	if g.isWorktree {
		g.mainRepoRoot = filepath.Dir(absCommonDir)
	} else {
		g.mainRepoRoot = g.repoRoot
	}

	return nil
}

// normalizeRepoRoot normalizes the repository root path
// Resolves symlinks and canonicalizes case on case-insensitive filesystems
func normalizeRepoRoot(path string) string {
	// Normalize Windows paths
	path = filepath.FromSlash(path)

	// Resolve symlinks
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	return path
}

// HasRemote returns true if any remote is configured
func (g *Git) HasRemote() bool {
	cmd := exec.Command("git", "remote")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return len(strings.TrimSpace(string(output))) > 0
}

// GetRemotes returns information about configured remotes
func (g *Git) GetRemotes() ([]vcs.RemoteInfo, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git remote -v failed: %w", err)
	}

	// Parse output: "origin url (fetch)"
	remotes := make(map[string]string) // name -> url
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		url := parts[1]

		// Only record fetch URLs (skip push duplicates)
		if len(parts) >= 3 && strings.Contains(parts[2], "fetch") {
			remotes[name] = url
		} else if _, exists := remotes[name]; !exists {
			// Record first occurrence if no fetch line yet
			remotes[name] = url
		}
	}

	var result []vcs.RemoteInfo
	for name, url := range remotes {
		result = append(result, vcs.RemoteInfo{
			Name: name,
			URL:  url,
		})
	}

	return result, nil
}

// IsInRebaseOrMerge returns true if currently in a rebase or merge operation
func (g *Git) IsInRebaseOrMerge() bool {
	// Check for rebase-merge directory (interactive rebase)
	rebaseMergePath := filepath.Join(g.vcsDir, "rebase-merge")
	if _, err := os.Stat(rebaseMergePath); err == nil {
		return true
	}

	// Check for rebase-apply directory (non-interactive rebase)
	rebaseApplyPath := filepath.Join(g.vcsDir, "rebase-apply")
	if _, err := os.Stat(rebaseApplyPath); err == nil {
		return true
	}

	// Check for MERGE_HEAD (merge in progress)
	mergeHeadPath := filepath.Join(g.vcsDir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHeadPath); err == nil {
		return true
	}

	return false
}

// HasUnmergedPaths returns true if there are unmerged paths (merge conflict state)
func (g *Git) HasUnmergedPaths() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	// Check for unmerged status codes (DD, AU, UD, UA, DU, AA, UU)
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) >= 2 {
			status := line[:2]
			if status == "DD" || status == "AU" || status == "UD" ||
				status == "UA" || status == "DU" || status == "AA" || status == "UU" {
				return true, nil
			}
		}
	}

	return false, nil
}

// HasConflicts returns true if there are unresolved conflicts
func (g *Git) HasConflicts() (bool, error) {
	return g.HasUnmergedPaths()
}

// GetConflictedFiles returns the list of files with conflicts
func (g *Git) GetConflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	var conflicts []string
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		if status == "DD" || status == "AU" || status == "UD" ||
			status == "UA" || status == "DU" || status == "AA" || status == "UU" {
			filepath := strings.TrimSpace(line[3:])
			conflicts = append(conflicts, filepath)
		}
	}

	return conflicts, nil
}
