// Package agent provides functions for managing agent bookmarks in Jujutsu.
//
// This package implements the agent lifecycle defined in docs/jj-agent-conventions.md:
//   - Spawn: Create new agent bookmark from main
//   - Work: Agent modifies files (auto-tracked by jj)
//   - Handoff: Transfer work between agents
//   - Complete: Merge agent work back to main
//   - Recover: Restore agent state from operation log
//
// All operations use the VCS interface from internal/vcs for portability.
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/steveyegge/beads/internal/vcs"
)

const (
	// AgentBookmarkPrefix is the required prefix for agent bookmarks
	AgentBookmarkPrefix = "agent-"

	// MainBookmark is the stable integration point
	MainBookmark = "main"

	// StagingBookmark is the optional pre-merge validation bookmark
	StagingBookmark = "staging"

	// ArchiveBookmarkPrefix is used for archived agent bookmarks
	ArchiveBookmarkPrefix = "archive/"
)

// Agent represents an active agent with a bookmark
type Agent struct {
	// ID is the agent identifier (e.g., "agent-47")
	ID string

	// Bookmark is the jj bookmark name (same as ID)
	Bookmark string

	// BasedOn is the bookmark this agent was spawned from (usually "main")
	BasedOn string

	// CreatedAt is when the agent was spawned
	CreatedAt time.Time

	// VCS is the version control system interface
	vcs vcs.VCS
}

// SpawnOptions configures agent spawning
type SpawnOptions struct {
	// AgentID is the agent identifier (e.g., "47", "orchestrator")
	// Will be prefixed with "agent-" if not already present
	AgentID string

	// BaseBranch is the bookmark to spawn from (default: "main")
	BaseBranch string

	// Description is the initial change description (optional)
	Description string
}

// HandoffOptions configures agent handoff
type HandoffOptions struct {
	// FromAgentID is the current agent
	FromAgentID string

	// ToAgentID is the new agent (will be created)
	ToAgentID string

	// Reason describes why the handoff is happening
	Reason string

	// ArchiveOld archives the old agent bookmark instead of deleting it
	ArchiveOld bool
}

// CompleteOptions configures agent completion
type CompleteOptions struct {
	// AgentID is the agent to complete
	AgentID string

	// TargetBookmark is where to merge the work (default: "main")
	TargetBookmark string

	// DeleteBookmark removes the agent bookmark after merge (default: true)
	DeleteBookmark bool

	// ArchiveBookmark archives the agent bookmark instead of deleting
	ArchiveBookmark bool
}

// RecoverOptions configures agent recovery
type RecoverOptions struct {
	// AgentID is the agent to recover
	AgentID string

	// RecoverToID is the new agent ID for recovered state (default: original + "-recovered")
	RecoverToID string

	// OperationID is the specific operation to recover from (optional)
	// If empty, uses the most recent operation for this agent
	OperationID string
}

// BookmarkStatus represents the status of an agent bookmark
type BookmarkStatus struct {
	// AgentID is the agent identifier
	AgentID string

	// Bookmark is the bookmark name
	Bookmark string

	// Exists is true if the bookmark exists
	Exists bool

	// ChangeID is the current change ID (if exists)
	ChangeID string

	// HasChanges indicates if there are uncommitted changes
	HasChanges bool

	// IsArchived indicates if this is an archived bookmark
	IsArchived bool
}

