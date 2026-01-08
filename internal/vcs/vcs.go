// Package vcs provides a unified interface for version control operations.
//
// This package abstracts the differences between git and jj (Jujutsu),
// enabling beads to work seamlessly with either VCS backend. The design
// follows a strategy pattern with runtime detection and factory creation.
//
// # Architecture
//
// The VCS interface defines operations needed by beads:
//   - Repository discovery and status
//   - Reference management (branches/bookmarks)
//   - Commit and sync operations
//   - Isolated workspaces for sync branch functionality
//
// # Usage
//
//	// Auto-detect VCS type
//	v, err := vcs.Get()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Check VCS type
//	fmt.Println("Using:", v.Name())
//
//	// Create workspace for sync operations
//	ws, err := v.CreateWorkspace(vcs.WorkspaceOptions{
//	    Name: "beads-sync",
//	    Ref:  "beads-sync",
//	})
//
// # Implementations
//
//   - internal/vcs/git: Git implementation using worktrees
//   - internal/vcs/jj: Jujutsu implementation using changes/bookmarks
//
// See architecture-design.md for full design documentation.
package vcs

import (
	"context"
	"time"
)

// Type represents the VCS backend type
type Type string

const (
	// TypeGit indicates a git-only repository
	TypeGit Type = "git"

	// TypeJJ indicates a jj-only repository (non-colocated)
	TypeJJ Type = "jj"

	// TypeColocate indicates a colocated repository (jj + git together)
	TypeColocate Type = "colocate"
)

// String returns the string representation of the VCS type
func (t Type) String() string {
	return string(t)
}

// VCS defines the interface for version control operations.
// Implementations exist for git (internal/vcs/git) and jj (internal/vcs/jj).
//
// The interface is designed to accommodate both git's and jj's unique
// capabilities while providing a unified experience for beads users.
type VCS interface {
	// ===================
	// Identity
	// ===================

	// Name returns the VCS type (git, jj, or colocate)
	Name() Type

	// Version returns the VCS binary version string
	Version() (string, error)

	// ===================
	// Repository Information
	// ===================

	// RepoRoot returns the repository root directory path.
	// For worktrees, this returns the main repository root.
	RepoRoot() (string, error)

	// VCSDir returns the VCS metadata directory path.
	// For git: the .git directory (or worktree git dir)
	// For jj: the .jj directory
	VCSDir() (string, error)

	// IsInVCS returns true if the current directory is inside a VCS repository
	IsInVCS() bool

	// ===================
	// Reference Operations
	// ===================
	// References are branches in git, bookmarks in jj.
	// These are named pointers to specific commits/changes.

	// CurrentRef returns the current branch name (git) or bookmark (jj).
	// Returns empty string if in detached HEAD state (git) or no bookmark (jj).
	CurrentRef() (string, error)

	// RefExists returns true if the named reference exists
	RefExists(name string) bool

	// CreateRef creates a new reference at the specified base.
	// If base is empty, creates at current HEAD/@.
	CreateRef(name string, base string) error

	// DeleteRef deletes the named reference
	DeleteRef(name string) error

	// MoveRef moves the reference to point to the specified target
	MoveRef(name string, target string) error

	// ListRefs returns all references (local and remote)
	ListRefs() ([]RefInfo, error)

	// ===================
	// Status Operations
	// ===================

	// HasChanges returns true if there are uncommitted changes.
	// If paths are specified, only checks those paths.
	HasChanges(paths ...string) (bool, error)

	// HasUnmergedPaths returns true if there are unmerged paths (merge conflict state)
	HasUnmergedPaths() (bool, error)

	// IsInRebaseOrMerge returns true if currently in a rebase or merge operation
	IsInRebaseOrMerge() bool

	// HasRemote returns true if any remote is configured
	HasRemote() bool

	// GetRemotes returns information about configured remotes
	GetRemotes() ([]RemoteInfo, error)

	// ===================
	// File Operations
	// ===================

	// Add stages files for commit.
	// In jj, this is a no-op as files are auto-tracked.
	Add(paths []string) error

	// Status returns the status of files in the working directory.
	// If paths are specified, only checks those paths.
	Status(paths ...string) ([]FileStatus, error)

	// ===================
	// Commit Operations
	// ===================

	// Commit creates a commit with the specified options.
	// In git: stages and commits files.
	// In jj: describes the current change and optionally creates a new one.
	Commit(ctx context.Context, opts CommitOptions) error

	// GetCommitHash returns the commit hash for the given reference.
	// For jj, returns the commit ID (not change ID).
	GetCommitHash(ref string) (string, error)

	// ===================
	// Remote Operations
	// ===================

	// Fetch fetches from the specified remote and reference.
	// If remote is empty, uses the default remote.
	Fetch(ctx context.Context, remote, ref string) error

	// Pull pulls changes from the remote.
	// In jj, this is equivalent to fetch (no auto-merge).
	Pull(ctx context.Context, opts PullOptions) error

	// Push pushes changes to the remote.
	Push(ctx context.Context, opts PushOptions) error

	// ===================
	// Diff Operations
	// ===================

	// HasDivergence checks if local and remote refs have diverged.
	// Returns information about how many commits are ahead/behind.
	HasDivergence(local, remote string) (DivergenceInfo, error)

	// ExtractFileFromRef extracts a file's content from a specific ref.
	// Used for 3-way merge operations.
	ExtractFileFromRef(ref, path string) ([]byte, error)

	// ===================
	// Workspace Operations
	// ===================
	// Workspaces provide isolated operations for sync branch functionality.
	// In git, this uses worktrees. In jj, this uses changes.

	// CreateWorkspace creates an isolated workspace for sync operations.
	// The workspace allows working on a different branch/bookmark without
	// affecting the user's working directory.
	CreateWorkspace(opts WorkspaceOptions) (Workspace, error)

	// ListWorkspaces returns information about existing workspaces
	ListWorkspaces() ([]WorkspaceInfo, error)

	// ===================
	// Conflict Detection
	// ===================

	// HasConflicts returns true if there are unresolved conflicts.
	// In jj, this checks for conflicted files that can still be committed.
	HasConflicts() (bool, error)

	// GetConflictedFiles returns the list of files with conflicts
	GetConflictedFiles() ([]string, error)

	// ===================
	// Undo/Recovery
	// ===================

	// CanUndo returns true if undo is supported and possible.
	// Always true for jj, limited for git.
	CanUndo() bool

	// Undo undoes the last operation.
	// In jj: uses operation log. In git: limited support via reflog.
	Undo(ctx context.Context) error

	// GetOperationLog returns recent VCS operations.
	// Full support in jj, limited in git.
	GetOperationLog(limit int) ([]OperationInfo, error)

	// ===================
	// Raw Command Execution
	// ===================

	// Exec executes a raw VCS command (escape hatch).
	// Use sparingly; prefer interface methods.
	Exec(ctx context.Context, args ...string) ([]byte, error)
}

