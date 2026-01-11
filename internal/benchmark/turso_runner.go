package benchmark

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/loadtest"
)

// RunTursoBenchmark executes a benchmark using the Turso database implementation.
//
// This creates a test database with the specified number of tasks, spawns
// concurrent agents that query for ready work, and measures performance metrics.
func RunTursoBenchmark(config BenchmarkConfig) (*BenchmarkResult, error) {
	// Clean up any existing database
	_ = os.Remove(config.DBPath)
	defer func() { _ = os.Remove(config.DBPath) }()

	// Measure memory before
	memBefore := GetMemoryStats()

	// Measure time to create and populate database
	setupStart := time.Now()

	// Create test database with tasks
	testDB, err := loadtest.CreateTestDatabase(config.DBPath, config.NumTasks, config.BlockedPct)
	if err != nil {
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}
	defer func() { _ = testDB.Close() }()

	setupDuration := time.Since(setupStart)

	// Get database stats
	dbStats := testDB.GetStats()
	taskCount := dbStats["total_tasks"].(int)
	readyCount := dbStats["ready_tasks"].(int)

	// Get database file size
	fileInfo, err := os.Stat(config.DBPath)
	var dbSize int64
	if err == nil {
		dbSize = fileInfo.Size()
	}

	// Measure time to first query
	firstQueryStart := time.Now()
	_, err = testDB.DB.GetReadyTasks(context.Background(), db.ReadyTasksOptions{})
	if err != nil {
		return nil, fmt.Errorf("first query failed: %w", err)
	}
	timeToFirst := time.Since(firstQueryStart)

	// Run concurrent queries
	benchStart := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allDurations []time.Duration
	var errorCount int

	// Channel to collect results
	resultsChan := make(chan []time.Duration, config.NumAgents)
	errorsChan := make(chan error, config.NumAgents)

	// Launch concurrent agents
	for i := 0; i < config.NumAgents; i++ {
		wg.Add(1)
		go func(agentID int) {
			defer wg.Done()

			durations := make([]time.Duration, 0, config.QueriesPerAgent)
			ctx := context.Background()

			for j := 0; j < config.QueriesPerAgent; j++ {
				start := time.Now()

				_, err := testDB.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
				elapsed := time.Since(start)

				durations = append(durations, elapsed)

				if err != nil {
					errorsChan <- fmt.Errorf("agent %d query %d failed: %w", agentID, j, err)
					return
				}
			}

			resultsChan <- durations
		}(i)
	}

	// Wait for all agents to complete
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	benchDuration := time.Since(benchStart)

	// Collect errors
	for err := range errorsChan {
		if err != nil {
			errorCount++
		}
	}

	// Collect all durations
	for durations := range resultsChan {
		mu.Lock()
		allDurations = append(allDurations, durations...)
		mu.Unlock()
	}

	// Measure memory after
	memAfter := GetMemoryStats()
	memStats := CompareMemoryStats(memBefore, memAfter)

	// Calculate statistics
	latencyStats := ComputeStats(allDurations)

	totalQueries := len(allDurations)
	qps := 0.0
	if benchDuration.Seconds() > 0 {
		qps = float64(totalQueries) / benchDuration.Seconds()
	}

	errorRate := 0.0
	if totalQueries > 0 {
		errorRate = float64(errorCount) / float64(totalQueries)
	}

	result := &BenchmarkResult{
		Config:        config,
		Latency:       latencyStats,
		TotalDuration: benchDuration,
		ErrorCount:    errorCount,
		ErrorRate:     errorRate,
		Success:       errorCount == 0,
		Throughput: ThroughputMetrics{
			QueriesPerSecond: qps,
			TotalQueries:     totalQueries,
		},
		Resources: memStats,
		Concurrency: ConcurrencyMetrics{
			TargetAgents:   config.NumAgents,
			ActualAgents:   config.NumAgents,
			LockContention: 0, // Turso uses WAL, no file locking
		},
		Database: DatabaseMetrics{
			SizeBytes:      dbSize,
			SyncTimeMs:     setupDuration.Milliseconds(),
			TimeToFirstMs:  timeToFirst.Milliseconds(),
			TaskCount:      taskCount,
			ReadyTaskCount: readyCount,
		},
	}

	return result, nil
}
