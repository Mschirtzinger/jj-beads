package scientific

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// PrintGraphs prints ASCII graphs of benchmark results to the terminal.
func PrintGraphs(results *SuiteResults) {
	aggregated := aggregateResults(results)

	for _, taskCount := range results.Config.TaskCounts {
		fmt.Printf("\n")
		fmt.Printf("=== GRAPHS: %d TASKS ===\n", taskCount)
		fmt.Printf("\n")

		// Latency curve (agents vs P95 latency)
		printLatencyGraph(aggregated[taskCount], results.Config.AgentCounts)

		// Throughput curve (agents vs QPS)
		printThroughputGraph(aggregated[taskCount], results.Config.AgentCounts)
	}
}

// printLatencyGraph prints a comparison of P95 latency.
func printLatencyGraph(data map[int]map[string]*DataPoint, agentCounts []int) {
	fmt.Printf("P95 Latency vs Agent Count\n")
	fmt.Printf("%s\n", strings.Repeat("-", 70))

	// Find max latency for scaling
	maxLatency := int64(0)
	for _, agentCount := range agentCounts {
		if beads := data[agentCount]["beads-sqlite"]; beads != nil {
			if beads.LatencyP95 > maxLatency {
				maxLatency = beads.LatencyP95
			}
		}
		if turso := data[agentCount]["jj-turso"]; turso != nil {
			if turso.LatencyP95 > maxLatency {
				maxLatency = turso.LatencyP95
			}
		}
	}

	graphWidth := 50
	for _, agentCount := range agentCounts {
		beads := data[agentCount]["beads-sqlite"]
		turso := data[agentCount]["jj-turso"]

		if beads == nil || turso == nil {
			continue
		}

		beadsP95 := time.Duration(beads.LatencyP95)
		tursoP95 := time.Duration(turso.LatencyP95)

		beadsBar := int(float64(beads.LatencyP95) / float64(maxLatency) * float64(graphWidth))
		tursoBar := int(float64(turso.LatencyP95) / float64(maxLatency) * float64(graphWidth))

		fmt.Printf("%3d agents:\n", agentCount)
		fmt.Printf("  beads: %s %v\n", strings.Repeat("█", beadsBar), beadsP95)
		fmt.Printf("  turso: %s %v\n", strings.Repeat("█", tursoBar), tursoP95)
		fmt.Printf("\n")
	}
}

// printThroughputGraph prints a comparison of queries per second.
func printThroughputGraph(data map[int]map[string]*DataPoint, agentCounts []int) {
	fmt.Printf("Throughput (Queries/Second) vs Agent Count\n")
	fmt.Printf("%s\n", strings.Repeat("-", 70))

	// Find max QPS for scaling
	maxQPS := 0.0
	for _, agentCount := range agentCounts {
		if beads := data[agentCount]["beads-sqlite"]; beads != nil {
			if beads.QueriesPerSecond > maxQPS {
				maxQPS = beads.QueriesPerSecond
			}
		}
		if turso := data[agentCount]["jj-turso"]; turso != nil {
			if turso.QueriesPerSecond > maxQPS {
				maxQPS = turso.QueriesPerSecond
			}
		}
	}

	graphWidth := 50
	for _, agentCount := range agentCounts {
		beads := data[agentCount]["beads-sqlite"]
		turso := data[agentCount]["jj-turso"]

		if beads == nil || turso == nil {
			continue
		}

		beadsBar := int(beads.QueriesPerSecond / maxQPS * float64(graphWidth))
		tursoBar := int(turso.QueriesPerSecond / maxQPS * float64(graphWidth))

		fmt.Printf("%3d agents:\n", agentCount)
		fmt.Printf("  beads: %s %.0f qps\n", strings.Repeat("█", beadsBar), beads.QueriesPerSecond)
		fmt.Printf("  turso: %s %.0f qps\n", strings.Repeat("█", tursoBar), turso.QueriesPerSecond)
		fmt.Printf("\n")
	}
}

// PrintScalingAnalysis prints scaling efficiency analysis.
func PrintScalingAnalysis(results *SuiteResults) {
	aggregated := aggregateResults(results)

	fmt.Printf("\n")
	fmt.Printf("=== SCALING ANALYSIS ===\n")
	fmt.Printf("\n")

	for _, taskCount := range results.Config.TaskCounts {
		fmt.Printf("Tasks: %d\n", taskCount)
		fmt.Printf("%s\n", strings.Repeat("-", 70))

		analyzeScaling("beads-sqlite", aggregated[taskCount], results.Config.AgentCounts)
		analyzeScaling("jj-turso", aggregated[taskCount], results.Config.AgentCounts)

		fmt.Printf("\n")
	}
}

