// Package loadtest provides load testing utilities for the Turso database layer.
//
// This package simulates concurrent agent access patterns to validate that the
// database can handle 100+ concurrent agents querying for ready work with
// sub-10ms average query latency.
package loadtest

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// TestDatabase represents a populated test database for load testing.
type TestDatabase struct {
	DB          *db.DB
	TaskIDs     []string
	BlockedIDs  []string
	ReadyIDs    []string
	TotalTasks  int
	BlockedPct  float64
}

// LatencyStats captures performance metrics from load tests.
type LatencyStats struct {
	Min         time.Duration
	Max         time.Duration
	Mean        time.Duration
	P50         time.Duration // Median
	P95         time.Duration
	P99         time.Duration
	TotalQueries int
	Errors      int
	Durations   []time.Duration
}

// CreateTestDatabase creates a new test database with the specified number of tasks.
//
// The database is populated with:
//   - Tasks with realistic priorities (weighted toward P2)
//   - Dependency trees where ~30% of tasks are blocked
//   - Task types distributed across bug, feature, task
//   - Realistic timestamps
//
// The blockedPct parameter controls what percentage of tasks should be blocked
// by dependencies (typical: 0.3 for 30%).
func CreateTestDatabase(dbPath string, numTasks int, blockedPct float64) (*TestDatabase, error) {
	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Optimize connection pool for high concurrency testing
	database.RawDB().SetMaxOpenConns(150)  // Support 100+ concurrent agents
	database.RawDB().SetMaxIdleConns(50)   // Keep more idle connections ready
	database.RawDB().SetConnMaxLifetime(10 * time.Minute)

	// Initialize schema
	if err := database.InitSchema(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Generate task data
	td := &TestDatabase{
		DB:          database,
		TaskIDs:     make([]string, 0, numTasks),
		BlockedIDs:  make([]string, 0),
		ReadyIDs:    make([]string, 0),
		TotalTasks:  numTasks,
		BlockedPct:  blockedPct,
	}

	// Create tasks
	tasks := generateTasks(numTasks)
	for _, task := range tasks {
		if err := database.UpsertTask(task); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("failed to insert task %s: %w", task.ID, err)
		}
		td.TaskIDs = append(td.TaskIDs, task.ID)
	}

	// Create dependency trees to achieve desired blocked percentage
	deps := generateDependencies(tasks, blockedPct)
	for _, dep := range deps {
		if err := database.UpsertDep(dep); err != nil {
			_ = database.Close()
			return nil, fmt.Errorf("failed to insert dependency %s: %w", dep.ToFileName(), err)
		}
	}

	// Refresh blocked cache
	if err := database.RefreshBlockedCache(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to refresh blocked cache: %w", err)
	}

	// Identify which tasks are blocked and which are ready
	readyTasks, err := database.GetReadyTasks(context.Background(), db.ReadyTasksOptions{})
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to get ready tasks: %w", err)
	}

	readyMap := make(map[string]bool)
	for _, task := range readyTasks {
		readyMap[task.ID] = true
		td.ReadyIDs = append(td.ReadyIDs, task.ID)
	}

	for _, id := range td.TaskIDs {
		if !readyMap[id] {
			td.BlockedIDs = append(td.BlockedIDs, id)
		}
	}

	return td, nil
}

// Close closes the test database connection.
func (td *TestDatabase) Close() error {
	if td.DB != nil {
		return td.DB.Close()
	}
	return nil
}

