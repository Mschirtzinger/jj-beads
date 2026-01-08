package vcs

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectionResult contains information about the detected VCS
type DetectionResult struct {
	// Type is the detected VCS type
	Type Type

	// RepoRoot is the repository root directory path
	RepoRoot string

	// VCSDir is the VCS metadata directory path (.git or .jj)
	VCSDir string

	// HasGit indicates a .git directory/file was found
	HasGit bool

	// HasJJ indicates a .jj directory was found
	HasJJ bool

	// Colocated indicates both git and jj are present
	Colocated bool

	// IsWorktree indicates this is a git worktree (not main repo)
	IsWorktree bool

	// MainRepoRoot is the main repo root (different from RepoRoot for worktrees)
	MainRepoRoot string
}

// Detect identifies the VCS type for a given directory.
//
// Detection precedence:
//  1. Check for .jj directory (indicates jj or colocated mode)
//  2. Check for .git directory or file (indicates git or worktree)
//  3. Walk up parent directories until VCS found or root reached
//
// For colocated repositories (both .jj and .git present), the Type
// will be TypeColocate. Use PreferredVCS() to determine which
// implementation to use.
//
// Returns ErrNotInVCS if no VCS is found.
func Detect(path string) (*DetectionResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	result := &DetectionResult{}

	// Walk up the directory tree
	current := absPath
	for {
		jjDir := filepath.Join(current, ".jj")
		gitPath := filepath.Join(current, ".git")

		// Check for .jj directory
		if info, err := os.Stat(jjDir); err == nil && info.IsDir() {
			result.HasJJ = true
			if result.RepoRoot == "" {
				result.RepoRoot = current
				result.VCSDir = jjDir
			}
		}

		// Check for .git (directory or file for worktrees)
		if info, err := os.Stat(gitPath); err == nil {
			result.HasGit = true

			if info.Mode().IsRegular() {
				// .git is a file - this is a worktree
				result.IsWorktree = true
				mainRoot, vcsDir := resolveGitWorktreeRoot(current, gitPath)
				if result.RepoRoot == "" {
					result.RepoRoot = current
					result.VCSDir = vcsDir
				}
				result.MainRepoRoot = mainRoot
			} else if info.IsDir() {
				// .git is a directory - regular repo
				if result.RepoRoot == "" {
					result.RepoRoot = current
					result.VCSDir = gitPath
				}
				result.MainRepoRoot = current
			}
		}

		// If we found VCS markers, determine the type and return
		if result.HasJJ || result.HasGit {
			result.Colocated = result.HasJJ && result.HasGit

			switch {
			case result.HasJJ && result.HasGit:
				result.Type = TypeColocate
			case result.HasJJ:
				result.Type = TypeJJ
			default:
				result.Type = TypeGit
			}

			// Ensure MainRepoRoot is set
			if result.MainRepoRoot == "" {
				result.MainRepoRoot = result.RepoRoot
			}

			return result, nil
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding VCS
			return nil, ErrNotInVCS
		}
		current = parent
	}
}

// resolveGitWorktreeRoot resolves the main repository root from a worktree's .git file.
// Returns (mainRepoRoot, worktreeGitDir).
//
// Git worktrees have a .git file (not directory) containing:
//
//	gitdir: /path/to/main/.git/worktrees/worktree-name
//
// We parse this to find the main repository.
func resolveGitWorktreeRoot(worktreePath, gitFile string) (string, string) {
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return worktreePath, gitFile
	}

	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return worktreePath, gitFile
	}

	gitDir := strings.TrimPrefix(line, "gitdir: ")

	// Handle relative paths
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreePath, gitDir)
	}

	// Clean the path
	gitDir = filepath.Clean(gitDir)

	// Find the main repo by looking for /worktrees/ in path
	// gitdir points to: /main/.git/worktrees/name
	// We want: /main
	if idx := strings.Index(gitDir, string(filepath.Separator)+"worktrees"+string(filepath.Separator)); idx > 0 {
		mainGitDir := gitDir[:idx]
		return filepath.Dir(mainGitDir), gitDir
	}

	// For beads-worktrees pattern: /main/.git/beads-worktrees/name
	if idx := strings.Index(gitDir, string(filepath.Separator)+"beads-worktrees"+string(filepath.Separator)); idx > 0 {
		mainGitDir := gitDir[:idx]
		return filepath.Dir(mainGitDir), gitDir
	}

	return worktreePath, gitDir
}

// PreferredVCS returns the preferred VCS type for colocated repositories.
//
// Preference order:
//  1. BD_VCS environment variable ("git" or "jj")
//  2. Default preference (jj for colocated, as it's more capable)
//
// For non-colocated repos, this returns the detected type directly.
func PreferredVCS() Type {
	if pref := os.Getenv("BD_VCS"); pref != "" {
		switch strings.ToLower(pref) {
		case "jj", "jujutsu":
			return TypeJJ
		case "git":
			return TypeGit
		}
	}

	// Default: prefer jj for colocated repos
	// Rationale: jj has superior undo, conflict handling, and operation logging
	return TypeJJ
}

// IsJJAvailable checks if the jj command is available on the system
func IsJJAvailable() bool {
	// Check common paths where jj might be installed
	paths := []string{
		"/usr/local/bin/jj",
		"/opt/homebrew/bin/jj",
		filepath.Join(os.Getenv("HOME"), ".cargo/bin/jj"),
	}

	// Check PATH
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		for _, dir := range strings.Split(pathEnv, string(os.PathListSeparator)) {
			jjPath := filepath.Join(dir, "jj")
			if _, err := os.Stat(jjPath); err == nil {
				return true
			}
		}
	}

	// Check specific paths as fallback
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// IsGitAvailable checks if the git command is available on the system
func IsGitAvailable() bool {
	// Check common paths where git might be installed
	paths := []string{
		"/usr/bin/git",
		"/usr/local/bin/git",
		"/opt/homebrew/bin/git",
	}

	// Check PATH
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		for _, dir := range strings.Split(pathEnv, string(os.PathListSeparator)) {
			gitPath := filepath.Join(dir, "git")
			if _, err := os.Stat(gitPath); err == nil {
				return true
			}
		}
	}

	// Check specific paths as fallback
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// DetectWithAvailability performs detection and checks binary availability.
// Returns an error if the required VCS binary is not available.
func DetectWithAvailability(path string) (*DetectionResult, error) {
	result, err := Detect(path)
	if err != nil {
		return nil, err
	}

	// Check that the required binary is available
	switch result.Type {
	case TypeGit:
		if !IsGitAvailable() {
			return nil, ErrVCSNotAvailable
		}
	case TypeJJ:
		if !IsJJAvailable() {
			return nil, ErrVCSNotAvailable
		}
	case TypeColocate:
		// For colocated, we need at least one to be available
		hasGit := IsGitAvailable()
		hasJJ := IsJJAvailable()
		if !hasGit && !hasJJ {
			return nil, ErrVCSNotAvailable
		}
		// If only one is available, adjust preference
		if hasJJ && !hasGit {
			// Update result to indicate jj preference
			result.HasGit = false
		} else if hasGit && !hasJJ {
			// Update result to indicate git preference
			result.HasJJ = false
			result.Type = TypeGit
			result.Colocated = false
		}
	}

	return result, nil
}

// MustDetect is like Detect but returns a zero result if not in VCS.
// Useful for cases where being outside VCS is acceptable.
func MustDetect(path string) *DetectionResult {
	result, err := Detect(path)
	if err != nil {
		return &DetectionResult{}
	}
	return result
}
