package daemon_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/steveyegge/beads/internal/turso/daemon"
)

// ExampleFileWatcher demonstrates basic usage of the FileWatcher.
func ExampleFileWatcher() {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "watcher-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		log.Fatal(err)
	}

	// Create and start watcher
	fw, err := daemon.NewFileWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		log.Fatal(err)
	}

	// Start event listener
	go func() {
		for event := range fw.Events() {
			fmt.Printf("%s: %s (%s)\n", event.Op, filepath.Base(event.Path), event.Type)
		}
	}()

	// Simulate file changes
	taskFile := filepath.Join(tasksDir, "bd-test.json")
	if err := os.WriteFile(taskFile, []byte(`{"id":"bd-test"}`), 0644); err != nil {
		log.Fatal(err)
	}

	// Give watcher time to process
	time.Sleep(100 * time.Millisecond)

	// Output:
	// create: bd-test.json (task)
}

// ExampleFileWatcher_errorHandling demonstrates error handling.
func ExampleFileWatcher_errorHandling() {
	tmpDir, err := os.MkdirTemp("", "watcher-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tasksDir := filepath.Join(tmpDir, "tasks")
	depsDir := filepath.Join(tmpDir, "deps")

	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		log.Fatal(err)
	}

	fw, err := daemon.NewFileWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer fw.Stop()

	if err := fw.Start(tasksDir, depsDir); err != nil {
		log.Fatal(err)
	}

	// Monitor both events and errors
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-fw.Events():
				if !ok {
					done <- true
					return
				}
				fmt.Printf("Event: %s\n", event.Op)

			case err, ok := <-fw.Errors():
				if !ok {
					done <- true
					return
				}
				fmt.Printf("Error: %v\n", err)
			}
		}
	}()

	// Stop watcher (closes channels)
	fw.Stop()
	<-done

	fmt.Println("Watcher stopped")
	// Output:
	// Watcher stopped
}
