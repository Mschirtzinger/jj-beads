// Package git provides a git implementation of the vcs.VCS interface.
//
// This implementation uses git's native commands and worktrees for
// workspace isolation. It automatically registers itself with the
// VCS factory on import.
//
// The implementation follows these design principles:
//   - Use git worktrees for isolated sync branch operations
//   - Minimize git command invocations for performance
//   - Cache repository metadata where safe
//   - Handle both regular repos and worktrees transparently
//
// Usage:
//
//	import _ "github.com/steveyegge/beads/internal/vcs/git" // Auto-registers via init()
//
//	v, err := vcs.Get()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// v is a git implementation if in a git repo
package git

import "github.com/steveyegge/beads/internal/vcs"

// init registers the git VCS implementation with the factory.
// This is called automatically when the package is imported.
func init() {
	vcs.Register(vcs.TypeGit, func(path string) (vcs.VCS, error) {
		return New(path)
	})
}
