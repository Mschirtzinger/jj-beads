# Benchmark Plan: Turso vs Baseline (JSONL)

## Overview

This document explains how to run and interpret the comprehensive performance benchmarks comparing the jj-turso implementation against the baseline JSONL implementation for the beads issue tracker.

## Why Benchmark?

The jj-turso architecture replaces beads' traditional JSONL-based storage with:
- **jj (Jujutsu)** for version control instead of git
- **Turso (embedded libSQL)** for fast concurrent queries

This benchmark validates that Turso provides better performance than the baseline JSONL approach, especially under high concurrency (100+ concurrent agents).

## Running Benchmarks

### Quick Start

```bash
# Compare with default settings (100 agents, 1000 tasks)
bd benchmark

# Compare with different parameters
bd benchmark --agents 200 --tasks 2000 --queries 10

# Run only Turso benchmark
bd benchmark --mode turso --agents 100

# Run only baseline (JSONL) benchmark
bd benchmark --mode baseline --agents 100

# Output as JSON
bd benchmark --json
```

### Running Tests

```bash
# Run all benchmark tests
go test ./internal/benchmark/ -v

# Run comparison test (100 agents)
go test ./internal/benchmark/ -v -run TestTursoVsBaseline_100Agents

# Run stress test (200 agents)
go test ./internal/benchmark/ -v -run TestTursoVsBaseline_200Agents

# Run scalability test (10-200 agents)
go test ./internal/benchmark/ -v -run TestScalability

# Run quick validation
go test ./internal/benchmark/ -v -run TestTursoOnly
go test ./internal/benchmark/ -v -run TestBaselineOnly

# Skip slow tests
go test ./internal/benchmark/ -short
```

### Running Go Benchmarks

```bash
# Run Go benchmark framework tests
go test ./internal/benchmark/ -bench=.

# Benchmark Turso only
go test ./internal/benchmark/ -bench=BenchmarkTurso

# Benchmark Baseline only
go test ./internal/benchmark/ -bench=BenchmarkBaseline
```

## Metrics Captured

### Latency Metrics
- **Min**: Fastest query time
- **P50 (Median)**: 50th percentile - half of queries are faster
- **Mean**: Average query time
- **P95**: 95th percentile - 95% of queries are faster (key reliability metric)
- **P99**: 99th percentile - 99% of queries are faster
- **Max**: Slowest query time

### Throughput Metrics
- **Queries per Second (QPS)**: Total throughput
- **Total Queries**: Number of queries executed

### Resource Metrics
- **Memory Before**: Memory usage before benchmark
- **Memory After**: Memory usage after benchmark
- **Memory Peak**: Peak memory usage during benchmark
- **Memory Delta**: Change in memory usage

### Concurrency Metrics
- **Target Agents**: Number of agents configured
- **Actual Agents**: Number of agents that successfully ran
- **Lock Contention**: Number of lock contention events (baseline only)

### Database Metrics
- **Size**: Database/file size in bytes
- **Sync Time**: Time to write/sync changes (ms)
- **Time to First Query**: Startup latency (ms)
- **Task Count**: Total tasks in database
- **Ready Task Count**: Tasks available for work

## Expected Results

### Turso Advantages
1. **Better P95 latency**: More consistent performance under load
2. **Higher QPS**: Better throughput with many concurrent agents
3. **Zero lock contention**: Turso uses WAL mode, eliminating file locks
4. **Better scalability**: Performance degrades less as agent count increases

### Baseline Characteristics
1. **File lock contention**: Increases with concurrent agents
2. **Degraded P95/P99**: Long tail latencies under high concurrency
3. **Lower QPS**: Throughput bottlenecked by file I/O
4. **Poor scalability**: Performance degrades significantly at 100+ agents

## Interpreting Results

### Comparison Report

The benchmark outputs a comparison table showing:

```
================================================================================
BENCHMARK COMPARISON: Turso vs Baseline (JSONL)
================================================================================

Configuration:
  Concurrent Agents:  100
  Total Tasks:        1000
  Queries per Agent:  10
  Blocked %:          30.0%

LATENCY COMPARISON:
Metric     | Turso        | Baseline     | Improvement
----------------------------------------------------------
Min        | 1.23ms       | 2.45ms       | +49.8% ✓
P50        | 2.34ms       | 8.67ms       | +73.0% ✓
Mean       | 2.56ms       | 12.34ms      | +79.2% ✓
P95        | 4.12ms       | 45.67ms      | +91.0% ✓
P99        | 5.23ms       | 89.12ms      | +94.1% ✓
Max        | 8.45ms       | 123.45ms     | +93.2% ✓

THROUGHPUT COMPARISON:
  Turso:      42,735 queries/sec
  Baseline:   8,123 queries/sec
  Improvement: +426.1%

MEMORY COMPARISON:
  Turso Delta:    12.5 MB
  Baseline Delta: 45.7 MB
  Improvement:    +72.6%

CONCURRENCY:
  Turso Lock Contention:    0 events
  Baseline Lock Contention: 247 events
  Reduction:                100%

SUMMARY:
  Turso Wins:     9 metrics
  Baseline Wins:  0 metrics
  Overall Winner: TURSO

KEY INSIGHTS:
  ✓ Turso P95 latency is 91.0% better (more consistent)
  ✓ Turso throughput is 426.1% higher
  ✓ Turso eliminates file lock contention (247 events in baseline)
  ✓ Turso had zero errors vs 3 errors in baseline
```

