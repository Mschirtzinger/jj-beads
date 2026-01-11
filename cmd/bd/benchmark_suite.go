package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/benchmark/scientific"
)

var (
	benchmarkOutputDir string
	benchmarkQuick     bool
	benchmarkJSON      bool
	benchmarkCSV       bool
)

var benchmarkSuiteCmd = &cobra.Command{
	Use:   "benchmark-suite",
	Short: "Run scientific benchmark suite comparing implementations",
	Long: `Run a comprehensive, scientifically rigorous benchmark suite comparing
beads-sqlite and jj-turso implementations.

This benchmark suite:
- Uses deterministic test data (reproducible via seed)
- Runs warmup iterations to eliminate cold-start effects
- Performs multiple measurement runs for statistical analysis
- Tests multiple agent counts and task database sizes
- Exports results in JSON, CSV, and markdown formats
- Generates ASCII graphs for terminal viewing

The results are suitable for publication and external verification.`,
	RunE: runBenchmarkSuite,
}

func init() {
	rootCmd.AddCommand(benchmarkSuiteCmd)

	benchmarkSuiteCmd.Flags().StringVarP(&benchmarkOutputDir, "output-dir", "o", "./benchmark-results", "Output directory for results")
	benchmarkSuiteCmd.Flags().BoolVar(&benchmarkQuick, "quick", false, "Run quick benchmark (fewer data points, faster)")
	benchmarkSuiteCmd.Flags().BoolVar(&benchmarkJSON, "json", false, "Only output JSON summary at the end")
	benchmarkSuiteCmd.Flags().BoolVar(&benchmarkCSV, "csv", false, "Only output CSV path at the end")
}

func runBenchmarkSuite(cmd *cobra.Command, args []string) error {
	// Select configuration
	var config scientific.SuiteConfig
	if benchmarkQuick {
		config = scientific.QuickConfig()
	} else {
		config = scientific.DefaultConfig()
	}

	// Create absolute output path
	outputDir, err := filepath.Abs(benchmarkOutputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}

	// Run the suite
	if !benchmarkJSON && !benchmarkCSV {
		fmt.Printf("Running benchmark suite...\n")
		fmt.Printf("Output directory: %s\n", outputDir)
		if benchmarkQuick {
			fmt.Printf("Mode: QUICK (reduced data points for faster execution)\n")
		} else {
			fmt.Printf("Mode: FULL (comprehensive benchmark suite)\n")
		}
		fmt.Printf("\n")
	}

	results, err := scientific.RunSuite(config, outputDir)
	if err != nil {
		return fmt.Errorf("benchmark suite failed: %w", err)
	}

	// Generate reports
	if err := scientific.GenerateReports(results, outputDir); err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	// Print graphs and analysis
	if !benchmarkJSON && !benchmarkCSV {
		scientific.PrintGraphs(results)
		scientific.PrintScalingAnalysis(results)
		scientific.PrintStatisticalSignificance(results)

		fmt.Printf("\n")
		fmt.Printf("=== FILES GENERATED ===\n")
		fmt.Printf("\n")
		fmt.Printf("Results directory: %s\n", outputDir)
		fmt.Printf("  - results.json     (complete results for external analysis)\n")
		fmt.Printf("  - results.csv      (importable into Excel, matplotlib, etc.)\n")
		fmt.Printf("  - REPORT.md        (markdown report with tables)\n")
		fmt.Printf("\n")
		fmt.Printf("Next steps:\n")
		fmt.Printf("  1. Review REPORT.md for summary\n")
		fmt.Printf("  2. Import results.csv into your analysis tool of choice\n")
		fmt.Printf("  3. See docs/BENCHMARK_METHODOLOGY.md for interpretation guide\n")
		fmt.Printf("\n")
	} else if benchmarkJSON {
		// JSON-only output mode (for scripting)
		summary := map[string]interface{}{
			"status":       "success",
			"output_dir":   outputDir,
			"config":       config,
			"data_points":  len(results.DataPoints),
			"start_time":   results.StartTime,
			"end_time":     results.EndTime,
			"duration_sec": results.EndTime.Sub(results.StartTime).Seconds(),
			"system_info":  results.SystemInfo,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(summary); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
	} else if benchmarkCSV {
		// CSV-only output mode (for scripting)
		csvPath := filepath.Join(outputDir, "results.csv")
		fmt.Println(csvPath)
	}

	return nil
}
