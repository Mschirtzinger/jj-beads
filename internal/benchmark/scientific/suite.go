package scientific

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// RunSuite executes the full benchmark suite and returns results.
func RunSuite(config SuiteConfig, outputDir string) (*SuiteResults, error) {
	results := &SuiteResults{
		Config:     config,
		DataPoints: make([]DataPoint, 0),
		StartTime:  time.Now(),
		SystemInfo: GetSystemInfo(),
	}

	// Get git commit hash for reproducibility
	if commit, err := getGitCommit(); err == nil {
		results.SystemInfo.GitCommit = commit
	}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		results.SystemInfo.Hostname = hostname
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	totalRuns := config.TotalRuns()
	currentRun := 0

	fmt.Printf("Starting benchmark suite with %d total runs\n", totalRuns)
	fmt.Printf("System: %s/%s, %d CPUs, Go %s\n", results.SystemInfo.OS, results.SystemInfo.Arch, results.SystemInfo.CPUs, results.SystemInfo.GoVersion)
	fmt.Printf("\n")

	// Run benchmarks for each task count
	for _, taskCount := range config.TaskCounts {
		// Run benchmarks for each agent count
		for _, agentCount := range config.AgentCounts {
			fmt.Printf("Task Count: %d, Agent Count: %d\n", taskCount, agentCount)

			// Benchmark beads-sqlite
			beadsPoints, err := runImplementation(
				"beads-sqlite",
				taskCount,
				agentCount,
				config,
				outputDir,
				&currentRun,
				totalRuns,
			)
			if err != nil {
				return nil, fmt.Errorf("beads-sqlite benchmark failed: %w", err)
			}
			results.DataPoints = append(results.DataPoints, beadsPoints...)

			// Benchmark jj-turso
			tursoPoints, err := runImplementation(
				"jj-turso",
				taskCount,
				agentCount,
				config,
				outputDir,
				&currentRun,
				totalRuns,
			)
			if err != nil {
				return nil, fmt.Errorf("jj-turso benchmark failed: %w", err)
			}
			results.DataPoints = append(results.DataPoints, tursoPoints...)

			fmt.Printf("\n")
		}
	}

	results.EndTime = time.Now()

	fmt.Printf("Benchmark suite complete in %v\n", results.EndTime.Sub(results.StartTime))

	return results, nil
}

// runImplementation runs the benchmark for a specific implementation.
func runImplementation(
	impl string,
	taskCount int,
	agentCount int,
	config SuiteConfig,
	outputDir string,
	currentRun *int,
	totalRuns int,
) ([]DataPoint, error) {
	dbPath := filepath.Join(outputDir, fmt.Sprintf("bench_%s_%d_%d.db", impl, taskCount, agentCount))
	var runner interface {
		RunBenchmark(ctx context.Context, agentCount int, queriesPerAgent int) (*BenchmarkResult, error)
		Close() error
	}

	var err error
	if impl == "beads-sqlite" {
		runner, err = NewBeadsRunner(dbPath, taskCount, config.BlockedPercent, config.Seed)
	} else if impl == "jj-turso" {
		runner, err = NewTursoRunner(dbPath, taskCount, config.BlockedPercent, config.Seed)
	} else {
		return nil, fmt.Errorf("unknown implementation: %s", impl)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}
	defer runner.Close()

	dataPoints := make([]DataPoint, 0, config.MeasurementRuns)
	ctx := context.Background()

	// Warmup runs
	fmt.Printf("  %s: Warmup (%d runs)... ", impl, config.WarmupRuns)
	for i := 0; i < config.WarmupRuns; i++ {
		*currentRun++
		if _, err := runner.RunBenchmark(ctx, agentCount, config.QueriesPerAgent); err != nil {
			return nil, fmt.Errorf("warmup run %d failed: %w", i+1, err)
		}
	}
	fmt.Printf("Done\n")

	// Measurement runs
	fmt.Printf("  %s: Measurement (%d runs)... ", impl, config.MeasurementRuns)
	for i := 0; i < config.MeasurementRuns; i++ {
		*currentRun++

		result, err := runner.RunBenchmark(ctx, agentCount, config.QueriesPerAgent)
		if err != nil {
			return nil, fmt.Errorf("measurement run %d failed: %w", i+1, err)
		}

		// Convert to data point
		dp := DataPoint{
			AgentCount:       agentCount,
			TaskCount:        taskCount,
			Implementation:   impl,
			LatencyMin:       int64(result.Min),
			LatencyP50:       int64(result.P50),
			LatencyP95:       int64(result.P95),
			LatencyP99:       int64(result.P99),
			LatencyMax:       int64(result.Max),
			LatencyMean:      int64(result.Mean),
			LatencyStdDev:    int64(result.StdDev),
			QueriesPerSecond: result.QueriesPerSecond,
			ErrorCount:       result.ErrorCount,
			ErrorRate:        float64(result.ErrorCount) / float64(result.TotalQueries),
			TotalDurationNs:  int64(result.TotalDuration),
			RunNumber:        i + 1,
		}

		dataPoints = append(dataPoints, dp)
	}
	fmt.Printf("Done\n")

	// Print summary stats (average across runs)
	avgP95 := int64(0)
	avgQPS := 0.0
	for _, dp := range dataPoints {
		avgP95 += dp.LatencyP95
		avgQPS += dp.QueriesPerSecond
	}
	avgP95 /= int64(len(dataPoints))
	avgQPS /= float64(len(dataPoints))

	fmt.Printf("    Avg P95: %v, Avg QPS: %.0f\n", time.Duration(avgP95), avgQPS)

	return dataPoints, nil
}

// getGitCommit returns the current git commit hash.
func getGitCommit() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	commit := string(output)
	// Trim whitespace
	if len(commit) > 0 && commit[len(commit)-1] == '\n' {
		commit = commit[:len(commit)-1]
	}
	return commit, nil
}