// Workspace provides isolated operations for sync branch functionality.
//
// In git, a workspace is implemented as a worktree - a separate working
// directory that shares the same .git directory.
//
// In jj, a workspace is implemented using the change model - creating
// a temporary change for sync operations without affecting the user's
// working directory.
//
// The workspace abstraction allows the sync daemon to commit to a
// separate branch without disturbing the user's current work.
type Workspace interface {
	// Path returns the filesystem path to the workspace.
	// For git: the worktree directory path.
	// For jj: the repository root (changes work in-place).
	Path() string

	// Ref returns the reference (branch/bookmark) this workspace is based on
	Ref() string

	// SyncToWorkspace copies a file from the main repo to the workspace.
	// srcPath is absolute, dstRelPath is relative to workspace.
	SyncToWorkspace(srcPath, dstRelPath string) error

	// SyncFromWorkspace copies a file from the workspace to the main repo.
	// srcRelPath is relative to workspace, dstPath is absolute.
	SyncFromWorkspace(srcRelPath, dstPath string) error

	// HasChanges returns true if there are uncommitted changes in the workspace.
	// If paths are specified, only checks those paths.
	HasChanges(paths ...string) (bool, error)

	// Commit commits changes in this workspace with the given message.
	// paths specifies which files to commit (empty = all changes).
	Commit(ctx context.Context, message string, paths []string) error

	// Push pushes the workspace's reference to the remote.
	// If remote is empty, uses the configured remote.
	Push(ctx context.Context, remote string) error

	// Pull pulls changes from the remote into the workspace.
	// If remote is empty, uses the configured remote.
	Pull(ctx context.Context, remote string) error

	// Cleanup removes the workspace and cleans up resources.
	// For git: removes the worktree.
	// For jj: abandons the sync change if empty.
	Cleanup() error

	// IsHealthy verifies the workspace is in a good state.
	// Returns error describing any issues found.
	IsHealthy() error
}

// ===================
// Supporting Types
// ===================

// RefInfo contains information about a reference (branch/bookmark)
type RefInfo struct {
	// Name is the reference name (e.g., "main", "beads-sync")
	Name string

	// Hash is the commit hash (git) or change ID (jj)
	Hash string

	// Remote is the remote name for remote refs, empty for local
	Remote string

	// IsRemote indicates if this is a remote-tracking reference
	IsRemote bool
}