// Spawn creates a new agent bookmark from the base branch.
//
// Example:
//
//	agent, err := agent.Spawn(ctx, v, agent.SpawnOptions{
//	    AgentID: "47",
//	    BaseBranch: "main",
//	    Description: "Work on bd-123",
//	})
func Spawn(ctx context.Context, v vcs.VCS, opts SpawnOptions) (*Agent, error) {
	// Normalize agent ID
	agentID := normalizeAgentID(opts.AgentID)

	// Default base branch
	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		baseBranch = MainBookmark
	}

	// Check if agent bookmark already exists
	if v.RefExists(agentID) {
		return nil, fmt.Errorf("agent bookmark %s already exists", agentID)
	}

	// Ensure base branch exists
	if !v.RefExists(baseBranch) {
		return nil, fmt.Errorf("base bookmark %s does not exist", baseBranch)
	}

	// Create new change from base branch
	// jj new {base}
	args := []string{"new", baseBranch}
	if opts.Description != "" {
		args = append(args, "-m", opts.Description)
	}

	if _, err := v.Exec(ctx, args...); err != nil {
		return nil, fmt.Errorf("failed to create new change: %w", err)
	}

	// Create bookmark pointing to the new change
	// jj bookmark create {agent-id}
	if _, err := v.Exec(ctx, "bookmark", "create", agentID); err != nil {
		return nil, fmt.Errorf("failed to create agent bookmark: %w", err)
	}

	return &Agent{
		ID:        agentID,
		Bookmark:  agentID,
		BasedOn:   baseBranch,
		CreatedAt: time.Now(),
		vcs:       v,
	}, nil
}

// Handoff transfers work from one agent to another.
//
// This creates a new agent bookmark based on the current agent's state,
// optionally archiving the old agent's bookmark.
//
// Example:
//
//	newAgent, err := agent.Handoff(ctx, v, agent.HandoffOptions{
//	    FromAgentID: "agent-47",
//	    ToAgentID: "agent-48",
//	    Reason: "context limit reached",
//	    ArchiveOld: true,
//	})
func Handoff(ctx context.Context, v vcs.VCS, opts HandoffOptions) (*Agent, error) {
	fromAgent := normalizeAgentID(opts.FromAgentID)
	toAgent := normalizeAgentID(opts.ToAgentID)

	// Verify source agent exists
	if !v.RefExists(fromAgent) {
		return nil, fmt.Errorf("source agent bookmark %s does not exist", fromAgent)
	}

	// Verify target agent doesn't exist
	if v.RefExists(toAgent) {
		return nil, fmt.Errorf("target agent bookmark %s already exists", toAgent)
	}

	// Create new agent from current agent's state
	// jj new {from-agent}
	description := fmt.Sprintf("Handoff from %s: %s", fromAgent, opts.Reason)
	args := []string{"new", fromAgent, "-m", description}

	if _, err := v.Exec(ctx, args...); err != nil {
		return nil, fmt.Errorf("failed to create new change for handoff: %w", err)
	}

	// Create bookmark for the new agent
	// jj bookmark create {to-agent}
	if _, err := v.Exec(ctx, "bookmark", "create", toAgent); err != nil {
		return nil, fmt.Errorf("failed to create handoff bookmark: %w", err)
	}

	// Handle old agent bookmark
	if opts.ArchiveOld {
		archiveName := ArchiveBookmarkPrefix + fromAgent
		if err := v.CreateRef(archiveName, fromAgent); err != nil {
			return nil, fmt.Errorf("failed to archive old bookmark: %w", err)
		}
		if err := v.DeleteRef(fromAgent); err != nil {
			return nil, fmt.Errorf("failed to delete old bookmark: %w", err)
		}
	}

	return &Agent{
		ID:        toAgent,
		Bookmark:  toAgent,
		BasedOn:   fromAgent,
		CreatedAt: time.Now(),
		vcs:       v,
	}, nil
}

