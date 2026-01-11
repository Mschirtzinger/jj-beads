package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/benchmark"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks comparing Turso vs baseline (JSONL)",
	Long: `Run comprehensive performance benchmarks comparing jj-turso implementation
against baseline JSONL implementation.

This command creates test databases with the specified number of tasks and
concurrent agents, then measures query latency, throughput, memory usage,
and concurrency characteristics.

Modes:
  compare  - Run both Turso and baseline, show comparison (default)
  turso    - Run only Turso benchmark
  baseline - Run only baseline (JSONL) benchmark

Examples:
  # Compare with default settings (100 agents, 1000 tasks)
  bd benchmark

  # Compare with 200 agents and 2000 tasks
  bd benchmark --agents 200 --tasks 2000

  # Run only Turso benchmark
  bd benchmark --mode turso --agents 100

  # Output comparison as JSON
  bd benchmark --json
`,
	Run:     runBenchmark,
	GroupID: "maint",
}

func init() {
	benchmarkCmd.Flags().Int("agents", 100, "Number of concurrent agents to simulate")
	benchmarkCmd.Flags().Int("tasks", 1000, "Total number of tasks in the database")
	benchmarkCmd.Flags().Int("queries", 10, "Number of queries per agent")
	benchmarkCmd.Flags().Float64("blocked", 0.3, "Percentage of tasks that should be blocked (0.0-1.0)")
	benchmarkCmd.Flags().String("mode", "compare", "Benchmark mode: compare, turso, or baseline")
	benchmarkCmd.Flags().Bool("json", false, "Output results as JSON")
	rootCmd.AddCommand(benchmarkCmd)
}

func runBenchmark(cmd *cobra.Command, args []string) {
	agents, _ := cmd.Flags().GetInt("agents")
	tasks, _ := cmd.Flags().GetInt("tasks")
	queries, _ := cmd.Flags().GetInt("queries")
	blocked, _ := cmd.Flags().GetFloat64("blocked")
	mode, _ := cmd.Flags().GetString("mode")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Validate flags
	if agents <= 0 {
		fmt.Fprintf(os.Stderr, "Error: --agents must be positive\n")
		os.Exit(1)
	}
	if tasks <= 0 {
		fmt.Fprintf(os.Stderr, "Error: --tasks must be positive\n")
		os.Exit(1)
	}
	if queries <= 0 {
		fmt.Fprintf(os.Stderr, "Error: --queries must be positive\n")
		os.Exit(1)
	}
	if blocked < 0 || blocked > 1 {
		fmt.Fprintf(os.Stderr, "Error: --blocked must be between 0.0 and 1.0\n")
		os.Exit(1)
	}
	if mode != "compare" && mode != "turso" && mode != "baseline" {
		fmt.Fprintf(os.Stderr, "Error: --mode must be 'compare', 'turso', or 'baseline'\n")
		os.Exit(1)
	}

	// Create config
	config := benchmark.BenchmarkConfig{
		NumAgents:       agents,
		NumTasks:        tasks,
		QueriesPerAgent: queries,
		BlockedPct:      blocked,
	}

	// Run benchmark based on mode
	switch mode {
	case "compare":
		runCompareBenchmark(config, jsonOutput)
	case "turso":
		runTursoOnlyBenchmark(config, jsonOutput)
	case "baseline":
		runBaselineOnlyBenchmark(config, jsonOutput)
	}
}

func runCompareBenchmark(config benchmark.BenchmarkConfig, jsonOutput bool) {
	fmt.Println("Running comprehensive benchmark comparison...")
	fmt.Printf("Configuration: %d agents, %d tasks, %d queries/agent, %.0f%% blocked\n\n",
		config.NumAgents, config.NumTasks, config.QueriesPerAgent, config.BlockedPct*100)

	result, err := benchmark.Compare(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		outputComparisonJSON(result)
	} else {
		benchmark.PrintComparison(result)
	}

	// Exit with code 1 if Turso didn't win (for CI/CD validation)
	if result.OverallWinner != "turso" {
		fmt.Fprintf(os.Stderr, "WARNING: Expected Turso to win, but %s won\n", result.OverallWinner)
		os.Exit(1)
	}
}

func runTursoOnlyBenchmark(config benchmark.BenchmarkConfig, jsonOutput bool) {
	fmt.Println("Running Turso-only benchmark...")
	fmt.Printf("Configuration: %d agents, %d tasks, %d queries/agent\n\n",
		config.NumAgents, config.NumTasks, config.QueriesPerAgent)

	config.Mode = "turso"
	config.DBPath = "/tmp/bench-turso.db"

	result, err := benchmark.RunTursoBenchmark(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		outputResultJSON(result)
	} else {
		benchmark.PrintResult(*result)
	}

	if result.ErrorCount > 0 {
		os.Exit(1)
	}
}

func runBaselineOnlyBenchmark(config benchmark.BenchmarkConfig, jsonOutput bool) {
	fmt.Println("Running Baseline (JSONL) benchmark...")
	fmt.Printf("Configuration: %d agents, %d tasks, %d queries/agent\n\n",
		config.NumAgents, config.NumTasks, config.QueriesPerAgent)

	config.Mode = "baseline"
	config.DBPath = "/tmp/bench-baseline"

	result, err := benchmark.RunBaselineBenchmark(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		outputResultJSON(result)
	} else {
		benchmark.PrintResult(*result)
	}

	if result.ErrorCount > 0 {
		os.Exit(1)
	}
}

