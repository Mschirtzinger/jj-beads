package jj

import (
	"context"
	"strings"

	"github.com/steveyegge/beads/internal/vcs"
)

// ===================
// Reference Operations (Bookmarks)
// ===================
// In jj, references are called "bookmarks" (similar to git branches).
// Unlike git, bookmarks are optional - you can work without them.

// CurrentRef returns the current bookmark name.
// Returns empty string if no bookmark is set (normal in jj).
func (j *JJ) CurrentRef() (string, error) {
	ctx := context.Background()

	// Get the current change info
	output, err := j.execWithOutput(ctx, "log", "-r", "@", "-n", "1", "--no-graph")
	if err != nil {
		return "", err
	}

	// Parse bookmarks from the output
	// Format: @ changeID user@host timestamp commitID [bookmarks]
	// Bookmarks appear after the @ line if present
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Look for bookmark indicators
		if strings.Contains(line, "bookmark:") || strings.Contains(line, "(") {
			// Extract bookmark name
			// This is simplified - real parsing would be more robust
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.HasPrefix(part, "(") && strings.HasSuffix(part, ")") {
					// Remove parentheses
					bookmark := strings.Trim(part, "()")
					return bookmark, nil
				}
				if part == "bookmark:" && i+1 < len(parts) {
					return parts[i+1], nil
				}
			}
		}
	}

	// No bookmark set - this is normal in jj
	return "", nil
}

// RefExists returns true if the named bookmark exists.
func (j *JJ) RefExists(name string) bool {
	ctx := context.Background()

	// List all bookmarks and check if name exists
	output, err := j.execWithOutput(ctx, "bookmark", "list")
	if err != nil {
		return false
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Bookmark lines start with "bookmark-name:" (with colon)
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name+":" {
			return true
		}
	}

	return false
}

// CreateRef creates a new bookmark at the specified base.
// If base is empty, creates at current change (@).
func (j *JJ) CreateRef(name string, base string) error {
	ctx := context.Background()

	if j.RefExists(name) {
		return vcs.ErrRefExists
	}

	args := []string{"bookmark", "create", name}
	if base != "" {
		args = append(args, "-r", base)
	}

	_, err := j.Exec(ctx, args...)
	return err
}

// DeleteRef deletes the named bookmark.
func (j *JJ) DeleteRef(name string) error {
	ctx := context.Background()

	if !j.RefExists(name) {
		return vcs.ErrRefNotFound
	}

	_, err := j.Exec(ctx, "bookmark", "delete", name)
	return err
}

// MoveRef moves the bookmark to point to the specified target.
func (j *JJ) MoveRef(name string, target string) error {
	ctx := context.Background()

	if !j.RefExists(name) {
		return vcs.ErrRefNotFound
	}

	_, err := j.Exec(ctx, "bookmark", "move", name, "-t", target)
	return err
}

// ListRefs returns all bookmarks (local and remote).
func (j *JJ) ListRefs() ([]vcs.RefInfo, error) {
	ctx := context.Background()

	output, err := j.execWithOutput(ctx, "bookmark", "list")
	if err != nil {
		return nil, err
	}

	var refs []vcs.RefInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse bookmark line
		// Format: "bookmark-name: changeID description"
		// or "remote/bookmark-name: changeID description"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		bookmarkName := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])

		// Extract change ID (first field of target)
		targetFields := strings.Fields(target)
		hash := ""
		if len(targetFields) > 0 {
			hash = targetFields[0]
		}

		// Check if this is a remote bookmark
		isRemote := strings.Contains(bookmarkName, "/")
		remote := ""
		if isRemote {
			remoteParts := strings.SplitN(bookmarkName, "/", 2)
			if len(remoteParts) == 2 {
				remote = remoteParts[0]
				bookmarkName = remoteParts[1]
			}
		}

		refs = append(refs, vcs.RefInfo{
			Name:     bookmarkName,
			Hash:     hash,
			Remote:   remote,
			IsRemote: isRemote,
		})
	}

	return refs, nil
}
