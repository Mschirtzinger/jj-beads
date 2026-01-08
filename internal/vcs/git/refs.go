package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// CurrentRef returns the current branch name
// Returns empty string if in detached HEAD state
func (g *Git) CurrentRef() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		// Check if detached HEAD
		if strings.Contains(err.Error(), "not a symbolic ref") {
			return "", nil // Detached HEAD
		}
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// RefExists returns true if the named reference exists
func (g *Git) RefExists(name string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = g.repoRoot
	return cmd.Run() == nil
}

// CreateRef creates a new branch at the specified base
// If base is empty, creates at current HEAD
func (g *Git) CreateRef(name string, base string) error {
	if g.RefExists(name) {
		return vcs.ErrRefExists
	}

	var cmd *exec.Cmd
	if base == "" {
		cmd = exec.Command("git", "branch", name)
	} else {
		cmd = exec.Command("git", "branch", name, base)
	}
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch: %w\n%s", err, string(output))
	}

	return nil
}

// DeleteRef deletes the named branch
func (g *Git) DeleteRef(name string) error {
	if !g.RefExists(name) {
		return vcs.ErrRefNotFound
	}

	cmd := exec.Command("git", "branch", "-D", name)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete branch: %w\n%s", err, string(output))
	}

	return nil
}

// MoveRef moves the branch to point to the specified target
func (g *Git) MoveRef(name string, target string) error {
	if !g.RefExists(name) {
		return vcs.ErrRefNotFound
	}

	cmd := exec.Command("git", "branch", "-f", name, target)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to move branch: %w\n%s", err, string(output))
	}

	return nil
}

// ListRefs returns all references (local and remote)
func (g *Git) ListRefs() ([]vcs.RefInfo, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname) %(objectname)")
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref failed: %w", err)
	}

	var refs []vcs.RefInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		refName := parts[0]
		hash := parts[1]

		// Parse ref type
		ref := vcs.RefInfo{
			Hash: hash,
		}

		if strings.HasPrefix(refName, "refs/heads/") {
			// Local branch
			ref.Name = strings.TrimPrefix(refName, "refs/heads/")
			ref.IsRemote = false
		} else if strings.HasPrefix(refName, "refs/remotes/") {
			// Remote-tracking branch
			remotePath := strings.TrimPrefix(refName, "refs/remotes/")
			parts := strings.SplitN(remotePath, "/", 2)
			if len(parts) == 2 {
				ref.Remote = parts[0]
				ref.Name = parts[1]
				ref.IsRemote = true
			}
		} else {
			// Skip tags and other refs
			continue
		}

		refs = append(refs, ref)
	}

	return refs, nil
}

// GetCommitHash returns the commit hash for the given reference
func (g *Git) GetCommitHash(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve ref %s: %w", ref, err)
	}

	return strings.TrimSpace(string(output)), nil
}

// HasDivergence checks if local and remote refs have diverged
func (g *Git) HasDivergence(local, remote string) (vcs.DivergenceInfo, error) {
	info := vcs.DivergenceInfo{}

	// Get commits in local but not in remote
	aheadCmd := exec.Command("git", "rev-list", "--count", remote+".."+local)
	aheadCmd.Dir = g.repoRoot
	aheadOutput, err := aheadCmd.Output()
	if err != nil {
		return info, fmt.Errorf("failed to count ahead commits: %w", err)
	}
	fmt.Sscanf(strings.TrimSpace(string(aheadOutput)), "%d", &info.LocalAhead)

	// Get commits in remote but not in local
	behindCmd := exec.Command("git", "rev-list", "--count", local+".."+remote)
	behindCmd.Dir = g.repoRoot
	behindOutput, err := behindCmd.Output()
	if err != nil {
		return info, fmt.Errorf("failed to count behind commits: %w", err)
	}
	fmt.Sscanf(strings.TrimSpace(string(behindOutput)), "%d", &info.RemoteAhead)

	info.IsDiverged = info.LocalAhead > 0 && info.RemoteAhead > 0
	info.IsSignificant = (info.LocalAhead + info.RemoteAhead) >= vcs.SignificantDivergenceThreshold

	return info, nil
}

// ExtractFileFromRef extracts a file's content from a specific ref
func (g *Git) ExtractFileFromRef(ref, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", ref+":"+path)
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to extract file from ref: %w", err)
	}

	return output, nil
}
