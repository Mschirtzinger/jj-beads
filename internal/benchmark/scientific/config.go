// Package scientific provides a publishable benchmark suite for comparing
// beads SQLite and jj-turso implementations.
//
// This package implements scientific benchmarking with:
// - Reproducible test data via deterministic seeding
// - Warmup runs to eliminate cold-start effects
// - Multiple measurement runs with statistical analysis
// - Fair comparison (both use SQLite, different optimizations)
// - Exportable results (JSON, CSV) for external analysis
package scientific

import (
	"runtime"
	"time"
)

// SuiteConfig configures the benchmark suite parameters.
type SuiteConfig struct {
	// AgentCounts is the list of concurrent agent counts to test
	// Example: []int{10, 25, 50, 75, 100, 150, 200}
	AgentCounts []int

	// TaskCounts is the list of task database sizes to test
	// Example: []int{500, 1000, 2000}
	TaskCounts []int

	// QueriesPerAgent is how many queries each agent performs per run
	QueriesPerAgent int

	// WarmupRuns is the number of warmup iterations before measuring
	// This eliminates cold-start effects (page cache, connection pool warmup)
	WarmupRuns int

	// MeasurementRuns is the number of runs to average for final metrics
	// More runs = more stable statistics but longer runtime
	MeasurementRuns int

	// BlockedPercent is the percentage of tasks that should be blocked by dependencies
	// Typical: 0.3 for 30% blocked (realistic for development workflows)
	BlockedPercent float64

	// Seed is the random seed for reproducible test data generation
	Seed int64
}

// DefaultConfig returns a comprehensive benchmark configuration suitable for publication.
func DefaultConfig() SuiteConfig {
	return SuiteConfig{
		AgentCounts:     []int{10, 25, 50, 75, 100, 150, 200},
		TaskCounts:      []int{500, 1000, 2000},
		QueriesPerAgent: 50,
		WarmupRuns:      3,
		MeasurementRuns: 5,
		BlockedPercent:  0.3,
		Seed:            42, // Reproducibility
	}
}

// QuickConfig returns a faster configuration for development and CI.
func QuickConfig() SuiteConfig {
	return SuiteConfig{
		AgentCounts:     []int{10, 50, 100},
		TaskCounts:      []int{500, 1000},
		QueriesPerAgent: 20,
		WarmupRuns:      1,
		MeasurementRuns: 3,
		BlockedPercent:  0.3,
		Seed:            42,
	}
}

// DataPoint represents a single benchmark measurement.
type DataPoint struct {
	// Test configuration
	AgentCount     int    `json:"agent_count"`
	TaskCount      int    `json:"task_count"`
	Implementation string `json:"implementation"` // "beads-sqlite" or "jj-turso"

	// Latency metrics (nanoseconds for precision)
	LatencyMin    int64 `json:"latency_min_ns"`
	LatencyP50    int64 `json:"latency_p50_ns"`
	LatencyP95    int64 `json:"latency_p95_ns"`
	LatencyP99    int64 `json:"latency_p99_ns"`
	LatencyMax    int64 `json:"latency_max_ns"`
	LatencyMean   int64 `json:"latency_mean_ns"`
	LatencyStdDev int64 `json:"latency_stddev_ns"`

	// Throughput
	QueriesPerSecond float64 `json:"queries_per_second"`

	// Errors
	ErrorCount int     `json:"error_count"`
	ErrorRate  float64 `json:"error_rate"`

	// Resources (not yet implemented - future enhancement)
	MemoryDeltaBytes int64 `json:"memory_delta_bytes,omitempty"`

	// Contention (not yet implemented - future enhancement)
	LockWaitEvents int `json:"lock_wait_events,omitempty"`

	// Timing
	TotalDurationNs int64 `json:"total_duration_ns"`

	// Run metadata
	RunNumber int `json:"run_number"` // Which measurement run (1-based)
}

// SuiteResults contains all benchmark results and metadata.
type SuiteResults struct {
	Config     SuiteConfig  `json:"config"`
	DataPoints []DataPoint  `json:"data_points"`
	StartTime  time.Time    `json:"start_time"`
	EndTime    time.Time    `json:"end_time"`
	SystemInfo SystemInfo   `json:"system_info"`
}

// SystemInfo captures system details for reproducibility.
type SystemInfo struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	GoVersion string `json:"go_version"`
	GitCommit string `json:"git_commit,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
}

// GetSystemInfo captures current system information.
func GetSystemInfo() SystemInfo {
	return SystemInfo{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
		GoVersion: runtime.Version(),
		// GitCommit and Hostname filled in by suite runner
	}
}

// TotalRuns returns the total number of benchmark runs that will be executed.
func (c SuiteConfig) TotalRuns() int {
	// 2 implementations × task counts × agent counts × (warmup + measurement)
	return 2 * len(c.TaskCounts) * len(c.AgentCounts) * (c.WarmupRuns + c.MeasurementRuns)
}
