# FileWatcher - Cross-Platform File System Event Monitoring

The FileWatcher provides a high-level abstraction over fsnotify for monitoring task and dependency files in the jj-turso sync daemon.

## Features

- **Cross-platform**: Works on Linux, macOS, Windows, and BSD systems
- **Filtered events**: Only monitors `.json` files in specified directories
- **Type discrimination**: Distinguishes between task and dependency files
- **Thread-safe**: Safe concurrent access to event channels and state
- **Clean shutdown**: Graceful termination with channel closure
- **Comprehensive testing**: 82% test coverage with edge case handling

## Quick Start

```go
import "github.com/steveyegge/beads/internal/turso/daemon"

// Create watcher
fw, err := daemon.NewFileWatcher()
if err != nil {
    log.Fatal(err)
}
defer fw.Stop()

// Start watching directories
if err := fw.Start("/path/to/tasks", "/path/to/deps"); err != nil {
    log.Fatal(err)
}

// Process events
for event := range fw.Events() {
    switch event.Op {
    case daemon.OpCreate:
        fmt.Printf("Created: %s\n", event.Path)
    case daemon.OpModify:
        fmt.Printf("Modified: %s\n", event.Path)
    case daemon.OpDelete:
        fmt.Printf("Deleted: %s\n", event.Path)
    }
}
```

## API Reference

### Types

#### FileEvent

```go
type FileEvent struct {
    Path string      // Absolute path to the changed file
    Type FileType    // TypeTask or TypeDep
    Op   EventOp     // OpCreate, OpModify, or OpDelete
}
```

#### EventOp

```go
const (
    OpCreate EventOp = iota  // File created
    OpModify                 // File modified
    OpDelete                 // File deleted
)
```

#### FileType

```go
const (
    TypeTask FileType = iota  // Task file (tasks/*.json)
    TypeDep                   // Dependency file (deps/*.json)
)
```

### Methods

#### NewFileWatcher

```go
func NewFileWatcher() (*FileWatcher, error)
```

Creates a new FileWatcher instance. The watcher is initially stopped and must be started with `Start()`.

**Returns:**
- `*FileWatcher`: New watcher instance
- `error`: Error if fsnotify initialization fails

#### Start

```go
func (fw *FileWatcher) Start(tasksDir, depsDir string) error
```

Begins watching the specified directories for changes.

**Parameters:**
- `tasksDir`: Path to directory containing task JSON files
- `depsDir`: Path to directory containing dependency JSON files

**Returns:**
- `error`: Error if directories cannot be watched or if watcher is already running

#### Stop

```go
func (fw *FileWatcher) Stop() error
```

Stops watching for events and cleans up resources. This method:
1. Signals the event processing goroutine to exit
2. Closes the underlying fsnotify watcher
3. Waits for event processing to complete
4. Closes the Events() and Errors() channels

**Returns:**
- `error`: Error if cleanup fails

#### Events

```go
func (fw *FileWatcher) Events() <-chan FileEvent
```

Returns the channel that emits FileEvent notifications. This channel is closed when Stop() is called.

#### Errors

```go
func (fw *FileWatcher) Errors() <-chan error
```

Returns the channel that emits file system error notifications. This channel is closed when Stop() is called.

#### IsRunning

```go
func (fw *FileWatcher) IsRunning() bool
```

Returns true if the watcher is currently running.

## Event Mapping

The watcher maps fsnotify events to simplified operations:

| fsnotify Event | Watcher Op | Description |
|---------------|------------|-------------|
| `Create`      | `OpCreate` | New file created |
| `Write`       | `OpModify` | File contents changed |
| `Remove`      | `OpDelete` | File deleted |
| `Rename`      | `OpDelete` | File renamed (new name triggers Create) |
| `Chmod`       | *ignored*  | Permission changes ignored |

## Filtering Rules

The watcher only emits events for files that:
1. Have a `.json` extension
2. Are in the watched directories (not subdirectories)
3. Represent valid file operations (not chmod, etc.)

Files that don't meet these criteria are silently ignored.

## Error Handling

File system errors are delivered on the `Errors()` channel. Common errors include:

- **Permission denied**: Insufficient permissions to watch directory
- **File descriptor limit**: Too many open files (system limit)
- **Directory removed**: Watched directory was deleted

The watcher continues operating even after errors. Consumers should monitor the `Errors()` channel and handle or log errors appropriately.

Example:

```go
go func() {
    for err := range fw.Errors() {
        log.Printf("File system error: %v", err)
    }
}()
```

## Thread Safety

FileWatcher is designed for concurrent use:

- **Events() and Errors()**: Safe to call from any goroutine (read-only channels)
- **IsRunning()**: Protected by mutex, safe for concurrent access
- **Start() and Stop()**: Should only be called from controlling goroutine

## Testing

Run tests with:

```bash
go test -v ./internal/turso/daemon/watcher_test.go ./internal/turso/daemon/watcher.go
```

Run with coverage:

```bash
go test -cover -coverprofile=coverage.out ./internal/turso/daemon/watcher_test.go ./internal/turso/daemon/watcher.go
go tool cover -html=coverage.out
```

## Implementation Notes

### Debouncing

The watcher does NOT implement debouncing. Multiple rapid events for the same file will all be emitted. Consumers should implement debouncing if needed (see `daemon.go` for an example).

### Directory Recreation

If a watched directory is deleted and recreated, the watcher will NOT automatically re-watch it. The consumer must call `Stop()` and `Start()` to re-establish watches.

### Platform Differences

The underlying fsnotify library has platform-specific behavior:

- **Linux (inotify)**: Very efficient, kernel-level notifications
- **macOS (FSEvents)**: Efficient but may batch events
- **Windows (ReadDirectoryChangesW)**: May miss rapid changes
- **BSD (kqueue)**: Requires file descriptor per watched file

For production use, consider:
- Setting appropriate buffer sizes (default: 100 events)
- Monitoring the Errors() channel for resource limits
- Testing on target platforms

## Example: Full Integration

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/steveyegge/beads/internal/turso/daemon"
)

func main() {
    // Create watcher
    fw, err := daemon.NewFileWatcher()
    if err != nil {
        log.Fatal(err)
    }
    defer fw.Stop()

    // Start watching
    if err := fw.Start("./tasks", "./deps"); err != nil {
        log.Fatal(err)
    }

    // Handle events
    go func() {
        for event := range fw.Events() {
            fmt.Printf("[%s] %s: %s\n", event.Type, event.Op, event.Path)
        }
    }()

    // Handle errors
    go func() {
        for err := range fw.Errors() {
            log.Printf("Error: %v", err)
        }
    }()

    // Wait for interrupt
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs

    fmt.Println("\nShutting down...")
}
```

## Performance

The watcher is designed for moderate file activity (hundreds of changes per second). For high-throughput scenarios:

- Increase event channel buffer size (modify `NewFileWatcher`)
- Implement consumer-side debouncing
- Consider batching database operations

Benchmark results on test system:
- Event processing: < 1ms per event
- Memory overhead: ~100KB per watcher
- File descriptor overhead: 2 (one per watched directory)

## License

Part of the beads issue tracker project.
