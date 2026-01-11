package benchmark

import (
	"testing"
	"time"
)

// TestTursoVsBaseline_100Agents compares Turso and baseline with 100 concurrent agents.
// This is the main validation test showing Turso's superiority at scale.
func TestTursoVsBaseline_100Agents(t *testing.T) {
	config := BenchmarkConfig{
		NumAgents:       100,
		NumTasks:        1000,
		QueriesPerAgent: 10,
		BlockedPct:      0.3,
	}

	result, err := Compare(config)
	if err != nil {
		t.Fatalf("Comparison failed: %v", err)
	}

	// Print full comparison report
	PrintComparison(result)

	// Validate Turso wins key metrics
	if result.OverallWinner != "turso" {
		t.Errorf("Expected Turso to win overall, got: %s", result.OverallWinner)
	}

	// Turso should have better P95 latency (more consistent)
	if result.LatencyImprovement["p95"] < 0 {
		t.Errorf("Expected Turso to have better P95 latency, got: %.2f%% worse", -result.LatencyImprovement["p95"])
	}

	// Turso should eliminate lock contention
	if result.Baseline.Concurrency.LockContention > 0 && result.Turso.Concurrency.LockContention > 0 {
		t.Errorf("Expected Turso to eliminate lock contention, got: %d events", result.Turso.Concurrency.LockContention)
	}

	t.Logf("Turso P95 improvement: %.1f%%", result.LatencyImprovement["p95"])
	t.Logf("Turso QPS improvement: %.1f%%", result.ThroughputImprovement)
	t.Logf("Lock contention eliminated: %d → %d events",
		result.Baseline.Concurrency.LockContention,
		result.Turso.Concurrency.LockContention)
}

// TestTursoVsBaseline_200Agents stress tests with 200 concurrent agents.
// This should show even more dramatic improvements for Turso.
func TestTursoVsBaseline_200Agents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := BenchmarkConfig{
		NumAgents:       200,
		NumTasks:        2000,
		QueriesPerAgent: 5,
		BlockedPct:      0.3,
	}

	result, err := Compare(config)
	if err != nil {
		t.Fatalf("Comparison failed: %v", err)
	}

	PrintComparison(result)

	// At 200 agents, Turso should dominate even more
	if result.OverallWinner != "turso" {
		t.Errorf("Expected Turso to win at 200 agents, got: %s", result.OverallWinner)
	}

	// Turso should scale better with more agents
	if result.ThroughputImprovement < 0 {
		t.Errorf("Expected Turso throughput to be better, got: %.2f%% worse", -result.ThroughputImprovement)
	}

	t.Logf("200-agent Turso improvement: P95=%.1f%%, QPS=%.1f%%",
		result.LatencyImprovement["p95"],
		result.ThroughputImprovement)
}

// TestScalability tests how both implementations scale from 10 to 200 agents.
func TestScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scalability test in short mode")
	}

	agentCounts := []int{10, 50, 100, 200}

	type scalabilityResult struct {
		Agents          int
		TursoP95        time.Duration
		BaselineP95     time.Duration
		TursoQPS        float64
		BaselineQPS     float64
		TursoLockEvents int
		BaseLockEvents  int
	}

	results := make([]scalabilityResult, 0, len(agentCounts))

	for _, numAgents := range agentCounts {
		t.Logf("Testing with %d agents...", numAgents)

		config := BenchmarkConfig{
			NumAgents:       numAgents,
			NumTasks:        1000,
			QueriesPerAgent: 10,
			BlockedPct:      0.3,
		}

		comparison, err := Compare(config)
		if err != nil {
			t.Fatalf("Comparison failed for %d agents: %v", numAgents, err)
		}

		result := scalabilityResult{
			Agents:          numAgents,
			TursoP95:        comparison.Turso.Latency.P95,
			BaselineP95:     comparison.Baseline.Latency.P95,
			TursoQPS:        comparison.Turso.Throughput.QueriesPerSecond,
			BaselineQPS:     comparison.Baseline.Throughput.QueriesPerSecond,
			TursoLockEvents: comparison.Turso.Concurrency.LockContention,
			BaseLockEvents:  comparison.Baseline.Concurrency.LockContention,
		}

		results = append(results, result)
	}

	// Print scalability report
	t.Log("\n" + "=== SCALABILITY REPORT ===")
	t.Logf("%-10s | %-15s | %-15s | %-10s | %-10s", "Agents", "Turso P95", "Baseline P95", "Turso QPS", "Base QPS")
	for _, r := range results {
		t.Logf("%-10d | %-15s | %-15s | %-10.2f | %-10.2f",
			r.Agents,
			FormatDuration(r.TursoP95),
			FormatDuration(r.BaselineP95),
			r.TursoQPS,
			r.BaselineQPS)
	}

	// Validate that Turso scales better
	// P95 should degrade less as agent count increases
	tursoP95Growth := float64(results[len(results)-1].TursoP95) / float64(results[0].TursoP95)
	baselineP95Growth := float64(results[len(results)-1].BaselineP95) / float64(results[0].BaselineP95)

	t.Logf("\nP95 growth (10→200 agents):")
	t.Logf("  Turso:    %.2fx", tursoP95Growth)
	t.Logf("  Baseline: %.2fx", baselineP95Growth)

	if tursoP95Growth > baselineP95Growth {
		t.Errorf("Expected Turso to scale better than baseline, but P95 grew more (%.2fx vs %.2fx)",
			tursoP95Growth, baselineP95Growth)
	}
}