// analyzeScaling prints scaling efficiency for an implementation.
func analyzeScaling(impl string, data map[int]map[string]*DataPoint, agentCounts []int) {
	fmt.Printf("%s:\n", impl)

	if len(agentCounts) < 2 {
		fmt.Printf("  (insufficient data points)\n")
		return
	}

	// Compute throughput scaling efficiency
	// Ideal scaling: QPS doubles when agents double
	// Efficiency = (actual QPS increase) / (ideal QPS increase)
	for i := 1; i < len(agentCounts); i++ {
		prev := data[agentCounts[i-1]][impl]
		curr := data[agentCounts[i]][impl]

		if prev == nil || curr == nil {
			continue
		}

		agentRatio := float64(agentCounts[i]) / float64(agentCounts[i-1])
		qpsRatio := curr.QueriesPerSecond / prev.QueriesPerSecond
		efficiency := (qpsRatio / agentRatio) * 100

		latencyIncrease := ((float64(curr.LatencyP95) - float64(prev.LatencyP95)) / float64(prev.LatencyP95)) * 100

		fmt.Printf("  %d → %d agents: QPS +%.1f%%, Latency +%.1f%%, Efficiency: %.1f%%\n",
			agentCounts[i-1], agentCounts[i], (qpsRatio-1)*100, latencyIncrease, efficiency)
	}
}

// PrintStatisticalSignificance analyzes whether differences are statistically significant.
func PrintStatisticalSignificance(results *SuiteResults) {
	if results.Config.MeasurementRuns < 3 {
		fmt.Printf("\n")
		fmt.Printf("=== STATISTICAL SIGNIFICANCE ===\n")
		fmt.Printf("(Skipped: need at least 3 measurement runs for statistical analysis)\n")
		return
	}

	fmt.Printf("\n")
	fmt.Printf("=== STATISTICAL SIGNIFICANCE ===\n")
	fmt.Printf("\n")

	// Group by task count, agent count
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

	for _, taskCount := range results.Config.TaskCounts {
		fmt.Printf("Tasks: %d\n", taskCount)
		fmt.Printf("%s\n", strings.Repeat("-", 70))

		for _, agentCount := range results.Config.AgentCounts {
			beadsPoints := groups[taskCount][agentCount]["beads-sqlite"]
			tursoPoints := groups[taskCount][agentCount]["jj-turso"]

			if len(beadsPoints) < 2 || len(tursoPoints) < 2 {
				continue
			}

			// Extract P95 latencies
			beadsLatencies := make([]float64, len(beadsPoints))
			tursoLatencies := make([]float64, len(tursoPoints))
			for i, p := range beadsPoints {
				beadsLatencies[i] = float64(p.LatencyP95)
			}
			for i, p := range tursoPoints {
				tursoLatencies[i] = float64(p.LatencyP95)
			}

			// Compute means and standard deviations
			beadsMean, beadsStdDev := meanAndStdDev(beadsLatencies)
			tursoMean, tursoStdDev := meanAndStdDev(tursoLatencies)

			// Compute coefficient of variation (CV) as a stability metric
			// Lower CV = more stable/reproducible
			beadsCV := (beadsStdDev / beadsMean) * 100
			tursoCV := (tursoStdDev / tursoMean) * 100

			// Compute effect size (difference in means normalized by pooled stddev)
			pooledStdDev := math.Sqrt((beadsStdDev*beadsStdDev + tursoStdDev*tursoStdDev) / 2)
			effectSize := math.Abs(beadsMean-tursoMean) / pooledStdDev

			significance := "small"
			if effectSize > 0.8 {
				significance = "large"
			} else if effectSize > 0.5 {
				significance = "medium"
			}

			fmt.Printf("%d agents:\n", agentCount)
			fmt.Printf("  beads-sqlite: mean=%.2fms, stddev=%.2fms, CV=%.1f%%\n",
				beadsMean/1e6, beadsStdDev/1e6, beadsCV)
			fmt.Printf("  jj-turso:     mean=%.2fms, stddev=%.2fms, CV=%.1f%%\n",
				tursoMean/1e6, tursoStdDev/1e6, tursoCV)
			fmt.Printf("  Effect size: %.2f (%s)\n", effectSize, significance)
			fmt.Printf("\n")
		}
	}

	fmt.Printf("Effect size interpretation:\n")
	fmt.Printf("  < 0.5  = small (may not be practically significant)\n")
	fmt.Printf("  0.5-0.8 = medium (likely practically significant)\n")
	fmt.Printf("  > 0.8  = large (definitely practically significant)\n")
	fmt.Printf("\n")
}

// meanAndStdDev computes mean and standard deviation of a slice of values.
func meanAndStdDev(values []float64) (mean float64, stdDev float64) {
	if len(values) == 0 {
		return 0, 0
	}

	// Compute mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))

	// Compute standard deviation
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	stdDev = math.Sqrt(variance)

	return mean, stdDev
}
