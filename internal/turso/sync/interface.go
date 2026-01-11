// Package sync provides interfaces and implementations for synchronizing
// file-based task storage with Turso cache database.
package sync

// Syncer keeps Turso cache in sync with file-based task storage.
//
// The syncer is responsible for reading task and dependency files from disk,
// validating them, and updating the Turso cache database accordingly.
// It handles both incremental sync (single file changes) and full sync
// (all files in directory).
//
// The syncer is designed to be resilient - individual file failures should
// not stop the entire sync process. Errors are logged and the sync continues
// with other files.
type Syncer interface {
	// SyncTask reads a task file and updates Turso cache.
	//
	// The taskPath should be an absolute path to a task JSON file.
	// The file is read, validated, and upserted into the database.
	//
	// Returns an error if the file cannot be read, is invalid,
	// or the database update fails.
	//
	// Example:
	//   err := syncer.SyncTask("/path/to/tasks/bd-xyz.json")
	SyncTask(taskPath string) error

	// SyncDep reads a dependency file and updates Turso cache.
	//
	// The depPath should be an absolute path to a dependency JSON file.
	// The file is read, validated, and upserted into the database.
	//
	// Returns an error if the file cannot be read, is invalid,
	// or the database update fails.
	//
	// Example:
	//   err := syncer.SyncDep("/path/to/deps/bd-abc--blocks--bd-xyz.json")
	SyncDep(depPath string) error

	// DeleteTask removes a task from Turso cache.
	//
	// This should be called when a task file is deleted from disk.
	// The task is removed from the tasks table, and any related
	// blocked cache entries are also cleaned up.
	//
	// Returns an error if the database operation fails.
	// Returns nil if the task doesn't exist (idempotent).
	//
	// Example:
	//   err := syncer.DeleteTask("bd-xyz")
	DeleteTask(taskID string) error

	// DeleteDep removes a dependency from Turso cache.
	//
	// This should be called when a dependency file is deleted from disk.
	// The dependency is removed from the deps table.
	//
	// Returns an error if the database operation fails.
	// Returns nil if the dependency doesn't exist (idempotent).
	//
	// Example:
	//   err := syncer.DeleteDep("bd-abc", "bd-xyz", "blocks")
	DeleteDep(from, to, typ string) error

	// FullSync performs a complete sync from files to cache.
	//
	// This reads all task files from tasksDir and all dependency files
	// from depsDir, and updates the Turso cache to match.
	//
	// The sync is resilient - individual file failures are logged but
	// do not stop the entire process. The function returns an error
	// only if the directory cannot be read or a critical database
	// operation fails.
	//
	// After syncing all files, the blocked cache is refreshed to
	// ensure dependency state is up to date.
	//
	// Example:
	//   err := syncer.FullSync("/path/to/tasks", "/path/to/deps")
	FullSync(tasksDir, depsDir string) error

	// RefreshBlockedCache recomputes blocked status for all tasks.
	//
	// This performs a transitive closure query over the deps table
	// to determine which tasks are blocked by open dependencies.
	// The results are stored in the blocked_cache table and the
	// is_blocked flag is updated on the tasks table.
	//
	// This should be called:
	// - After a full sync
	// - After task status changes (e.g., task closed)
	// - After dependency changes (added/removed)
	//
	// Returns an error if the database operation fails.
	//
	// Example:
	//   err := syncer.RefreshBlockedCache()
	RefreshBlockedCache() error
}
