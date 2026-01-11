package daemon_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/steveyegge/beads/internal/turso/daemon"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// Example_basicUsage demonstrates basic daemon setup and operation.
func Example_basicUsage() {
	// Create temporary directories
	tmpDir := os.TempDir()
	tasksDir := tmpDir + "/example-tasks"
	depsDir := tmpDir + "/example-deps"
	os.MkdirAll(tasksDir, 0755)
	os.MkdirAll(depsDir, 0755)
	defer os.RemoveAll(tasksDir)
	defer os.RemoveAll(depsDir)

	// Open database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create daemon with custom config (silent logger for example)
	config := &daemon.Config{
		BlockedCacheRefreshInterval: 1 * time.Second,
		DebounceInterval:            50 * time.Millisecond,
		Logger:                      log.New(os.Stderr, "[daemon] ", log.Ltime),
	}

	d, err := daemon.NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		log.Fatal(err)
	}

	// Start daemon in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for initialization
	time.Sleep(100 * time.Millisecond)

	// Create a task file
	task := &schema.TaskFile{
		ID:          "bd-example1",
		Title:       "Example task",
		Description: "This is an example",
		Type:        "task",
		Status:      "open",
		Priority:    1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := schema.WriteTaskFile(tasksDir, task); err != nil {
		log.Fatal(err)
	}

	// Wait for sync
	time.Sleep(200 * time.Millisecond)

	// Modify the task
	task.Status = "in_progress"
	task.UpdatedAt = time.Now()
	if err := schema.WriteTaskFile(tasksDir, task); err != nil {
		log.Fatal(err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	fmt.Println("Daemon processed file changes successfully")

	// Wait for shutdown
	<-ctx.Done()
	if err := <-errCh; err != nil {
		log.Printf("Daemon error: %v", err)
	}

	// Output:
	// Daemon processed file changes successfully
}

// Example_manualSync demonstrates triggering a manual full sync.
func Example_manualSync() {
	// Setup
	tmpDir := os.TempDir()
	tasksDir := tmpDir + "/sync-tasks"
	depsDir := tmpDir + "/sync-deps"
	os.MkdirAll(tasksDir, 0755)
	os.MkdirAll(depsDir, 0755)
	defer os.RemoveAll(tasksDir)
	defer os.RemoveAll(depsDir)

	// Create some tasks
	for i := 1; i <= 3; i++ {
		task := &schema.TaskFile{
			ID:        fmt.Sprintf("bd-sync%d", i),
			Title:     fmt.Sprintf("Task %d", i),
			Type:      "task",
			Status:    "open",
			Priority:  i,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		schema.WriteTaskFile(tasksDir, task)
	}

	// Setup database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create daemon with visible logger for output
	config := daemon.DefaultConfig()
	config.Logger = log.New(os.Stdout, "", log.Lmsgprefix)

	d, err := daemon.NewWithConfig(db, tasksDir, depsDir, config)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Stop()

	// Perform manual full sync
	if err := d.PerformFullSync(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Manual sync completed successfully")

	// Output:
	// Performing full sync
	// Syncing 3 tasks
	// Upserting task: bd-sync1 (Task 1)
	// Upserting task: bd-sync2 (Task 2)
	// Upserting task: bd-sync3 (Task 3)
	// Syncing 0 dependencies
	// Recomputing blocked cache
	// Full sync complete
	// Manual sync completed successfully
	// Stopping daemon
	// Daemon stopped
}

// Example_gracefulShutdown demonstrates clean daemon shutdown.
func Example_gracefulShutdown() {
	// Setup
	tmpDir := os.TempDir()
	tasksDir := tmpDir + "/shutdown-tasks"
	depsDir := tmpDir + "/shutdown-deps"
	os.MkdirAll(tasksDir, 0755)
	os.MkdirAll(depsDir, 0755)
	defer os.RemoveAll(tasksDir)
	defer os.RemoveAll(depsDir)

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create daemon
	d, err := daemon.New(db, tasksDir, depsDir)
	if err != nil {
		log.Fatal(err)
	}

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := d.Start(ctx); err != nil {
			log.Printf("Daemon error: %v", err)
		}
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown
	cancel()

	// Wait for shutdown
	time.Sleep(200 * time.Millisecond)

	fmt.Println("Daemon shut down gracefully")

	// Output:
	// Daemon shut down gracefully
}
