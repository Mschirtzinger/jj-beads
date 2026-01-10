// Package daemon provides the sync daemon that orchestrates file watching and Turso cache updates.
//
// The daemon:
// 1. Watches for file changes in tasks/ and deps/ directories
// 2. Syncs affected files to Turso database
// 3. Periodically refreshes the blocked cache
// 4. Handles graceful shutdown
package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// Config holds configuration for the daemon.
type Config struct {
	// BlockedCacheRefreshInterval is how often to recompute the blocked cache
	BlockedCacheRefreshInterval time.Duration

	// DebounceInterval is how long to wait before processing file changes
	// This batches rapid updates together
	DebounceInterval time.Duration

	// Logger for daemon activity
	Logger *log.Logger
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		BlockedCacheRefreshInterval: 5 * time.Second,
		DebounceInterval:            100 * time.Millisecond,
		Logger:                      log.New(os.Stderr, "[daemon] ", log.LstdFlags),
	}
}

// Daemon orchestrates file watching and database synchronization.
type Daemon struct {
	db       *sql.DB
	tasksDir string
	depsDir  string
	config   *Config

	watcher       *fsnotify.Watcher
	changeQueue   map[string]time.Time // filepath -> timestamp
	changeQueueMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Daemon instance.
//
// The daemon requires:
//   - db: SQLite database connection
//   - tasksDir: Directory containing task JSON files (tasks/*.json)
//   - depsDir: Directory containing dependency JSON files (deps/*.json)
//
// Use Start() to begin watching and syncing.
func New(db *sql.DB, tasksDir, depsDir string) (*Daemon, error) {
	return NewWithConfig(db, tasksDir, depsDir, DefaultConfig())
}

// NewWithConfig creates a daemon with custom configuration.
func NewWithConfig(db *sql.DB, tasksDir, depsDir string, config *Config) (*Daemon, error) {
	if db == nil {
		return nil, fmt.Errorf("db cannot be nil")
	}
	if tasksDir == "" {
		return nil, fmt.Errorf("tasksDir cannot be empty")
	}
	if depsDir == "" {
		return nil, fmt.Errorf("depsDir cannot be empty")
	}
	if config == nil {
		config = DefaultConfig()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		db:          db,
		tasksDir:    tasksDir,
		depsDir:     depsDir,
		config:      config,
		watcher:     watcher,
		changeQueue: make(map[string]time.Time),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Start begins the daemon's operation.
//
// The daemon will:
// 1. Perform a full sync from files to database
// 2. Start watching for file changes
// 3. Periodically refresh blocked cache
// 4. Process file changes with debouncing
//
// This blocks until ctx is cancelled or an error occurs.
func (d *Daemon) Start(ctx context.Context) error {
	d.config.Logger.Println("Starting daemon")

	// Perform initial full sync
	if err := d.PerformFullSync(); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	// Add watch directories
	if err := d.watcher.Add(d.tasksDir); err != nil {
		return fmt.Errorf("failed to watch tasks directory: %w", err)
	}
	if err := d.watcher.Add(d.depsDir); err != nil {
		return fmt.Errorf("failed to watch deps directory: %w", err)
	}

	d.config.Logger.Printf("Watching: %s, %s", d.tasksDir, d.depsDir)

	// Start background goroutines
	d.wg.Add(3)
	go d.watchFileEvents()
	go d.processChangeQueue()
	go d.refreshBlockedCache()

	// Wait for shutdown
	select {
	case <-ctx.Done():
		d.config.Logger.Println("Shutdown signal received")
		return d.Stop()
	case <-d.ctx.Done():
		return nil
	}
}

// Stop gracefully shuts down the daemon.
func (d *Daemon) Stop() error {
	d.config.Logger.Println("Stopping daemon")

	// Signal shutdown
	d.cancel()

	// Close watcher
	if err := d.watcher.Close(); err != nil {
		d.config.Logger.Printf("Error closing watcher: %v", err)
	}

	// Wait for goroutines to finish
	d.wg.Wait()

	d.config.Logger.Println("Daemon stopped")
	return nil
}

// PerformFullSync synchronizes all files to the database.
//
// This reads all task and dependency files and updates the database accordingly.
// It's called on startup and can be triggered manually.
func (d *Daemon) PerformFullSync() error {
	d.config.Logger.Println("Performing full sync")

	// Sync all tasks
	tasks, err := schema.ReadAllTaskFiles(d.tasksDir)
	if err != nil {
		return fmt.Errorf("failed to read tasks: %w", err)
	}

	d.config.Logger.Printf("Syncing %d tasks", len(tasks))
	for _, task := range tasks {
		if err := d.upsertTask(task); err != nil {
			d.config.Logger.Printf("Warning: failed to sync task %s: %v", task.ID, err)
		}
	}

	// Sync all dependencies
	deps, err := schema.ListAllDeps(d.depsDir)
	if err != nil {
		return fmt.Errorf("failed to read deps: %w", err)
	}

	d.config.Logger.Printf("Syncing %d dependencies", len(deps))
	for _, dep := range deps {
		if err := d.upsertDep(dep); err != nil {
			d.config.Logger.Printf("Warning: failed to sync dep %s->%s: %v", dep.From, dep.To, err)
		}
	}

	// Recompute blocked cache
	if err := d.recomputeBlockedCache(); err != nil {
		return fmt.Errorf("failed to compute blocked cache: %w", err)
	}

	d.config.Logger.Println("Full sync complete")
	return nil
}

// watchFileEvents monitors filesystem events and queues changes.
func (d *Daemon) watchFileEvents() {
	defer d.wg.Done()

	for {
		select {
		case <-d.ctx.Done():
			return

		case event, ok := <-d.watcher.Events:
			if !ok {
				return
			}

			// Only care about Create, Write, Remove
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
				continue
			}

			// Only process .json files
			if filepath.Ext(event.Name) != ".json" {
				continue
			}

			d.config.Logger.Printf("File event: %s %s", event.Op, event.Name)
			d.queueChange(event.Name)

		case err, ok := <-d.watcher.Errors:
			if !ok {
				return
			}
			d.config.Logger.Printf("Watcher error: %v", err)
		}
	}
}

// queueChange adds a file to the change queue with debouncing.
func (d *Daemon) queueChange(path string) {
	d.changeQueueMu.Lock()
	defer d.changeQueueMu.Unlock()

	d.changeQueue[path] = time.Now()
}

// processChangeQueue processes queued file changes with debouncing.
func (d *Daemon) processChangeQueue() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.DebounceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return

		case <-ticker.C:
			d.processPendingChanges()
		}
	}
}