// RemoteInfo contains information about a remote repository
type RemoteInfo struct {
	// Name is the remote name (e.g., "origin")
	Name string

	// URL is the remote URL
	URL string
}

// FileStatus represents the status of a file in the working directory
type FileStatus struct {
	// Path is the file path relative to repository root
	Path string

	// Status is the working directory status
	Status StatusCode

	// StagedCode is the staging area status (git only)
	// In jj, this is always StatusUnmodified as there's no staging area.
	StagedCode StatusCode
}

// StatusCode represents file status codes
type StatusCode string

const (
	StatusUnmodified StatusCode = " " // No changes
	StatusModified   StatusCode = "M" // Modified
	StatusAdded      StatusCode = "A" // Added/new file
	StatusDeleted    StatusCode = "D" // Deleted
	StatusRenamed    StatusCode = "R" // Renamed
	StatusCopied     StatusCode = "C" // Copied
	StatusUntracked  StatusCode = "?" // Untracked
	StatusIgnored    StatusCode = "!" // Ignored
	StatusConflict   StatusCode = "U" // Unmerged/conflict
)

// CommitOptions configures a commit operation
type CommitOptions struct {
	// Message is the commit message (required)
	Message string

	// Paths specifies files to commit. Empty = all staged changes (git) or all changes (jj).
	Paths []string

	// Author overrides the commit author (optional, format: "Name <email>")
	Author string

	// NoGPGSign disables GPG signing (git only)
	NoGPGSign bool

	// NoVerify skips pre-commit hooks
	NoVerify bool

	// AllowEmpty allows creating an empty commit
	AllowEmpty bool

	// CreateNew creates a new change after commit (jj only).
	// In git, the working directory is always clean after commit.
	// In jj, setting this true calls "jj new" after describe.
	CreateNew bool
}

// PullOptions configures a pull operation
type PullOptions struct {
	// Remote is the remote name. Empty uses default.
	Remote string

	// Ref is the reference to pull. Empty uses current branch.
	Ref string

	// Rebase uses rebase instead of merge (git only)
	Rebase bool

	// FFOnly only allows fast-forward merges
	FFOnly bool
}

// PushOptions configures a push operation
type PushOptions struct {
	// Remote is the remote name. Empty uses default.
	Remote string

	// Ref is the reference to push. Empty uses current branch.
	Ref string

	// SetUpstream configures the upstream tracking reference
	SetUpstream bool

	// Force enables force push (use with caution!)
	Force bool
}

// DivergenceInfo describes divergence between local and remote refs
type DivergenceInfo struct {
	// LocalAhead is the number of commits local is ahead of remote
	LocalAhead int

	// RemoteAhead is the number of commits remote is ahead of local
	RemoteAhead int

	// IsDiverged is true if both local and remote have unique commits
	IsDiverged bool

	// IsSignificant is true if divergence exceeds threshold for auto-merge.
	// When true, consider manual intervention or reset-to-remote.
	IsSignificant bool
}

// WorkspaceOptions configures workspace creation
type WorkspaceOptions struct {
	// Name is the workspace identifier (e.g., "beads-sync")
	Name string

	// Path is the filesystem path for the workspace (git worktree path)
	// For jj, this may be ignored as workspaces work in-place.
	Path string

	// Ref is the branch/bookmark to base the workspace on.
	// Will be created if it doesn't exist.
	Ref string

	// Sparse enables sparse checkout (git only).
	// Only the paths in SparsePaths will be checked out.
	Sparse bool

	// SparsePaths are the paths to include in sparse checkout.
	// Only relevant if Sparse is true.
	SparsePaths []string
}

// WorkspaceInfo describes an existing workspace
type WorkspaceInfo struct {
	// Name is the workspace identifier
	Name string

	// Path is the filesystem path
	Path string

	// Ref is the current reference
	Ref string

	// IsValid indicates if the workspace is healthy
	IsValid bool
}

// OperationInfo describes a VCS operation (primarily for jj operation log)
type OperationInfo struct {
	// ID is the operation identifier
	ID string

	// Timestamp is when the operation occurred
	Timestamp time.Time

	// Description describes what the operation did
	Description string

	// User is the user who performed the operation
	User string

	// Args are the command arguments that triggered the operation
	Args []string
}

// ===================
// Constants
// ===================

// SignificantDivergenceThreshold is the number of commits at which
// divergence is considered significant and may require manual intervention.
const SignificantDivergenceThreshold = 5

// DefaultSyncBranch is the default branch name for sync operations
const DefaultSyncBranch = "beads-sync"
