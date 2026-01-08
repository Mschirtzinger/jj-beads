# VCS Factory System

This document explains how the VCS factory and registration system works in jj-beads.

## Overview

The VCS factory provides a plugin-like architecture for git and jj implementations:

1. **Interface Definition** (`vcs.go`) - Defines the VCS interface
2. **Detection** (`detect.go`) - Detects which VCS is present
3. **Registry** (`registry.go`) - Maps VCS types to constructors
4. **Factory** (`factory.go`) - Creates VCS instances using the registry
5. **Implementations** (`git/`, `jj/`) - Auto-register via `init()`

## How It Works

### 1. Auto-Registration

When you import a VCS implementation package, its `init()` function automatically registers itself:

```go
// internal/vcs/git/init.go
package git

import "github.com/steveyegge/beads/internal/vcs"

func init() {
    vcs.Register(vcs.TypeGit, func(path string) (vcs.VCS, error) {
        return New(path)
    })
}
```

### 2. Factory Creation

The factory uses the registry to create VCS instances:

```go
// Get VCS for current directory (auto-detects type)
v, err := vcs.Get()

// Get VCS with specific preference
v, err := vcs.GetWithPreference(vcs.TypeJJ)

// Create factory with options
factory := vcs.NewFactory(
    vcs.WithPreferredType(vcs.TypeGit),
    vcs.WithCache(true),
)
v, err := factory.Create(".")
```

### 3. Detection and Selection

The factory:
1. Detects VCS type using `Detect()`
2. Checks binary availability
3. Applies preferences for colocated repos
4. Looks up constructor in registry
5. Creates and caches instance

```go
result, err := vcs.Detect(".")
// result.Type = TypeColocate (both .jj and .git present)

// Factory prefers jj by default for colocated
v, err := factory.Create(".")
// v.Name() = TypeJJ
```

## Usage Patterns

### Basic Usage

```go
import (
    "github.com/steveyegge/beads/internal/vcs"
    _ "github.com/steveyegge/beads/internal/vcs/git"  // Auto-register
    _ "github.com/steveyegge/beads/internal/vcs/jj"   // Auto-register
)

func main() {
    v, err := vcs.Get()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Using:", v.Name())
}
```

### With Preferences

```go
// Force git even in colocated repo
v, err := vcs.GetWithPreference(vcs.TypeGit)

// Create factory with custom options
factory := vcs.NewFactory(
    vcs.WithPreferredType(vcs.TypeJJ),
    vcs.WithFallbackType(vcs.TypeGit),
    vcs.WithCache(true),
)
```

### Testing

For testing, you can register mock implementations:

```go
func TestMyFeature(t *testing.T) {
    defer vcs.UnregisterAll()  // Clean up after test

    // Register mock
    vcs.Register("mock", func(path string) (vcs.VCS, error) {
        return &mockVCS{}, nil
    })

    // Test code that uses vcs.Get()...
}
```

## Architecture Benefits

### 1. Loose Coupling
- Core VCS package doesn't import git or jj packages
- Implementations are independent
- Easy to add new VCS backends

### 2. Lazy Loading
- Only imported implementations are registered
- Unused implementations don't bloat binary
- Can selectively disable backends with build tags

### 3. Testability
- Mock implementations for testing
- No global state (except registry)
- Clean reset for test isolation

### 4. Flexibility
- Runtime type selection
- User preferences respected
- Graceful fallback handling

## Adding a New VCS Implementation

1. Create package: `internal/vcs/newvcs/`
2. Implement `vcs.VCS` interface
3. Create `init.go` with registration:

```go
package newvcs

import "github.com/steveyegge/beads/internal/vcs"

func init() {
    vcs.Register("newvcs", func(path string) (vcs.VCS, error) {
        return New(path)
    })
}

func New(path string) (vcs.VCS, error) {
    // Implementation...
}
```

4. Import in main package:

```go
import _ "github.com/steveyegge/beads/internal/vcs/newvcs"
```

## Utilities

The `util.go` file provides common helpers:

- `ExecContext()` - Run commands with timeout
- `ParseLines()` - Parse command output
- `SanitizePath()` - Path handling
- `IsSubPath()` - Path validation

Both git and jj implementations use these utilities to avoid code duplication.

## Error Handling

The factory provides clear error messages:

```go
v, err := factory.Create(".")
if errors.Is(err, vcs.ErrNotInVCS) {
    // Not in a repository
}
if errors.Is(err, vcs.ErrVCSNotAvailable) {
    // Binary not installed
}
```

If a type isn't registered, the error message shows available types:

```
no registered constructor for VCS type: unknown (available: [git jj])
```

## Caching

The factory caches VCS instances by path:

```go
// First call - creates instance
v1, _ := factory.Create("/repo")

// Second call - returns cached instance
v2, _ := factory.Create("/repo")
// v1 == v2

// Reset cache
vcs.ResetCache()

// Disable caching
vcs.DisableCache()
factory := vcs.NewFactory(vcs.WithCache(false))
```

## Feature Flags

Environment variables control VCS behavior:

- `BD_VCS_ABSTRACTION=1` - Enable abstraction layer
- `BD_VCS_JJ=1` - Enable jj support
- `BD_VCS_PREFER_JJ=1` - Prefer jj in colocated repos
- `BD_VCS_LOG=1` - Log VCS operations
- `BD_VCS=jj|git` - Override preference

Check feature flags:

```go
if vcs.IsAbstractionEnabled() {
    // Use new abstraction layer
} else {
    // Use legacy git code
}
```

## Files Overview

```
internal/vcs/
├── vcs.go              # Interface definition
├── factory.go          # Factory implementation
├── registry.go         # Registration system
├── detect.go           # VCS detection
├── errors.go           # Error types
├── util.go             # Shared utilities
├── *_test.go           # Tests
├── git/
│   ├── init.go         # Auto-registration
│   ├── git.go          # Git implementation
│   └── ...
└── jj/
    ├── init.go         # Auto-registration
    ├── jj.go           # JJ implementation
    └── ...
```

## Design Principles

1. **Auto-registration** - Implementations self-register via `init()`
2. **Interface-based** - All VCS operations through `vcs.VCS` interface
3. **Detection-based** - Runtime detection of VCS type
4. **Preference-aware** - Respects user preferences for colocated repos
5. **Cached by default** - Instance caching for performance
6. **Error clarity** - Clear error messages with available options
7. **Test-friendly** - Easy to mock and reset for testing

## See Also

- `vcs.go` - Full VCS interface documentation
- `detect.go` - Detection logic details
- `architecture-design.md` - Overall VCS abstraction design
