// Package db provides Turso (embedded libSQL) database integration for jj-beads.
//
// This package implements the query cache layer for the jj-turso architecture,
// which replaces beads' git-based sync with jj (Jujutsu) for version control
// and Turso for fast concurrent queries.
//
// The database runs in EMBEDDED/SELF-HOSTED mode (NOT cloud mode) using libSQL
// with SQLite embedded mode and WAL for concurrency support.
//
// Architecture:
//   - Database file: .beads/turso.db
//   - WAL mode: Concurrent readers during writes
//   - Schema: tasks, deps, blocked_cache tables
//   - Indexes: Optimized for ready work queries (status, priority, defer_until)
//
// Workflow:
//  1. Agents modify task files in tasks/*.json (jj working copy)
//  2. Sync daemon watches jj op log for changes
//  3. Changes are synced to Turso for fast querying
//  4. CLI queries Turso for ready work, not filesystem
package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/beads/internal/turso/schema"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// DB wraps the libSQL database connection with Turso-specific functionality.
// This provides embedded SQLite with WAL mode for concurrent access.
type DB struct {
	conn *sql.DB
	path string
}

// Open creates a new database connection at the specified path using libSQL.
//
// The database is opened in embedded mode with WAL for concurrent reads.
// If the database doesn't exist, it will be created along with the schema.
//
// The caller MUST call Close() when done to ensure proper cleanup.
//
// Example:
//
//	db, err := db.Open(".beads/turso.db")
//	if err != nil {
//	    return err
//	}
//	defer db.Close()
func Open(path string) (*DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database using sqlite3 driver (ncruces/go-sqlite3)
	// Format for embedded mode: file:path
	connStr := fmt.Sprintf("file:%s", path)
	conn, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	db := &DB{
		conn: conn,
		path: path,
	}

	// Enable WAL mode for concurrent reads
	if _, err := db.conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout to 5 seconds
	if _, err := db.conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Enable foreign keys
	if _, err := db.conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return db, nil
}

// RawDB returns the underlying sql.DB connection.
// This is useful for integrating with other libraries that expect *sql.DB.
func (db *DB) RawDB() *sql.DB {
	return db.conn
}

// Close closes the database connection.
// Performs a WAL checkpoint to ensure all changes are persisted.
func (db *DB) Close() error {
	if db.conn == nil {
		return nil
	}

	// Checkpoint WAL before closing
	if _, err := db.conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to checkpoint WAL: %v\n", err)
	}

	if err := db.conn.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	db.conn = nil
	return nil
}

// InitSchema creates the database schema if it doesn't exist.
//
// This creates the tasks, deps, and blocked_cache tables along with
// necessary indexes for fast queries. This is idempotent - safe to call
// multiple times.
func (db *DB) InitSchema() error {
	return db.InitSchemaContext(context.Background())
}

