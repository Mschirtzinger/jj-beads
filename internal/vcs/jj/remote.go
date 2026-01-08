package jj

import (
	"context"
	"fmt"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// ===================
// Remote Operations
// ===================

// HasRemote returns true if any remote is configured.
func (j *JJ) HasRemote() bool {
	remotes, err := j.GetRemotes()
	if err != nil {
		return false
	}
	return len(remotes) > 0
}

// GetRemotes returns information about configured remotes.
// For jj, this queries the git remotes (since jj uses git as backend).
func (j *JJ) GetRemotes() ([]vcs.RemoteInfo, error) {
	ctx := context.Background()

	// jj doesn't have a direct "remote list" command
	// In colocated mode, we can use git config
	// For non-colocated, we need to query the git backend
	if j.isColocated {
		// Use git to get remotes
		output, err := j.execGit(ctx, "remote", "-v")
		if err != nil {
			return []vcs.RemoteInfo{}, nil
		}
		return parseGitRemotes(output), nil
	}

	// For non-colocated repos, remotes are stored in .jj/repo/store/git/config
	// This is more complex - for now return empty
	// TODO: Parse git config from .jj/repo/store/git/config
	return []vcs.RemoteInfo{}, nil
}

// execGit executes a git command (for colocated repos).
func (j *JJ) execGit(ctx context.Context, args ...string) (string, error) {
	if !j.isColocated {
		return "", fmt.Errorf("git commands only available in colocated mode")
	}

	output, err := j.Exec(ctx, append([]string{"git"}, args...)...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// parseGitRemotes parses the output of `git remote -v`.
// Format: "origin  https://github.com/user/repo.git (fetch)"
func parseGitRemotes(output string) []vcs.RemoteInfo {
	var remotes []vcs.RemoteInfo
	seen := make(map[string]bool)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		url := fields[1]

		// Avoid duplicates (fetch and push lines)
		if seen[name] {
			continue
		}
		seen[name] = true

		remotes = append(remotes, vcs.RemoteInfo{
			Name: name,
			URL:  url,
		})
	}

	return remotes
}

// Fetch fetches from the specified remote and reference.
func (j *JJ) Fetch(ctx context.Context, remote, ref string) error {
	args := []string{"git", "fetch"}

	if remote != "" {
		args = append(args, "--remote", remote)
	}

	if ref != "" {
		// jj git fetch doesn't take a ref argument directly
		// It fetches all refs from the remote
	}

	_, err := j.Exec(ctx, args...)
	return err
}

// Pull pulls changes from the remote.
// In jj, this is equivalent to fetch (no auto-merge).
func (j *JJ) Pull(ctx context.Context, opts vcs.PullOptions) error {
	// jj doesn't have a pull command - use fetch
	return j.Fetch(ctx, opts.Remote, opts.Ref)
}

// Push pushes changes to the remote.
func (j *JJ) Push(ctx context.Context, opts vcs.PushOptions) error {
	args := []string{"git", "push"}

	if opts.Remote != "" {
		args = append(args, "--remote", opts.Remote)
	}

	if opts.Ref != "" {
		args = append(args, "-b", opts.Ref)
	}

	if opts.Force {
		// jj git push doesn't have a force flag
		// Would need to use --allow-backwards or similar
	}

	_, err := j.Exec(ctx, args...)
	if err != nil {
		// Check if push was rejected
		if strings.Contains(err.Error(), "rejected") ||
			strings.Contains(err.Error(), "non-fast-forward") {
			return vcs.ErrPushRejected
		}
		return err
	}

	return nil
}

// ===================
// Diff Operations
// ===================

// HasDivergence checks if local and remote refs have diverged.
func (j *JJ) HasDivergence(local, remote string) (vcs.DivergenceInfo, error) {
	ctx := context.Background()

	// Use jj log to check divergence
	// This is a simplified implementation
	// Real implementation would need to count commits ahead/behind

	// Get commits in local but not in remote
	aheadOutput, err := j.execWithOutput(ctx, "log", "-r",
		fmt.Sprintf("%s..%s", remote, local), "-n", "100")
	if err != nil {
		return vcs.DivergenceInfo{}, err
	}

	// Get commits in remote but not in local
	behindOutput, err := j.execWithOutput(ctx, "log", "-r",
		fmt.Sprintf("%s..%s", local, remote), "-n", "100")
	if err != nil {
		return vcs.DivergenceInfo{}, err
	}

	// Count commits (simplified - count non-empty lines)
	aheadCount := countCommits(aheadOutput)
	behindCount := countCommits(behindOutput)

	info := vcs.DivergenceInfo{
		LocalAhead:  aheadCount,
		RemoteAhead: behindCount,
		IsDiverged:  aheadCount > 0 && behindCount > 0,
	}

	info.IsSignificant = info.LocalAhead > vcs.SignificantDivergenceThreshold ||
		info.RemoteAhead > vcs.SignificantDivergenceThreshold

	return info, nil
}

// countCommits counts the number of commits in log output.
func countCommits(output string) int {
	if strings.TrimSpace(output) == "" {
		return 0
	}

	count := 0
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Lines starting with @ or ○ indicate a commit
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@") || strings.HasPrefix(line, "○") ||
			strings.HasPrefix(line, "◆") {
			count++
		}
	}
	return count
}

// ExtractFileFromRef extracts a file's content from a specific ref.
func (j *JJ) ExtractFileFromRef(ref, path string) ([]byte, error) {
	ctx := context.Background()

	// Use jj cat to get file content at a specific revision
	output, err := j.Exec(ctx, "cat", "-r", ref, path)
	if err != nil {
		return nil, err
	}

	return output, nil
}
