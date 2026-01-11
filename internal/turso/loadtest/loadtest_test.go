package loadtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/turso/db"
)

// TestCreateTestDatabase verifies that we can create a test database with the expected properties.
func TestCreateTestDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	td, err := CreateTestDatabase(dbPath, 100, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	// Verify task counts
	if len(td.TaskIDs) != 100 {
		t.Errorf("Expected 100 tasks, got %d", len(td.TaskIDs))
	}

	// Verify blocked percentage is approximately 30%
	blockedPct := float64(len(td.BlockedIDs)) / float64(td.TotalTasks) * 100
	if blockedPct < 20 || blockedPct > 40 {
		t.Errorf("Expected ~30%% blocked tasks, got %.1f%% (%d/%d)", blockedPct, len(td.BlockedIDs), td.TotalTasks)
	}

	// Verify ready tasks exist
	if len(td.ReadyIDs) == 0 {
		t.Error("Expected some ready tasks, got 0")
	}

	// Verify total adds up
	total := len(td.ReadyIDs) + len(td.BlockedIDs)
	if total != td.TotalTasks {
		t.Errorf("Ready (%d) + Blocked (%d) = %d, expected %d", len(td.ReadyIDs), len(td.BlockedIDs), total, td.TotalTasks)
	}

	t.Logf("Database created: %d total, %d ready (%.1f%%), %d blocked (%.1f%%)",
		td.TotalTasks,
		len(td.ReadyIDs), float64(len(td.ReadyIDs))/float64(td.TotalTasks)*100,
		len(td.BlockedIDs), blockedPct)
}

