package jj

import (
	"os"
	"path/filepath"
)

// ===================
// Repository Detection and Initialization
// ===================

// IsJJRepo returns true if the given path is inside a jj repository.
func IsJJRepo(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Walk up the directory tree looking for .jj
	current := absPath
	for {
		jjDir := filepath.Join(current, ".jj")
		if info, err := os.Stat(jjDir); err == nil && info.IsDir() {
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return false
		}
		current = parent
	}
}

// FindRepoRoot finds the jj repository root by walking up the directory tree.
// Returns empty string if not in a jj repository.
func FindRepoRoot(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	current := absPath
	for {
		jjDir := filepath.Join(current, ".jj")
		if info, err := os.Stat(jjDir); err == nil && info.IsDir() {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return ""
		}
		current = parent
	}
}

// IsColocated returns true if the repository is colocated (has both .jj and .git).
func IsColocated(repoRoot string) bool {
	jjDir := filepath.Join(repoRoot, ".jj")
	gitPath := filepath.Join(repoRoot, ".git")

	jjInfo, jjErr := os.Stat(jjDir)
	gitInfo, gitErr := os.Stat(gitPath)

	return jjErr == nil && jjInfo.IsDir() && gitErr == nil &&
		(gitInfo.IsDir() || gitInfo.Mode().IsRegular())
}
