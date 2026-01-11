package scientific

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/steveyegge/beads/internal/turso/db"
	"github.com/steveyegge/beads/internal/turso/schema"
)

// TursoRunner benchmarks the jj-turso implementation.
type TursoRunner struct {
	db       *db.DB
	taskIDs  []string
	readyIDs []string
	dbPath   string
}

// NewTursoRunner creates a new benchmark runner for jj-turso.
// The database is created at dbPath and populated with test data.
func NewTursoRunner(dbPath string, taskCount int, blockedPercent float64, seed int64) (*TursoRunner, error) {
	// Remove existing database
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	// Create database
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Optimize connection pool for high concurrency
	database.RawDB().SetMaxOpenConns(150)
	database.RawDB().SetMaxIdleConns(50)
	database.RawDB().SetConnMaxLifetime(10 * time.Minute)

	// Initialize schema
	if err := database.InitSchema(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	runner := &TursoRunner{
		db:       database,
		taskIDs:  make([]string, 0, taskCount),
		readyIDs: make([]string, 0),
		dbPath:   dbPath,
	}

	// Generate test data
	if err := runner.generateTestData(taskCount, blockedPercent, seed); err != nil {
		_ = runner.Close()
		return nil, err
	}

	return runner, nil
}

// Close closes the database connection.
func (r *TursoRunner) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// generateTestData populates the database with test tasks and dependencies.
func (r *TursoRunner) generateTestData(taskCount int, blockedPercent float64, seed int64) error {
	rng := rand.New(rand.NewSource(seed))
	ctx := context.Background()

	// Task type distribution
	taskTypes := []string{"bug", "feature", "task"}

	// Priority distribution: weighted toward P2
	priorities := []int{0, 1, 2, 2, 2, 2, 2, 3, 3, 4}

	baseTime := time.Now().Add(-30 * 24 * time.Hour) // 30 days ago

	// Create tasks
	tasks := make([]*schema.TaskFile, taskCount)
	for i := 0; i < taskCount; i++ {
		taskID := fmt.Sprintf("bench-%05d", i)
		taskType := taskTypes[i%len(taskTypes)]
		priority := priorities[i%len(priorities)]
		createdAt := baseTime.Add(time.Duration(i) * time.Minute)

		task := &schema.TaskFile{
			ID:            taskID,
			Title:         fmt.Sprintf("Benchmark Task %d: %s", i, taskType),
			Description:   fmt.Sprintf("Test task for benchmarking (type: %s, priority: P%d)", taskType, priority),
			Type:          taskType,
			Status:        "open",
			Priority:      priority,
			AssignedAgent: "",
			Tags:          []string{"benchmark", fmt.Sprintf("batch-%d", i/100)},
			CreatedAt:     createdAt,
			UpdatedAt:     createdAt,
		}

		if err := r.db.UpsertTask(task); err != nil {
			return fmt.Errorf("failed to upsert task %s: %w", taskID, err)
		}

		r.taskIDs = append(r.taskIDs, taskID)
		tasks[i] = task
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

		blocker := tasks[blockerIdx]
		blocked := tasks[blockedIdx]

		dep := &schema.DepFile{
			From:      blocker.ID,
			To:        blocked.ID,
			Type:      "blocks",
			CreatedAt: blocked.CreatedAt,
		}

		if err := r.db.UpsertDep(dep); err != nil {
			return fmt.Errorf("failed to upsert dependency: %w", err)
		}
	}

	// Refresh blocked cache
	if err := r.db.RefreshBlockedCache(); err != nil {
		return fmt.Errorf("failed to refresh blocked cache: %w", err)
	}

	// Identify ready tasks (for validation)
	readyTasks, err := r.db.GetReadyTasks(ctx, db.ReadyTasksOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ready tasks: %w", err)
	}

	for _, task := range readyTasks {
		r.readyIDs = append(r.readyIDs, task.ID)
	}

	return nil
}

// RunBenchmark executes the benchmark with the specified number of concurrent agents.
func (r *TursoRunner) RunBenchmark(ctx context.Context, agentCount int, queriesPerAgent int) (*BenchmarkResult, error) {
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
				_, err := r.db.GetReadyTasks(ctx, db.ReadyTasksOptions{
					Limit: 100, // Typical limit for agent queries
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
func (r *TursoRunner) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_tasks":     len(r.taskIDs),
		"ready_tasks":     len(r.readyIDs),
		"blocked_tasks":   len(r.taskIDs) - len(r.readyIDs),
		"blocked_percent": float64(len(r.taskIDs)-len(r.readyIDs)) / float64(len(r.taskIDs)) * 100,
		"ready_percent":   float64(len(r.readyIDs)) / float64(len(r.taskIDs)) * 100,
	}
}