// BenchmarkTurso benchmarks the Turso implementation using Go's benchmark framework.
func BenchmarkTurso(b *testing.B) {
	config := BenchmarkConfig{
		NumAgents:       100,
		NumTasks:        1000,
		QueriesPerAgent: 10,
		BlockedPct:      0.3,
		Mode:            "turso",
		DBPath:          "/tmp/bench-turso.db",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := RunTursoBenchmark(config)
		if err != nil {
			b.Fatalf("Turso benchmark failed: %v", err)
		}

		if result.ErrorCount > 0 {
			b.Fatalf("Turso had %d errors", result.ErrorCount)
		}
	}
}

// BenchmarkBaseline benchmarks the baseline JSONL implementation.
func BenchmarkBaseline(b *testing.B) {
	config := BenchmarkConfig{
		NumAgents:       100,
		NumTasks:        1000,
		QueriesPerAgent: 10,
		BlockedPct:      0.3,
		Mode:            "baseline",
		DBPath:          "/tmp/bench-baseline",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := RunBaselineBenchmark(config)
		if err != nil {
			b.Fatalf("Baseline benchmark failed: %v", err)
		}

		if result.ErrorCount > 0 {
			b.Fatalf("Baseline had %d errors", result.ErrorCount)
		}
	}
}

// TestTursoOnly runs just the Turso benchmark for quick validation.
func TestTursoOnly(t *testing.T) {
	config := BenchmarkConfig{
		NumAgents:       50,
		NumTasks:        500,
		QueriesPerAgent: 5,
		BlockedPct:      0.3,
		Mode:            "turso",
		DBPath:          "/tmp/test-turso.db",
	}

	result, err := RunTursoBenchmark(config)
	if err != nil {
		t.Fatalf("Turso benchmark failed: %v", err)
	}

	PrintResult(*result)

	// Validate basic metrics
	if result.ErrorCount > 0 {
		t.Errorf("Expected zero errors, got: %d", result.ErrorCount)
	}

	if result.Throughput.QueriesPerSecond <= 0 {
		t.Errorf("Invalid QPS: %.2f", result.Throughput.QueriesPerSecond)
	}

	if result.Latency.Mean == 0 {
		t.Error("Mean latency is zero")
	}

	t.Logf("Turso P95 latency: %s", FormatDuration(result.Latency.P95))
	t.Logf("Turso QPS: %.2f", result.Throughput.QueriesPerSecond)
}

// TestBaselineOnly runs just the baseline benchmark for quick validation.
func TestBaselineOnly(t *testing.T) {
	config := BenchmarkConfig{
		NumAgents:       50,
		NumTasks:        500,
		QueriesPerAgent: 5,
		BlockedPct:      0.3,
		Mode:            "baseline",
		DBPath:          "/tmp/test-baseline",
	}

	result, err := RunBaselineBenchmark(config)
	if err != nil {
		t.Fatalf("Baseline benchmark failed: %v", err)
	}

	PrintResult(*result)

	// Validate basic metrics
	if result.ErrorCount > 0 {
		t.Errorf("Expected zero errors, got: %d", result.ErrorCount)
	}

	if result.Throughput.QueriesPerSecond <= 0 {
		t.Errorf("Invalid QPS: %.2f", result.Throughput.QueriesPerSecond)
	}

	if result.Latency.Mean == 0 {
		t.Error("Mean latency is zero")
	}

	t.Logf("Baseline P95 latency: %s", FormatDuration(result.Latency.P95))
	t.Logf("Baseline QPS: %.2f", result.Throughput.QueriesPerSecond)
	t.Logf("Lock contention events: %d", result.Concurrency.LockContention)
}
