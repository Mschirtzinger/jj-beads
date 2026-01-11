package scientific

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/steveyegge/beads/internal/storage/sqlite"
	"github.com/steveyegge/beads/internal/types"
)

// BeadsRunner benchmarks the main beads SQLite implementation.
type BeadsRunner struct {
	store    *sqlite.SQLiteStorage
	taskIDs  []string
	readyIDs []string
	dbPath   string
}

// NewBeadsRunner creates a new benchmark runner for beads SQLite.
// The database is created at dbPath and populated with test data.
func NewBeadsRunner(dbPath string, taskCount int, blockedPercent float64, seed int64) (*BeadsRunner, error) {
	// Remove existing database
	os.Remove(dbPath)

	// Create storage
	ctx := context.Background()
	store, err := sqlite.New(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Set issue prefix for ID generation
	if err := store.SetConfig(ctx, "issue_prefix", "bench"); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("failed to set issue prefix: %w", err)
	}

	runner := &BeadsRunner{
		store:    store,
		taskIDs:  make([]string, 0, taskCount),
		readyIDs: make([]string, 0),
		dbPath:   dbPath,
	}

	// Generate test data
	if err := runner.generateTestData(ctx, taskCount, blockedPercent, seed); err != nil {
		_ = runner.Close()
		return nil, err
	}

	return runner, nil
}

// Close closes the database connection.
func (r *BeadsRunner) Close() error {
	if r.store != nil {
		return r.store.Close()
	}
	return nil
}

// generateTestData populates the database with test issues and dependencies.
func (r *BeadsRunner) generateTestData(ctx context.Context, taskCount int, blockedPercent float64, seed int64) error {
	rng := rand.New(rand.NewSource(seed))

	// Task type distribution
	taskTypes := []types.IssueType{types.TypeBug, types.TypeFeature, types.TypeTask}

	// Priority distribution: weighted toward P2
	// P0: 5%, P1: 15%, P2: 50%, P3: 20%, P4: 10%
	priorities := []int{0, 1, 2, 2, 2, 2, 2, 3, 3, 4}

	baseTime := time.Now().Add(-30 * 24 * time.Hour) // 30 days ago

	// Create issues
	issues := make([]*types.Issue, taskCount)
	for i := 0; i < taskCount; i++ {
		issueType := taskTypes[i%len(taskTypes)]
		priority := priorities[i%len(priorities)]
		createdAt := baseTime.Add(time.Duration(i) * time.Minute)

		issue := &types.Issue{
			Title:       fmt.Sprintf("Benchmark Task %d: %s", i, issueType),
			Description: fmt.Sprintf("Test task for benchmarking (type: %s, priority: P%d)", issueType, priority),
			Status:      types.StatusOpen,
			Priority:    priority,
			IssueType:   issueType,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		}

		if err := r.store.CreateIssue(ctx, issue, "benchmark"); err != nil {
			return fmt.Errorf("failed to create issue %d: %w", i, err)
		}

		r.taskIDs = append(r.taskIDs, issue.ID)
		issues[i] = issue
	}

	// Create blocking dependencies
	numToBlock := int(float64(taskCount) * blockedPercent)
	for i := 0; i < numToBlock && i < taskCount-1; i++ {
		// Pick a blocker from the first half of tasks
		blockerIdx := rng.Intn(taskCount / 2)

		// Pick a blocked task from the second half
		blockedIdx := taskCount/2 + rng.Intn(taskCount/2)

		if blockerIdx >= blockedIdx {
			continue
		}

		blocker := issues[blockerIdx]
		blocked := issues[blockedIdx]

		dep := &types.Dependency{
			IssueID:     blocked.ID,
			DependsOnID: blocker.ID,
			Type:        types.DepBlocks,
			CreatedAt:   blocked.CreatedAt,
		}
		if err := r.store.AddDependency(ctx, dep, "benchmark"); err != nil {
			return fmt.Errorf("failed to add dependency %s blocks %s: %w", blocker.ID, blocked.ID, err)
		}
	}

	// Identify ready tasks (for validation)
	readyIssues, err := r.store.GetReadyWork(ctx, types.WorkFilter{
		Status: types.StatusOpen,
		Limit:  taskCount, // Get all ready tasks
	})
	if err != nil {
		return fmt.Errorf("failed to get ready work: %w", err)
	}

	for _, issue := range readyIssues {
		r.readyIDs = append(r.readyIDs, issue.ID)
	}

	return nil
}

