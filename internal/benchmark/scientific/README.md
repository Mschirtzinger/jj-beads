# Scientific Benchmark Suite

This package provides a comprehensive, scientifically rigorous benchmark suite for comparing beads-sqlite and jj-turso implementations.

## Quick Start

```bash
# Run full benchmark suite (30-60 minutes)
bd benchmark-suite --output-dir ./results

# Run quick benchmark (5-10 minutes)
bd benchmark-suite --quick --output-dir ./results
```

## What Gets Benchmarked

Both implementations use SQLite as the underlying database:

- **beads-sqlite**: Main implementation (`internal/storage/sqlite`)
- **jj-turso**: Optimized implementation (`internal/turso/db`) with WAL mode and concurrent access optimizations

The benchmark measures:
- **Latency**: Min, P50, P95, P99, Max (nanosecond precision)
- **Throughput**: Queries per second
- **Scalability**: Performance across different agent counts
- **Stability**: Variance across multiple runs

## Test Configurations

### Default Configuration

```
Agent Counts:     [10, 25, 50, 75, 100, 150, 200]
Task Counts:      [500, 1000, 2000]
Queries Per Agent: 50
Warmup Runs:      3
Measurement Runs: 5
```

**Total**: 336 runs, ~30-60 minutes

### Quick Configuration

```
Agent Counts:     [10, 50, 100]
Task Counts:      [500, 1000]
Queries Per Agent: 20
Warmup Runs:      1
Measurement Runs: 3
```

**Total**: 48 runs, ~5-10 minutes

## Output Files

The benchmark generates:

1. **results.json** - Complete results with all measurements
2. **results.csv** - Tabular data for Excel/Python/R
3. **REPORT.md** - Markdown report with tables and analysis

## Key Features

### Reproducibility

- Deterministic test data via fixed seed (default: 42)
- Same seed = identical test data every time
- System info captured for verification

### Statistical Rigor

- Warmup runs to eliminate cold-start effects
- Multiple measurement runs for stability analysis
- Effect size computation (Cohen's d)
- Coefficient of variation for reproducibility

### Fair Comparison

- Both implementations use SQLite
- Identical test data for both
- Same query patterns
- No network/distributed effects

## Interpreting Results

### Key Metrics

1. **P95 Latency** (most important)
   - Target: <10ms for 100 concurrent agents
   - Lower is better

2. **Throughput (QPS)**
   - Should scale with agent count
   - Higher is better

3. **Scaling Efficiency**
   - 70-90% = good
   - <50% = contention issues

### Statistical Significance

- Effect size < 0.5: Small difference
- Effect size 0.5-0.8: Medium difference
- Effect size > 0.8: Large difference

## Example Results

From our tests on Apple Silicon:

```
Tasks: 500, Agents: 100
- beads-sqlite: P95 = 65ms, QPS = 3,214
- jj-turso:     P95 = 8ms,  QPS = 12,442
- Speedup: 8.13x faster
```

## Running Tests

```bash
# Run all tests
go test ./internal/benchmark/scientific/...

# Run quick test only
go test ./internal/benchmark/scientific/... -run TestQuickSuite

# Skip long-running tests
go test ./internal/benchmark/scientific/... -short
```

## Methodology

See [docs/BENCHMARK_METHODOLOGY.md](../../../docs/BENCHMARK_METHODOLOGY.md) for:

- Complete methodology
- Reproduction instructions
- Statistical methods
- Graphing examples
- Interpretation guide

## Package Structure

```
internal/benchmark/scientific/
├── config.go          - Configuration and data structures
├── beads_runner.go    - Benchmark runner for beads-sqlite
├── turso_runner.go    - Benchmark runner for jj-turso
├── suite.go           - Main orchestration
├── report.go          - Report generation (JSON, CSV, markdown)
├── graphs.go          - ASCII terminal graphs
├── suite_test.go      - Test suite
└── README.md          - This file
```

## CLI Integration

The benchmark is integrated into the `bd` CLI:

```bash
bd benchmark-suite [flags]

Flags:
  --output-dir string   Output directory (default "./benchmark-results")
  --quick              Quick benchmark (fewer runs)
  --json               JSON output only (for scripting)
  --csv                CSV path only (for scripting)
```

## Future Enhancements

Potential improvements:

1. Memory profiling
2. Lock contention measurement
3. Mixed read/write workloads
4. Real project data import
5. Distributed sync benchmarks
6. Long-running stress tests

## License

Same as beads project.
