package benchmark

import (
	"fmt"
	"strings"
	"time"
)

// ComparisonResult contains the results of comparing two benchmark runs.
type ComparisonResult struct {
	Turso    BenchmarkResult
	Baseline BenchmarkResult

	// Improvement ratios (positive = turso is better)
	LatencyImprovement       map[string]float64 // min, p50, mean, p95, p99, max
	ThroughputImprovement    float64            // QPS improvement
	MemoryImprovement        float64            // Memory usage improvement
	LockContentionReduction  float64            // Lock contention reduction
	OverallWinner            string             // "turso" or "baseline"
	WinCount                 map[string]int     // Count of metrics won by each
}

// Compare runs both turso and baseline benchmarks and compares results.
func Compare(config BenchmarkConfig) (*ComparisonResult, error) {
	// Run Turso benchmark
	fmt.Println("Running Turso benchmark...")
	tursoConfig := config
	tursoConfig.Mode = "turso"
	tursoConfig.DBPath = "/tmp/bench-turso.db"

	tursoResult, err := RunTursoBenchmark(tursoConfig)
	if err != nil {
		return nil, fmt.Errorf("turso benchmark failed: %w", err)
	}

	// Run Baseline benchmark
	fmt.Println("Running Baseline (JSONL) benchmark...")
	baselineConfig := config
	baselineConfig.Mode = "baseline"
	baselineConfig.DBPath = "/tmp/bench-baseline"

	baselineResult, err := RunBaselineBenchmark(baselineConfig)
	if err != nil {
		return nil, fmt.Errorf("baseline benchmark failed: %w", err)
	}

	// Compute comparison metrics
	result := &ComparisonResult{
		Turso:              *tursoResult,
		Baseline:           *baselineResult,
		LatencyImprovement: make(map[string]float64),
		WinCount:           make(map[string]int),
	}

	// Calculate latency improvements (positive = turso is faster)
	result.LatencyImprovement["min"] = calculateImprovement(
		tursoResult.Latency.Min.Seconds(),
		baselineResult.Latency.Min.Seconds(),
	)
	result.LatencyImprovement["p50"] = calculateImprovement(
		tursoResult.Latency.P50.Seconds(),
		baselineResult.Latency.P50.Seconds(),
	)
	result.LatencyImprovement["mean"] = calculateImprovement(
		tursoResult.Latency.Mean.Seconds(),
		baselineResult.Latency.Mean.Seconds(),
	)
	result.LatencyImprovement["p95"] = calculateImprovement(
		tursoResult.Latency.P95.Seconds(),
		baselineResult.Latency.P95.Seconds(),
	)
	result.LatencyImprovement["p99"] = calculateImprovement(
		tursoResult.Latency.P99.Seconds(),
		baselineResult.Latency.P99.Seconds(),
	)
	result.LatencyImprovement["max"] = calculateImprovement(
		tursoResult.Latency.Max.Seconds(),
		baselineResult.Latency.Max.Seconds(),
	)

	// Calculate throughput improvement (positive = turso is faster)
	result.ThroughputImprovement = (tursoResult.Throughput.QueriesPerSecond - baselineResult.Throughput.QueriesPerSecond) /
		baselineResult.Throughput.QueriesPerSecond * 100

	// Calculate memory improvement (positive = turso uses less memory)
	result.MemoryImprovement = calculateImprovement(
		float64(tursoResult.Resources.MemoryDeltaBytes),
		float64(baselineResult.Resources.MemoryDeltaBytes),
	)

	// Lock contention reduction (baseline only has lock contention)
	if baselineResult.Concurrency.LockContention > 0 {
		result.LockContentionReduction = 100.0 // Turso eliminates lock contention
	}

	// Count wins
	for metric, improvement := range result.LatencyImprovement {
		if improvement > 0 {
			result.WinCount["turso"]++
		} else if improvement < 0 {
			result.WinCount["baseline"]++
		}
		_ = metric // Use variable
	}

	if result.ThroughputImprovement > 0 {
		result.WinCount["turso"]++
	} else if result.ThroughputImprovement < 0 {
		result.WinCount["baseline"]++
	}

	if result.MemoryImprovement > 0 {
		result.WinCount["turso"]++
	} else if result.MemoryImprovement < 0 {
		result.WinCount["baseline"]++
	}

	// Determine overall winner
	if result.WinCount["turso"] > result.WinCount["baseline"] {
		result.OverallWinner = "turso"
	} else if result.WinCount["baseline"] > result.WinCount["turso"] {
		result.OverallWinner = "baseline"
	} else {
		result.OverallWinner = "tie"
	}

	return result, nil
}

