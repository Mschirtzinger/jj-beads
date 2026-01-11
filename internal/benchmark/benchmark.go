// Package benchmark provides comprehensive performance comparison between
// jj-turso and baseline (JSONL) implementations for the beads issue tracker.
//
// This package implements a benchmark framework to validate that Turso provides
// better performance than the baseline JSONL approach, especially under high
// concurrency (100+ concurrent agents).
package benchmark

import (
	"fmt"
	"runtime"
	"sort"
	"time"
)

// BenchmarkConfig defines the parameters for a benchmark run.
type BenchmarkConfig struct {
	// NumAgents is the number of concurrent agents to simulate
	NumAgents int

	// NumTasks is the total number of tasks in the database
	NumTasks int

	// QueriesPerAgent is how many queries each agent performs
	QueriesPerAgent int

	// BlockedPct is the percentage of tasks that should be blocked (0.0-1.0)
	BlockedPct float64

	// Mode specifies which implementation to benchmark ("turso" or "baseline")
	Mode string

	// DBPath is the path to the database file (for turso) or JSONL file (for baseline)
	DBPath string
}

// DefaultConfig returns a benchmark configuration with sensible defaults.
func DefaultConfig() BenchmarkConfig {
	return BenchmarkConfig{
		NumAgents:       100,
		NumTasks:        1000,
		QueriesPerAgent: 10,
		BlockedPct:      0.3,
		Mode:            "turso",
		DBPath:          "/tmp/bench.db",
	}
}

// BenchmarkResult captures all metrics from a benchmark run.
type BenchmarkResult struct {
	// Configuration used for this run
	Config BenchmarkConfig

	// Latency metrics (query performance)
	Latency LatencyMetrics

	// Throughput metrics
	Throughput ThroughputMetrics

	// Resource usage metrics
	Resources ResourceMetrics

	// Concurrency metrics
	Concurrency ConcurrencyMetrics

	// Database metrics
	Database DatabaseMetrics

	// Overall test metrics
	TotalDuration time.Duration
	ErrorCount    int
	ErrorRate     float64
	Success       bool
}

// LatencyMetrics captures query latency statistics.
type LatencyMetrics struct {
	Min  time.Duration
	P50  time.Duration // Median
	Mean time.Duration
	P95  time.Duration
	P99  time.Duration
	Max  time.Duration

	// Raw durations for analysis
	Durations []time.Duration
}

// ThroughputMetrics captures queries-per-second metrics.
type ThroughputMetrics struct {
	QueriesPerSecond float64
	TotalQueries     int
}

// ResourceMetrics captures memory and CPU usage.
type ResourceMetrics struct {
	MemoryBeforeBytes uint64
	MemoryAfterBytes  uint64
	MemoryPeakBytes   uint64
	MemoryDeltaBytes  uint64
}

// ConcurrencyMetrics captures concurrent access patterns.
type ConcurrencyMetrics struct {
	TargetAgents   int
	ActualAgents   int
	LockContention int // Number of lock contention events (baseline only)
}

// DatabaseMetrics captures database/file statistics.
type DatabaseMetrics struct {
	SizeBytes      int64
	SyncTimeMs     int64 // Time to sync/write changes
	TimeToFirstMs  int64 // Time to first query
	TaskCount      int
	ReadyTaskCount int
}

// ComputeStats calculates statistics from raw durations.
func ComputeStats(durations []time.Duration) LatencyMetrics {
	if len(durations) == 0 {
		return LatencyMetrics{}
	}

	// Sort for percentile calculation
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate mean
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	mean := sum / time.Duration(len(sorted))

	// Calculate percentiles
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]

	return LatencyMetrics{
		Min:       sorted[0],
		P50:       p50,
		Mean:      mean,
		P95:       p95,
		P99:       p99,
		Max:       sorted[len(sorted)-1],
		Durations: sorted,
	}
}

// GetMemoryStats returns current memory usage statistics.
func GetMemoryStats() ResourceMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return ResourceMetrics{
		MemoryBeforeBytes: m.Alloc,
		MemoryAfterBytes:  m.Alloc,
		MemoryPeakBytes:   m.Sys,
		MemoryDeltaBytes:  0,
	}
}

