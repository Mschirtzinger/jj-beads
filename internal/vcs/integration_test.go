package vcs_test

import (
	"testing"

	"github.com/steveyegge/beads/internal/vcs"
	// Import implementations to trigger auto-registration
	_ "github.com/steveyegge/beads/internal/vcs/git"
	_ "github.com/steveyegge/beads/internal/vcs/jj"
)

// TestRegistrationIntegration verifies that git and jj implementations
// are properly registered with the factory via their init() functions.
func TestRegistrationIntegration(t *testing.T) {
	// Verify git is registered
	if !vcs.IsRegistered(vcs.TypeGit) {
		t.Error("Expected git to be auto-registered")
	}

	// Verify jj is registered
	if !vcs.IsRegistered(vcs.TypeJJ) {
		t.Error("Expected jj to be auto-registered")
	}

	// Verify we can see both types
	types := vcs.RegisteredTypes()
	if len(types) < 2 {
		t.Errorf("Expected at least 2 registered types, got %d: %v", len(types), types)
	}

	// Verify the types include git and jj
	hasGit := false
	hasJJ := false
	for _, typ := range types {
		if typ == vcs.TypeGit {
			hasGit = true
		}
		if typ == vcs.TypeJJ {
			hasJJ = true
		}
	}

	if !hasGit {
		t.Error("Expected TypeGit in registered types")
	}
	if !hasJJ {
		t.Error("Expected TypeJJ in registered types")
	}
}
