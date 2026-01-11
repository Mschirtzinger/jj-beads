package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

func TestDepFile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		dep     DepFile
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid dependency",
			dep: DepFile{
				From:      "bd-abc",
				To:        "bd-xyz",
				Type:      "blocks",
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing from",
			dep: DepFile{
				To:        "bd-xyz",
				Type:      "blocks",
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing to",
			dep: DepFile{
				From:      "bd-abc",
				Type:      "blocks",
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing type",
			dep: DepFile{
				From:      "bd-abc",
				To:        "bd-xyz",
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid type - too long",
			dep: DepFile{
				From:      "bd-abc",
				To:        "bd-xyz",
				Type:      "this-is-a-very-long-type-name-that-exceeds-fifty-characters-limit",
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing created_at",
			dep: DepFile{
				From: "bd-abc",
				To:   "bd-xyz",
				Type: "blocks",
			},
			wantErr: true,
		},
		{
			name: "valid related dependency",
			dep: DepFile{
				From:      "bd-abc",
				To:        "bd-xyz",
				Type:      "related",
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid parent-child dependency",
			dep: DepFile{
				From:      "bd-abc",
				To:        "bd-xyz",
				Type:      "parent-child",
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid discovered-from dependency",
			dep: DepFile{
				From:      "bd-abc",
				To:        "bd-xyz",
				Type:      "discovered-from",
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dep.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestDepFile_ToFileName(t *testing.T) {
	tests := []struct {
		name string
		dep  DepFile
		want string
	}{
		{
			name: "blocks dependency",
			dep: DepFile{
				From: "bd-abc",
				To:   "bd-xyz",
				Type: "blocks",
			},
			want: "bd-abc--blocks--bd-xyz.json",
		},
		{
			name: "related dependency",
			dep: DepFile{
				From: "bd-123",
				To:   "bd-456",
				Type: "related",
			},
			want: "bd-123--related--bd-456.json",
		},
		{
			name: "parent-child dependency",
			dep: DepFile{
				From: "bd-parent",
				To:   "bd-child",
				Type: "parent-child",
			},
			want: "bd-parent--parent-child--bd-child.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dep.ToFileName()
			if got != tt.want {
				t.Errorf("ToFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromFileName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantFrom string
		wantType string
		wantTo   string
		wantErr  bool
	}{
		{
			name:     "valid blocks filename",
			filename: "bd-abc--blocks--bd-xyz.json",
			wantFrom: "bd-abc",
			wantType: "blocks",
			wantTo:   "bd-xyz",
			wantErr:  false,
		},
		{
			name:     "valid related filename",
			filename: "bd-123--related--bd-456.json",
			wantFrom: "bd-123",
			wantType: "related",
			wantTo:   "bd-456",
			wantErr:  false,
		},
		{
			name:     "valid parent-child filename",
			filename: "bd-parent--parent-child--bd-child.json",
			wantFrom: "bd-parent",
			wantType: "parent-child",
			wantTo:   "bd-child",
			wantErr:  false,
		},
		{
			name:     "without extension still parses",
			filename: "bd-abc--blocks--bd-xyz",
			wantFrom: "bd-abc",
			wantType: "blocks",
			wantTo:   "bd-xyz",
			wantErr:  false,
		},
		{
			name:     "invalid - too few parts",
			filename: "bd-abc--blocks.json",
			wantErr:  true,
		},
		{
			name:     "invalid - too many parts",
			filename: "bd-abc--blocks--bd-xyz--extra.json",
			wantErr:  true,
		},
		{
			name:     "invalid - empty from",
			filename: "--blocks--bd-xyz.json",
			wantErr:  true,
		},
		{
			name:     "invalid - empty type",
			filename: "bd-abc----bd-xyz.json",
			wantErr:  true,
		},
		{
			name:     "invalid - empty to",
			filename: "bd-abc--blocks--.json",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFrom, gotType, gotTo, err := FromFileName(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Errorf("FromFileName() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("FromFileName() unexpected error = %v", err)
				return
			}
			if gotFrom != tt.wantFrom {
				t.Errorf("FromFileName() from = %v, want %v", gotFrom, tt.wantFrom)
			}
			if gotType != tt.wantType {
				t.Errorf("FromFileName() type = %v, want %v", gotType, tt.wantType)
			}
			if gotTo != tt.wantTo {
				t.Errorf("FromFileName() to = %v, want %v", gotTo, tt.wantTo)
			}
		})
	}
}

func TestReadWriteDepFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	createdAt := time.Date(2026, 1, 10, 7, 36, 29, 0, time.UTC)

	dep := &DepFile{
		From:      "bd-abc",
		To:        "bd-xyz",
		Type:      "blocks",
		CreatedAt: createdAt,
	}

	// Test write
	if err := WriteDepFile(tmpDir, dep); err != nil {
		t.Fatalf("WriteDepFile() error = %v", err)
	}

	// Verify file exists
	expectedPath := filepath.Join(tmpDir, "bd-abc--blocks--bd-xyz.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("Expected file not created: %v", expectedPath)
	}

	// Test read
	readDep, err := ReadDepFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadDepFile() error = %v", err)
	}

	// Verify contents
	if readDep.From != dep.From {
		t.Errorf("From = %v, want %v", readDep.From, dep.From)
	}
	if readDep.To != dep.To {
		t.Errorf("To = %v, want %v", readDep.To, dep.To)
	}
	if readDep.Type != dep.Type {
		t.Errorf("Type = %v, want %v", readDep.Type, dep.Type)
	}
	if !readDep.CreatedAt.Equal(dep.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", readDep.CreatedAt, dep.CreatedAt)
	}

	// Verify JSON format
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if parsed["from"] != "bd-abc" {
		t.Errorf("JSON from = %v, want bd-abc", parsed["from"])
	}
	if parsed["to"] != "bd-xyz" {
		t.Errorf("JSON to = %v, want bd-xyz", parsed["to"])
	}
	if parsed["type"] != "blocks" {
		t.Errorf("JSON type = %v, want blocks", parsed["type"])
	}
}

func TestWriteDepFile_InvalidDep(t *testing.T) {
	tmpDir := t.TempDir()

	// Missing required field
	dep := &DepFile{
		From: "bd-abc",
		To:   "bd-xyz",
		// Missing Type
		CreatedAt: time.Now(),
	}

	err := WriteDepFile(tmpDir, dep)
	if err == nil {
		t.Error("WriteDepFile() expected error for invalid dep, got nil")
	}
}

func TestListDepsForIssue(t *testing.T) {
	tmpDir := t.TempDir()

	createdAt := time.Now()

	// Create test dependencies
	deps := []*DepFile{
		{From: "bd-abc", To: "bd-xyz", Type: "blocks", CreatedAt: createdAt},
		{From: "bd-xyz", To: "bd-123", Type: "related", CreatedAt: createdAt},
		{From: "bd-abc", To: "bd-456", Type: "parent-child", CreatedAt: createdAt},
		{From: "bd-other", To: "bd-another", Type: "blocks", CreatedAt: createdAt},
	}

	for _, dep := range deps {
		if err := WriteDepFile(tmpDir, dep); err != nil {
			t.Fatalf("WriteDepFile() error = %v", err)
		}
	}

	// List deps for bd-abc (should appear as 'from' in 2 deps)
	abcDeps, err := ListDepsForIssue(tmpDir, "bd-abc")
	if err != nil {
		t.Fatalf("ListDepsForIssue() error = %v", err)
	}
	if len(abcDeps) != 2 {
		t.Errorf("ListDepsForIssue(bd-abc) count = %v, want 2", len(abcDeps))
	}

	// List deps for bd-xyz (should appear as 'to' in 1 dep and 'from' in 1 dep)
	xyzDeps, err := ListDepsForIssue(tmpDir, "bd-xyz")
	if err != nil {
		t.Fatalf("ListDepsForIssue() error = %v", err)
	}
	if len(xyzDeps) != 2 {
		t.Errorf("ListDepsForIssue(bd-xyz) count = %v, want 2", len(xyzDeps))
	}

	// List deps for non-existent issue
	noneDeps, err := ListDepsForIssue(tmpDir, "bd-nonexistent")
	if err != nil {
		t.Fatalf("ListDepsForIssue() error = %v", err)
	}
	if len(noneDeps) != 0 {
		t.Errorf("ListDepsForIssue(bd-nonexistent) count = %v, want 0", len(noneDeps))
	}

	// List deps from non-existent directory
	emptyDeps, err := ListDepsForIssue("/nonexistent/path", "bd-abc")
	if err != nil {
		t.Fatalf("ListDepsForIssue() error = %v", err)
	}
	if len(emptyDeps) != 0 {
		t.Errorf("ListDepsForIssue(nonexistent dir) count = %v, want 0", len(emptyDeps))
	}
}

func TestListAllDeps(t *testing.T) {
	tmpDir := t.TempDir()

	createdAt := time.Now()

	// Create test dependencies
	deps := []*DepFile{
		{From: "bd-abc", To: "bd-xyz", Type: "blocks", CreatedAt: createdAt},
		{From: "bd-xyz", To: "bd-123", Type: "related", CreatedAt: createdAt},
		{From: "bd-abc", To: "bd-456", Type: "parent-child", CreatedAt: createdAt},
	}

	for _, dep := range deps {
		if err := WriteDepFile(tmpDir, dep); err != nil {
			t.Fatalf("WriteDepFile() error = %v", err)
		}
	}

	// List all deps
	allDeps, err := ListAllDeps(tmpDir)
	if err != nil {
		t.Fatalf("ListAllDeps() error = %v", err)
	}
	if len(allDeps) != 3 {
		t.Errorf("ListAllDeps() count = %v, want 3", len(allDeps))
	}

	// List from empty directory
	emptyDir := t.TempDir()
	emptyDeps, err := ListAllDeps(emptyDir)
	if err != nil {
		t.Fatalf("ListAllDeps() error = %v", err)
	}
	if len(emptyDeps) != 0 {
		t.Errorf("ListAllDeps(empty) count = %v, want 0", len(emptyDeps))
	}

	// List from non-existent directory
	noneDeps, err := ListAllDeps("/nonexistent/path")
	if err != nil {
		t.Fatalf("ListAllDeps() error = %v", err)
	}
	if len(noneDeps) != 0 {
		t.Errorf("ListAllDeps(nonexistent) count = %v, want 0", len(noneDeps))
	}
}

func TestToTypeDependency(t *testing.T) {
	createdAt := time.Date(2026, 1, 10, 7, 36, 29, 0, time.UTC)

	depFile := &DepFile{
		From:      "bd-abc",
		To:        "bd-xyz",
		Type:      "blocks",
		CreatedAt: createdAt,
	}

	typeDep := depFile.ToTypeDependency()

	if typeDep.IssueID != "bd-xyz" {
		t.Errorf("IssueID = %v, want bd-xyz", typeDep.IssueID)
	}
	if typeDep.DependsOnID != "bd-abc" {
		t.Errorf("DependsOnID = %v, want bd-abc", typeDep.DependsOnID)
	}
	if typeDep.Type != types.DepBlocks {
		t.Errorf("Type = %v, want %v", typeDep.Type, types.DepBlocks)
	}
	if !typeDep.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", typeDep.CreatedAt, createdAt)
	}
}

func TestFromTypeDependency(t *testing.T) {
	createdAt := time.Date(2026, 1, 10, 7, 36, 29, 0, time.UTC)

	typeDep := &types.Dependency{
		IssueID:     "bd-xyz",
		DependsOnID: "bd-abc",
		Type:        types.DepBlocks,
		CreatedAt:   createdAt,
	}

	depFile := FromTypeDependency(typeDep)

	if depFile.From != "bd-abc" {
		t.Errorf("From = %v, want bd-abc", depFile.From)
	}
	if depFile.To != "bd-xyz" {
		t.Errorf("To = %v, want bd-xyz", depFile.To)
	}
	if depFile.Type != "blocks" {
		t.Errorf("Type = %v, want blocks", depFile.Type)
	}
	if !depFile.CreatedAt.Equal(createdAt) {
		t.Errorf("CreatedAt = %v, want %v", depFile.CreatedAt, createdAt)
	}
}

func TestRoundTripConversion(t *testing.T) {
	createdAt := time.Date(2026, 1, 10, 7, 36, 29, 0, time.UTC)

	// Start with DepFile
	original := &DepFile{
		From:      "bd-abc",
		To:        "bd-xyz",
		Type:      "blocks",
		CreatedAt: createdAt,
	}

	// Convert to types.Dependency
	typeDep := original.ToTypeDependency()

	// Convert back to DepFile
	converted := FromTypeDependency(typeDep)

	// Verify round-trip
	if converted.From != original.From {
		t.Errorf("From = %v, want %v", converted.From, original.From)
	}
	if converted.To != original.To {
		t.Errorf("To = %v, want %v", converted.To, original.To)
	}
	if converted.Type != original.Type {
		t.Errorf("Type = %v, want %v", converted.Type, original.Type)
	}
	if !converted.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", converted.CreatedAt, original.CreatedAt)
	}
}

func TestDeleteDepFile(t *testing.T) {
	tmpDir := t.TempDir()

	createdAt := time.Now()

	dep := &DepFile{
		From:      "bd-abc",
		To:        "bd-xyz",
		Type:      "blocks",
		CreatedAt: createdAt,
	}

	// Create the file
	if err := WriteDepFile(tmpDir, dep); err != nil {
		t.Fatalf("WriteDepFile() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "bd-abc--blocks--bd-xyz.json")

	// Verify file exists
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("File was not created")
	}

	// Delete the file
	if err := DeleteDepFile(tmpDir, "bd-abc", "blocks", "bd-xyz"); err != nil {
		t.Fatalf("DeleteDepFile() error = %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Error("File still exists after deletion")
	}

	// Delete again (should not error)
	if err := DeleteDepFile(tmpDir, "bd-abc", "blocks", "bd-xyz"); err != nil {
		t.Errorf("DeleteDepFile() on non-existent file should not error, got: %v", err)
	}
}

func TestListDepsForIssue_SkipsInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()

	createdAt := time.Now()

	// Create valid dependency
	validDep := &DepFile{
		From:      "bd-abc",
		To:        "bd-xyz",
		Type:      "blocks",
		CreatedAt: createdAt,
	}
	if err := WriteDepFile(tmpDir, validDep); err != nil {
		t.Fatalf("WriteDepFile() error = %v", err)
	}

	// Create invalid files that should be skipped
	invalidFiles := []string{
		"invalid-format.json",           // Wrong format
		"bd-abc--blocks.json",           // Missing part
		"not-json.txt",                  // Not JSON
		"bd-abc--blocks--bd-xyz--extra.json", // Too many parts
	}

	for _, filename := range invalidFiles {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte("invalid"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// List deps - should only find the valid one
	deps, err := ListDepsForIssue(tmpDir, "bd-abc")
	if err != nil {
		t.Fatalf("ListDepsForIssue() error = %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("ListDepsForIssue() count = %v, want 1 (should skip invalid files)", len(deps))
	}
}
