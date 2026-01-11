// Package daemon provides file system watching and synchronization for the jj-turso sync daemon.
//
// The daemon monitors tasks/*.json and deps/*.json files for changes and manages
// synchronization with the Turso (libSQL) cache database.
//
// # Architecture
//
// The daemon consists of several components:
//
//   - FileWatcher: Cross-platform file system event monitoring using fsnotify
//   - Daemon: Orchestrates file watching, change debouncing, and database sync
//   - OpLog: Integration with jj operation log for version control events (polling-based)
//
// # Operation Log Watching (OpLog)
//
// The OpLog component provides real-time monitoring of jj operations. It polls
// the jj operation log to detect changes to task and dependency files:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	config := daemon.OpLogWatcherConfig{
//	    RepoPath:     "/path/to/jj/repo",
//	    PollInterval: 100 * time.Millisecond,
//	    TasksDir:     "tasks",
//	    DepsDir:      "deps",
//	}
//
//	err := daemon.WatchOpLog(ctx, config, func(entries []daemon.OpLogEntry) error {
//	    for _, entry := range entries {
//	        log.Printf("Operation: %s", entry.Description)
//	        for _, file := range entry.AffectedFiles {
//	            // Sync file to Turso
//	        }
//	    }
//	    return nil
//	})
//
// The OpLog watcher:
//   - Polls `jj op log` at configurable intervals (default: 100ms)
//   - Parses operation metadata (ID, description, timestamp)
//   - Runs `jj op show` to determine affected files
//   - Delivers operations in chronological order (oldest first)
//   - Continues watching even after transient errors
//
// Performance characteristics:
//   - ParseOpLog: ~28μs for 50 operations, 28KB memory
//   - GetAffectedFiles: ~28μs per operation, 12KB memory
//   - Configurable poll interval and batch size
//
// # File Watching
//
// The FileWatcher component provides a high-level abstraction over fsnotify:
//
//	fw, err := daemon.NewFileWatcher()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer fw.Stop()
//
//	if err := fw.Start("/path/to/tasks", "/path/to/deps"); err != nil {
//	    log.Fatal(err)
//	}
//
//	for event := range fw.Events() {
//	    switch event.Op {
//	    case daemon.OpCreate:
//	        fmt.Printf("Created: %s (%s)\n", event.Path, event.Type)
//	    case daemon.OpModify:
//	        fmt.Printf("Modified: %s (%s)\n", event.Path, event.Type)
//	    case daemon.OpDelete:
//	        fmt.Printf("Deleted: %s (%s)\n", event.Path, event.Type)
//	    }
//	}
//
// The watcher automatically:
//   - Filters to only .json files
//   - Distinguishes between task and dependency files
//   - Handles directory recreation gracefully
//   - Provides clean shutdown with channel closure
//
// # Event Types
//
// FileEvent contains three pieces of information:
//   - Path: Absolute path to the changed file
//   - Type: TypeTask or TypeDep
//   - Op: OpCreate, OpModify, or OpDelete
//
// The watcher maps fsnotify operations as follows:
//   - fsnotify.Create → OpCreate
//   - fsnotify.Write → OpModify
//   - fsnotify.Remove → OpDelete
//   - fsnotify.Rename → OpDelete (the new name triggers a separate Create)
//
// # Thread Safety
//
// FileWatcher is thread-safe. Multiple goroutines can safely call:
//   - Events() and Errors() (read-only channel access)
//   - IsRunning() (protected by mutex)
//
// Start() and Stop() should only be called from a single controlling goroutine.
//
// # Error Handling
//
// File system errors are delivered on the Errors() channel. The watcher continues
// operating even after errors (e.g., if a watched directory is temporarily unavailable).
//
// Common errors include:
//   - Permission denied when accessing files
//   - File system limits (too many open files)
//   - Watched directory deleted and recreated
//
// Consumers should monitor the Errors() channel and log or handle errors appropriately.
//
// # Graceful Shutdown
//
// Call Stop() to gracefully shut down the watcher:
//
//	if err := fw.Stop(); err != nil {
//	    log.Printf("Error stopping watcher: %v", err)
//	}
//
// Stop() will:
//  1. Signal the event processing goroutine to exit
//  2. Close the underlying fsnotify watcher
//  3. Wait for the event loop to finish
//  4. Close the Events() and Errors() channels
//
// After Stop() is called, IsRunning() returns false and the channels are closed.
package daemon
