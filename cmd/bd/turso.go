package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/beads"
	"github.com/steveyegge/beads/internal/turso/daemon"
	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/sync"
	"github.com/steveyegge/beads/internal/ui"
)

var tursoCmd = &cobra.Command{
	Use:     "turso",
	GroupID: "sync",
	Short:   "Turso cache management for jj-beads",
	Long: `Manage the Turso query cache for fast multi-agent access.

The Turso cache is a local SQLite database (.beads/turso.db) that provides
fast concurrent queries for ready work. It syncs from jj task/dep files.`,
}

var tursoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Full sync from task/dep files to Turso cache",
	Long: `Sync all task and dependency files to the Turso cache database.

This performs a full sync:
  1. Reads all tasks/*.json files
  2. Reads all deps/*.json files
  3. Updates Turso cache (.beads/turso.db)
  4. Refreshes blocked task cache`,
	Run: func(cmd *cobra.Command, args []string) {
		beadsDir := beads.FindBeadsDir()
		if beadsDir == "" {
			fmt.Fprintf(os.Stderr, "Error: .beads directory not found\n")
			os.Exit(1)
		}

		tursoPath := filepath.Join(beadsDir, "turso.db")

		// Open/create Turso database
		database, err := db.Open(tursoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening Turso database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		// Initialize schema if needed
		if err := database.InitSchema(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing schema: %v\n", err)
			os.Exit(1)
		}

		// Create syncer
		syncer := sync.New(database, nil)

		// Determine tasks/deps directories
		// For jj-turso mode: .beads/tasks/ and .beads/deps/
		// For current mode: use JSONL (fallback)
		tasksDir := filepath.Join(beadsDir, "tasks")
		depsDir := filepath.Join(beadsDir, "deps")

		// Check if jj-turso directories exist
		if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: tasks directory not found at %s\n", tasksDir)
			fmt.Fprintf(os.Stderr, "Note: jj-turso requires tasks/*.json and deps/*.json files\n")
		}

		// Perform full sync
		fmt.Printf("%s Syncing from %s and %s...\n", ui.RenderAccent("🔄"), tasksDir, depsDir)
		start := time.Now()

		if err := syncer.FullSync(tasksDir, depsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error during sync: %v\n", err)
			os.Exit(1)
		}

		elapsed := time.Since(start)
		taskCount, _ := database.GetTaskCount()
		depCount, _ := database.GetDepCount()

		fmt.Printf("%s Sync complete in %v\n", ui.RenderPass("✓"), elapsed.Round(time.Millisecond))
		fmt.Printf("   Tasks: %d\n", taskCount)
		fmt.Printf("   Deps: %d\n", depCount)
		fmt.Printf("   Cache: %s\n", tursoPath)
	},
}

var tursoStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Turso cache status",
	Long: `Display the current status of the Turso query cache.

Shows:
  - Cache file location and size
  - Number of tasks and dependencies
  - Last sync time (if daemon is running)`,
	Run: func(cmd *cobra.Command, args []string) {
		beadsDir := beads.FindBeadsDir()
		if beadsDir == "" {
			fmt.Fprintf(os.Stderr, "Error: .beads directory not found\n")
			os.Exit(1)
		}

		tursoPath := filepath.Join(beadsDir, "turso.db")

		// Check if cache exists
		info, err := os.Stat(tursoPath)
		if os.IsNotExist(err) {
			fmt.Printf("\n%s Turso cache not initialized\n", ui.RenderWarn("⚠"))
			fmt.Printf("   Run 'bd turso sync' to create the cache\n\n")
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking cache: %v\n", err)
			os.Exit(1)
		}

		// Open database to get stats
		database, err := db.Open(tursoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		taskCount, err := database.GetTaskCount()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting task count: %v\n", err)
			os.Exit(1)
		}

		depCount, err := database.GetDepCount()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting dep count: %v\n", err)
			os.Exit(1)
		}

		// Format file size
		size := info.Size()
		sizeStr := fmt.Sprintf("%d bytes", size)
		if size > 1024*1024 {
			sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
		} else if size > 1024 {
			sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
		}

		// Display status
		fmt.Printf("\n%s Turso Cache Status\n\n", ui.RenderAccent("📊"))
		fmt.Printf("Location: %s\n", tursoPath)
		fmt.Printf("Size: %s\n", sizeStr)
		fmt.Printf("Tasks: %d\n", taskCount)
		fmt.Printf("Dependencies: %d\n", depCount)
		fmt.Printf("Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
		fmt.Println()
	},
}

var tursoDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start Turso sync daemon (foreground)",
	Long: `Start the Turso sync daemon in foreground mode for debugging.

The daemon watches jj operation log for changes and syncs them to Turso cache.
This is useful for development and debugging. For production use, run the
daemon in the background or use a process manager.

The daemon will:
  1. Watch jj op log for commits
  2. Detect changes to tasks/*.json and deps/*.json
  3. Sync changes to Turso cache
  4. Update blocked task cache`,
	Run: func(cmd *cobra.Command, args []string) {
		beadsDir := beads.FindBeadsDir()
		if beadsDir == "" {
			fmt.Fprintf(os.Stderr, "Error: .beads directory not found\n")
			os.Exit(1)
		}

		// Check if jj repo exists
		jjDir := filepath.Join(beadsDir, "..", ".jj")
		if _, err := os.Stat(jjDir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Not a jj repository (no .jj directory found)\n")
			fmt.Fprintf(os.Stderr, "The Turso daemon requires jj (Jujutsu) version control\n")
			os.Exit(1)
		}

		tursoPath := filepath.Join(beadsDir, "turso.db")
		tasksDir := filepath.Join(beadsDir, "tasks")
		depsDir := filepath.Join(beadsDir, "deps")

		// Open/create Turso database
		database, err := db.Open(tursoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening Turso database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		// Initialize schema
		if err := database.InitSchema(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing schema: %v\n", err)
			os.Exit(1)
		}

		// Get the underlying sql.DB connection
		// Note: We need to update the db.DB struct to expose this, or use a different approach
		// For now, create daemon using the simplified API
		d, err := daemon.New(database.RawDB(), tasksDir, depsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating daemon: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("%s Starting Turso sync daemon...\n", ui.RenderAccent("🚀"))
		fmt.Printf("   Tasks dir: %s\n", tasksDir)
		fmt.Printf("   Deps dir: %s\n", depsDir)
		fmt.Printf("   Cache: %s\n", tursoPath)
		fmt.Printf("\nPress Ctrl+C to stop\n\n")

		// Create context for running daemon
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start daemon (this blocks)
		if err := d.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Daemon stopped with error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	tursoCmd.AddCommand(tursoSyncCmd)
	tursoCmd.AddCommand(tursoStatusCmd)
	tursoCmd.AddCommand(tursoDaemonCmd)
	rootCmd.AddCommand(tursoCmd)
}
