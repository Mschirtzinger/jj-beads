// Package sync provides the synchronization bridge between file-based storage and Turso cache.
//
// Overview
//
// The sync package implements the core synchronization logic for the jj-turso architecture.
// It reads task and dependency files from disk (written by jj) and updates the Turso cache
// database for fast queries.
//
// Architecture
//
// The syncer watches for file changes and syncs them to the database:
//
//	File System (jj working copy)
//	     ├── tasks/*.json          → TaskFile structs
//	     └── deps/*.json           → DepFile structs
//	                                      ↓
//	                                   Syncer
//	                                      ↓
//	                                   Turso DB
//	                                   (cached for fast queries)
//
// Usage
//
// Basic usage:
//
//	// Open database
//	database, err := db.Open("file:.beads/turso.db")
//	if err != nil {
//	    return err
//	}
//	defer database.Close()
//
//	// Initialize schema (first time only)
//	if err := database.InitSchema(); err != nil {
//	    return err
//	}
//
//	// Create syncer
//	syncer := sync.New(database, nil)
//
//	// Full sync from files
//	if err := syncer.FullSync("tasks/", "deps/"); err != nil {
//	    return err
//	}
//
// Incremental sync:
//
//	// Sync single task file
//	if err := syncer.SyncTask("tasks/bd-xyz.json"); err != nil {
//	    return err
//	}
//
//	// Sync single dependency file
//	if err := syncer.SyncDep("deps/bd-abc--blocks--bd-xyz.json"); err != nil {
//	    return err
//	}
//
//	// Delete task from cache
//	if err := syncer.DeleteTask("bd-xyz"); err != nil {
//	    return err
//	}
//
// Integration with Daemon
//
// The sync package is designed to be used by the sync daemon (to be implemented):
//
//	1. Daemon watches jj op log for changes
//	2. On file changes, daemon calls syncer methods:
//	   - File created/modified → SyncTask() or SyncDep()
//	   - File deleted → DeleteTask() or DeleteDep()
//	3. After changes, daemon calls RefreshBlockedCache()
//	4. Dashboard/CLI queries Turso for ready work
//
// Error Handling
//
// The syncer is resilient to individual file failures:
//
//   - Invalid files are logged and skipped
//   - Database errors are returned to caller
//   - FullSync continues processing even if some files fail
//
// Concurrency
//
// The syncer is safe for concurrent use:
//
//   - Multiple syncers can share the same database (WAL mode)
//   - Upsert operations are idempotent
//   - Last write wins for conflicts
//
// Example:
//
//	// Multiple goroutines can sync simultaneously
//	var wg sync.WaitGroup
//	wg.Add(2)
//
//	go func() {
//	    defer wg.Done()
//	    syncer.FullSync("tasks/", "deps/")
//	}()
//
//	go func() {
//	    defer wg.Done()
//	    syncer.SyncTask("tasks/bd-new.json")
//	}()
//
//	wg.Wait()
package sync
