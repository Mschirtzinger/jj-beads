package benchmark

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

// RunBaselineBenchmark executes a benchmark using the baseline JSONL implementation.
//
// This simulates the current bd ready workflow:
// 1. Read entire JSONL file
// 2. Parse all issues
// 3. Filter for ready work (open, not blocked, no deferred)
// 4. Sort and return results
//
// This tests file-based concurrency which should show degradation at high agent counts.
func RunBaselineBenchmark(config BenchmarkConfig) (*BenchmarkResult, error) {
	// Clean up any existing JSONL file
	jsonlPath := config.DBPath + ".jsonl"
	os.Remove(jsonlPath)
	defer os.Remove(jsonlPath)

	// Measure memory before
	memBefore := GetMemoryStats()

	// Measure time to create JSONL file
	setupStart := time.Now()

	// Generate test data
	issues, blockedIDs := generateTestIssues(config.NumTasks, config.BlockedPct)

	// Write to JSONL file
	if err := writeJSONL(jsonlPath, issues); err != nil {
		return nil, fmt.Errorf("failed to write JSONL: %w", err)
	}

	setupDuration := time.Since(setupStart)

	// Get file size
	fileInfo, err := os.Stat(jsonlPath)
	var fileSize int64
	if err == nil {
		fileSize = fileInfo.Size()
	}

	// Count ready tasks
	readyCount := 0
	for _, issue := range issues {
		if isReady(issue, blockedIDs) {
			readyCount++
		}
	}

	// Measure time to first query
	firstQueryStart := time.Now()
	_, err = queryReadyFromJSONL(jsonlPath, blockedIDs)
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
	var lockContentionCount int64

	// Channel to collect results
	resultsChan := make(chan []time.Duration, config.NumAgents)
	errorsChan := make(chan error, config.NumAgents)

	// Launch concurrent agents
	for i := 0; i < config.NumAgents; i++ {
		wg.Add(1)
		go func(agentID int) {
			defer wg.Done()

			durations := make([]time.Duration, 0, config.QueriesPerAgent)

			for j := 0; j < config.QueriesPerAgent; j++ {
				start := time.Now()

				// Attempt to read file - this may block on file locks
				lockStart := time.Now()
				_, err := queryReadyFromJSONL(jsonlPath, blockedIDs)
				lockDuration := time.Since(lockStart)

				// If query took > 100ms, likely file lock contention
				if lockDuration > 100*time.Millisecond {
					atomic.AddInt64(&lockContentionCount, 1)
				}

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
			LockContention: int(lockContentionCount),
		},
		Database: DatabaseMetrics{
			SizeBytes:      fileSize,
			SyncTimeMs:     setupDuration.Milliseconds(),
			TimeToFirstMs:  timeToFirst.Milliseconds(),
			TaskCount:      config.NumTasks,
			ReadyTaskCount: readyCount,
		},
	}

	return result, nil
}

// generateTestIssues creates a set of test issues with dependencies.
func generateTestIssues(numTasks int, blockedPct float64) ([]*types.Issue, map[string]bool) {
	issues := make([]*types.Issue, numTasks)
	blockedIDs := make(map[string]bool)

	baseTime := time.Now().Add(-30 * 24 * time.Hour)

	// Create issues
	for i := 0; i < numTasks; i++ {
		issueID := fmt.Sprintf("test-%05d", i)
		createdAt := baseTime.Add(time.Duration(i) * time.Minute)

		issue := &types.Issue{
			ID:          issueID,
			Title:       fmt.Sprintf("Test Issue %d", i),
			Description: "Test issue for baseline benchmark",
			Status:      types.StatusOpen,
			Priority:    i % 5, // P0-P4
			IssueType:   types.TypeTask,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		}

		issues[i] = issue
	}

	// Create dependencies to achieve blocked percentage
	if blockedPct > 0 && blockedPct < 1 {
		numToBlock := int(float64(numTasks) * blockedPct)
		rng := rand.New(rand.NewSource(42))

		for i := 0; i < numToBlock && i < numTasks-1; i++ {
			blockedIdx := rng.Intn(numTasks)
			blockedIDs[issues[blockedIdx].ID] = true
		}
	}

	return issues, blockedIDs
}

// writeJSONL writes issues to a JSONL file.
func writeJSONL(path string, issues []*types.Issue) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, issue := range issues {
		if err := encoder.Encode(issue); err != nil {
			return err
		}
	}

	return nil
}

// queryReadyFromJSONL reads JSONL file and filters for ready issues.
// This simulates the current bd ready workflow.
func queryReadyFromJSONL(path string, blockedIDs map[string]bool) ([]*types.Issue, error) {
	// Read entire file
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Parse all issues
	var issues []*types.Issue
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var issue types.Issue
		if err := decoder.Decode(&issue); err != nil {
			return nil, err
		}
		issues = append(issues, &issue)
	}

	// Filter for ready work
	var ready []*types.Issue
	for _, issue := range issues {
		if isReady(issue, blockedIDs) {
			ready = append(ready, issue)
		}
	}

	return ready, nil
}

// isReady checks if an issue is ready for work.
func isReady(issue *types.Issue, blockedIDs map[string]bool) bool {
	// Must be open
	if issue.Status != types.StatusOpen {
		return false
	}

	// Must not be blocked
	if blockedIDs[issue.ID] {
		return false
	}

	// Must not be deferred
	if issue.DeferUntil != nil && issue.DeferUntil.After(time.Now()) {
		return false
	}

	return true
}
