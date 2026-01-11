package daemon

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestParseOpLog(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []OpLogEntry
		wantErr bool
	}{
		{
			name: "single operation",
			input: `abc123def456abc123def456abc123def456abc123def456abc123def456abc123de
snapshot working copy
---
`,
			want: []OpLogEntry{
				{
					ID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc123de",
					Description: "snapshot working copy",
				},
			},
			wantErr: false,
		},
		{
			name: "multiple operations",
			input: `abc123def456abc123def456abc123def456abc123def456abc123def456abc123de
snapshot working copy
---
def789abc012def789abc012def789abc012def789abc012def789abc012def789ab
rebase
---
ghi345jkl678ghi345jkl678ghi345jkl678ghi345jkl678ghi345jkl678ghi345jk
new empty commit
---
`,
			want: []OpLogEntry{
				{
					ID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc123de",
					Description: "snapshot working copy",
				},
				{
					ID:          "def789abc012def789abc012def789abc012def789abc012def789abc012def789ab",
					Description: "rebase",
				},
				{
					ID:          "ghi345jkl678ghi345jkl678ghi345jkl678ghi345jkl678ghi345jkl678ghi345jk",
					Description: "new empty commit",
				},
			},
			wantErr: false,
		},
		{
			name: "operation with long description",
			input: `abc123def456abc123def456abc123def456abc123def456abc123def456abc123de
restore to operation abc123
---
`,
			want: []OpLogEntry{
				{
					ID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc123de",
					Description: "restore to operation abc123",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			want:    nil,
			wantErr: false,
		},
		{
			name: "missing separator at end",
			input: `abc123def456abc123def456abc123def456abc123def456abc123def456abc123de
snapshot working copy`,
			want: []OpLogEntry{
				{
					ID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc123de",
					Description: "snapshot working copy",
				},
			},
			wantErr: false,
		},
		{
			name: "whitespace handling",
			input: `  abc123def456abc123def456abc123def456abc123def456abc123def456abc123de
  snapshot working copy
---
`,
			want: []OpLogEntry{
				{
					ID:          "abc123def456abc123def456abc123def456abc123def456abc123def456abc123de",
					Description: "snapshot working copy",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOpLog([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOpLog() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseOpLog() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAffectedFiles(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tasksDir string
		depsDir  string
		want     []string
	}{
		{
			name: "task file added",
			input: `Changed commits:
+ povwtowv 5109f3f8 (no description set)
- povwtowv hidden 57dfc47c (no description set)
Added regular file tasks/bd-123.json:
        1: {
        2:   "id": "bd-123",
        3:   "title": "Test task"
        4: }
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     []string{"tasks/bd-123.json"},
		},
		{
			name: "dep file modified",
			input: `Changed commits:
Modified regular file deps/bd-abc--blocks--bd-xyz.json:
       1:   1: {
       2:   2:   "from": "bd-abc",
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     []string{"deps/bd-abc--blocks--bd-xyz.json"},
		},
		{
			name: "multiple files changed",
			input: `Changed commits:
Added regular file tasks/bd-123.json:
        1: {"id": "bd-123"}
Modified regular file tasks/bd-456.json:
       1: {"id": "bd-456"}
Added regular file deps/bd-123--blocks--bd-456.json:
        1: {"from": "bd-123"}
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want: []string{
				"tasks/bd-123.json",
				"tasks/bd-456.json",
				"deps/bd-123--blocks--bd-456.json",
			},
		},
		{
			name: "removed file",
			input: `Changed commits:
Removed regular file tasks/bd-789.json:
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     []string{"tasks/bd-789.json"},
		},
		{
			name: "non-task/dep files ignored",
			input: `Changed commits:
Added regular file README.md:
Modified regular file src/main.go:
Added regular file tasks/bd-123.json:
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     []string{"tasks/bd-123.json"},
		},
		{
			name: "non-json files ignored",
			input: `Changed commits:
Added regular file tasks/temp.txt:
Added regular file tasks/bd-123.json:
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     []string{"tasks/bd-123.json"},
		},
		{
			name:     "no matching files",
			input:    `Changed commits:\nAdded regular file src/main.go:\n`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     nil,
		},
		{
			name: "duplicate files",
			input: `Changed commits:
Added regular file tasks/bd-123.json:
Modified regular file tasks/bd-123.json:
`,
			tasksDir: "tasks",
			depsDir:  "deps",
			want:     []string{"tasks/bd-123.json"}, // Should deduplicate
		},
		{
			name: "custom directories",
			input: `Changed commits:
Added regular file mytasks/bd-123.json:
Added regular file mydeps/bd-abc--blocks--bd-xyz.json:
`,
			tasksDir: "mytasks",
			depsDir:  "mydeps",
			want: []string{
				"mytasks/bd-123.json",
				"mydeps/bd-abc--blocks--bd-xyz.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAffectedFiles([]byte(tt.input), tt.tasksDir, tt.depsDir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAffectedFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindNewOperations(t *testing.T) {
	entries := []OpLogEntry{
		{ID: "op5", Description: "newest"},
		{ID: "op4", Description: "newer"},
		{ID: "op3", Description: "middle"},
		{ID: "op2", Description: "older"},
		{ID: "op1", Description: "oldest"},
	}

	tests := []struct {
		name       string
		entries    []OpLogEntry
		lastSeenID string
		want       []OpLogEntry
	}{
		{
			name:       "first run - no last seen",
			entries:    entries,
			lastSeenID: "",
			want:       []OpLogEntry{{ID: "op5", Description: "newest"}},
		},
		{
			name:       "two new operations",
			entries:    entries,
			lastSeenID: "op3",
			want: []OpLogEntry{
				{ID: "op5", Description: "newest"},
				{ID: "op4", Description: "newer"},
			},
		},
		{
			name:       "no new operations",
			entries:    entries,
			lastSeenID: "op5",
			want:       nil,
		},
		{
			name:       "all operations are new",
			entries:    entries,
			lastSeenID: "op-unknown",
			want:       []OpLogEntry{{ID: "op5", Description: "newest"}}, // Returns only most recent
		},
		{
			name:       "empty entries",
			entries:    []OpLogEntry{},
			lastSeenID: "op1",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findNewOperations(tt.entries, tt.lastSeenID)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findNewOperations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReverseSlice(t *testing.T) {
	tests := []struct {
		name  string
		input []OpLogEntry
		want  []OpLogEntry
	}{
		{
			name: "odd number of elements",
			input: []OpLogEntry{
				{ID: "1"},
				{ID: "2"},
				{ID: "3"},
			},
			want: []OpLogEntry{
				{ID: "3"},
				{ID: "2"},
				{ID: "1"},
			},
		},
		{
			name: "even number of elements",
			input: []OpLogEntry{
				{ID: "1"},
				{ID: "2"},
				{ID: "3"},
				{ID: "4"},
			},
			want: []OpLogEntry{
				{ID: "4"},
				{ID: "3"},
				{ID: "2"},
				{ID: "1"},
			},
		},
		{
			name:  "empty slice",
			input: []OpLogEntry{},
			want:  []OpLogEntry{},
		},
		{
			name:  "single element",
			input: []OpLogEntry{{ID: "1"}},
			want:  []OpLogEntry{{ID: "1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the test data
			got := make([]OpLogEntry, len(tt.input))
			copy(got, tt.input)

			reverseSlice(got)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reverseSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWatchOpLogIntegration tests the WatchOpLog function with a mock callback.
// This is an integration-style test that doesn't actually run jj commands.
func TestWatchOpLogIntegration(t *testing.T) {
	// This test would require mocking exec.Command, which is complex.
	// For now, we'll skip it and rely on the unit tests above.
	// In a real implementation, you'd use dependency injection or
	// an interface to make the command execution mockable.
	t.Skip("Integration test requires jj repository")
}

// Example showing how to use the operation log parser
func ExampleParseOpLog() {
	output := `c87242843bda95cb4c9904b8601624e1e7162f19dc14aef0c02eddbd46943388
snapshot working copy
---
f44e864825775d714ce1ec8f487f53b8c201c81f8349a19577865a3f1bc0926f
rebase
---
`

	entries, err := ParseOpLog([]byte(output))
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		fmt.Printf("Operation: %s... - %s\n", entry.ID[:12], entry.Description)
	}

	// Output:
	// Operation: c87242843bda... - snapshot working copy
	// Operation: f44e86482577... - rebase
}

// Example showing how to watch for operations
func ExampleWatchOpLog() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config := OpLogWatcherConfig{
		RepoPath:     ".",
		PollInterval: 100 * time.Millisecond,
		TasksDir:     "tasks",
		DepsDir:      "deps",
	}

	// This would run until context is cancelled
	_ = WatchOpLog(ctx, config, func(entries []OpLogEntry) error {
		for _, entry := range entries {
			fmt.Printf("New operation: %s\n", entry.Description)
			for _, file := range entry.AffectedFiles {
				fmt.Printf("  Changed: %s\n", file)
			}
		}
		return nil
	})
}

// Benchmark for ParseOpLog with realistic operation count
func BenchmarkParseOpLog(b *testing.B) {
	// Generate realistic op log output with 50 operations
	output := ""
	for i := 0; i < 50; i++ {
		output += "abc123def456abc123def456abc123def456abc123def456abc123def456abc123de\n"
		output += "snapshot working copy\n"
		output += "---\n"
	}

	data := []byte(output)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseOpLog(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark for parseAffectedFiles
func BenchmarkParseAffectedFiles(b *testing.B) {
	input := `Changed commits:
+ povwtowv 5109f3f8 (no description set)
- povwtowv hidden 57dfc47c (no description set)
Added regular file tasks/bd-123.json:
        1: {"id": "bd-123"}
Modified regular file tasks/bd-456.json:
        1: {"id": "bd-456"}
Added regular file deps/bd-123--blocks--bd-456.json:
        1: {"from": "bd-123"}
Modified regular file src/main.go:
        1: package main
`

	data := []byte(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseAffectedFiles(data, "tasks", "deps")
	}
}

// Test GetLatestOperationID would require a jj repository
func TestGetLatestOperationID(t *testing.T) {
	t.Skip("Requires jj repository")

	// Example test when jj is available:
	// ctx := context.Background()
	// id, err := GetLatestOperationID(ctx, ".")
	// if err != nil {
	//     t.Fatal(err)
	// }
	// if len(id) != 128 { // jj operation IDs are 64 bytes = 128 hex chars
	//     t.Errorf("Invalid operation ID length: %d", len(id))
	// }
}
