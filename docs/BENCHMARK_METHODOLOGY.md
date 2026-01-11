# Benchmark Methodology: Scientific Comparison of beads Implementations

## Overview

This document describes the scientific methodology used to benchmark and compare the beads-sqlite and jj-turso implementations. The benchmark suite is designed to produce **reproducible, verifiable, and publishable results**.

## Executive Summary

### What We're Comparing

- **beads-sqlite**: Main beads implementation using `internal/storage/sqlite`
- **jj-turso**: jj-turso implementation using `internal/turso/db` with concurrent access optimizations

### Key Point: Both Use SQLite

**This is a fair comparison.** Both implementations use SQLite as the underlying database. The difference is in optimization approach:

- **beads-sqlite**: Traditional SQLite with standard connection pooling
- **jj-turso**: Embedded libSQL (SQLite-compatible) with WAL mode, optimized connection pool settings, and materialized blocked cache

The jj-turso implementation uses the `ncruces/go-sqlite3` driver in embedded mode with specific optimizations for concurrent access patterns common in multi-agent workflows.

## Test Environment Requirements

### Hardware Recommendations

For reproducible results across runs:

- **Dedicated hardware** (not a shared VM)
- **Consistent CPU frequency** (disable turbo boost for more stable results)
- **Sufficient RAM** (8GB minimum, 16GB recommended)
- **SSD storage** (database I/O is critical)

### Software Requirements

- Go 1.21 or later
- Linux, macOS, or Windows
- No other heavy processes running during benchmark

### Preparing Your System

```bash
# Recommended: Disable CPU frequency scaling (Linux)
sudo cpupower frequency-set --governor performance

# Recommended: Drop filesystem caches before run (Linux)
sync; echo 3 | sudo tee /proc/sys/vm/drop_caches

# Recommended: Close unnecessary applications
```

## Benchmark Design

### Test Data Generation

Test data is generated **deterministically** using a fixed random seed (default: 42). This ensures:

1. **Reproducibility**: Same seed = identical test data
2. **Fairness**: Both implementations test against the same data
3. **Realism**: Priority distribution and dependency trees match real-world usage

#### Task Distribution

- **Types**: bug, feature, task (evenly distributed)
- **Priorities**: Weighted toward P2
  - P0: 5%
  - P1: 15%
  - P2: 50%
  - P3: 20%
  - P4: 10%
- **Blocked percentage**: Configurable (default 30%)

#### Dependency Generation

- Blocking dependencies created between tasks
- Earlier tasks block later tasks (realistic workflow)
- Deterministic selection based on seed

### Warmup Runs

**Problem**: First queries are slower due to:
- OS page cache cold start
- SQLite page cache loading
- Connection pool initialization
- Query plan compilation

**Solution**: Run warmup iterations (default: 3) that are **not measured**. Only subsequent measurement runs count toward results.

### Measurement Runs

Each configuration (task count × agent count × implementation) is tested **multiple times** (default: 5 runs).

This allows us to compute:
- **Mean**: Average performance
- **Standard deviation**: Measurement stability
- **Confidence**: Statistical significance of differences

### Metrics Collected

For each run, we collect:

#### Latency Metrics (nanosecond precision)
- **Min**: Fastest query
- **P50 (Median)**: 50th percentile
- **P95**: 95th percentile (key metric for tail latency)
- **P99**: 99th percentile
- **Max**: Slowest query
- **Mean**: Average latency
- **StdDev**: Standard deviation

#### Throughput Metrics
- **Queries Per Second**: Total queries / total time
- **Total Duration**: Wall clock time for all agents to complete

#### Error Metrics
- **Error Count**: Number of failed queries
- **Error Rate**: Percentage of queries that failed

### Test Configurations

#### Default (Full) Configuration

```go
AgentCounts:     []int{10, 25, 50, 75, 100, 150, 200}
TaskCounts:      []int{500, 1000, 2000}
QueriesPerAgent: 50
WarmupRuns:      3
MeasurementRuns: 5
BlockedPercent:  0.3
Seed:            42
```

Total runs: **2 implementations × 3 task counts × 7 agent counts × (3 warmup + 5 measurement) = 336 runs**

Estimated duration: **30-60 minutes** depending on hardware

#### Quick Configuration

```go
AgentCounts:     []int{10, 50, 100}
TaskCounts:      []int{500, 1000}
QueriesPerAgent: 20
WarmupRuns:      1
MeasurementRuns: 3
BlockedPercent:  0.3
Seed:            42
```

Total runs: **2 × 2 × 3 × 4 = 48 runs**

Estimated duration: **5-10 minutes**

## Running the Benchmark

### Basic Usage

```bash
# Full benchmark suite (30-60 minutes)
bd benchmark-suite --output-dir ./results

# Quick benchmark for development (5-10 minutes)
bd benchmark-suite --quick --output-dir ./results

# JSON output for scripting
bd benchmark-suite --json > summary.json

# CSV output only
bd benchmark-suite --csv
```