### Key Metrics to Watch

1. **P95 Latency Improvement**: Should be >50% for Turso
2. **QPS Improvement**: Should be >100% for Turso at 100+ agents
3. **Lock Contention**: Should be 0 for Turso, >0 for baseline
4. **Error Count**: Should be 0 for both

### Failure Scenarios

If Turso doesn't win:
1. Check that database is using WAL mode (should be automatic)
2. Verify connection pool settings (default: 25 max connections)
3. Check for system resource constraints (CPU, disk I/O)
4. Review benchmark parameters (too few agents may not show difference)

## Configuration Parameters

### --agents
Number of concurrent agents to simulate.
- **Default**: 100
- **Recommended**: 100-200 for comparison
- **Range**: 10-500

At low agent counts (10-50), the baseline may be competitive. At 100+, Turso's advantages become clear.

### --tasks
Total number of tasks in the database.
- **Default**: 1000
- **Recommended**: 1000-2000 for realistic workloads
- **Range**: 100-10000

More tasks = more realistic, but slower benchmark execution.

### --queries
Number of queries each agent performs.
- **Default**: 10
- **Recommended**: 10-20 for balanced benchmarks
- **Range**: 5-100

More queries = longer benchmark time, more accurate statistics.

### --blocked
Percentage of tasks that should be blocked by dependencies.
- **Default**: 0.3 (30%)
- **Recommended**: 0.2-0.4 for realistic scenarios
- **Range**: 0.0-0.9

This affects the number of ready tasks returned by queries.

### --mode
Which implementation(s) to benchmark.
- **compare**: Run both and show comparison (default)
- **turso**: Run only Turso benchmark
- **baseline**: Run only baseline (JSONL) benchmark

### --json
Output results as JSON instead of formatted tables.

## Implementation Details

### Turso Runner
1. Creates embedded SQLite database with WAL mode
2. Populates with test tasks and dependencies
3. Spawns N concurrent goroutines (agents)
4. Each agent queries `GetReadyTasks()` M times
5. Measures latency, throughput, memory, errors

### Baseline Runner
1. Creates JSONL file with test issues
2. Spawns N concurrent goroutines (agents)
3. Each agent:
   - Reads entire JSONL file
   - Parses all issues
   - Filters for ready work
   - Returns results
4. Measures latency, throughput, memory, lock contention

### Comparison Logic
1. Runs both benchmarks with identical parameters
2. Calculates improvement ratios for each metric
3. Counts wins for each implementation
4. Determines overall winner
5. Generates comparison report

## Use Cases

### CI/CD Validation
```bash
# Run in CI to ensure Turso wins
bd benchmark --agents 100 --tasks 1000
if [ $? -ne 0 ]; then
  echo "Turso did not win the benchmark"
  exit 1
fi
```

### Performance Regression Testing
```bash
# Run before/after changes to validate no regressions
bd benchmark --json > before.json
# ... make changes ...
bd benchmark --json > after.json
# Compare results
```

### Scalability Analysis
```bash
# Run scalability test
go test ./internal/benchmark/ -v -run TestScalability
```

This shows how both implementations scale from 10 to 200 agents.

### Quick Validation
```bash
# Quick sanity check (50 agents, 500 tasks)
bd benchmark --agents 50 --tasks 500 --queries 5
```

## Troubleshooting

### "Turso benchmark failed: failed to create test database"
- Check disk space in /tmp
- Verify Go SQLite driver is installed
- Check permissions on /tmp directory

### "Baseline had N errors"
- Common at high concurrency (expected)
- Indicates file lock contention
- Not a benchmark failure, proves the point

### "Expected Turso to win, but baseline won"
- Check agent count (may be too low, try 100+)
- Verify Turso is using WAL mode
- Check system resources (CPU, disk)
- Review benchmark parameters

### Slow benchmark execution
- Reduce --tasks (e.g., 500 instead of 1000)
- Reduce --queries (e.g., 5 instead of 10)
- Reduce --agents (e.g., 50 instead of 100)

## Future Enhancements

Potential benchmark improvements:
1. **Mixed workload**: Read + write queries
2. **Network latency**: Simulate remote database
3. **Disk I/O patterns**: Measure fsync times
4. **Memory profiling**: Track allocations
5. **CPU profiling**: Identify hotspots
6. **Comparison with other databases**: PostgreSQL, SQLite (without WAL)

## See Also

- [jj-turso.md](../ai_docs/jj-turso.md) - Architecture documentation
- [internal/benchmark/](../internal/benchmark/) - Benchmark implementation
- [internal/turso/loadtest/](../internal/turso/loadtest/) - Load testing utilities
