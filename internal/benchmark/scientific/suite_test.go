package scientific

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBeadsRunner(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_beads.db")

	// Create runner
	runner, err := NewBeadsRunner(dbPath, 100, 0.3, 42)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Close()

	// Verify stats
	stats := runner.GetStats()
	totalTasks := stats["total_tasks"].(int)
	readyTasks := stats["ready_tasks"].(int)

	if totalTasks != 100 {
		t.Errorf("expected 100 total tasks, got %d", totalTasks)
	}

	if readyTasks == 0 {
		t.Errorf("expected some ready tasks, got 0")
	}

	t.Logf("Stats: %+v", stats)

	// Run quick benchmark
	ctx := context.Background()
	result, err := runner.RunBenchmark(ctx, 5, 10)
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	if result.TotalQueries != 50 {
		t.Errorf("expected 50 queries (5 agents * 10 queries), got %d", result.TotalQueries)
	}

	if result.ErrorCount > 0 {
		t.Errorf("got %d errors", result.ErrorCount)
	}

	t.Logf("Benchmark: P50=%v, P95=%v, QPS=%.0f", result.P50, result.P95, result.QueriesPerSecond)
}

func TestTursoRunner(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_turso.db")

	// Create runner
	runner, err := NewTursoRunner(dbPath, 100, 0.3, 42)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Close()

	// Verify stats
	stats := runner.GetStats()
	totalTasks := stats["total_tasks"].(int)
	readyTasks := stats["ready_tasks"].(int)

	if totalTasks != 100 {
		t.Errorf("expected 100 total tasks, got %d", totalTasks)
	}

	if readyTasks == 0 {
		t.Errorf("expected some ready tasks, got 0")
	}

	t.Logf("Stats: %+v", stats)

	// Run quick benchmark
	ctx := context.Background()
	result, err := runner.RunBenchmark(ctx, 5, 10)
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	if result.TotalQueries != 50 {
		t.Errorf("expected 50 queries (5 agents * 10 queries), got %d", result.TotalQueries)
	}

	if result.ErrorCount > 0 {
		t.Errorf("got %d errors", result.ErrorCount)
	}

	t.Logf("Benchmark: P50=%v, P95=%v, QPS=%.0f", result.P50, result.P95, result.QueriesPerSecond)
}

func TestFullSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full suite in short mode")
	}

	tmpDir := t.TempDir()

	// Use quick config for testing
	config := QuickConfig()

	// Run suite
	results, err := RunSuite(config, tmpDir)
	if err != nil {
		t.Fatalf("suite failed: %v", err)
	}

	// Verify we got data points
	expectedPoints := len(config.TaskCounts) * len(config.AgentCounts) * 2 * config.MeasurementRuns
	if len(results.DataPoints) != expectedPoints {
		t.Errorf("expected %d data points, got %d", expectedPoints, len(results.DataPoints))
	}

	// Verify no errors
	for _, dp := range results.DataPoints {
		if dp.ErrorCount > 0 {
			t.Errorf("data point had errors: %+v", dp)
		}
	}

	// Generate reports
	if err := GenerateReports(results, tmpDir); err != nil {
		t.Fatalf("failed to generate reports: %v", err)
	}

	// Verify files were created
	expectedFiles := []string{"results.json", "results.csv", "REPORT.md"}
	for _, filename := range expectedFiles {
		path := filepath.Join(tmpDir, filename)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not created: %v", filename, err)
		}
	}
}

func TestReproducibility(t *testing.T) {
	// Run the same benchmark twice with the same seed
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	const seed = 42
	const taskCount = 100
	const agentCount = 10
	const queriesPerAgent = 20

	// First run
	runner1, err := NewBeadsRunner(filepath.Join(tmpDir1, "test.db"), taskCount, 0.3, seed)
	if err != nil {
		t.Fatalf("failed to create runner1: %v", err)
	}
	defer runner1.Close()

	stats1 := runner1.GetStats()

	// Second run
	runner2, err := NewBeadsRunner(filepath.Join(tmpDir2, "test.db"), taskCount, 0.3, seed)
	if err != nil {
		t.Fatalf("failed to create runner2: %v", err)
	}
	defer runner2.Close()

	stats2 := runner2.GetStats()

	// Verify identical stats
	if stats1["total_tasks"] != stats2["total_tasks"] {
		t.Errorf("total_tasks mismatch: %v vs %v", stats1["total_tasks"], stats2["total_tasks"])
	}

	if stats1["ready_tasks"] != stats2["ready_tasks"] {
		t.Errorf("ready_tasks mismatch: %v vs %v", stats1["ready_tasks"], stats2["ready_tasks"])
	}

	// Verify task count is identical
	if len(runner1.taskIDs) != len(runner2.taskIDs) {
		t.Errorf("taskIDs length mismatch: %d vs %d", len(runner1.taskIDs), len(runner2.taskIDs))
	}

	// Note: We don't compare task IDs themselves because beads uses hash-based IDs
	// which include timestamps and may not be perfectly deterministic.
	// The important part is that the stats (ready/blocked counts) are identical.

	t.Logf("Reproducibility verified: same seed produces identical test data distribution")
}

func TestQuickSuite(t *testing.T) {
	// Quick test for CI - minimal configuration
	tmpDir := t.TempDir()

	config := SuiteConfig{
		AgentCounts:     []int{5, 10},
		TaskCounts:      []int{50},
		QueriesPerAgent: 5,
		WarmupRuns:      1,
		MeasurementRuns: 2,
		BlockedPercent:  0.3,
		Seed:            42,
	}

	results, err := RunSuite(config, tmpDir)
	if err != nil {
		t.Fatalf("quick suite failed: %v", err)
	}

	// Just verify it ran successfully
	if len(results.DataPoints) == 0 {
		t.Error("no data points generated")
	}

	// Print summary
	PrintGraphs(results)
	PrintScalingAnalysis(results)
}