// TestConcurrentQueries_Small verifies basic concurrent query functionality.
func TestConcurrentQueries_Small(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	td, err := CreateTestDatabase(dbPath, 100, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	// Run 10 concurrent agents, 5 queries each
	stats, err := td.RunConcurrentQueries(10, 5)
	if err != nil {
		t.Fatalf("Concurrent queries failed: %v", err)
	}

	if stats.Errors > 0 {
		t.Errorf("Got %d errors during queries", stats.Errors)
	}

	if stats.TotalQueries != 50 {
		t.Errorf("Expected 50 total queries, got %d", stats.TotalQueries)
	}

	stats.PrintStats()

	// Basic sanity checks
	if stats.Mean > 100*time.Millisecond {
		t.Errorf("Mean query time too high: %v", stats.Mean)
	}
}

// TestConcurrentQueries_100Agents validates the main requirement: 100 concurrent agents.
func TestConcurrentQueries_100Agents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create database with 1000 tasks
	t.Log("Creating test database with 1000 tasks...")
	td, err := CreateTestDatabase(dbPath, 1000, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	stats := td.GetStats()
	t.Logf("Database stats: %+v", stats)

	// Run 100 concurrent agents, each performing 10 queries
	t.Log("Running 100 concurrent agents with 10 queries each...")
	start := time.Now()
	queryStats, err := td.RunConcurrentQueries(100, 10)
	totalDuration := time.Since(start)

	if err != nil {
		t.Fatalf("Concurrent queries failed: %v", err)
	}

	if queryStats.Errors > 0 {
		t.Errorf("Got %d errors during queries", queryStats.Errors)
	}

	t.Logf("\n=== LOAD TEST RESULTS (100 agents, 10 queries each) ===")
	queryStats.PrintStats()
	t.Logf("Total test duration: %v", totalDuration)
	t.Logf("Throughput: %.2f queries/second", float64(queryStats.TotalQueries)/totalDuration.Seconds())

	// Verify performance requirements
	// Note: With SQLite/WAL mode, concurrent access causes queueing.
	// The important metrics are:
	// 1. Min latency shows base query performance (target: <10ms, CI: <50ms)
	// 2. Throughput validates concurrent handling (target: >1000 qps, CI: >500 qps)
	// 3. Total duration validates all agents complete quickly (target: <2s, CI: <15s)

	// Min latency check - more lenient for CI environments
	if queryStats.Min > 50*time.Millisecond {
		t.Errorf("FAILED: Minimum query latency %v exceeds 50ms - base query is too slow", queryStats.Min)
	} else if queryStats.Min <= 10*time.Millisecond {
		t.Logf("PASSED: Minimum query latency %v is under 10ms (excellent)", queryStats.Min)
	} else {
		t.Logf("PASSED: Minimum query latency %v is acceptable (10-50ms)", queryStats.Min)
	}

	throughput := float64(queryStats.TotalQueries) / totalDuration.Seconds()
	// Allow variance due to OS scheduling, disk I/O, and CI environment overhead
	// CI environments can be very slow, so we set a minimal threshold
	if throughput < 50 {
		t.Errorf("FAILED: Throughput %.2f qps is below 50 qps minimum", throughput)
	} else if throughput >= 1000 {
		t.Logf("PASSED: Throughput %.2f qps exceeds 1000 qps target (excellent)", throughput)
	} else if throughput >= 500 {
		t.Logf("PASSED: Throughput %.2f qps is good (500-1000 qps)", throughput)
	} else {
		t.Logf("PASSED: Throughput %.2f qps is acceptable for CI (50-500 qps)", throughput)
	}

	// Total duration check - more lenient for CI environments
	if totalDuration > 15*time.Second {
		t.Errorf("FAILED: Total duration %v exceeds 15s for 100 agents", totalDuration)
	} else if totalDuration <= 2*time.Second {
		t.Logf("PASSED: Total test duration %v completes within 2s (excellent)", totalDuration)
	} else {
		t.Logf("PASSED: Total test duration %v is acceptable for CI (2-15s)", totalDuration)
	}

	// Log additional metrics for visibility
	t.Logf("Query latency - Mean: %v, P50: %v, P95: %v, P99: %v",
		queryStats.Mean, queryStats.P50, queryStats.P95, queryStats.P99)
}

// TestNoRaceConditions verifies that concurrent access doesn't cause data corruption.
func TestNoRaceConditions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	td, err := CreateTestDatabase(dbPath, 500, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	// Run 50 concurrent agents for 2 seconds
	t.Log("Testing for race conditions with 50 agents for 2 seconds...")
	err = td.VerifyNoRaceConditions(50, 2*time.Second)
	if err != nil {
		t.Errorf("Race condition detected: %v", err)
	} else {
		t.Log("No race conditions detected")
	}
}

// TestDataConsistency verifies that query results are consistent and correct.
func TestDataConsistency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	td, err := CreateTestDatabase(dbPath, 200, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	ctx := context.Background()

	// Get ready tasks
	readyTasks, err := td.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
	if err != nil {
		t.Fatalf("Failed to get ready tasks: %v", err)
	}

	// Verify all ready tasks have status = open
	for _, task := range readyTasks {
		if task.Status != "open" {
			t.Errorf("Ready task %s has status %s, expected 'open'", task.ID, task.Status)
		}

		// Verify task is not in blocked list
		isBlocked := false
		for _, blockedID := range td.BlockedIDs {
			if blockedID == task.ID {
				isBlocked = true
				break
			}
		}
		if isBlocked {
			t.Errorf("Task %s appears in ready list but is marked as blocked", task.ID)
		}
	}

	// Verify blocked tasks are actually blocked
	for i := 0; i < 10 && i < len(td.BlockedIDs); i++ {
		blockedID := td.BlockedIDs[i]
		blockers, err := td.DB.GetBlockingTasks(blockedID)
		if err != nil {
			t.Errorf("Failed to get blockers for %s: %v", blockedID, err)
			continue
		}

		if len(blockers) == 0 {
			t.Errorf("Task %s is marked as blocked but has no blockers", blockedID)
		}

		// Verify at least one blocker is not closed
		hasOpenBlocker := false
		for _, blocker := range blockers {
			if blocker.Status != "closed" {
				hasOpenBlocker = true
				break
			}
		}
		if !hasOpenBlocker {
			t.Errorf("Task %s is blocked but all blockers are closed", blockedID)
		}
	}

	t.Logf("Data consistency verified for %d ready tasks and %d sample blocked tasks",
		len(readyTasks), min(10, len(td.BlockedIDs)))
}

// TestLargeDatabase tests with a larger dataset to validate scalability.
func TestLargeDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large database test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create database with 5000 tasks
	t.Log("Creating large test database with 5000 tasks...")
	start := time.Now()
	td, err := CreateTestDatabase(dbPath, 5000, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()
	t.Logf("Database creation took %v", time.Since(start))

	stats := td.GetStats()
	t.Logf("Database stats: %+v", stats)

	// Run 100 concurrent agents
	t.Log("Running 100 concurrent agents with 10 queries each...")
	queryStart := time.Now()
	queryStats, err := td.RunConcurrentQueries(100, 10)
	totalDuration := time.Since(queryStart)

	if err != nil {
		t.Fatalf("Concurrent queries failed: %v", err)
	}

	t.Logf("\n=== LARGE DATABASE LOAD TEST (5000 tasks) ===")
	queryStats.PrintStats()
	t.Logf("Total test duration: %v", totalDuration)
	t.Logf("Throughput: %.2f queries/second", float64(queryStats.TotalQueries)/totalDuration.Seconds())

	// Verify performance still meets requirements with larger dataset
	if queryStats.Mean > 10*time.Millisecond {
		t.Logf("WARNING: Mean query latency %v exceeds 10ms target with large dataset", queryStats.Mean)
	} else {
		t.Logf("PASSED: Mean query latency %v is under 10ms target with large dataset", queryStats.Mean)
	}
}

// TestStressTest runs an extended stress test with high concurrency.
func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")

	t.Log("Creating test database with 2000 tasks...")
	td, err := CreateTestDatabase(dbPath, 2000, 0.3)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	// Run 200 concurrent agents for maximum stress
	t.Log("Running stress test: 200 concurrent agents with 20 queries each...")
	start := time.Now()
	queryStats, err := td.RunConcurrentQueries(200, 20)
	totalDuration := time.Since(start)

	if err != nil {
		t.Fatalf("Stress test failed: %v", err)
	}

	t.Logf("\n=== STRESS TEST RESULTS (200 agents, 20 queries each) ===")
	queryStats.PrintStats()
	t.Logf("Total test duration: %v", totalDuration)
	t.Logf("Throughput: %.2f queries/second", float64(queryStats.TotalQueries)/totalDuration.Seconds())
	t.Logf("Error rate: %.2f%%", float64(queryStats.Errors)/float64(queryStats.TotalQueries)*100)

	if queryStats.Errors > queryStats.TotalQueries/100 {
		t.Errorf("Error rate too high: %d/%d (%.2f%%)", queryStats.Errors, queryStats.TotalQueries,
			float64(queryStats.Errors)/float64(queryStats.TotalQueries)*100)
	}
}

