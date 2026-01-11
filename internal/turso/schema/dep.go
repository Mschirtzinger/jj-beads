// Package schema defines file-based JSON schemas for jj-turso storage.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/beads/internal/types"
)

// DepFile represents a single dependency stored in deps/*.json
// Filename convention: {from}--{type}--{to}.json
type DepFile struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks if the DepFile has valid field values
func (d *DepFile) Validate() error {
	if d.From == "" {
		return fmt.Errorf("from is required")
	}
	if d.To == "" {
		return fmt.Errorf("to is required")
	}
	if d.Type == "" {
		return fmt.Errorf("type is required")
	}

	// Validate dependency type using types package
	depType := types.DependencyType(d.Type)
	if !depType.IsValid() {
		return fmt.Errorf("invalid dependency type: %s (must be 1-50 characters)", d.Type)
	}

	if d.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}

	return nil
}

// ToFileName generates the filename for this dependency
// Format: {from}--{type}--{to}.json
func (d *DepFile) ToFileName() string {
	return fmt.Sprintf("%s--%s--%s.json", d.From, d.Type, d.To)
}

// FromFileName parses a dependency filename and returns the components
// Returns (from, type, to, error)
func FromFileName(filename string) (string, string, string, error) {
	// Remove .json extension
	name := strings.TrimSuffix(filename, ".json")

	// Split on --
	parts := strings.Split(name, "--")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid filename format: expected {from}--{type}--{to}.json, got %s", filename)
	}

	from := parts[0]
	typ := parts[1]
	to := parts[2]

	if from == "" || typ == "" || to == "" {
		return "", "", "", fmt.Errorf("invalid filename: from, type, and to cannot be empty")
	}

	return from, typ, to, nil
}

// ReadDepFile reads and validates a dependency file
func ReadDepFile(path string) (*DepFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read dep file: %w", err)
	}

	var dep DepFile
	if err := json.Unmarshal(data, &dep); err != nil {
		return nil, fmt.Errorf("failed to parse dep file: %w", err)
	}

	if err := dep.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dep file: %w", err)
	}

	return &dep, nil
}

// WriteDepFile writes a dependency file with validation
func WriteDepFile(dir string, dep *DepFile) error {
	if err := dep.Validate(); err != nil {
		return fmt.Errorf("invalid dependency: %w", err)
	}

	filename := dep.ToFileName()
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(dep, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal dep file: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write dep file: %w", err)
	}

	return nil
}

// ListDepsForIssue lists all dependency files involving a given issue ID
// Returns both dependencies (where issue is 'from') and dependents (where issue is 'to')
func ListDepsForIssue(depsDir string, issueID string) ([]*DepFile, error) {
	entries, err := os.ReadDir(depsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*DepFile{}, nil
		}
		return nil, fmt.Errorf("failed to read deps directory: %w", err)
	}

	var deps []*DepFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Parse filename to check if it involves this issue
		from, _, to, err := FromFileName(entry.Name())
		if err != nil {
			// Skip invalid filenames
			continue
		}

		// Include if issue is either from or to
		if from == issueID || to == issueID {
			path := filepath.Join(depsDir, entry.Name())
			dep, err := ReadDepFile(path)
			if err != nil {
				// Skip invalid files but continue processing
				continue
			}
			deps = append(deps, dep)
		}
	}

	return deps, nil
}

// ListAllDeps lists all dependency files in the directory
func ListAllDeps(depsDir string) ([]*DepFile, error) {
	entries, err := os.ReadDir(depsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*DepFile{}, nil
		}
		return nil, fmt.Errorf("failed to read deps directory: %w", err)
	}

	var deps []*DepFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(depsDir, entry.Name())
		dep, err := ReadDepFile(path)
		if err != nil {
			// Skip invalid files but continue processing
			continue
		}
		deps = append(deps, dep)
	}

	return deps, nil
}

// ToTypeDependency converts a DepFile to types.Dependency
func (d *DepFile) ToTypeDependency() *types.Dependency {
	return &types.Dependency{
		IssueID:     d.To,
		DependsOnID: d.From,
		Type:        types.DependencyType(d.Type),
		CreatedAt:   d.CreatedAt,
	}
}

// FromTypeDependency creates a DepFile from types.Dependency
func FromTypeDependency(dep *types.Dependency) *DepFile {
	return &DepFile{
		From:      dep.DependsOnID,
		To:        dep.IssueID,
		Type:      string(dep.Type),
		CreatedAt: dep.CreatedAt,
	}
}

// DeleteDepFile deletes a dependency file
func DeleteDepFile(depsDir string, from, typ, to string) error {
	filename := fmt.Sprintf("%s--%s--%s.json", from, typ, to)
	path := filepath.Join(depsDir, filename)

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, no error
		}
		return fmt.Errorf("failed to delete dep file: %w", err)
	}

	return nil
}
