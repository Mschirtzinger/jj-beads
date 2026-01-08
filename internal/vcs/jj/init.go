// Package jj provides a Jujutsu (jj) implementation of the vcs.VCS interface.
//
// This implementation leverages jj's unique features:
//   - Working-copy-as-commit model (no staging area)
//   - First-class conflict resolution
//   - Operation log for unlimited undo
//   - Changes and bookmarks for flexible workflows
//
// The implementation automatically registers itself with the VCS factory
// on import.
//
// Key implementation details:
//   - Uses jj's change model for workspace isolation
//   - Leverages operation log for undo support
//   - Handles both standalone jj repos and colocated (jj+git) repos
//   - Bookmarks map to branches in the VCS interface
//
// Usage:
//
//	import _ "github.com/steveyegge/beads/internal/vcs/jj" // Auto-registers via init()
//
//	v, err := vcs.Get()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// v is a jj implementation if in a jj repo
package jj

import "github.com/steveyegge/beads/internal/vcs"

// init registers the jj VCS implementation with the factory.
// This is called automatically when the package is imported.
func init() {
	vcs.Register(vcs.TypeJJ, func(path string) (vcs.VCS, error) {
		return New(path)
	})
}