func outputComparisonJSON(result *benchmark.ComparisonResult) {
	output := map[string]interface{}{
		"config": map[string]interface{}{
			"agents":  result.Turso.Config.NumAgents,
			"tasks":   result.Turso.Config.NumTasks,
			"queries": result.Turso.Config.QueriesPerAgent,
			"blocked": result.Turso.Config.BlockedPct,
		},
		"turso": map[string]interface{}{
			"latency": map[string]interface{}{
				"min_ms":  result.Turso.Latency.Min.Milliseconds(),
				"p50_ms":  result.Turso.Latency.P50.Milliseconds(),
				"mean_ms": result.Turso.Latency.Mean.Milliseconds(),
				"p95_ms":  result.Turso.Latency.P95.Milliseconds(),
				"p99_ms":  result.Turso.Latency.P99.Milliseconds(),
				"max_ms":  result.Turso.Latency.Max.Milliseconds(),
			},
			"throughput": map[string]interface{}{
				"qps":     result.Turso.Throughput.QueriesPerSecond,
				"queries": result.Turso.Throughput.TotalQueries,
			},
			"memory": map[string]interface{}{
				"before_bytes": result.Turso.Resources.MemoryBeforeBytes,
				"after_bytes":  result.Turso.Resources.MemoryAfterBytes,
				"peak_bytes":   result.Turso.Resources.MemoryPeakBytes,
				"delta_bytes":  result.Turso.Resources.MemoryDeltaBytes,
			},
			"errors": result.Turso.ErrorCount,
		},
		"baseline": map[string]interface{}{
			"latency": map[string]interface{}{
				"min_ms":  result.Baseline.Latency.Min.Milliseconds(),
				"p50_ms":  result.Baseline.Latency.P50.Milliseconds(),
				"mean_ms": result.Baseline.Latency.Mean.Milliseconds(),
				"p95_ms":  result.Baseline.Latency.P95.Milliseconds(),
				"p99_ms":  result.Baseline.Latency.P99.Milliseconds(),
				"max_ms":  result.Baseline.Latency.Max.Milliseconds(),
			},
			"throughput": map[string]interface{}{
				"qps":     result.Baseline.Throughput.QueriesPerSecond,
				"queries": result.Baseline.Throughput.TotalQueries,
			},
			"memory": map[string]interface{}{
				"before_bytes": result.Baseline.Resources.MemoryBeforeBytes,
				"after_bytes":  result.Baseline.Resources.MemoryAfterBytes,
				"peak_bytes":   result.Baseline.Resources.MemoryPeakBytes,
				"delta_bytes":  result.Baseline.Resources.MemoryDeltaBytes,
			},
			"lock_contention": result.Baseline.Concurrency.LockContention,
			"errors":          result.Baseline.ErrorCount,
		},
		"improvement": map[string]interface{}{
			"latency": result.LatencyImprovement,
			"throughput_pct": result.ThroughputImprovement,
			"memory_pct":     result.MemoryImprovement,
			"lock_contention_reduction_pct": result.LockContentionReduction,
		},
		"winner": result.OverallWinner,
		"wins": map[string]int{
			"turso":    result.WinCount["turso"],
			"baseline": result.WinCount["baseline"],
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func outputResultJSON(result *benchmark.BenchmarkResult) {
	output := map[string]interface{}{
		"config": map[string]interface{}{
			"mode":    result.Config.Mode,
			"agents":  result.Config.NumAgents,
			"tasks":   result.Config.NumTasks,
			"queries": result.Config.QueriesPerAgent,
			"blocked": result.Config.BlockedPct,
		},
		"latency": map[string]interface{}{
			"min_ms":  result.Latency.Min.Milliseconds(),
			"p50_ms":  result.Latency.P50.Milliseconds(),
			"mean_ms": result.Latency.Mean.Milliseconds(),
			"p95_ms":  result.Latency.P95.Milliseconds(),
			"p99_ms":  result.Latency.P99.Milliseconds(),
			"max_ms":  result.Latency.Max.Milliseconds(),
		},
		"throughput": map[string]interface{}{
			"qps":     result.Throughput.QueriesPerSecond,
			"queries": result.Throughput.TotalQueries,
		},
		"memory": map[string]interface{}{
			"before_bytes": result.Resources.MemoryBeforeBytes,
			"after_bytes":  result.Resources.MemoryAfterBytes,
			"peak_bytes":   result.Resources.MemoryPeakBytes,
			"delta_bytes":  result.Resources.MemoryDeltaBytes,
		},
		"database": map[string]interface{}{
			"size_bytes":       result.Database.SizeBytes,
			"sync_time_ms":     result.Database.SyncTimeMs,
			"time_to_first_ms": result.Database.TimeToFirstMs,
			"task_count":       result.Database.TaskCount,
			"ready_count":      result.Database.ReadyTaskCount,
		},
		"duration_ms": result.TotalDuration.Milliseconds(),
		"errors":      result.ErrorCount,
		"error_rate":  result.ErrorRate,
		"success":     result.Success,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
