package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/turso/dashboard"
)

var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	GroupID: "advanced",
	Short:   "Start real-time WebSocket dashboard for agent coordination",
	Long: `Start a WebSocket dashboard server for monitoring task state in real-time.

The dashboard server broadcasts task and dependency updates to connected clients,
enabling real-time coordination between AI agents working on the same repository.

WebSocket messages include:
- task_update: Task created, updated, or deleted
- dep_update: Dependency added or removed
- sync_complete: Full sync operation completed
- stats: Task statistics (total, by status, blocked count)
- blocked_cache: Blocked cache refresh completed

Example usage:
  bd dashboard                   # Start on default port 8080
  bd dashboard --port 9000       # Start on custom port

Connect with a WebSocket client:
  ws://localhost:8080/ws

The dashboard is intended for use with the jj-turso architecture where a sync
daemon monitors file changes and updates a Turso database cache.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")

		// Create dashboard server
		config := &dashboard.Config{
			Port:   port,
			Logger: log.New(os.Stderr, "[dashboard] ", log.LstdFlags),
		}

		server := dashboard.NewServer(config)

		// Start server
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start dashboard: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Dashboard server started on http://localhost:%d\n", port)
		fmt.Printf("WebSocket endpoint: ws://localhost:%d/ws\n", port)
		fmt.Printf("Health check: http://localhost:%d/health\n", port)
		fmt.Println("\nPress Ctrl+C to stop...")

		// Wait for interrupt signal
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		<-ctx.Done()

		// Graceful shutdown
		fmt.Println("\nShutting down dashboard server...")
		if err := server.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Dashboard server stopped")
	},
}

func init() {
	// Register flags
	dashboardCmd.Flags().IntP("port", "p", 8080, "Port to listen on")

	// Register command
	rootCmd.AddCommand(dashboardCmd)
}