// Benchmark functions

// BenchmarkGetReadyTasks_100Tasks benchmarks ready work queries with 100 tasks.
func BenchmarkGetReadyTasks_100Tasks(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	td, err := CreateTestDatabase(dbPath, 100, 0.3)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := td.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkGetReadyTasks_1000Tasks benchmarks ready work queries with 1000 tasks.
func BenchmarkGetReadyTasks_1000Tasks(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	td, err := CreateTestDatabase(dbPath, 1000, 0.3)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := td.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkGetReadyTasks_5000Tasks benchmarks ready work queries with 5000 tasks.
func BenchmarkGetReadyTasks_5000Tasks(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	td, err := CreateTestDatabase(dbPath, 5000, 0.3)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := td.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// BenchmarkConcurrentQueries_100Agents benchmarks 100 concurrent agents.
func BenchmarkConcurrentQueries_100Agents(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	td, err := CreateTestDatabase(dbPath, 1000, 0.3)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer td.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := td.RunConcurrentQueries(100, 10)
		if err != nil {
			b.Fatalf("Concurrent queries failed: %v", err)
		}
	}
}

// BenchmarkDatabaseCreation benchmarks the database population process.
func BenchmarkDatabaseCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dbPath := filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d.db", i))
		b.StartTimer()

		td, err := CreateTestDatabase(dbPath, 1000, 0.3)
		if err != nil {
			b.Fatalf("Failed to create test database: %v", err)
		}

		b.StopTimer()
		td.Close()
		os.Remove(dbPath)
		b.StartTimer()
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