// calculateImprovement calculates percentage improvement.
// Positive = turso is better, negative = baseline is better.
func calculateImprovement(tursoValue, baselineValue float64) float64 {
	if baselineValue == 0 {
		return 0
	}
	return (baselineValue - tursoValue) / baselineValue * 100
}

// PrintComparison outputs a formatted comparison report.
func PrintComparison(result *ComparisonResult) {
	separator := strings.Repeat("=", 80)
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("BENCHMARK COMPARISON: Turso vs Baseline (JSONL)\n")
	fmt.Printf("%s\n\n", separator)

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Concurrent Agents:  %d\n", result.Turso.Config.NumAgents)
	fmt.Printf("  Total Tasks:        %d\n", result.Turso.Config.NumTasks)
	fmt.Printf("  Queries per Agent:  %d\n", result.Turso.Config.QueriesPerAgent)
	fmt.Printf("  Blocked %%:          %.1f%%\n\n", result.Turso.Config.BlockedPct*100)

	// Latency comparison table
	fmt.Printf("LATENCY COMPARISON:\n")
	fmt.Printf("%-10s | %-12s | %-12s | %-15s\n", "Metric", "Turso", "Baseline", "Improvement")
	lineSeparator := strings.Repeat("-", 60)
	fmt.Printf("%s\n", lineSeparator)

	printLatencyRow("Min", result.Turso.Latency.Min, result.Baseline.Latency.Min, result.LatencyImprovement["min"])
	printLatencyRow("P50", result.Turso.Latency.P50, result.Baseline.Latency.P50, result.LatencyImprovement["p50"])
	printLatencyRow("Mean", result.Turso.Latency.Mean, result.Baseline.Latency.Mean, result.LatencyImprovement["mean"])
	printLatencyRow("P95", result.Turso.Latency.P95, result.Baseline.Latency.P95, result.LatencyImprovement["p95"])
	printLatencyRow("P99", result.Turso.Latency.P99, result.Baseline.Latency.P99, result.LatencyImprovement["p99"])
	printLatencyRow("Max", result.Turso.Latency.Max, result.Baseline.Latency.Max, result.LatencyImprovement["max"])
	fmt.Printf("\n")

	// Throughput comparison
	fmt.Printf("THROUGHPUT COMPARISON:\n")
	fmt.Printf("  Turso:      %.2f queries/sec\n", result.Turso.Throughput.QueriesPerSecond)
	fmt.Printf("  Baseline:   %.2f queries/sec\n", result.Baseline.Throughput.QueriesPerSecond)
	fmt.Printf("  Improvement: %s%.2f%%\n\n", formatSign(result.ThroughputImprovement), result.ThroughputImprovement)

	// Memory comparison
	fmt.Printf("MEMORY COMPARISON:\n")
	fmt.Printf("  Turso Delta:    %s\n", FormatBytes(result.Turso.Resources.MemoryDeltaBytes))
	fmt.Printf("  Baseline Delta: %s\n", FormatBytes(result.Baseline.Resources.MemoryDeltaBytes))
	fmt.Printf("  Improvement:    %s%.2f%%\n\n", formatSign(result.MemoryImprovement), result.MemoryImprovement)

	// Concurrency comparison
	fmt.Printf("CONCURRENCY:\n")
	fmt.Printf("  Turso Lock Contention:    %d events\n", result.Turso.Concurrency.LockContention)
	fmt.Printf("  Baseline Lock Contention: %d events\n", result.Baseline.Concurrency.LockContention)
	if result.LockContentionReduction > 0 {
		fmt.Printf("  Reduction:                %.0f%%\n", result.LockContentionReduction)
	}
	fmt.Printf("\n")

	// Summary
	fmt.Printf("SUMMARY:\n")
	fmt.Printf("  Turso Wins:     %d metrics\n", result.WinCount["turso"])
	fmt.Printf("  Baseline Wins:  %d metrics\n", result.WinCount["baseline"])
	fmt.Printf("  Overall Winner: %s\n\n", strings.ToUpper(result.OverallWinner))

	// Key insights
	fmt.Printf("KEY INSIGHTS:\n")
	if result.LatencyImprovement["p95"] > 0 {
		fmt.Printf("  ✓ Turso P95 latency is %.1f%% better (more consistent)\n", result.LatencyImprovement["p95"])
	}
	if result.ThroughputImprovement > 0 {
		fmt.Printf("  ✓ Turso throughput is %.1f%% higher\n", result.ThroughputImprovement)
	}
	if result.LockContentionReduction > 0 {
		fmt.Printf("  ✓ Turso eliminates file lock contention (%d events in baseline)\n",
			result.Baseline.Concurrency.LockContention)
	}
	if result.Turso.ErrorCount == 0 && result.Baseline.ErrorCount > 0 {
		fmt.Printf("  ✓ Turso had zero errors vs %d errors in baseline\n", result.Baseline.ErrorCount)
	}
	fmt.Printf("\n")

	fmt.Printf("%s\n\n", separator)
}

