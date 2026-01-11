# JJ Agent Conventions for Beads

Complete conventions for using Jujutsu (jj) bookmarks to coordinate 100+ concurrent AI agents. Part of the jj-turso implementation (see `ai_docs/jj-turso.md`).

## Bookmark Naming

```bash
# Agent bookmarks
agent-{id}           # e.g., agent-47, agent-orchestrator

# Special bookmarks
main                 # Stable integration point
staging              # Pre-merge validation (optional)

# Archive
archive/agent-{id}   # Archived agents (audit trail)
```

## Agent Lifecycle

### 1. Spawn
```bash
jj new main             # Create new change
jj bookmark create agent-47  # Create bookmark
```

### 2. Work
Files are auto-tracked by jj. No explicit add/stage needed.

### 3. Handoff (optional)
```bash
jj new agent-47         # Fork from current agent
jj bookmark create agent-48
jj bookmark rename agent-47 archive/agent-47  # Archive old
```

### 4. Complete
```bash
jj rebase -b agent-47 -d main      # Rebase onto latest main
jj bookmark set main -r agent-47    # Move main forward
jj bookmark delete agent-47         # Clean up
```

## Merge Strategies

**CRDT-Friendly Schema**
- Task files use flat JSON with `updated_at` timestamps
- Last-write-wins for field conflicts based on timestamp
- JJ allows committing with conflicts (resolve later)

**Example conflict resolution:**
```bash
# After rebase creates conflicts
jj describe -m "Merge with conflicts"  # Can commit!

# Resolve using timestamp rules
bd merge-task base.json ours.json theirs.json > merged.json
```

## Recovery

**View operation log:**
```bash
jj op log              # All operations logged
jj op log -n 20        # Last 20
```

**Undo operations:**
```bash
jj op undo             # Undo last operation
jj op undo --operation {id}  # Undo specific operation
```

**Recover lost bookmark:**
```bash
jj op log | grep agent-47
jj bookmark create agent-47-recovered -r {change-id}
```

## Code Interface

See `internal/turso/agent/bookmark.go` for Go API:

```go
// Spawn new agent
agent, err := agent.Spawn(ctx, v, agent.SpawnOptions{
    AgentID: "47",
    BaseBranch: "main",
})

// Handoff to new agent
newAgent, err := agent.Handoff(ctx, v, agent.HandoffOptions{
    FromAgentID: "agent-47",
    ToAgentID: "agent-48",
    Reason: "context limit",
    ArchiveOld: true,
})

// Complete and merge to main
err = agent.Complete(ctx, v, agent.CompleteOptions{
    AgentID: "agent-47",
    TargetBookmark: "main",
    DeleteBookmark: true,
})
```

## Turso Integration

JJ provides version control, Turso provides fast queries:

```
┌─────────────────────┐
│   jj bookmarks      │
│   (agent isolation) │
└──────────┬──────────┘
           │ op log events
           ▼
    ┌──────────────┐
    │  Turso DB    │ ← Fast queries (bd ready, bd list)
    └──────────────┘
```

Sync daemon watches jj operation log and updates Turso on file changes.

## Best Practices

**DO:**
- Spawn agents from `main`
- Use descriptive agent IDs
- Archive completed agents for audit trail
- Trust automatic conflict handling
- Use `jj op log` for debugging

**DON'T:**
- Create deep bookmark chains
- Leave stale agent bookmarks
- Manually edit `.jj/` directory
- Worry about losing work (op log has everything)

## Common Workflows

**Single Agent:**
```bash
jj new main && jj bookmark create agent-1
# ... work ...
jj rebase -b agent-1 -d main && jj bookmark set main -r agent-1
```

**Parallel Agents (no coordination needed!):**
```bash
jj new main && jj bookmark create agent-1
jj new main && jj bookmark create agent-2
jj new main && jj bookmark create agent-3
# Each merges when ready, JJ handles conflicts
```

**Long-Running Agent with Periodic Sync:**
```bash
jj new main && jj bookmark create agent-orchestrator
# ... work ...
jj rebase -b agent-orchestrator -d main  # Pull latest
# ... continue working ...
```

## References

- [JJ-Turso Implementation Plan](../ai_docs/jj-turso.md)
- [Jujutsu Documentation](https://martinvonz.github.io/jj/)
- [VCS Interface](../internal/vcs/vcs.go)
- [Agent Bookmark API](../internal/turso/agent/bookmark.go)
