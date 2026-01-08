# JJ (Jujutsu) VCS Implementation

This package implements the VCS interface for Jujutsu (jj), a Git-compatible version control system with powerful features including automatic change tracking, operation log with undo, first-class conflicts, and stable change IDs.

## Architecture

The implementation wraps the `jj` CLI using `os/exec` to provide a Go interface for beads issue tracker integration. This approach was chosen over FFI bindings because:
- Simple and straightforward
- No dependencies on Rust toolchain
- Works with any jj version installed on the system
- Easy to test and debug

## Files

### Core Implementation

- **`jj.go`** - Main JJ struct and core operations
  - VCS identity (Name, Version)
  - Repository information (RepoRoot, VCSDir, IsInVCS)
  - Raw command execution (Exec)
  - Undo/recovery operations (CanUndo, Undo, GetOperationLog)

### Feature Modules

- **`bookmarks.go`** - Reference operations (jj's equivalent of git branches)
  - Create, delete, move bookmarks
  - List bookmarks (local and remote)
  - Check bookmark existence
  - Get current bookmark

- **`changes.go`** - Commit and file operations
  - File status tracking
  - Commit operations (using `jj describe` and `jj new`)
  - Change detection
  - Conflict detection and listing

- **`remote.go`** - Remote repository operations
  - Remote detection and listing
  - Fetch and pull operations
  - Push operations
  - Divergence checking
  - File extraction from refs

- **`workspace.go`** - Isolated workspace operations
  - Workspace creation using jj's change model
  - File synchronization
  - Workspace commit/push/pull
  - Workspace cleanup

- **`repo.go`** - Repository detection utilities
  - Check if directory is in jj repo
  - Find repository root
  - Detect colocated mode

### Testing

- **`jj_test.go`** - Comprehensive unit tests
  - Repository initialization (normal and colocated)
  - Bookmark operations
  - Commit operations
  - Status operations
  - Workspace operations
  - Repository detection

## Key Concepts

### JJ vs Git Differences

1. **No Staging Area**: jj automatically tracks changes. The `Add()` method is a no-op for interface compatibility.

2. **Bookmarks vs Branches**: In jj, references are called "bookmarks" and are optional. You can work without them.

3. **Change IDs**: jj uses stable change IDs that persist through rewrites, unlike git commit hashes which change on rebase.

4. **Automatic Undo**: jj's operation log allows undoing any operation with `jj op undo`.

5. **First-Class Conflicts**: Conflicts are stored in commits and can be resolved later without blocking operations.

### Workspace Strategy

Unlike git worktrees which create separate directories, jj workspaces use the change model:

1. Create a new change with a descriptive message
2. Create a bookmark pointing to this change
3. Return to user's original work (undisturbed)
4. Workspace operations switch to the sync change temporarily

This provides isolation without requiring separate working directories.

## Usage

### Basic Initialization

```go
import "github.com/steveyegge/beads/internal/vcs/jj"

// Initialize new jj repository
j, err := jj.Init("/path/to/repo", false)

// Initialize colocated repository (jj + git)
j, err := jj.Init("/path/to/repo", true)

// Open existing repository
j, err := jj.New("/path/to/repo")
```

### Bookmark Operations

```go
// Create bookmark
err := j.CreateRef("feature-branch", "")

// List bookmarks
refs, err := j.ListRefs()

// Delete bookmark
err := j.DeleteRef("feature-branch")
```

### Commit Operations

```go
ctx := context.Background()

// Describe current changes (like git commit --amend)
err := j.Commit(ctx, vcs.CommitOptions{
    Message: "Implement feature X",
})

// Describe and create new change
err := j.Commit(ctx, vcs.CommitOptions{
    Message: "Implement feature Y",
    CreateNew: true,
})
```

### Workspace Operations

```go
// Create isolated workspace for sync operations
ws, err := j.CreateWorkspace(vcs.WorkspaceOptions{
    Name: "beads-sync",
    Ref:  "beads-sync",
})

// Commit in workspace
err := ws.Commit(ctx, "Sync changes", nil)

// Push workspace
err := ws.Push(ctx, "origin")

// Cleanup workspace
err := ws.Cleanup()
```

## Implementation Status

### ✅ Implemented

- [x] Core repository operations
- [x] Bookmark management (create, delete, move, list)
- [x] Commit operations (describe, new)
- [x] Status operations
- [x] Conflict detection
- [x] Remote operations (fetch, push, pull)
- [x] Workspace abstraction using change model
- [x] Operation log and undo
- [x] Repository detection
- [x] Colocated mode support
- [x] Comprehensive unit tests

### ⚠️ Partial Implementation

- [ ] **Author override** - jj doesn't have direct `--author` flag (line 135 in changes.go)
- [ ] **Non-colocated remote listing** - Requires parsing `.jj/repo/store/git/config` (line 43 in remote.go)
- [ ] **Force push** - jj doesn't have exact `--force` equivalent (line 101 in remote.go)
- [ ] **Operation log parsing** - Simplified parser, could be more robust (line 200 in jj.go)

### 📝 Future Enhancements

- [ ] Structured output parsing (consider `jj log --template` for JSON output)
- [ ] Better error categorization and mapping
- [ ] Change ID tracking in issues
- [ ] Conflict resolution helpers
- [ ] Sparse checkout support in workspaces

## Testing

Run tests with jj installed:

```bash
# Run all tests
go test ./internal/vcs/jj/...

# Run specific test
go test -run TestBookmarkOperations ./internal/vcs/jj/

# Run with verbose output
go test -v ./internal/vcs/jj/...
```

Tests will skip automatically if jj is not installed.

## Dependencies

- **jj binary**: Must be installed and in PATH
- **Go 1.21+**: For module support
- **github.com/steveyegge/beads/internal/vcs**: VCS interface definitions

## Integration

To integrate with the beads factory, update `internal/vcs/factory.go`:

```go
case TypeJJ:
    // Import jj package
    return jj.New(result.RepoRoot)
```

## References

- [JJ Documentation](https://jj-vcs.github.io/jj/latest/)
- [JJ CLI Reference](https://jj-vcs.github.io/jj/latest/cli-reference/)
- [JJ Capabilities Research](../../../.agents/research/jj-capabilities-research.md)
- [VCS Architecture Design](../../../.agents/architecture-design.md)
