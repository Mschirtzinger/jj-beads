package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// syncer implements the Syncer interface.
type syncer struct {
	db     *db.DB
	logger *log.Logger
}

// New creates a new Syncer instance.
//
// The database connection must be initialized and have schema created
// before passing to this function.
//
// If logger is nil, a default logger writing to stderr is used.
//
// Example:
//
//	database, err := db.Open(".beads/turso.db")
//	if err != nil {
//	    return err
//	}
//	if err := database.InitSchema(); err != nil {
//	    return err
//	}
//	syncer := sync.New(database, nil)
func New(database *db.DB, logger *log.Logger) Syncer {
	if logger == nil {
		logger = log.New(os.Stderr, "[sync] ", log.LstdFlags)
	}
	return &syncer{
		db:     database,
		logger: logger,
	}
}

// SyncTask implements Syncer.SyncTask.
func (s *syncer) SyncTask(taskPath string) error {
	// Read task file
	task, err := schema.ReadTaskFile(taskPath)
	if err != nil {
		return fmt.Errorf("failed to read task file: %w", err)
	}

	// Upsert to database
	if err := s.db.UpsertTask(task); err != nil {
		return fmt.Errorf("failed to sync task to database: %w", err)
	}

	s.logger.Printf("Synced task: %s (%s)", task.ID, task.Title)
	return nil
}

// SyncDep implements Syncer.SyncDep.
func (s *syncer) SyncDep(depPath string) error {
	// Read dependency file
	dep, err := schema.ReadDepFile(depPath)
	if err != nil {
		return fmt.Errorf("failed to read dep file: %w", err)
	}

	// Upsert to database
	if err := s.db.UpsertDep(dep); err != nil {
		return fmt.Errorf("failed to sync dep to database: %w", err)
	}

	s.logger.Printf("Synced dependency: %s --%s--> %s", dep.From, dep.Type, dep.To)
	return nil
}

// DeleteTask implements Syncer.DeleteTask.
func (s *syncer) DeleteTask(taskID string) error {
	if err := s.db.DeleteTask(taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	s.logger.Printf("Deleted task: %s", taskID)
	return nil
}

// DeleteDep implements Syncer.DeleteDep.
func (s *syncer) DeleteDep(from, to, typ string) error {
	if err := s.db.DeleteDep(from, to, typ); err != nil {
		return fmt.Errorf("failed to delete dep: %w", err)
	}

	s.logger.Printf("Deleted dependency: %s --%s--> %s", from, typ, to)
	return nil
}

// FullSync implements Syncer.FullSync.
func (s *syncer) FullSync(tasksDir, depsDir string) error {
	s.logger.Printf("Starting full sync from tasks=%s, deps=%s", tasksDir, depsDir)

	// Track statistics
	var (
		tasksRead    int
		tasksFailed  int
		depsRead     int
		depsFailed   int
	)

	// Sync all task files
	if err := s.syncAllTasks(tasksDir, &tasksRead, &tasksFailed); err != nil {
		return fmt.Errorf("failed to sync tasks: %w", err)
	}

	// Sync all dependency files
	if err := s.syncAllDeps(depsDir, &depsRead, &depsFailed); err != nil {
		return fmt.Errorf("failed to sync deps: %w", err)
	}

	// Refresh blocked cache after syncing all files
	s.logger.Printf("Refreshing blocked cache...")
	if err := s.RefreshBlockedCache(); err != nil {
		return fmt.Errorf("failed to refresh blocked cache: %w", err)
	}

	s.logger.Printf("Full sync complete: tasks=%d (failed=%d), deps=%d (failed=%d)",
		tasksRead, tasksFailed, depsRead, depsFailed)

	return nil
}

// syncAllTasks reads and syncs all task files from the directory.
// Individual file failures are logged but don't stop the sync.
func (s *syncer) syncAllTasks(tasksDir string, tasksRead, tasksFailed *int) error {
	// Check if directory exists
	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		s.logger.Printf("Tasks directory doesn't exist: %s (skipping)", tasksDir)
		return nil
	}

	// Read directory
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return fmt.Errorf("failed to read tasks directory: %w", err)
	}

	// Process each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name())

		// Try to sync the task
		if err := s.SyncTask(path); err != nil {
			s.logger.Printf("WARNING: Failed to sync task %s: %v", entry.Name(), err)
			*tasksFailed++
			continue
		}

		*tasksRead++
	}

	return nil
}

// syncAllDeps reads and syncs all dependency files from the directory.
// Individual file failures are logged but don't stop the sync.
func (s *syncer) syncAllDeps(depsDir string, depsRead, depsFailed *int) error {
	// Check if directory exists
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		s.logger.Printf("Deps directory doesn't exist: %s (skipping)", depsDir)
		return nil
	}

	// Read directory
	entries, err := os.ReadDir(depsDir)
	if err != nil {
		return fmt.Errorf("failed to read deps directory: %w", err)
	}

	// Process each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(depsDir, entry.Name())

		// Try to sync the dependency
		if err := s.SyncDep(path); err != nil {
			s.logger.Printf("WARNING: Failed to sync dep %s: %v", entry.Name(), err)
			*depsFailed++
			continue
		}

		*depsRead++
	}

	return nil
}

// RefreshBlockedCache implements Syncer.RefreshBlockedCache.
func (s *syncer) RefreshBlockedCache() error {
	if err := s.db.RefreshBlockedCache(); err != nil {
		return fmt.Errorf("failed to refresh blocked cache: %w", err)
	}

	s.logger.Printf("Blocked cache refreshed")
	return nil
}
