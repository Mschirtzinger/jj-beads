# Task File JSON Schema Implementation

**Status:** Complete ✅
**Date:** 2026-01-10
**Package:** `github.com/steveyegge/beads/internal/turso/schema`

## Deliverables

### 1. Core Implementation (`task.go`)

**Location:** `/Users/mike/dev/jj-beads/internal/turso/schema/task.go`

**Key Components:**

- `TaskFile` struct - CRDT-friendly flat JSON schema
- `Validate()` - Comprehensive validation with clear error messages
- `Filename()` - Canonical filename generation ({id}.json)
- `ToIssue()` - Convert to types.Issue for compatibility
- `FromIssue()` - Convert from types.Issue for migration
- `ReadTaskFile()` - Read and parse single task file
- `WriteTaskFile()` - Write task to disk with pretty JSON
- `ReadAllTaskFiles()` - Read entire tasks directory
- `SetDefaults()` - Apply default values for optional fields
- `UpdateTimestamp()` - Update modified timestamp

### 2. Comprehensive Tests (`task_test.go`)

**Location:** `/Users/mike/dev/jj-beads/internal/turso/schema/task_test.go`

**Test Coverage:**

✅ Validation (all edge cases)
✅ File I/O (read/write operations)
✅ Conversion to/from types.Issue
✅ JSON serialization/deserialization
✅ Default value handling
✅ Timestamp management
✅ Error handling (invalid JSON, missing files)
✅ Round-trip conversions

**Test Results:**
```
PASS: TestTaskFile_Validate (12 sub-tests)
PASS: TestTaskFile_Filename
PASS: TestTaskFile_SetDefaults
PASS: TestTaskFile_UpdateTimestamp
PASS: TestTaskFile_ToIssue
PASS: TestFromIssue
PASS: TestWriteTaskFile
PASS: TestReadTaskFile
PASS: TestReadTaskFile_InvalidJSON
PASS: TestReadAllTaskFiles
PASS: TestReadAllTaskFiles_EmptyDirectory
PASS: TestReadAllTaskFiles_NonexistentDirectory
PASS: TestJSONRoundTrip
```

All tests passing ✅

### 3. Documentation

- **README.md** - Complete API documentation and usage guide
- **examples/sample-task.json** - Working example demonstrating schema
- **IMPLEMENTATION.md** - This file

## JSON Schema

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

## CRDT-Friendly Design

✅ **Flat structure** - No nested objects
✅ **Last-write-wins** - Per-field updates with timestamps
✅ **Agent ownership** - `assigned_agent` field
✅ **Timestamps** - For conflict resolution

## Filename Convention

Tasks stored as `{id}.json` in `tasks/` directory:
- `tasks/bd-xyz.json`
- `tasks/bd-abc.json`
- `tasks/bd-123.json`

## Validation Rules

| Field | Requirement |
|-------|-------------|
| `id` | Required |
| `title` | Required, max 500 chars |
| `type` | Required (bug, feature, task, epic, chore) |
| `status` | Required (open, in_progress, blocked, closed, etc.) |
| `priority` | 0-4 (P0=critical, P4=backlog) |
| `created_at` | Required |
| `updated_at` | Required |
| `assigned_agent` | Optional |
| `tags` | Optional array |
| `description` | Optional |
| `due_at` | Optional timestamp |
| `defer_until` | Optional timestamp |

## Usage Examples

### Reading Tasks

```go
// Read single task
task, err := schema.ReadTaskFile("tasks/bd-xyz.json")
if err != nil {
    log.Fatal(err)
}

// Read all tasks
tasks, err := schema.ReadAllTaskFiles("tasks/")
if err != nil {
    log.Fatal(err)
}
```

### Writing Tasks

```go
task := &schema.TaskFile{
    ID:       "bd-new",
    Title:    "New task",
    Type:     "task",
    Status:   "open",
    Priority: 2,
}
task.SetDefaults()

err := schema.WriteTaskFile("tasks/", task)
```

### Validation

```go
if err := task.Validate(); err != nil {
    fmt.Fprintf(os.Stderr, "Invalid task: %v\n", err)
}
```

### Conversion

```go
// types.Issue → TaskFile
taskFile := schema.FromIssue(issue)

// TaskFile → types.Issue
issue := taskFile.ToIssue()
```

## Integration with Existing Code

The package provides bidirectional conversion between `TaskFile` and `types.Issue`:

1. **Migration path**: Convert existing issues to task files using `FromIssue()`
2. **Compatibility**: Convert task files back to issues using `ToIssue()`
3. **No breaking changes**: Existing storage layer continues to work

## What Was NOT Modified

✅ Did NOT modify existing storage layer
✅ Did NOT add nested objects
✅ Did NOT use external validation libraries
✅ Did NOT break existing code

## Next Steps (from jj-turso plan)

1. ✅ **DONE**: Create task file JSON schema
2. **TODO**: Create dependency file schema (`deps/*.json`)
3. **TODO**: Implement Turso database schema
4. **TODO**: Build sync daemon for jj operation log
5. **TODO**: Update CLI commands to use new format

## Build Verification

```bash
# Build package
go build ./internal/turso/schema/...

# Run tests
go test ./internal/turso/schema/... -v

# Check coverage
go test -coverprofile=coverage.out ./internal/turso/schema/
go tool cover -html=coverage.out
```

All builds successful ✅

## Files Created

```
/Users/mike/dev/jj-beads/internal/turso/schema/
├── task.go                          # Core implementation
├── task_test.go                     # Comprehensive tests
├── README.md                        # API documentation
├── IMPLEMENTATION.md                # This file
└── examples/
    └── sample-task.json             # Working example
```

## Success Criteria Met

✅ Created `internal/turso/schema/task.go` with TaskFile struct
✅ JSON validation function implemented
✅ Read/write helpers for `tasks/*.json` files
✅ Filename convention: `{id}.json`
✅ Comprehensive test suite in `task_test.go`
✅ All tests passing
✅ TaskFile convertible to/from Issue
✅ No modifications to existing storage layer
✅ No nested objects (CRDT-friendly)
✅ No external validation libraries

**Implementation complete and tested!** 🎉
