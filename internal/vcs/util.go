package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ===================
// Command Execution Utilities
// ===================

// ExecContext executes a VCS command with timeout and context support.
// This is a common utility for git and jj implementations.
//
// Example:
//
//	output, err := ExecContext(ctx, 30*time.Second, repoRoot, "git", "status", "--porcelain")
func ExecContext(ctx context.Context, timeout time.Duration, workDir string, name string, args ...string) ([]byte, error) {
	// Create context with timeout if specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Create command
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	err := cmd.Run()
	if err != nil {
		// Include stderr in error message for debugging
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

// ExecSimple is a simplified version of ExecContext with default timeout.
// Uses a 30 second timeout by default.
func ExecSimple(workDir string, name string, args ...string) ([]byte, error) {
	return ExecContext(context.Background(), 30*time.Second, workDir, name, args...)
}

// ExecLines executes a command and returns the output as lines.
// Empty lines are filtered out.
func ExecLines(ctx context.Context, timeout time.Duration, workDir string, name string, args ...string) ([]string, error) {
	output, err := ExecContext(ctx, timeout, workDir, name, args...)
	if err != nil {
		return nil, err
	}

	return ParseLines(output), nil
}

// ===================
// Output Parsing Utilities
// ===================

// ParseLines splits command output into non-empty lines.
// This is a common pattern for parsing VCS command output.
func ParseLines(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	lines := strings.Split(string(output), "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}

// ParseKeyValue parses "key: value" format output.
// Common in git and jj informational commands.
func ParseKeyValue(output []byte) map[string]string {
	result := make(map[string]string)
	lines := ParseLines(output)

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}

	return result
}

// SplitFirstLine returns the first line and remaining lines separately.
// Useful for parsing commands where first line has special meaning.
func SplitFirstLine(output []byte) (string, []string) {
	lines := ParseLines(output)
	if len(lines) == 0 {
		return "", nil
	}
	return lines[0], lines[1:]
}

// ===================
// Path Utilities
// ===================

// SanitizePath ensures a path is absolute and clean.
// Relative paths are resolved relative to the given base directory.
func SanitizePath(path string, baseDir string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}

	// Handle absolute paths
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	// Resolve relative paths
	if baseDir == "" {
		return "", fmt.Errorf("cannot resolve relative path without base directory")
	}

	absPath := filepath.Join(baseDir, path)
	return filepath.Clean(absPath), nil
}

// RelativePath returns the relative path from base to target.
// Returns an error if the paths cannot be related.
func RelativePath(base, target string) (string, error) {
	base = filepath.Clean(base)
	target = filepath.Clean(target)

	relPath, err := filepath.Rel(base, target)
	if err != nil {
		return "", fmt.Errorf("cannot determine relative path: %w", err)
	}

	return relPath, nil
}

// IsSubPath returns true if target is inside base directory.
func IsSubPath(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)

	relPath, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}

	// If relative path starts with "..", it's outside base
	return !strings.HasPrefix(relPath, "..")
}

// ===================
// String Utilities
// ===================

// TrimOutput trims whitespace and trailing newlines from command output.
func TrimOutput(output []byte) string {
	return strings.TrimSpace(string(output))
}

// FirstWord returns the first whitespace-separated word from output.
// Useful for extracting single values from command output.
func FirstWord(output []byte) string {
	s := TrimOutput(output)
	if s == "" {
		return ""
	}

	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

// HasPrefix checks if command output starts with a prefix (case-insensitive).
func HasPrefix(output []byte, prefix string) bool {
	s := TrimOutput(output)
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

// ===================
// Error Utilities
// ===================

// IsExitError returns true if the error is an exit error with non-zero status.
func IsExitError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*exec.ExitError)
	return ok
}

// GetExitCode returns the exit code from an error, or -1 if not an exit error.
func GetExitCode(err error) int {
	if err == nil {
		return 0
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}

	return -1
}
