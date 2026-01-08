package vcs

import "errors"

// Common errors returned by VCS operations.
//
// These errors can be checked using errors.Is() for proper error handling:
//
//	if errors.Is(err, vcs.ErrNotInVCS) {
//	    // Handle case where we're outside any VCS repository
//	}
var (
	// ErrNotInVCS is returned when the operation requires being inside
	// a VCS repository but none was found.
	ErrNotInVCS = errors.New("not in a VCS repository")

	// ErrVCSNotAvailable is returned when the required VCS binary
	// (git or jj) is not installed or not in PATH.
	ErrVCSNotAvailable = errors.New("VCS binary not available")

	// ErrWorkspaceExists is returned when attempting to create a workspace
	// that already exists.
	ErrWorkspaceExists = errors.New("workspace already exists")

	// ErrWorkspaceNotFound is returned when attempting to operate on
	// a workspace that doesn't exist.
	ErrWorkspaceNotFound = errors.New("workspace not found")

	// ErrRefExists is returned when attempting to create a reference
	// (branch/bookmark) that already exists.
	ErrRefExists = errors.New("reference already exists")

	// ErrRefNotFound is returned when attempting to operate on
	// a reference that doesn't exist.
	ErrRefNotFound = errors.New("reference not found")

	// ErrNoRemote is returned when an operation requires a remote
	// but none is configured.
	ErrNoRemote = errors.New("no remote configured")

	// ErrConflicts is returned when an operation cannot complete
	// due to unresolved conflicts.
	ErrConflicts = errors.New("unresolved conflicts")

	// ErrDirtyWorkspace is returned when an operation requires
	// a clean workspace but there are uncommitted changes.
	ErrDirtyWorkspace = errors.New("workspace has uncommitted changes")

	// ErrNotSupported is returned when an operation is not supported
	// by the current VCS backend.
	ErrNotSupported = errors.New("operation not supported by this VCS")

	// ErrDetached is returned when an operation requires being on
	// a branch/bookmark but HEAD is detached (git) or no bookmark
	// is set (jj).
	ErrDetached = errors.New("not on a branch or bookmark")

	// ErrAborted is returned when an operation was aborted by the user
	// or a pre-operation hook.
	ErrAborted = errors.New("operation aborted")

	// ErrPushRejected is returned when a push is rejected by the remote,
	// typically due to non-fast-forward updates.
	ErrPushRejected = errors.New("push rejected by remote")

	// ErrMergeRequired is returned when a pull results in divergent
	// histories that require a merge.
	ErrMergeRequired = errors.New("merge required")

	// ErrTimeout is returned when a VCS operation exceeds its timeout.
	ErrTimeout = errors.New("operation timed out")
)

// IsRetryable returns true if the error is likely to succeed on retry.
// This is useful for transient network errors or temporary lock conflicts.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Timeouts are often transient
	if errors.Is(err, ErrTimeout) {
		return true
	}

	// Push rejections might succeed after a pull
	if errors.Is(err, ErrPushRejected) {
		return true
	}

	// Merge required can be resolved by user action
	if errors.Is(err, ErrMergeRequired) {
		return true
	}

	return false
}

// IsUserActionRequired returns true if the error requires user intervention
// to resolve (conflicts, divergent history, etc).
func IsUserActionRequired(err error) bool {
	if err == nil {
		return false
	}

	// Conflicts need manual resolution
	if errors.Is(err, ErrConflicts) {
		return true
	}

	// Divergent histories need merge decision
	if errors.Is(err, ErrMergeRequired) {
		return true
	}

	// Push rejected usually means divergent remote
	if errors.Is(err, ErrPushRejected) {
		return true
	}

	return false
}

// IsFatal returns true if the error indicates a non-recoverable state
// that requires manual intervention or re-initialization.
func IsFatal(err error) bool {
	if err == nil {
		return false
	}

	// Not in VCS means we can't do anything
	if errors.Is(err, ErrNotInVCS) {
		return true
	}

	// Binary not available means we can't execute commands
	if errors.Is(err, ErrVCSNotAvailable) {
		return true
	}

	return false
}