// InitSchemaContext creates the database schema with context support.
func (db *DB) InitSchemaContext(ctx context.Context) error {
	schema := `
	-- Core tables
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		priority INTEGER NOT NULL DEFAULT 2,
		assigned_agent TEXT,
		description TEXT,
		tags TEXT,  -- JSON array
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		due_at TEXT,
		defer_until TEXT,

		-- Computed for fast queries
		is_blocked INTEGER NOT NULL DEFAULT 0,
		blocking_count INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS deps (
		from_id TEXT NOT NULL,
		to_id TEXT NOT NULL,
		type TEXT NOT NULL,  -- blocks, related, parent-child, discovered-from
		created_at TEXT NOT NULL,
		PRIMARY KEY (from_id, to_id, type),
		FOREIGN KEY (from_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY (to_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	-- Materialized view for ready queries
	CREATE TABLE IF NOT EXISTS blocked_cache (
		task_id TEXT PRIMARY KEY,
		blocked_by TEXT,  -- JSON array of blocking task IDs
		computed_at TEXT NOT NULL,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	-- Indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
	CREATE INDEX IF NOT EXISTS idx_tasks_assigned ON tasks(assigned_agent);
	CREATE INDEX IF NOT EXISTS idx_tasks_defer ON tasks(defer_until);
	CREATE INDEX IF NOT EXISTS idx_tasks_blocked ON tasks(is_blocked);
	CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);

	-- Composite index for ready work optimization
	CREATE INDEX IF NOT EXISTS idx_tasks_ready_work
	    ON tasks(status, is_blocked, defer_until, priority);

	CREATE INDEX IF NOT EXISTS idx_deps_to ON deps(to_id);
	CREATE INDEX IF NOT EXISTS idx_deps_from ON deps(from_id);
	CREATE INDEX IF NOT EXISTS idx_deps_type ON deps(type);
	CREATE INDEX IF NOT EXISTS idx_deps_blocks
	    ON deps(type, from_id) WHERE type = 'blocks';
	`

	if _, err := db.conn.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// UpsertTask inserts or updates a task in the database.
//
// If a task with the same ID exists, it is updated.
// Tags are stored as a JSON array string.
func (db *DB) UpsertTask(task *schema.TaskFile) error {
	return db.UpsertTaskContext(context.Background(), task)
}

// UpsertTaskContext inserts or updates a task with context support.
func (db *DB) UpsertTaskContext(ctx context.Context, task *schema.TaskFile) error {
	if err := task.Validate(); err != nil {
		return fmt.Errorf("invalid task: %w", err)
	}

	// Serialize tags to JSON
	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
	INSERT INTO tasks (
		id, title, description, type, status, priority,
		assigned_agent, tags, created_at, updated_at,
		due_at, defer_until, is_blocked, blocking_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		description = excluded.description,
		type = excluded.type,
		status = excluded.status,
		priority = excluded.priority,
		assigned_agent = excluded.assigned_agent,
		tags = excluded.tags,
		updated_at = excluded.updated_at,
		due_at = excluded.due_at,
		defer_until = excluded.defer_until
	`

	_, err = db.conn.ExecContext(ctx, query,
		task.ID,
		task.Title,
		task.Description,
		task.Type,
		task.Status,
		task.Priority,
		task.AssignedAgent,
		string(tagsJSON),
		task.CreatedAt.Format(time.RFC3339),
		task.UpdatedAt.Format(time.RFC3339),
		timeToNullString(task.DueAt),
		timeToNullString(task.DeferUntil),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert task: %w", err)
	}

	return nil
}

// DeleteTask removes a task from the database.
//
// This also cascades to remove dependencies and blocked cache entries.
// Returns nil if the task doesn't exist (idempotent).
func (db *DB) DeleteTask(taskID string) error {
	return db.DeleteTaskContext(context.Background(), taskID)
}

// DeleteTaskContext removes a task with context support.
func (db *DB) DeleteTaskContext(ctx context.Context, taskID string) error {
	query := `DELETE FROM tasks WHERE id = ?`
	_, err := db.conn.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete task %s: %w", taskID, err)
	}
	return nil
}

// UpsertDep inserts or updates a dependency in the database.
func (db *DB) UpsertDep(dep *schema.DepFile) error {
	return db.UpsertDepContext(context.Background(), dep)
}

// UpsertDepContext inserts or updates a dependency with context support.
func (db *DB) UpsertDepContext(ctx context.Context, dep *schema.DepFile) error {
	if err := dep.Validate(); err != nil {
		return fmt.Errorf("invalid dependency: %w", err)
	}

	query := `
	INSERT INTO deps (from_id, to_id, type, created_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(from_id, to_id, type) DO UPDATE SET
		created_at = excluded.created_at
	`

	_, err := db.conn.ExecContext(ctx, query,
		dep.From,
		dep.To,
		dep.Type,
		dep.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert dependency %s--%s--%s: %w", dep.From, dep.Type, dep.To, err)
	}

	return nil
}

// DeleteDep removes a dependency from the database.
//
// Returns nil if the dependency doesn't exist (idempotent).
func (db *DB) DeleteDep(from, to, typ string) error {
	return db.DeleteDepContext(context.Background(), from, to, typ)
}

// DeleteDepContext removes a dependency with context support.
func (db *DB) DeleteDepContext(ctx context.Context, from, to, typ string) error {
	query := `DELETE FROM deps WHERE from_id = ? AND to_id = ? AND type = ?`
	_, err := db.conn.ExecContext(ctx, query, from, to, typ)
	if err != nil {
		return fmt.Errorf("failed to delete dependency %s--%s--%s: %w", from, typ, to, err)
	}
	return nil
}

// RefreshBlockedCache recomputes the blocked status for all tasks.
//
// This performs a transitive closure query to find all tasks that are
// blocked by open tasks with "blocks" dependencies.
func (db *DB) RefreshBlockedCache() error {
	return db.RefreshBlockedCacheContext(context.Background())
}

// RefreshBlockedCacheContext recomputes the blocked status with context support.
func (db *DB) RefreshBlockedCacheContext(ctx context.Context) error {
	// Start transaction
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing cache
	if _, err := tx.ExecContext(ctx, "DELETE FROM blocked_cache"); err != nil {
		return fmt.Errorf("failed to clear blocked cache: %w", err)
	}

	// Compute transitive closure of blocks dependencies
	query := `
	WITH RECURSIVE blocked AS (
		-- Base case: direct blocks dependencies
		SELECT to_id as task_id, from_id as blocker
		FROM deps
		WHERE type = 'blocks'
		  AND from_id IN (SELECT id FROM tasks WHERE status != 'closed')

		UNION

		-- Recursive case: transitive dependencies
		SELECT b.task_id, d.from_id
		FROM blocked b
		JOIN deps d ON d.to_id = b.blocker
		WHERE d.type = 'blocks'
		  AND d.from_id IN (SELECT id FROM tasks WHERE status != 'closed')
	)
	INSERT INTO blocked_cache (task_id, blocked_by, computed_at)
	SELECT
		task_id,
		json_group_array(blocker) as blocked_by,
		datetime('now') as computed_at
	FROM blocked
	GROUP BY task_id
	`

	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to compute blocked cache: %w", err)
	}

	// Update is_blocked flag on tasks
	updateQuery := `
	UPDATE tasks SET is_blocked =
		CASE
			WHEN id IN (SELECT task_id FROM blocked_cache) THEN 1
			ELSE 0
		END
	`

	if _, err := tx.ExecContext(ctx, updateQuery); err != nil {
		return fmt.Errorf("failed to update is_blocked flags: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTaskCount returns the total number of tasks in the database.
func (db *DB) GetTaskCount() (int, error) {
	return db.GetTaskCountContext(context.Background())
}

// GetTaskCountContext returns the total number of tasks with context support.
func (db *DB) GetTaskCountContext(ctx context.Context) (int, error) {
	var count int
	err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get task count: %w", err)
	}
	return count, nil
}

// GetDepCount returns the total number of dependencies in the database.
func (db *DB) GetDepCount() (int, error) {
	return db.GetDepCountContext(context.Background())
}

// GetDepCountContext returns the total number of dependencies with context support.
func (db *DB) GetDepCountContext(ctx context.Context) (int, error) {
	var count int
	err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM deps").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get dep count: %w", err)
	}
	return count, nil
}

// ReadyTasksOptions configures the GetReadyTasks query.
type ReadyTasksOptions struct {
	// IncludeDeferred includes tasks that are deferred but otherwise ready
	IncludeDeferred bool
	// Limit restricts the number of results (0 = no limit)
	Limit int
	// AssignedAgent filters to tasks assigned to a specific agent (empty = all)
	AssignedAgent string
}

// GetReadyTasks finds tasks that are ready to work on.
// A task is ready if:
//   - status = 'open'
//   - is_blocked = 0 (no blocking dependencies)
//   - defer_until IS NULL OR defer_until <= now (unless IncludeDeferred is true)
//
// Results are ordered by priority ASC (P0 first), then created_at ASC.
func (db *DB) GetReadyTasks(ctx context.Context, opts ReadyTasksOptions) ([]*schema.TaskFile, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "status = ?")
	args = append(args, "open")

	conditions = append(conditions, "is_blocked = 0")

	if !opts.IncludeDeferred {
		conditions = append(conditions, "(defer_until IS NULL OR defer_until <= ?)")
		args = append(args, time.Now().Format(time.RFC3339))
	}

	if opts.AssignedAgent != "" {
		conditions = append(conditions, "assigned_agent = ?")
		args = append(args, opts.AssignedAgent)
	}

	query := `
		SELECT id, title, description, type, status, priority,
		       assigned_agent, tags, created_at, updated_at,
		       due_at, defer_until
		FROM tasks
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY priority ASC, created_at ASC
	`

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query ready tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// scanTasks is a helper function to scan multiple tasks from query results.
func scanTasks(rows *sql.Rows) ([]*schema.TaskFile, error) {
	var tasks []*schema.TaskFile

	for rows.Next() {
		var task schema.TaskFile
		var tagsJSON string
		var createdAt, updatedAt string
		var dueAt, deferUntil sql.NullString

		err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&task.Type,
			&task.Status,
			&task.Priority,
			&task.AssignedAgent,
			&tagsJSON,
			&createdAt,
			&updatedAt,
			&dueAt,
			&deferUntil,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		// Parse timestamps
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			task.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			task.UpdatedAt = t
		}

		// Parse tags
		if tagsJSON != "" && tagsJSON != "null" {
			if err := json.Unmarshal([]byte(tagsJSON), &task.Tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		} else {
			task.Tags = []string{}
		}

		// Parse optional time fields
		task.DueAt = nullStringToTime(dueAt)
		task.DeferUntil = nullStringToTime(deferUntil)

		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// timeToNullString converts a time pointer to a nullable string for SQL.
func timeToNullString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}

// nullStringToTime converts a nullable SQL string to a time pointer.
func nullStringToTime(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}

// UpdateBlockedCache is an alias for RefreshBlockedCache for API compatibility.
func (db *DB) UpdateBlockedCache() error {
	return db.RefreshBlockedCache()
}

// UpdateBlockedCacheContext is an alias for RefreshBlockedCacheContext for API compatibility.
func (db *DB) UpdateBlockedCacheContext(ctx context.Context) error {
	return db.RefreshBlockedCacheContext(ctx)
}

// GetBlockingTasks returns the list of tasks that are blocking the given task.
// This performs a transitive closure over "blocks" dependencies to find all
// blocking tasks, not just direct dependencies.
func (db *DB) GetBlockingTasks(taskID string) ([]*schema.TaskFile, error) {
	return db.GetBlockingTasksContext(context.Background(), taskID)
}

// GetBlockingTasksContext returns blocking tasks with context support.
func (db *DB) GetBlockingTasksContext(ctx context.Context, taskID string) ([]*schema.TaskFile, error) {
	query := `
	WITH RECURSIVE blocking AS (
		-- Base case: direct blockers
		SELECT from_id as blocker_id
		FROM deps
		WHERE to_id = ? AND type = 'blocks'

		UNION

		-- Recursive case: blockers of blockers
		SELECT d.from_id
		FROM deps d
		JOIN blocking b ON d.to_id = b.blocker_id
		WHERE d.type = 'blocks'
	)
	SELECT DISTINCT t.id, t.title, t.description, t.type, t.status, t.priority,
	       t.assigned_agent, t.tags, t.created_at, t.updated_at,
	       t.due_at, t.defer_until
	FROM tasks t
	JOIN blocking b ON t.id = b.blocker_id
	WHERE t.status != 'closed'
	ORDER BY t.priority ASC, t.created_at ASC
	`

	rows, err := db.conn.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocking tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// GetTaskByID retrieves a single task by ID.
// Returns sql.ErrNoRows if the task is not found.
func (db *DB) GetTaskByID(id string) (*schema.TaskFile, error) {
	return db.GetTaskByIDContext(context.Background(), id)
}

// GetTaskByIDContext retrieves a single task by ID with context support.
func (db *DB) GetTaskByIDContext(ctx context.Context, id string) (*schema.TaskFile, error) {
	query := `
	SELECT id, title, description, type, status, priority,
	       assigned_agent, tags, created_at, updated_at,
	       due_at, defer_until
	FROM tasks
	WHERE id = ?
	`

	row := db.conn.QueryRowContext(ctx, query, id)

	var task schema.TaskFile
	var tagsJSON string
	var createdAt, updatedAt string
	var dueAt, deferUntil sql.NullString

	err := row.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Type,
		&task.Status,
		&task.Priority,
		&task.AssignedAgent,
		&tagsJSON,
		&createdAt,
		&updatedAt,
		&dueAt,
		&deferUntil,
	)
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		task.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		task.UpdatedAt = t
	}

	// Parse tags
	if tagsJSON != "" && tagsJSON != "null" {
		if err := json.Unmarshal([]byte(tagsJSON), &task.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	} else {
		task.Tags = []string{}
	}

	// Parse optional time fields
	task.DueAt = nullStringToTime(dueAt)
	task.DeferUntil = nullStringToTime(deferUntil)

	return &task, nil
}

// ListTasksFilter configures the ListTasks query.
type ListTasksFilter struct {
	// Status filters by task status (empty = all statuses)
	Status string
	// Type filters by task type (empty = all types)
	Type string
	// Priority filters by exact priority (-1 = all priorities)
	Priority int
	// AssignedAgent filters by assigned agent (empty = all agents)
	AssignedAgent string
	// Tag filters by tag (empty = all tags)
	Tag string
	// Limit restricts the number of results (0 = no limit)
	Limit int
	// Offset skips the first N results (for pagination)
	Offset int
}

// ListTasks retrieves tasks matching the given filters.
// Results are ordered by priority ASC, then created_at ASC.
func (db *DB) ListTasks(filter ListTasksFilter) ([]*schema.TaskFile, error) {
	return db.ListTasksContext(context.Background(), filter)
}

// ListTasksContext retrieves tasks with context support.
func (db *DB) ListTasksContext(ctx context.Context, filter ListTasksFilter) ([]*schema.TaskFile, error) {
	var conditions []string
	var args []interface{}

	if filter.Status != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, filter.Status)
	}

	if filter.Type != "" {
		conditions = append(conditions, "t.type = ?")
		args = append(args, filter.Type)
	}

	if filter.Priority >= 0 {
		conditions = append(conditions, "t.priority = ?")
		args = append(args, filter.Priority)
	}

	if filter.AssignedAgent != "" {
		conditions = append(conditions, "t.assigned_agent = ?")
		args = append(args, filter.AssignedAgent)
	}

	// Build SELECT clause - only use DISTINCT when joining with json_each
	selectClause := "SELECT"
	if filter.Tag != "" {
		selectClause += " DISTINCT"
	}

	query := selectClause + ` t.id, t.title, t.description, t.type, t.status, t.priority,
	       t.assigned_agent, t.tags, t.created_at, t.updated_at,
	       t.due_at, t.defer_until
	FROM tasks t
	`

	// Add tag join if filtering by tag
	if filter.Tag != "" {
		query += `, json_each(t.tags)`
		conditions = append(conditions, "json_each.value = ?")
		args = append(args, filter.Tag)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY t.priority ASC, t.created_at ASC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	return scanTasks(rows)
}

// GetDepsForTask returns all dependencies for a given task.
// This includes both dependencies (tasks this task depends on)
// and dependents (tasks that depend on this task).
func (db *DB) GetDepsForTask(taskID string) ([]*schema.DepFile, error) {
	return db.GetDepsForTaskContext(context.Background(), taskID)
}

// GetDepsForTaskContext returns dependencies with context support.
func (db *DB) GetDepsForTaskContext(ctx context.Context, taskID string) ([]*schema.DepFile, error) {
	query := `
	SELECT from_id, to_id, type, created_at
	FROM deps
	WHERE from_id = ? OR to_id = ?
	ORDER BY created_at ASC
	`

	rows, err := db.conn.QueryContext(ctx, query, taskID, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer rows.Close()

	var deps []*schema.DepFile
	for rows.Next() {
		var dep schema.DepFile
		var createdAtStr string

		err := rows.Scan(&dep.From, &dep.To, &dep.Type, &createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}

		// Parse created_at timestamp
		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		dep.CreatedAt = createdAt

		deps = append(deps, &dep)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating dependencies: %w", err)
	}

	return deps, nil
}
