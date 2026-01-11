package scientific

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateReports creates all report outputs (JSON, CSV, markdown, terminal).
func GenerateReports(results *SuiteResults, outputDir string) error {
	// Export JSON
	if err := exportJSON(results, filepath.Join(outputDir, "results.json")); err != nil {
		return fmt.Errorf("failed to export JSON: %w", err)
	}

	// Export CSV
	if err := exportCSV(results, filepath.Join(outputDir, "results.csv")); err != nil {
		return fmt.Errorf("failed to export CSV: %w", err)
	}

	// Generate markdown report
	if err := generateMarkdownReport(results, filepath.Join(outputDir, "REPORT.md")); err != nil {
		return fmt.Errorf("failed to generate markdown report: %w", err)
	}

	// Print terminal summary
	printTerminalSummary(results)

	return nil
}

// exportJSON writes results to JSON file.
func exportJSON(results *SuiteResults, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return err
	}

	fmt.Printf("Exported JSON: %s\n", path)
	return nil
}

// exportCSV writes results to CSV file for external analysis (Excel, matplotlib, etc.).
func exportCSV(results *SuiteResults, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Write header
	header := []string{
		"implementation",
		"task_count",
		"agent_count",
		"run_number",
		"latency_min_ms",
		"latency_p50_ms",
		"latency_p95_ms",
		"latency_p99_ms",
		"latency_max_ms",
		"latency_mean_ms",
		"latency_stddev_ms",
		"queries_per_second",
		"error_count",
		"error_rate",
		"total_duration_ms",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	// Write data points
	for _, dp := range results.DataPoints {
		row := []string{
			dp.Implementation,
			fmt.Sprintf("%d", dp.TaskCount),
			fmt.Sprintf("%d", dp.AgentCount),
			fmt.Sprintf("%d", dp.RunNumber),
			fmt.Sprintf("%.3f", float64(dp.LatencyMin)/1e6),
			fmt.Sprintf("%.3f", float64(dp.LatencyP50)/1e6),
			fmt.Sprintf("%.3f", float64(dp.LatencyP95)/1e6),
			fmt.Sprintf("%.3f", float64(dp.LatencyP99)/1e6),
			fmt.Sprintf("%.3f", float64(dp.LatencyMax)/1e6),
			fmt.Sprintf("%.3f", float64(dp.LatencyMean)/1e6),
			fmt.Sprintf("%.3f", float64(dp.LatencyStdDev)/1e6),
			fmt.Sprintf("%.2f", dp.QueriesPerSecond),
			fmt.Sprintf("%d", dp.ErrorCount),
			fmt.Sprintf("%.4f", dp.ErrorRate),
			fmt.Sprintf("%.3f", float64(dp.TotalDurationNs)/1e6),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	fmt.Printf("Exported CSV: %s\n", path)
	return nil
}

// generateMarkdownReport creates a markdown report with tables and analysis.
func generateMarkdownReport(results *SuiteResults, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Compute aggregated metrics (average across runs)
	aggregated := aggregateResults(results)

	// Write report
	_, _ = fmt.Fprintf(f, "# Benchmark Report: beads-sqlite vs jj-turso\n\n")
	_, _ = fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format(time.RFC3339))

	// System info
	fmt.Fprintf(f, "## System Information\n\n")
	fmt.Fprintf(f, "- **OS:** %s\n", results.SystemInfo.OS)
	fmt.Fprintf(f, "- **Architecture:** %s\n", results.SystemInfo.Arch)
	fmt.Fprintf(f, "- **CPUs:** %d\n", results.SystemInfo.CPUs)
	fmt.Fprintf(f, "- **Go Version:** %s\n", results.SystemInfo.GoVersion)
	if results.SystemInfo.GitCommit != "" {
		fmt.Fprintf(f, "- **Git Commit:** %s\n", results.SystemInfo.GitCommit)
	}
	if results.SystemInfo.Hostname != "" {
		fmt.Fprintf(f, "- **Hostname:** %s\n", results.SystemInfo.Hostname)
	}
	fmt.Fprintf(f, "- **Duration:** %v\n", results.EndTime.Sub(results.StartTime))
	fmt.Fprintf(f, "\n")

	// Configuration
	fmt.Fprintf(f, "## Benchmark Configuration\n\n")
	fmt.Fprintf(f, "- **Task Counts:** %v\n", results.Config.TaskCounts)
	fmt.Fprintf(f, "- **Agent Counts:** %v\n", results.Config.AgentCounts)
	fmt.Fprintf(f, "- **Queries Per Agent:** %d\n", results.Config.QueriesPerAgent)
	fmt.Fprintf(f, "- **Warmup Runs:** %d\n", results.Config.WarmupRuns)
	fmt.Fprintf(f, "- **Measurement Runs:** %d\n", results.Config.MeasurementRuns)
	fmt.Fprintf(f, "- **Blocked Percent:** %.1f%%\n", results.Config.BlockedPercent*100)
	fmt.Fprintf(f, "- **Random Seed:** %d\n", results.Config.Seed)
	fmt.Fprintf(f, "\n")

	// Results tables
	for _, taskCount := range results.Config.TaskCounts {
		fmt.Fprintf(f, "## Results: %d Tasks\n\n", taskCount)

		fmt.Fprintf(f, "### Latency (P95, milliseconds)\n\n")
		fmt.Fprintf(f, "| Agents | beads-sqlite | jj-turso | Speedup |\n")
		fmt.Fprintf(f, "|--------|--------------|----------|----------|\n")

		for _, agentCount := range results.Config.AgentCounts {
			beads := aggregated[taskCount][agentCount]["beads-sqlite"]
			turso := aggregated[taskCount][agentCount]["jj-turso"]

			if beads != nil && turso != nil {
				beadsP95 := float64(beads.LatencyP95) / 1e6
				tursoP95 := float64(turso.LatencyP95) / 1e6
				speedup := beadsP95 / tursoP95

				fmt.Fprintf(f, "| %d | %.2f | %.2f | %.2fx |\n", agentCount, beadsP95, tursoP95, speedup)
			}
		}
		fmt.Fprintf(f, "\n")

		fmt.Fprintf(f, "### Throughput (queries/sec)\n\n")
		fmt.Fprintf(f, "| Agents | beads-sqlite | jj-turso | Improvement |\n")
		fmt.Fprintf(f, "|--------|--------------|----------|-------------|\n")

		for _, agentCount := range results.Config.AgentCounts {
			beads := aggregated[taskCount][agentCount]["beads-sqlite"]
			turso := aggregated[taskCount][agentCount]["jj-turso"]

			if beads != nil && turso != nil {
				beadsQPS := beads.QueriesPerSecond
				tursoQPS := turso.QueriesPerSecond
				improvement := ((tursoQPS - beadsQPS) / beadsQPS) * 100

				fmt.Fprintf(f, "| %d | %.0f | %.0f | %+.1f%% |\n", agentCount, beadsQPS, tursoQPS, improvement)
			}
		}
		fmt.Fprintf(f, "\n")
	}

	// Analysis
	fmt.Fprintf(f, "## Analysis\n\n")
	fmt.Fprintf(f, "### Methodology\n\n")
	fmt.Fprintf(f, "This benchmark compares two SQLite-based implementations:\n\n")
	fmt.Fprintf(f, "- **beads-sqlite:** Main beads implementation using `internal/storage/sqlite`\n")
	fmt.Fprintf(f, "- **jj-turso:** jj-turso implementation using `internal/turso/db` with optimizations for concurrent access\n\n")
	fmt.Fprintf(f, "Both implementations use SQLite as the underlying database. The difference is in optimization approach:\n\n")
	fmt.Fprintf(f, "- beads-sqlite: Traditional SQLite with standard connection pooling\n")
	fmt.Fprintf(f, "- jj-turso: Embedded libSQL with WAL mode, optimized connection pool, and materialized blocked cache\n\n")
	fmt.Fprintf(f, "Test data is generated deterministically (seed=%d) to ensure fair comparison.\n\n", results.Config.Seed)
	fmt.Fprintf(f, "### Key Findings\n\n")
	fmt.Fprintf(f, "*Add your analysis here after reviewing the results.*\n\n")
	fmt.Fprintf(f, "### Recommendations\n\n")
	fmt.Fprintf(f, "*Add recommendations based on the benchmark results.*\n\n")

	// Footer
	fmt.Fprintf(f, "---\n\n")
	fmt.Fprintf(f, "See `results.csv` for raw data and `results.json` for complete results.\n")

	fmt.Printf("Generated report: %s\n", path)
	return nil
}

// aggregateResults computes average metrics across measurement runs.
func aggregateResults(results *SuiteResults) map[int]map[int]map[string]*DataPoint {
	// aggregated[taskCount][agentCount][impl] = average DataPoint
	aggregated := make(map[int]map[int]map[string]*DataPoint)

	// Group by task count, agent count, implementation
	groups := make(map[int]map[int]map[string][]DataPoint)
	for _, dp := range results.DataPoints {
		if _, ok := groups[dp.TaskCount]; !ok {
			groups[dp.TaskCount] = make(map[int]map[string][]DataPoint)
		}
		if _, ok := groups[dp.TaskCount][dp.AgentCount]; !ok {
			groups[dp.TaskCount][dp.AgentCount] = make(map[string][]DataPoint)
		}
		groups[dp.TaskCount][dp.AgentCount][dp.Implementation] = append(
			groups[dp.TaskCount][dp.AgentCount][dp.Implementation],
			dp,
		)
	}

	// Compute averages
	for taskCount, agentGroups := range groups {
		if _, ok := aggregated[taskCount]; !ok {
			aggregated[taskCount] = make(map[int]map[string]*DataPoint)
		}
		for agentCount, implGroups := range agentGroups {
			if _, ok := aggregated[taskCount][agentCount]; !ok {
				aggregated[taskCount][agentCount] = make(map[string]*DataPoint)
			}
			for impl, points := range implGroups {
				avgPoint := averageDataPoints(points)
				aggregated[taskCount][agentCount][impl] = &avgPoint
			}
		}
	}

	return aggregated
}

// averageDataPoints computes the average of multiple data points.
func averageDataPoints(points []DataPoint) DataPoint {
	if len(points) == 0 {
		return DataPoint{}
	}

	avg := DataPoint{
		AgentCount:     points[0].AgentCount,
		TaskCount:      points[0].TaskCount,
		Implementation: points[0].Implementation,
	}

	for _, p := range points {
		avg.LatencyMin += p.LatencyMin
		avg.LatencyP50 += p.LatencyP50
		avg.LatencyP95 += p.LatencyP95
		avg.LatencyP99 += p.LatencyP99
		avg.LatencyMax += p.LatencyMax
		avg.LatencyMean += p.LatencyMean
		avg.LatencyStdDev += p.LatencyStdDev
		avg.QueriesPerSecond += p.QueriesPerSecond
		avg.ErrorCount += p.ErrorCount
		avg.ErrorRate += p.ErrorRate
		avg.TotalDurationNs += p.TotalDurationNs
	}

	n := int64(len(points))
	avg.LatencyMin /= n
	avg.LatencyP50 /= n
	avg.LatencyP95 /= n
	avg.LatencyP99 /= n
	avg.LatencyMax /= n
	avg.LatencyMean /= n
	avg.LatencyStdDev /= n
	avg.QueriesPerSecond /= float64(len(points))
	avg.ErrorCount /= int(n)
	avg.ErrorRate /= float64(len(points))
	avg.TotalDurationNs /= n

	return avg
}

// printTerminalSummary prints a summary to the terminal.
func printTerminalSummary(results *SuiteResults) {
	aggregated := aggregateResults(results)

	fmt.Printf("\n")
	fmt.Printf("=== BENCHMARK SUMMARY ===\n")
	fmt.Printf("\n")

	for _, taskCount := range results.Config.TaskCounts {
		fmt.Printf("Tasks: %d\n", taskCount)
		fmt.Printf("%s\n", strings.Repeat("-", 70))

		fmt.Printf("%-10s  %-20s  %-20s  %-10s\n", "Agents", "beads-sqlite P95", "jj-turso P95", "Speedup")
		for _, agentCount := range results.Config.AgentCounts {
			beads := aggregated[taskCount][agentCount]["beads-sqlite"]
			turso := aggregated[taskCount][agentCount]["jj-turso"]

			if beads != nil && turso != nil {
				beadsP95 := time.Duration(beads.LatencyP95)
				tursoP95 := time.Duration(turso.LatencyP95)
				speedup := float64(beads.LatencyP95) / float64(turso.LatencyP95)

				fmt.Printf("%-10d  %-20v  %-20v  %.2fx\n", agentCount, beadsP95, tursoP95, speedup)
			}
		}
		fmt.Printf("\n")
	}
}
