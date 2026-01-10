// Package daemon provides file system watching for the jj-turso sync daemon.
package daemon

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// EventOp represents the type of file system operation.
type EventOp int

const (
	// OpCreate indicates a new file was created.
	OpCreate EventOp = iota
	// OpModify indicates an existing file was modified.
	OpModify
	// OpDelete indicates a file was deleted.
	OpDelete
)

// String returns a human-readable representation of the operation.
func (op EventOp) String() string {
	switch op {
	case OpCreate:
		return "create"
	case OpModify:
		return "modify"
	case OpDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// FileType represents whether the event is for a task or dependency file.
type FileType int

const (
	// TypeTask indicates a task file (tasks/*.json).
	TypeTask FileType = iota
	// TypeDep indicates a dependency file (deps/*.json).
	TypeDep
)

// String returns a human-readable representation of the file type.
func (ft FileType) String() string {
	switch ft {
	case TypeTask:
		return "task"
	case TypeDep:
		return "dep"
	default:
		return "unknown"
	}
}

// FileEvent represents a file system event for task or dependency files.
type FileEvent struct {
	// Path is the absolute path to the file that changed.
	Path string
	// Type indicates whether this is a task or dependency file.
	Type FileType
	// Op is the operation that occurred (create, modify, delete).
	Op EventOp
}

// FileWatcher watches task and dependency directories for changes.
// It uses fsnotify for cross-platform file system event monitoring.
type FileWatcher struct {
	watcher   *fsnotify.Watcher
	events    chan FileEvent
	errors    chan error
	done      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	running   bool
	tasksDir  string
	depsDir   string
}

// NewFileWatcher creates a new FileWatcher instance.
// The watcher must be started with Start() before it will emit events.
func NewFileWatcher() (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &FileWatcher{
		watcher: watcher,
		events:  make(chan FileEvent, 100),
		errors:  make(chan error, 10),
		done:    make(chan struct{}),
	}, nil
}

// Start begins watching the specified directories for changes.
// It monitors both directories for *.json file events.
// Returns an error if the directories cannot be watched.
func (fw *FileWatcher) Start(tasksDir, depsDir string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.running {
		return fmt.Errorf("watcher already running")
	}

	// Store directory paths
	fw.tasksDir = tasksDir
	fw.depsDir = depsDir

	// Add directories to watcher
	if err := fw.watcher.Add(tasksDir); err != nil {
		return fmt.Errorf("failed to watch tasks directory %s: %w", tasksDir, err)
	}

	if err := fw.watcher.Add(depsDir); err != nil {
		// Clean up tasks watch if deps watch fails
		fw.watcher.Remove(tasksDir)
		return fmt.Errorf("failed to watch deps directory %s: %w", depsDir, err)
	}

	fw.running = true
	fw.wg.Add(1)
	go fw.processEvents()

	return nil
}

// Stop stops watching for file system events and cleans up resources.
// It blocks until the event processing goroutine has exited.
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	if !fw.running {
		fw.mu.Unlock()
		return nil
	}
	fw.running = false
	fw.mu.Unlock()

	// Signal shutdown
	close(fw.done)

	// Close the underlying watcher (this will unblock the event loop)
	if err := fw.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	// Wait for event processing to finish
	fw.wg.Wait()

	// Close channels
	close(fw.events)
	close(fw.errors)

	return nil
}

// Events returns the channel that emits FileEvent notifications.
// This channel is closed when the watcher is stopped.
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Errors returns the channel that emits error notifications.
// This channel is closed when the watcher is stopped.
func (fw *FileWatcher) Errors() <-chan error {
	return fw.errors
}

// processEvents is the main event loop that processes fsnotify events
// and converts them to FileEvent notifications.
func (fw *FileWatcher) processEvents() {
	defer fw.wg.Done()

	for {
		select {
		case <-fw.done:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Process the event
			if fileEvent, ok := fw.convertEvent(event); ok {
				select {
				case fw.events <- fileEvent:
				case <-fw.done:
					return
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}

			select {
			case fw.errors <- err:
			case <-fw.done:
				return
			}
		}
	}
}

// convertEvent converts an fsnotify event to a FileEvent.
// Returns (FileEvent, true) if the event should be processed,
// or (FileEvent{}, false) if the event should be ignored.
func (fw *FileWatcher) convertEvent(event fsnotify.Event) (FileEvent, bool) {
	// Only process .json files
	if !strings.HasSuffix(event.Name, ".json") {
		return FileEvent{}, false
	}

	// Determine file type based on parent directory
	fileType, ok := fw.determineFileType(event.Name)
	if !ok {
		return FileEvent{}, false
	}

	// Convert fsnotify operation to our EventOp
	var op EventOp
	switch {
	case event.Has(fsnotify.Create):
		op = OpCreate
	case event.Has(fsnotify.Write):
		op = OpModify
	case event.Has(fsnotify.Remove):
		op = OpDelete
	case event.Has(fsnotify.Rename):
		// Treat rename as delete (the new name will trigger a create)
		op = OpDelete
	default:
		// Ignore chmod and other events
		return FileEvent{}, false
	}

	return FileEvent{
		Path: event.Name,
		Type: fileType,
		Op:   op,
	}, true
}

// determineFileType checks if the file path is in tasks/ or deps/
// and returns the corresponding FileType.
func (fw *FileWatcher) determineFileType(path string) (FileType, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}

	dir := filepath.Dir(absPath)

	absTasksDir, _ := filepath.Abs(fw.tasksDir)
	absDepsDir, _ := filepath.Abs(fw.depsDir)

	if dir == absTasksDir {
		return TypeTask, true
	}
	if dir == absDepsDir {
		return TypeDep, true
	}

	return 0, false
}

// IsRunning returns true if the watcher is currently running.
func (fw *FileWatcher) IsRunning() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.running
}
