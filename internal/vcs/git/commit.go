package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// HasChanges returns true if there are uncommitted changes
// If paths are specified, only checks those paths
func (g *Git) HasChanges(paths ...string) (bool, error) {
	args := []string{"status", "--porcelain"}
	args = append(args, paths...)

	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// Add stages files for commit
func (g *Git) Add(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	args := append([]string{"add"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %w\n%s", err, string(output))
	}

	return nil
}

// Status returns the status of files in the working directory
func (g *Git) Status(paths ...string) ([]vcs.FileStatus, error) {
	args := []string{"status", "--porcelain"}
	args = append(args, paths...)

	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	var statuses []vcs.FileStatus
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		if len(line) < 3 {
			continue
		}

		// Parse status format: XY filename
		// X = staged status, Y = unstaged status
		staged := line[0:1]
		unstaged := line[1:2]
		filepath := strings.TrimSpace(line[3:])

		status := vcs.FileStatus{
			Path:       filepath,
			Status:     parseStatusCode(unstaged),
			StagedCode: parseStatusCode(staged),
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// parseStatusCode converts git status code to vcs.StatusCode
func parseStatusCode(code string) vcs.StatusCode {
	switch code {
	case " ":
		return vcs.StatusUnmodified
	case "M":
		return vcs.StatusModified
	case "A":
		return vcs.StatusAdded
	case "D":
		return vcs.StatusDeleted
	case "R":
		return vcs.StatusRenamed
	case "C":
		return vcs.StatusCopied
	case "?":
		return vcs.StatusUntracked
	case "!":
		return vcs.StatusIgnored
	case "U":
		return vcs.StatusConflict
	default:
		return vcs.StatusUnmodified
	}
}

// Commit creates a commit with the specified options
func (g *Git) Commit(ctx context.Context, opts vcs.CommitOptions) error {
	if opts.Message == "" {
		return fmt.Errorf("commit message is required")
	}

	// Stage files if paths specified
	if len(opts.Paths) > 0 {
		if err := g.Add(opts.Paths); err != nil {
			return err
		}
	}

	// Build commit arguments
	args := []string{"commit", "-m", opts.Message}

	if opts.Author != "" {
		args = append(args, "--author", opts.Author)
	}

	if opts.NoGPGSign {
		args = append(args, "--no-gpg-sign")
	}

	if opts.NoVerify {
		args = append(args, "--no-verify")
	}

	if opts.AllowEmpty {
		args = append(args, "--allow-empty")
	}

	// Add paths with -- to ensure they're treated as paths
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %w\n%s", err, string(output))
	}

	return nil
}