// RunBenchmark executes the benchmark with the specified number of concurrent agents.
func (r *BeadsRunner) RunBenchmark(ctx context.Context, agentCount int, queriesPerAgent int) (*BenchmarkResult, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allDurations []time.Duration
	var errorCount int

	// Channel to collect results
	resultsChan := make(chan []time.Duration, agentCount)
	errorsChan := make(chan error, agentCount)

	startTime := time.Now()

	// Launch concurrent agents
	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(agentID int) {
			defer wg.Done()

			durations := make([]time.Duration, 0, queriesPerAgent)

			for j := 0; j < queriesPerAgent; j++ {
				queryStart := time.Now()

				// Execute the ready work query (this is what agents do)
				_, err := r.store.GetReadyWork(ctx, types.WorkFilter{
					Status: types.StatusOpen,
					Limit:  100, // Typical limit for agent queries
				})

				elapsed := time.Since(queryStart)
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

	totalDuration := time.Since(startTime)

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

	if len(allDurations) == 0 {
		return nil, fmt.Errorf("no successful queries completed")
	}

	// Compute statistics
	result := computeStats(allDurations, errorCount, totalDuration)
	return result, nil
}

// GetStats returns statistics about the test database.
func (r *BeadsRunner) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_tasks":     len(r.taskIDs),
		"ready_tasks":     len(r.readyIDs),
		"blocked_tasks":   len(r.taskIDs) - len(r.readyIDs),
		"blocked_percent": float64(len(r.taskIDs)-len(r.readyIDs)) / float64(len(r.taskIDs)) * 100,
		"ready_percent":   float64(len(r.readyIDs)) / float64(len(r.taskIDs)) * 100,
	}
}

// BenchmarkResult contains the results of a single benchmark run.
type BenchmarkResult struct {
	TotalQueries     int
	ErrorCount       int
	TotalDuration    time.Duration
	Durations        []time.Duration
	Min              time.Duration
	Max              time.Duration
	Mean             time.Duration
	P50              time.Duration
	P95              time.Duration
	P99              time.Duration
	StdDev           time.Duration
	QueriesPerSecond float64
}

// computeStats calculates statistics from benchmark durations.
func computeStats(durations []time.Duration, errorCount int, totalDuration time.Duration) *BenchmarkResult {
	if len(durations) == 0 {
		return &BenchmarkResult{
			ErrorCount: errorCount,
		}
	}

	// Sort for percentile calculation
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate mean
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	mean := sum / time.Duration(len(durations))

	// Calculate standard deviation
	var variance int64
	for _, d := range durations {
		diff := int64(d) - int64(mean)
		variance += diff * diff
	}
	variance /= int64(len(durations))
	stdDev := time.Duration(sqrt(variance))

	// Calculate percentiles
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]

	// Calculate throughput
	qps := float64(len(durations)) / totalDuration.Seconds()

	return &BenchmarkResult{
		TotalQueries:     len(durations),
		ErrorCount:       errorCount,
		TotalDuration:    totalDuration,
		Durations:        sorted,
		Min:              sorted[0],
		Max:              sorted[len(sorted)-1],
		Mean:             mean,
		P50:              p50,
		P95:              p95,
		P99:              p99,
		StdDev:           stdDev,
		QueriesPerSecond: qps,
	}
}

// sqrt computes integer square root using Newton's method.
func sqrt(n int64) int64 {
	if n <= 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}
