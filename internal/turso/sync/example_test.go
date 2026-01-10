package sync_test

import (
	"fmt"
	"log"

	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/sync"
)

// This example demonstrates basic usage of the sync package.
// Note: This is for documentation only and won't run as a test.
func ExampleNew() {
	// Open database
	database, err := db.Open("file:.beads/turso.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Initialize schema (first time only)
	if err := database.InitSchema(); err != nil {
		log.Fatal(err)
	}

	// Create syncer
	syncer := sync.New(database, nil)

	// Perform full sync from file directories
	if err := syncer.FullSync("tasks/", "deps/"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Sync complete")
}

// This example demonstrates syncing individual files.
func ExampleSyncer_SyncTask() {
	database, err := db.Open("file:.beads/turso.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	syncer := sync.New(database, nil)

	// Sync a single task file
	if err := syncer.SyncTask("tasks/bd-123.json"); err != nil {
		log.Fatal(err)
	}

	// Sync a single dependency file
	if err := syncer.SyncDep("deps/bd-123--blocks--bd-456.json"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Incremental sync complete")
}

// This example demonstrates deleting from cache.
func ExampleSyncer_DeleteTask() {
	database, err := db.Open("file:.beads/turso.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	syncer := sync.New(database, nil)

	// Delete a task from cache
	if err := syncer.DeleteTask("bd-123"); err != nil {
		log.Fatal(err)
	}

	// Delete a dependency from cache
	if err := syncer.DeleteDep("bd-123", "bd-456", "blocks"); err != nil {
		log.Fatal(err)
	}

	// Refresh blocked status after changes
	if err := syncer.RefreshBlockedCache(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Delete operations complete")
}