// CompareMemoryStats computes the delta between before and after memory stats.
func CompareMemoryStats(before, after ResourceMetrics) ResourceMetrics {
	delta := after.MemoryAfterBytes - before.MemoryBeforeBytes

	return ResourceMetrics{
		MemoryBeforeBytes: before.MemoryBeforeBytes,
		MemoryAfterBytes:  after.MemoryAfterBytes,
		MemoryPeakBytes:   after.MemoryPeakBytes,
		MemoryDeltaBytes:  delta,
	}
}

// FormatBytes formats bytes into a human-readable string.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a duration into a human-readable string.
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000.0)
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// PrintResult outputs a formatted benchmark result.
func PrintResult(result BenchmarkResult) {
	fmt.Printf("\n=== Benchmark Results (%s mode) ===\n\n", result.Config.Mode)

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Concurrent Agents:  %d\n", result.Config.NumAgents)
	fmt.Printf("  Total Tasks:        %d\n", result.Config.NumTasks)
	fmt.Printf("  Queries per Agent:  %d\n", result.Config.QueriesPerAgent)
	fmt.Printf("  Blocked %%:          %.1f%%\n", result.Config.BlockedPct*100)
	fmt.Printf("\n")

	fmt.Printf("Latency:\n")
	fmt.Printf("  Min:       %s\n", FormatDuration(result.Latency.Min))
	fmt.Printf("  P50:       %s\n", FormatDuration(result.Latency.P50))
	fmt.Printf("  Mean:      %s\n", FormatDuration(result.Latency.Mean))
	fmt.Printf("  P95:       %s\n", FormatDuration(result.Latency.P95))
	fmt.Printf("  P99:       %s\n", FormatDuration(result.Latency.P99))
	fmt.Printf("  Max:       %s\n", FormatDuration(result.Latency.Max))
	fmt.Printf("\n")

	fmt.Printf("Throughput:\n")
	fmt.Printf("  Queries/sec:       %.2f\n", result.Throughput.QueriesPerSecond)
	fmt.Printf("  Total Queries:     %d\n", result.Throughput.TotalQueries)
	fmt.Printf("\n")

	fmt.Printf("Resources:\n")
	fmt.Printf("  Memory Before:     %s\n", FormatBytes(result.Resources.MemoryBeforeBytes))
	fmt.Printf("  Memory After:      %s\n", FormatBytes(result.Resources.MemoryAfterBytes))
	fmt.Printf("  Memory Peak:       %s\n", FormatBytes(result.Resources.MemoryPeakBytes))
	fmt.Printf("  Memory Delta:      %s\n", FormatBytes(result.Resources.MemoryDeltaBytes))
	fmt.Printf("\n")

	fmt.Printf("Concurrency:\n")
	fmt.Printf("  Target Agents:     %d\n", result.Concurrency.TargetAgents)
	fmt.Printf("  Actual Agents:     %d\n", result.Concurrency.ActualAgents)
	if result.Config.Mode == "baseline" {
		fmt.Printf("  Lock Contention:   %d events\n", result.Concurrency.LockContention)
	}
	fmt.Printf("\n")

	fmt.Printf("Database:\n")
	fmt.Printf("  Size:              %s\n", FormatBytes(uint64(result.Database.SizeBytes)))
	fmt.Printf("  Sync Time:         %dms\n", result.Database.SyncTimeMs)
	fmt.Printf("  Time to First:     %dms\n", result.Database.TimeToFirstMs)
	fmt.Printf("  Tasks:             %d\n", result.Database.TaskCount)
	fmt.Printf("  Ready Tasks:       %d\n", result.Database.ReadyTaskCount)
	fmt.Printf("\n")

	fmt.Printf("Overall:\n")
	fmt.Printf("  Total Duration:    %s\n", FormatDuration(result.TotalDuration))
	fmt.Printf("  Errors:            %d (%.2f%%)\n", result.ErrorCount, result.ErrorRate*100)
	fmt.Printf("  Success:           %v\n", result.Success)
	fmt.Printf("\n")
}