### Output Files

The benchmark generates:

1. **results.json**: Complete results with all data points and metadata
2. **results.csv**: Tabular data for external analysis (Excel, Python, R, etc.)
3. **REPORT.md**: Markdown report with tables and analysis template

## Reproducing Results

To reproduce results:

1. **Use the same seed**: The default seed (42) is consistent
2. **Use the same configuration**: Document which config you used (default or quick)
3. **Same system architecture**: CPU architecture affects performance
4. **Similar hardware**: Results vary with CPU speed, RAM, storage

### Verification Checklist

When publishing results, include:

- [ ] Git commit hash (automatically captured)
- [ ] System information (automatically captured)
- [ ] Configuration used (default or quick)
- [ ] Date and time of run
- [ ] Any system tuning performed

## Interpreting Results

### Key Metrics to Compare

1. **P95 Latency**: Most important for user experience
   - Lower is better
   - Goal: Sub-10ms for 100 agents

2. **Throughput (QPS)**: System capacity
   - Higher is better
   - Should scale approximately linearly with agent count

3. **Scaling Efficiency**: How well throughput increases with agents
   - 100% = perfect linear scaling
   - 70-90% = good scaling
   - <50% = contention issues

### Statistical Significance

The benchmark computes **effect size** using Cohen's d:

- **Effect size < 0.5**: Small difference (may not be practically significant)
- **Effect size 0.5-0.8**: Medium difference (likely significant)
- **Effect size > 0.8**: Large difference (definitely significant)

### Coefficient of Variation (CV)

Measures result stability:

- **CV < 5%**: Very stable (excellent reproducibility)
- **CV 5-10%**: Stable (good reproducibility)
- **CV 10-20%**: Moderate variability
- **CV > 20%**: High variability (investigate environmental factors)

## Known Limitations

### 1. Synthetic Workload

The benchmark uses **read-only queries** (GetReadyTasks). Real-world usage includes:
- Task creation (writes)
- Status updates (writes)
- Dependency modifications (writes)

Future work: Add mixed read/write workloads.

### 2. Single Machine

This benchmark tests **local database performance**. The jj-turso architecture is designed for distributed sync via Jujutsu, which this benchmark doesn't test.

### 3. No Network Latency

Real-world multi-agent workflows may involve network delays. This benchmark tests pure database performance.

### 4. Deterministic Data

Real-world data has different characteristics:
- More complex dependency graphs
- Variable task sizes
- Different priority distributions

The 30% blocked percentage is a reasonable estimate but may vary by project.

## Future Enhancements

Potential improvements to the benchmark suite:

1. **Memory profiling**: Track memory usage during runs
2. **Lock contention measurement**: Instrument SQLite's busy handler
3. **Write workload**: Mix of reads and writes
4. **Realistic data**: Import real project data for testing
5. **Distributed testing**: Test jj sync performance
6. **Long-running stress test**: Hours-long workload for stability

## Graphing Results

The benchmark exports CSV for external graphing. Here are recommended visualizations:

### 1. Latency vs Agent Count

```python
import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv('results.csv')

# Filter to P95 latency, one task count
df_filtered = df[df['task_count'] == 1000]

# Group by implementation and agent count
grouped = df_filtered.groupby(['implementation', 'agent_count'])['latency_p95_ms'].mean()

# Plot
grouped.unstack(0).plot(kind='line', marker='o')
plt.xlabel('Agent Count')
plt.ylabel('P95 Latency (ms)')
plt.title('Latency Scaling: 1000 Tasks')
plt.legend(title='Implementation')
plt.grid(True)
plt.savefig('latency_scaling.png', dpi=300)
```

### 2. Throughput vs Agent Count

```python
grouped = df_filtered.groupby(['implementation', 'agent_count'])['queries_per_second'].mean()
grouped.unstack(0).plot(kind='line', marker='o')
plt.xlabel('Agent Count')
plt.ylabel('Queries Per Second')
plt.title('Throughput Scaling: 1000 Tasks')
plt.legend(title='Implementation')
plt.grid(True)
plt.savefig('throughput_scaling.png', dpi=300)
```

### 3. Box Plot for Variability

```python
import seaborn as sns

# Show distribution across measurement runs
sns.boxplot(data=df_filtered, x='agent_count', y='latency_p95_ms', hue='implementation')
plt.xlabel('Agent Count')
plt.ylabel('P95 Latency (ms)')
plt.title('Latency Distribution: 1000 Tasks')
plt.savefig('latency_distribution.png', dpi=300)
```

## Contact and Questions

For questions about the benchmark methodology or results interpretation:

- Open an issue on GitHub
- See the main README for project contact information

## Version History

- **v1.0.0** (2025-01-10): Initial scientific benchmark suite
  - Reproducible test data generation
  - Warmup and measurement runs
  - Statistical analysis
  - Multiple output formats
