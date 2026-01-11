# Turso Schema Package

This package provides data structures for the jj-turso architecture, which replaces beads' git-based sync with jj (Jujutsu) for version control and Turso (embedded libSQL) for query caching.

## Overview

The jj-turso architecture stores tasks as individual JSON files in `tasks/*.json`, with dependencies in `deps/*.json`. This CRDT-friendly design enables 100+ concurrent agents with sub-millisecond coordination.

## Task File Schema

### Structure

Tasks are stored as flat JSON files with the following structure:

```json
{
  "id": "bd-xyz",
  "title": "Implement feature X",
  "description": "...",
  "type": "task",
  "status": "in_progress",
  "priority": 1,
  "assigned_agent": "agent-47",
  "tags": ["backend", "api"],
  "created_at": "2026-01-10T07:36:29Z",
  "updated_at": "2026-01-10T08:15:00Z",
  "due_at": null,
  "defer_until": null
}
```

### CRDT-Friendly Design Principles

1. **Flat Structure**: No nested objects to avoid complex merge conflicts
2. **Last-Write-Wins**: Each field can be updated independently with timestamp-based conflict resolution
3. **Agent Ownership**: `assigned_agent` field identifies ownership
4. **Timestamps**: `created_at` and `updated_at` for conflict resolution

### Filename Convention

Tasks are stored as `{id}.json` in the `tasks/` directory.

Examples:
- `bd-xyz.json`
- `bd-abc.json`
- `bd-123.json`

## Go API

### TaskFile Struct

```go
type TaskFile struct {
    ID            string     `json:"id"`
    Title         string     `json:"title"`
    Description   string     `json:"description,omitempty"`
    Type          string     `json:"type"`
    Status        string     `json:"status"`
    Priority      int        `json:"priority"`
    AssignedAgent string     `json:"assigned_agent,omitempty"`
    Tags          []string   `json:"tags,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
    DueAt         *time.Time `json:"due_at,omitempty"`
    DeferUntil    *time.Time `json:"defer_until,omitempty"`
}
```

### Core Functions

#### Reading Tasks

```go
// Read a single task file
task, err := schema.ReadTaskFile("tasks/bd-xyz.json")

// Read all tasks from directory
tasks, err := schema.ReadAllTaskFiles("tasks/")
```

#### Writing Tasks

```go
task := &schema.TaskFile{
    ID:       "bd-new",
    Title:    "New task",
    Type:     "task",
    Status:   "open",
    Priority: 2,
}
task.SetDefaults() // Apply default values

err := schema.WriteTaskFile("tasks/", task)
```

#### Validation

```go
task := &schema.TaskFile{...}
if err := task.Validate(); err != nil {
    // Handle validation error
}
```

#### Conversion to/from types.Issue

```go
// Convert TaskFile to Issue (for compatibility with existing storage)
issue := taskFile.ToIssue()

// Convert Issue to TaskFile
taskFile := schema.FromIssue(issue)
```

### Validation Rules

- `id` is required
- `title` is required (max 500 characters)
- `priority` must be 0-4 (P0=critical, P4=backlog)
- `type` is required (bug, feature, task, epic, chore)
- `status` is required (open, in_progress, blocked, closed, etc.)
- `created_at` is required
- `updated_at` is required

### Helper Methods

```go
// Get canonical filename
filename := task.Filename() // Returns "bd-xyz.json"

// Set default values for optional fields
task.SetDefaults()

// Update timestamp to current time
task.UpdateTimestamp()
```

## Examples

See `examples/sample-task.json` for a complete example.

## Migration from types.Issue

The package provides bidirectional conversion between `TaskFile` and `types.Issue`:

```go
// From Issue to TaskFile (for migration)
issue := &types.Issue{...}
taskFile := schema.FromIssue(issue)
err := schema.WriteTaskFile("tasks/", taskFile)

// From TaskFile to Issue (for compatibility)
taskFile, err := schema.ReadTaskFile("tasks/bd-xyz.json")
issue := taskFile.ToIssue()
```

## Testing

Run tests:

```bash
go test ./internal/turso/schema/... -v
```

All core functionality is covered by tests:
- Validation
- File I/O (read/write)
- Conversion to/from types.Issue
- JSON serialization/deserialization
- Default value handling
- Timestamp management

## Design Rationale

### Why Flat JSON Files?

1. **CRDT-Friendly**: No nested objects means simpler conflict resolution
2. **Git/JJ Compatible**: Easy to diff and merge
3. **Human Readable**: Easy to inspect and debug
4. **Tool Friendly**: Standard JSON works with jq, grep, etc.

### Why Individual Files?

1. **Parallel Edits**: Multiple agents can modify different tasks without conflicts
2. **Fine-Grained Tracking**: JJ tracks changes per-file
3. **Performance**: Only load tasks you need
4. **Scalability**: 100+ agents can work concurrently

### Why Timestamps?

1. **Conflict Resolution**: Last-write-wins based on `updated_at`
2. **Audit Trail**: Track when changes occurred
3. **Ordering**: Consistent ordering across all agents

## Future Enhancements

- Vector embeddings for smart task routing
- CRDT conflict resolution algorithms
- Turso database sync daemon
- jj operation log integration