// processPendingChanges syncs files that have been queued for long enough.
func (d *Daemon) processPendingChanges() {
	d.changeQueueMu.Lock()
	defer d.changeQueueMu.Unlock()

	now := time.Now()
	needsBlockedRefresh := false

	for path, queuedAt := range d.changeQueue {
		// Only process if enough time has passed (debouncing)
		if now.Sub(queuedAt) < d.config.DebounceInterval {
			continue
		}

		d.config.Logger.Printf("Processing change: %s", path)

		// Determine if this is a task or dep file
		if filepath.Dir(path) == d.tasksDir {
			if err := d.syncTaskFile(path); err != nil {
				d.config.Logger.Printf("Error syncing task %s: %v", path, err)
			}
			needsBlockedRefresh = true
		} else if filepath.Dir(path) == d.depsDir {
			if err := d.syncDepFile(path); err != nil {
				d.config.Logger.Printf("Error syncing dep %s: %v", path, err)
			}
			needsBlockedRefresh = true
		}

		delete(d.changeQueue, path)
	}

	// Refresh blocked cache if we made changes
	if needsBlockedRefresh {
		if err := d.recomputeBlockedCache(); err != nil {
			d.config.Logger.Printf("Error recomputing blocked cache: %v", err)
		}
	}
}

// syncTaskFile syncs a single task file to the database.
func (d *Daemon) syncTaskFile(path string) error {
	// Check if file was deleted
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Extract task ID from filename
		filename := filepath.Base(path)
		taskID := filename[:len(filename)-5] // Remove .json

		d.config.Logger.Printf("Deleting task: %s", taskID)
		return d.deleteTask(taskID)
	}

	// Read and sync task
	task, err := schema.ReadTaskFile(path)
	if err != nil {
		return fmt.Errorf("failed to read task file: %w", err)
	}

	return d.upsertTask(task)
}

// syncDepFile syncs a single dependency file to the database.
func (d *Daemon) syncDepFile(path string) error {
	// Check if file was deleted
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Extract dep info from filename
		filename := filepath.Base(path)
		from, typ, to, err := schema.FromFileName(filename)
		if err != nil {
			return fmt.Errorf("failed to parse dep filename: %w", err)
		}

		d.config.Logger.Printf("Deleting dep: %s --%s--> %s", from, typ, to)
		return d.deleteDep(from, typ, to)
	}

	// Read and sync dependency
	dep, err := schema.ReadDepFile(path)
	if err != nil {
		return fmt.Errorf("failed to read dep file: %w", err)
	}

	return d.upsertDep(dep)
}

// refreshBlockedCache periodically recomputes the blocked cache.
func (d *Daemon) refreshBlockedCache() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.BlockedCacheRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return

		case <-ticker.C:
			if err := d.recomputeBlockedCache(); err != nil {
				d.config.Logger.Printf("Error refreshing blocked cache: %v", err)
			}
		}
	}
}

// upsertTask inserts or updates a task in the database.
func (d *Daemon) upsertTask(task *schema.TaskFile) error {
	// TODO: This will be implemented when we have the database schema
	// For now, just log
	d.config.Logger.Printf("Upserting task: %s (%s)", task.ID, task.Title)
	return nil
}

// deleteTask removes a task from the database.
func (d *Daemon) deleteTask(taskID string) error {
	// TODO: Implement database deletion
	d.config.Logger.Printf("Deleting task: %s", taskID)
	return nil
}

// upsertDep inserts or updates a dependency in the database.
func (d *Daemon) upsertDep(dep *schema.DepFile) error {
	// TODO: This will be implemented when we have the database schema
	d.config.Logger.Printf("Upserting dep: %s --%s--> %s", dep.From, dep.Type, dep.To)
	return nil
}

// deleteDep removes a dependency from the database.
func (d *Daemon) deleteDep(from, typ, to string) error {
	// TODO: Implement database deletion
	d.config.Logger.Printf("Deleting dep: %s --%s--> %s", from, typ, to)
	return nil
}

// recomputeBlockedCache updates the blocked_cache table with transitive closure.
func (d *Daemon) recomputeBlockedCache() error {
	// TODO: Implement transitive closure query
	// This is a recursive CTE that computes all tasks blocked by dependencies
	d.config.Logger.Printf("Recomputing blocked cache")
	return nil
}