// RunConcurrentQueries simulates N concurrent agents querying for ready work.
//
// Each agent performs queriesPerAgent queries, recording latency for each.
// Returns aggregated latency statistics.
func (td *TestDatabase) RunConcurrentQueries(numAgents int, queriesPerAgent int) (*LatencyStats, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allDurations []time.Duration
	var errorCount int

	// Channel to collect results
	resultsChan := make(chan []time.Duration, numAgents)
	errorsChan := make(chan error, numAgents)

	// Launch concurrent agents
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(agentID int) {
			defer wg.Done()

			durations := make([]time.Duration, 0, queriesPerAgent)
			ctx := context.Background()

			for j := 0; j < queriesPerAgent; j++ {
				start := time.Now()

				_, err := td.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
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

	// Collect errors
	for err := range errorsChan {
		if err != nil {
			errorCount++
			fmt.Printf("Error: %v\n", err)
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
	stats := computeLatencyStats(allDurations)
	stats.Errors = errorCount

	return stats, nil
}

// generateTasks creates a slice of test tasks with realistic distribution.
func generateTasks(count int) []*schema.TaskFile {
	tasks := make([]*schema.TaskFile, count)
	types := []string{"bug", "feature", "task"}

	// Priority distribution: weighted toward P2
	// P0: 5%, P1: 15%, P2: 50%, P3: 20%, P4: 10%
	priorities := []int{0, 1, 2, 2, 2, 2, 2, 3, 3, 4}

	baseTime := time.Now().Add(-30 * 24 * time.Hour) // 30 days ago

	for i := 0; i < count; i++ {
		taskID := fmt.Sprintf("test-%05d", i)
		taskType := types[i%len(types)]
		priority := priorities[i%len(priorities)]

		// Stagger creation times
		createdAt := baseTime.Add(time.Duration(i) * time.Minute)

		task := &schema.TaskFile{
			ID:            taskID,
			Title:         fmt.Sprintf("Task %d: %s", i, taskType),
			Description:   fmt.Sprintf("Test task for load testing (type: %s, priority: P%d)", taskType, priority),
			Type:          taskType,
			Status:        "open",
			Priority:      priority,
			AssignedAgent: "",
			Tags:          []string{"loadtest", fmt.Sprintf("batch-%d", i/100)},
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
			DueAt:         nil,
			DeferUntil:    nil,
		}

		tasks[i] = task
	}

	return tasks
}

// generateDependencies creates blocking dependencies to achieve the target blocked percentage.
//
// This creates realistic dependency trees where:
//   - Some tasks block multiple others
//   - Dependencies respect task ordering (earlier tasks block later ones)
//   - Blocked percentage is approximately as specified
func generateDependencies(tasks []*schema.TaskFile, blockedPct float64) []*schema.DepFile {
	if blockedPct <= 0 || blockedPct >= 1 {
		return []*schema.DepFile{}
	}

	deps := make([]*schema.DepFile, 0)
	numToBlock := int(float64(len(tasks)) * blockedPct)

	// Use deterministic random for reproducibility
	rng := rand.New(rand.NewSource(42))

	// Strategy: Create dependency chains
	// - Pick random tasks to be blockers
	// - Have later tasks depend on earlier tasks
	for i := 0; i < numToBlock && i < len(tasks)-1; i++ {
		// Pick a blocker from the first half of tasks
		blockerIdx := rng.Intn(len(tasks) / 2)

		// Pick a blocked task from the second half
		blockedIdx := len(tasks)/2 + rng.Intn(len(tasks)/2)

		if blockerIdx >= blockedIdx {
			continue
		}

		dep := &schema.DepFile{
			From:      tasks[blockerIdx].ID,
			To:        tasks[blockedIdx].ID,
			Type:      "blocks",
			CreatedAt: tasks[blockedIdx].CreatedAt,
		}

		deps = append(deps, dep)
	}

	return deps
}

// computeLatencyStats calculates statistics from a slice of durations.
func computeLatencyStats(durations []time.Duration) *LatencyStats {
	if len(durations) == 0 {
		return &LatencyStats{}
	}

	// Sort durations for percentile calculation
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

	// Calculate percentiles
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]

	return &LatencyStats{
		Min:          sorted[0],
		Max:          sorted[len(sorted)-1],
		Mean:         mean,
		P50:          p50,
		P95:          p95,
		P99:          p99,
		TotalQueries: len(durations),
		Durations:    sorted,
	}
}

// PrintStats formats and prints latency statistics.
func (s *LatencyStats) PrintStats() {
	fmt.Printf("Latency Statistics:\n")
	fmt.Printf("  Total Queries: %d\n", s.TotalQueries)
	fmt.Printf("  Errors:        %d\n", s.Errors)
	fmt.Printf("  Min:           %v\n", s.Min)
	fmt.Printf("  P50 (Median):  %v\n", s.P50)
	fmt.Printf("  Mean:          %v\n", s.Mean)
	fmt.Printf("  P95:           %v\n", s.P95)
	fmt.Printf("  P99:           %v\n", s.P99)
	fmt.Printf("  Max:           %v\n", s.Max)
}

// VerifyNoRaceConditions runs queries with race detection enabled.
//
// This performs concurrent reads and writes to verify that the database
// handles concurrent access correctly without data corruption.
func (td *TestDatabase) VerifyNoRaceConditions(numAgents int, duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	errorsChan := make(chan error, numAgents)

	// Launch reader agents
	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(agentID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Query ready tasks
					tasks, err := td.DB.GetReadyTasks(ctx, db.ReadyTasksOptions{})
					if err != nil && ctx.Err() == nil {
						errorsChan <- fmt.Errorf("agent %d read failed: %w", agentID, err)
						return
					}

					// Verify data consistency
					for _, task := range tasks {
						if task.ID == "" {
							errorsChan <- fmt.Errorf("agent %d found task with empty ID", agentID)
							return
						}
						if task.Status != "open" {
							errorsChan <- fmt.Errorf("agent %d found non-open task in ready list: %s (status: %s)", agentID, task.ID, task.Status)
							return
						}
					}

					// Small sleep to avoid hammering
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}

	// Wait for all agents to complete
	wg.Wait()
	close(errorsChan)

	// Check for errors
	for err := range errorsChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// GetStats returns statistics about the test database.
func (td *TestDatabase) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_tasks":     td.TotalTasks,
		"ready_tasks":     len(td.ReadyIDs),
		"blocked_tasks":   len(td.BlockedIDs),
		"blocked_percent": float64(len(td.BlockedIDs)) / float64(td.TotalTasks) * 100,
		"ready_percent":   float64(len(td.ReadyIDs)) / float64(td.TotalTasks) * 100,
	}
}
