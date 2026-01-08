package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// Fetch fetches from the specified remote and reference
// If remote is empty, uses the default remote (origin)
func (g *Git) Fetch(ctx context.Context, remote, ref string) error {
	if !g.HasRemote() {
		return nil // Skip if no remotes configured
	}

	if remote == "" {
		remote = "origin"
	}

	args := []string{"fetch", remote}
	if ref != "" {
		args = append(args, ref)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch failed: %w\n%s", err, string(output))
	}

	return nil
}

// Pull pulls changes from the remote
func (g *Git) Pull(ctx context.Context, opts vcs.PullOptions) error {
	if !g.HasRemote() {
		return nil // Skip if no remotes configured (local-only mode)
	}

	// Determine remote
	remote := opts.Remote
	if remote == "" {
		// Try to get configured remote for current branch
		branch, err := g.CurrentRef()
		if err != nil {
			return err
		}

		if branch != "" {
			remoteCmd := exec.Command("git", "config", "--get", fmt.Sprintf("branch.%s.remote", branch))
			remoteCmd.Dir = g.repoRoot
			remoteOutput, err := remoteCmd.Output()
			if err == nil {
				remote = strings.TrimSpace(string(remoteOutput))
			}
		}

		// Default to origin if not configured
		if remote == "" {
			remote = "origin"
		}
	}

	// Determine ref
	ref := opts.Ref
	if ref == "" {
		// Use current branch
		var err error
		ref, err = g.CurrentRef()
		if err != nil {
			return err
		}
		if ref == "" {
			return vcs.ErrDetached
		}
	}

	// Build pull arguments
	args := []string{"pull"}

	if opts.Rebase {
		args = append(args, "--rebase")
	}

	if opts.FFOnly {
		args = append(args, "--ff-only")
	}

	args = append(args, remote, ref)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Check for common error types
		if strings.Contains(outputStr, "non-fast-forward") {
			return vcs.ErrMergeRequired
		}
		if strings.Contains(outputStr, "conflicts") || strings.Contains(outputStr, "CONFLICT") {
			return vcs.ErrConflicts
		}

		return fmt.Errorf("git pull failed: %w\n%s", err, outputStr)
	}

	return nil
}

// Push pushes changes to the remote
func (g *Git) Push(ctx context.Context, opts vcs.PushOptions) error {
	if !g.HasRemote() {
		return nil // Skip if no remotes configured (local-only mode)
	}

	// Determine remote
	remote := opts.Remote
	if remote == "" {
		// Try to get configured remote for current branch
		branch, err := g.CurrentRef()
		if err != nil {
			return err
		}

		if branch != "" {
			remoteCmd := exec.Command("git", "config", "--get", fmt.Sprintf("branch.%s.remote", branch))
			remoteCmd.Dir = g.repoRoot
			remoteOutput, err := remoteCmd.Output()
			if err == nil {
				remote = strings.TrimSpace(string(remoteOutput))
			}
		}

		// Default to origin if not configured
		if remote == "" {
			remote = "origin"
		}
	}

	// Determine ref
	ref := opts.Ref
	if ref == "" {
		// Use current branch
		var err error
		ref, err = g.CurrentRef()
		if err != nil {
			return err
		}
		if ref == "" {
			return vcs.ErrDetached
		}
	}

	// Build push arguments
	args := []string{"push"}

	if opts.SetUpstream {
		args = append(args, "-u")
	}

	if opts.Force {
		args = append(args, "--force")
	}

	args = append(args, remote, ref)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Check for push rejection
		if strings.Contains(outputStr, "rejected") || strings.Contains(outputStr, "non-fast-forward") {
			return vcs.ErrPushRejected
		}

		return fmt.Errorf("git push failed: %w\n%s", err, outputStr)
	}

	return nil
}