// Complete merges the agent's work back to the target bookmark (usually main).
//
// This rebases the agent's changes onto the target, moves the target bookmark
// to include the agent's work, and optionally cleans up the agent bookmark.
//
// Example:
//
//	err := agent.Complete(ctx, v, agent.CompleteOptions{
//	    AgentID: "agent-47",
//	    TargetBookmark: "main",
//	    DeleteBookmark: true,
//	})
func Complete(ctx context.Context, v vcs.VCS, opts CompleteOptions) error {
	agentID := normalizeAgentID(opts.AgentID)

	// Default target
	targetBookmark := opts.TargetBookmark
	if targetBookmark == "" {
		targetBookmark = MainBookmark
	}

	// Verify agent exists
	if !v.RefExists(agentID) {
		return fmt.Errorf("agent bookmark %s does not exist", agentID)
	}

	// Verify target exists
	if !v.RefExists(targetBookmark) {
		return fmt.Errorf("target bookmark %s does not exist", targetBookmark)
	}

	// Rebase agent's work onto target
	// jj rebase -b {agent} -d {target}
	if _, err := v.Exec(ctx, "rebase", "-b", agentID, "-d", targetBookmark); err != nil {
		return fmt.Errorf("failed to rebase agent work: %w", err)
	}

	// Move target bookmark to include agent's work
	// jj bookmark set {target} -r {agent}
	if err := v.MoveRef(targetBookmark, agentID); err != nil {
		return fmt.Errorf("failed to move target bookmark: %w", err)
	}

	// Handle agent bookmark cleanup
	if opts.DeleteBookmark && !opts.ArchiveBookmark {
		// Delete agent bookmark
		if err := v.DeleteRef(agentID); err != nil {
			return fmt.Errorf("failed to delete agent bookmark: %w", err)
		}
	} else if opts.ArchiveBookmark {
		// Archive agent bookmark
		archiveName := ArchiveBookmarkPrefix + agentID
		if err := v.CreateRef(archiveName, agentID); err != nil {
			return fmt.Errorf("failed to archive agent bookmark: %w", err)
		}
		if err := v.DeleteRef(agentID); err != nil {
			return fmt.Errorf("failed to delete agent bookmark after archive: %w", err)
		}
	}

	return nil
}

// Status checks the current status of an agent bookmark.
//
// Example:
//
//	status, err := agent.Status(ctx, v, "agent-47")
func Status(ctx context.Context, v vcs.VCS, agentID string) (*BookmarkStatus, error) {
	agentID = normalizeAgentID(agentID)

	status := &BookmarkStatus{
		AgentID:    agentID,
		Bookmark:   agentID,
		Exists:     v.RefExists(agentID),
		IsArchived: strings.HasPrefix(agentID, ArchiveBookmarkPrefix),
	}

	if !status.Exists {
		return status, nil
	}

	// Get change ID
	hash, err := v.GetCommitHash(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %w", err)
	}
	status.ChangeID = hash

	// Check for uncommitted changes
	// Switch to the agent's bookmark first
	currentRef, err := v.CurrentRef()
	if err != nil {
		return nil, fmt.Errorf("failed to get current ref: %w", err)
	}

	// If not already on this bookmark, we can't check for changes
	// This is a limitation - would need to switch to the bookmark
	if currentRef == agentID {
		hasChanges, err := v.HasChanges()
		if err != nil {
			return nil, fmt.Errorf("failed to check for changes: %w", err)
		}
		status.HasChanges = hasChanges
	}

	return status, nil
}

// List returns all active agent bookmarks.
//
// Example:
//
//	agents, err := agent.List(ctx, v)
//	for _, a := range agents {
//	    fmt.Printf("Agent: %s\n", a.AgentID)
//	}
func List(ctx context.Context, v vcs.VCS) ([]BookmarkStatus, error) {
	refs, err := v.ListRefs()
	if err != nil {
		return nil, fmt.Errorf("failed to list refs: %w", err)
	}

	var agents []BookmarkStatus
	for _, ref := range refs {
		// Skip remote refs
		if ref.IsRemote {
			continue
		}

		// Check if this is an agent bookmark
		if !strings.HasPrefix(ref.Name, AgentBookmarkPrefix) &&
			!strings.HasPrefix(ref.Name, ArchiveBookmarkPrefix+AgentBookmarkPrefix) {
			continue
		}

		agents = append(agents, BookmarkStatus{
			AgentID:    ref.Name,
			Bookmark:   ref.Name,
			Exists:     true,
			ChangeID:   ref.Hash,
			IsArchived: strings.HasPrefix(ref.Name, ArchiveBookmarkPrefix),
		})
	}

	return agents, nil
}