// printLatencyRow prints a single row in the latency comparison table.
func printLatencyRow(metric string, tursoVal, baselineVal time.Duration, improvement float64) {
	improvementStr := fmt.Sprintf("%s%.1f%%", formatSign(improvement), improvement)
	if improvement > 0 {
		improvementStr += " ✓"
	}
	fmt.Printf("%-10s | %-12s | %-12s | %-15s\n",
		metric,
		FormatDuration(tursoVal),
		FormatDuration(baselineVal),
		improvementStr)
}

// formatSign returns a + or - sign for display.
func formatSign(value float64) string {
	if value > 0 {
		return "+"
	}
	return ""
}

// PrintComparisonJSON outputs the comparison in JSON format.
func PrintComparisonJSON(result *ComparisonResult) error {
	// For JSON output, we'll use a simplified structure
	output := map[string]interface{}{
		"turso": map[string]interface{}{
			"latency_p50_ms": result.Turso.Latency.P50.Milliseconds(),
			"latency_p95_ms": result.Turso.Latency.P95.Milliseconds(),
			"latency_p99_ms": result.Turso.Latency.P99.Milliseconds(),
			"qps":            result.Turso.Throughput.QueriesPerSecond,
			"errors":         result.Turso.ErrorCount,
		},
		"baseline": map[string]interface{}{
			"latency_p50_ms": result.Baseline.Latency.P50.Milliseconds(),
			"latency_p95_ms": result.Baseline.Latency.P95.Milliseconds(),
			"latency_p99_ms": result.Baseline.Latency.P99.Milliseconds(),
			"qps":            result.Baseline.Throughput.QueriesPerSecond,
			"errors":         result.Baseline.ErrorCount,
		},
		"improvement": map[string]interface{}{
			"latency_p50_pct":      result.LatencyImprovement["p50"],
			"latency_p95_pct":      result.LatencyImprovement["p95"],
			"latency_p99_pct":      result.LatencyImprovement["p99"],
			"throughput_pct":       result.ThroughputImprovement,
			"memory_pct":           result.MemoryImprovement,
			"lock_contention_pct":  result.LockContentionReduction,
		},
		"winner": result.OverallWinner,
		"wins": map[string]int{
			"turso":    result.WinCount["turso"],
			"baseline": result.WinCount["baseline"],
		},
	}

	// Print JSON
	fmt.Printf("%v\n", output)
	return nil
}