// Recover attempts to recover an agent bookmark from the operation log.
//
// This is useful when an agent crashes or a bookmark is accidentally deleted.
// The operation log is searched for the last operation involving this agent,
// and a new bookmark is created at that state.
//
// Example:
//
//	recovered, err := agent.Recover(ctx, v, agent.RecoverOptions{
//	    AgentID: "agent-47",
//	    RecoverToID: "agent-47-recovered",
//	})
func Recover(ctx context.Context, v vcs.VCS, opts RecoverOptions) (*Agent, error) {
	agentID := normalizeAgentID(opts.AgentID)

	recoverToID := opts.RecoverToID
	if recoverToID == "" {
		recoverToID = agentID + "-recovered"
	}
	recoverToID = normalizeAgentID(recoverToID)

	// Verify target doesn't exist
	if v.RefExists(recoverToID) {
		return nil, fmt.Errorf("recovery target bookmark %s already exists", recoverToID)
	}

	// Get operation log
	ops, err := v.GetOperationLog(100) // Check last 100 operations
	if err != nil {
		return nil, fmt.Errorf("failed to get operation log: %w", err)
	}

	// Find last operation involving this agent
	var targetOpID string
	for _, op := range ops {
		// Check if operation mentions this agent
		if strings.Contains(op.Description, agentID) ||
			containsString(op.Args, agentID) {
			targetOpID = op.ID
			break
		}
	}

	if targetOpID == "" && opts.OperationID == "" {
		return nil, fmt.Errorf("no operations found for agent %s", agentID)
	}

	if opts.OperationID != "" {
		targetOpID = opts.OperationID
	}

	// Restore from that operation
	// This is simplified - real implementation would need to:
	// 1. Get the change ID at that operation
	// 2. Create bookmark pointing to that change
	//
	// For now, we'll just try to create a bookmark from the last known state
	// This requires manual implementation based on jj's operation log format

	return nil, fmt.Errorf("recovery not yet fully implemented - use jj op log manually")
}

// DeleteAgent removes an agent bookmark.
//
// Example:
//
//	err := agent.DeleteAgent(ctx, v, "agent-47")
func DeleteAgent(ctx context.Context, v vcs.VCS, agentID string) error {
	agentID = normalizeAgentID(agentID)

	if !v.RefExists(agentID) {
		return fmt.Errorf("agent bookmark %s does not exist", agentID)
	}

	return v.DeleteRef(agentID)
}

// ArchiveAgent moves an agent bookmark to the archive namespace.
//
// Example:
//
//	err := agent.ArchiveAgent(ctx, v, "agent-47")
func ArchiveAgent(ctx context.Context, v vcs.VCS, agentID string) error {
	agentID = normalizeAgentID(agentID)

	if !v.RefExists(agentID) {
		return fmt.Errorf("agent bookmark %s does not exist", agentID)
	}

	archiveName := ArchiveBookmarkPrefix + agentID

	// Create archive bookmark
	if err := v.CreateRef(archiveName, agentID); err != nil {
		return fmt.Errorf("failed to create archive bookmark: %w", err)
	}

	// Delete original
	if err := v.DeleteRef(agentID); err != nil {
		// Try to clean up archive bookmark
		_ = v.DeleteRef(archiveName)
		return fmt.Errorf("failed to delete original bookmark: %w", err)
	}

	return nil
}

// ===================
// Helper Functions
// ===================

// normalizeAgentID ensures the agent ID has the correct prefix.
func normalizeAgentID(id string) string {
	// If already has prefix, return as-is
	if strings.HasPrefix(id, AgentBookmarkPrefix) {
		return id
	}

	// If has archive prefix, return as-is
	if strings.HasPrefix(id, ArchiveBookmarkPrefix) {
		return id
	}

	// Add agent prefix
	return AgentBookmarkPrefix + id
}

// containsString checks if a string slice contains a value
func containsString(slice []string, value string) bool {
	for _, item := range slice {
		if strings.Contains(item, value) {
			return true
		}
	}
	return false
}
