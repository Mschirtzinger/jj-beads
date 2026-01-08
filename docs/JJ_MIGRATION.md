# Migrating Beads to Jujutsu (jj)

**Document Version:** 1.0
**Date:** 2026-01-07
**Status:** Production Ready

Beads supports both git and Jujutsu (jj) through a unified VCS abstraction layer. This comprehensive guide covers why you should consider jj, how to migrate safely, and how to leverage jj's powerful features for AI-supervised coding workflows.

---

## Table of Contents

1. [Why Jujutsu?](#why-jujutsu)
2. [Prerequisites](#prerequisites)
3. [Understanding Colocated Mode](#understanding-colocated-mode)
4. [Installation](#installation)
5. [Getting Started](#getting-started)
6. [Migration Paths](#migration-paths)
7. [Daily Workflow with jj-beads](#daily-workflow-with-jj-beads)
8. [Multi-Agent Workflows](#multi-agent-workflows)
9. [Sync Branch Operations](#sync-branch-operations)
10. [Configuration](#configuration)
11. [How It Works](#how-it-works)
12. [Workspace Differences](#workspace-differences)
13. [Troubleshooting](#troubleshooting)
14. [FAQ](#faq)
15. [Resources](#resources)

---

## Why Jujutsu?

Jujutsu (jj) is a next-generation version control system designed to smooth out Git's pain points while maintaining full Git compatibility. For AI-supervised coding workflows and issue tracking with Beads, jj offers several compelling advantages.

### Key Benefits for AI Workflows

#### 1. **No More "Dirty Working Copy" Errors**

In Git, AI agents frequently encounter blocking errors when trying to switch branches with uncommitted changes. The working copy is represented directly by a real commit in jj, so commands never fail because the working copy is "dirty."

**Git Problem:**
```bash
$ git checkout feature-branch
error: Your local changes to the following files would be overwritten by checkout:
    .beads/issues.jsonl
Please commit your changes or stash them before you switch branches.
```

**jj Solution:**
```bash
$ jj edit feature-branch
# Works immediately, no stashing needed
```

**Why this matters:** AI agents can switch between tasks freely without complex state management or stashing logic.

#### 2. **Conflicts Are Not Stop-the-World Events**

Unlike Git where conflicts need to be resolved immediately, Jujutsu allows conflicts to be saved in the commit tree as a logical representation. You can deal with them when convenient—conflicts won't prevent commands from running to completion.

**For AI Agents:**
- Agents can discover conflicts without being blocked
- Multiple agents can work in parallel even when conflicts exist
- Resolution can be deferred to a dedicated conflict-resolution agent
- Conflicts are visible in `jj log` but don't halt operations

#### 3. **Powerful Undo with Operation Log**

Unlike Git's limited reflog, jj's operation log tracks *every* operation atomically, treating complex operations like rebases as single entries. The `jj op undo` command can undo the effects of any other jj command.

```bash
# Accidentally rebased wrong? One command fixes it:
jj op undo

# Want to see what happened?
jj op log --limit 10
```

**For AI Agents:** Recoverable mistakes enable aggressive experimentation. An agent can try risky operations knowing any mistake can be rolled back atomically.

#### 4. **Stable Change IDs**

When you revise a commit in Git, the commit ID changes, breaking references. Jujutsu assigns stable "change IDs" that persist across amendments. If you amend a commit, jj knows to rebase all subsequent changes on that updated commit automatically.

```bash
# Amend a commit anywhere in history
jj describe -m "Updated commit message" -r abc123

# All descendants automatically rebase
# Change ID stays the same: abc123
```

**For AI Agents:** Agents can reference specific changes by ID without worrying about hash updates after amendments. It's like `commit --amend` anywhere in history, without the complexity of fixup commits and autosquash.

#### 5. **Anonymous Branches (Bookmark-Free Work)**

Git requires you to be "on" a branch at all times. Jujutsu's primary method is anonymous branches—you work directly with commits, creating bookmarks only when you need them (e.g., for pushing to remotes).

**Workflow:**
```bash
# Start working without creating a bookmark
jj new main

# Work on multiple commits
jj commit -m "Implement feature X"
jj new
jj commit -m "Add tests for X"

# Create bookmark only when ready to push
jj bookmark create feature-x
jj git push -b feature-x
```

**For AI Agents:** Reduces cognitive overhead—agents don't need to manage branch names for local work.

#### 6. **Full Git Compatibility via Colocated Mode**

The Git backend is fully featured and allows you to use jj with any Git remote. Commits you create look like regular Git commits. This hybrid approach allows gradual adoption without completely switching tools or retraining everyone at once.

**For Beads:**
- Existing Git workflows continue to work
- Team members can use Git while you use jj
- CI/CD pipelines see standard Git commits
- GitHub/GitLab integration works seamlessly

**Summary:** Jujutsu maintains compatibility while providing a significantly better UX for the types of operations AI agents perform frequently.

---

## Prerequisites

Before migrating to jj, ensure you have the following:

### 1. System Requirements

- **Operating System:** Linux, macOS, or Windows
- **Git:** Must be installed (jj uses Git's backend in colocated mode)
- **Rust (optional):** Required only if installing via `cargo`

### 2. Verify Git Installation

```bash
git --version
# Should show: git version 2.30.0 or later
```

### 3. Backup Your Repository (Recommended)

While jj is safe and non-destructive, it's good practice to backup:

```bash
# Create a backup
cd /path/to/your/repo
tar -czf ../repo-backup-$(date +%Y%m%d).tar.gz .

# Or use git clone
git clone /path/to/your/repo /path/to/backup
```

---

## Understanding Colocated Mode

**Colocated mode** is the recommended way to use jj with Beads and the safest migration path. Understanding how it works is key to a successful migration.

### What is Colocated Mode?

A colocated workspace is a hybrid jj/Git workspace where both `.git` and `.jj` directories exist in the same repository. Jujutsu automatically imports from and exports to Git on every jj command.

**Directory Structure:**
```
your-repo/
├── .git/           # Git metadata (fully functional)
├── .jj/            # Jujutsu metadata
├── .beads/         # Beads issue tracker (works with either)
└── (your files)
```

### How Colocated Mode Works

1. **Automatic Synchronization:** Every `jj` command:
   - Imports changes from Git before executing
   - Exports changes to Git after executing
   - Keeps both systems in sync automatically

2. **Interoperable Commands:** You can mix `jj` and `git` commands:
   ```bash
   jj new -m "Add feature"     # jj command
   git log --oneline           # git command (read-only)
   jj git push                 # jj command wrapping git
   ```

3. **Git Compatibility:** From Git's perspective, it's just a normal Git repo:
   - Standard Git commits
   - Works with GitHub/GitLab
   - CI/CD sees normal Git history
   - No special setup required on remotes

### Key Behaviors

**Detached HEAD State:**
- jj commands typically put Git in "detached HEAD" state
- This is normal and expected
- There's no concept of "currently tracked branch" in jj

**Git Staging Area:**
- jj ignores Git's staging area (index)
- Changes are auto-tracked
- No need for `git add`

**Best Practices:**
- Use jj commands for mutations (commit, rebase, etc.)
- Use git commands only for read-only operations (`git log`, `git diff`)
- Avoid mixing git and jj branch/bookmark operations

### Benefits for Beads

- **Zero Risk:** Can revert to pure Git by deleting `.jj`
- **Team Flexibility:** Some members use Git, others use jj
- **Gradual Migration:** Convert one developer at a time
- **Tool Compatibility:** Git-based tools continue working
- **CI/CD Compatible:** Build systems unchanged

### Caveats

- **IDE Background Operations:** Some IDEs run `git fetch` automatically, which can cause minor branch conflicts (not data loss)
- **Interleaving Commands:** Frequent switching between git and jj mutation commands may cause branch divergence (annoying but recoverable)
- **Merge States:** jj doesn't understand Git's in-progress merge/rebase states

**Recommendation:** Mostly use jj, occasionally use git for read-only queries.

---

## Installation

### Install Jujutsu

**macOS:**
```bash
brew install jj
```

**Linux:**
```bash
cargo install jj-cli
```

**Windows:**
See [https://github.com/martinvonz/jj#installation](https://github.com/martinvonz/jj#installation)

### Verify Installation

```bash
jj --version
# jj 0.x.x
```

## Getting Started

### Option 1: New Repository with JJ

```bash
# Create new jj repository with git backend (colocated)
jj git init --colocate my-project
cd my-project

# Initialize beads
bd init

# Start using beads
bd create "First task" -p 1
bd ready
```

### Option 2: Add JJ to Existing Git+Beads Repository (Recommended)

This is the recommended migration path—you keep git while gaining jj capabilities. **This is completely safe and reversible.**

**Step 1: Navigate to Your Repository**
```bash
# Navigate to your existing beads repo
cd your-existing-repo

# Verify it's a git repo with beads
git status
ls .beads/
```

**Step 2: Initialize jj in Colocated Mode**
```bash
# Initialize jj (creates .jj alongside .git)
jj git init --colocate

# Verify both exist
ls -la | grep -E '\.(git|jj)'
# Should show both .git and .jj directories
```

**Step 3: Verify jj Can See Your History**
```bash
# Check jj log shows your git history
jj log --limit 10

# Should see your existing commits
```

**Step 4: Verify Beads Detects jj**
```bash
# Run beads doctor
bd doctor

# Expected output includes:
# ✓ VCS Type: colocate (jj + git)
# ✓ VCS Version: jj 0.24.0 (or later)
```

**Step 5: Test Basic Operations**
```bash
# List existing issues (should work as before)
bd list

# Create a test issue
bd create "Test jj integration" -t task -p 3 --json

# Export to JSONL
bd export -o .beads/issues.jsonl

# Check status with jj (should show modified file)
jj status
# Output: M .beads/issues.jsonl
```

**Step 6: First Commit with jj**
```bash
# Describe the current change
jj describe -m "test: verify jj integration with beads"

# Push (uses your existing git remote)
jj git push
```

**Step 7: Update Documentation for Team**
```bash
# Add jj notes to your CLAUDE.md or AGENTS.md
cat >> CLAUDE.md <<'EOF'

## Jujutsu (jj) Integration

This repository now supports both Git and jj via colocated mode.

Recommended workflow:
- Use `jj` commands for making changes
- Use `git` commands for read-only operations if needed
- Beads (`bd`) automatically works with both

Quick jj reference:
- `jj new` - Create new change
- `jj describe -m "msg"` - Add/update commit message
- `jj git push` - Push to remote
- `jj log` - Show history
- `jj op undo` - Undo last operation
EOF

# Commit the documentation
jj describe -m "docs: add jj integration notes"
jj git push
```

**Rollback (if needed):**
```bash
# Remove jj (completely safe, Git unchanged)
rm -rf .jj

# Beads now uses Git again
bd doctor  # Shows VCS Type: git
```

### Option 3: Pure JJ Repository

```bash
# Create jj-only repository (no .git)
jj init my-project
cd my-project

# Add git remote
jj git remote add origin https://github.com/user/repo.git

# Initialize beads
bd init
```

## Migration Paths

### Colocated Mode (Recommended)

Colocated mode runs both git and jj in the same repository. This is the safest migration path:

**Advantages:**
- No risk - git still works normally
- Try jj without commitment
- Gradual team migration
- Fallback to git anytime

**How it works:**
```bash
your-repo/
├── .git/           # Git metadata (still works)
├── .jj/            # JJ metadata
└── .beads/         # Beads data (works with either)
```

Beads automatically prefers jj when both are present, but you can configure this.

### Full Migration (Git → JJ)

If you want to fully migrate from git to jj:

1. **Start with colocated mode** (see above)
2. **Verify everything works** with jj
3. **Optional: Remove .git** if you no longer need git compatibility
   ```bash
   # DANGER: Only do this if you're sure!
   rm -rf .git
   ```

**Note:** We recommend keeping colocated mode for maximum flexibility.

## Configuration

### Auto-Detection (Default)

By default, beads auto-detects your VCS:

- If `.jj` exists → use jj
- If both `.jj` and `.git` exist → prefer jj
- If only `.git` exists → use git

### Explicit Configuration

Create or edit `.beads/config.yaml`:

```yaml
vcs:
  # Preferred VCS for colocated repos: "git" | "jj" | "auto"
  preferred: jj

  # Fallback if preferred unavailable
  fallback: git

  # JJ-specific settings
  jj:
    # Use jj workspace for sync (experimental)
    use-workspace: false

    # Auto-create bookmark for sync
    auto-bookmark: true

  # Git-specific settings
  git:
    # Use sparse checkout for sync worktree
    sparse-checkout: true
```

### Environment Variable

Override VCS selection per-command:

```bash
# Force jj
export BD_VCS=jj
bd ready

# Force git
export BD_VCS=git
bd ready

# Auto-detect (default)
unset BD_VCS
bd ready
```

## How It Works

### VCS Abstraction Layer

Beads uses a unified VCS interface that abstracts git and jj differences:

```
┌─────────────────┐
│  Beads Commands │
└────────┬────────┘
         │
    ┌────▼─────┐
    │ VCS API  │  (internal/vcs/vcs.go)
    └────┬─────┘
         │
    ┌────▼────────────┐
    │   Auto-Detect   │
    └────┬────────────┘
         │
    ┌────▼────┬──────────┐
    │         │          │
┌───▼───┐ ┌──▼──┐ ┌─────▼─────┐
│  Git  │ │ JJ  │ │ Colocated │
└───────┘ └─────┘ └───────────┘
```

### Sync Branch Implementation

The sync daemon works differently between git and jj:

**Git:**
- Uses worktrees for isolation
- Creates separate directory for sync branch
- Commits to sync branch without affecting main work

**JJ:**
- Uses change-based isolation
- Creates temporary change for sync
- Moves bookmark, commits, then returns to user's work
- No separate directory needed

Both provide the same functionality - background sync without disrupting your workflow.

---

## Daily Workflow with jj-beads

### Starting Work on a New Task

```bash
# Check for ready work
bd ready --json

# Found: bd-x3k9 "Implement user authentication"

# Claim the task
bd update bd-x3k9 --status in_progress --json

# Create new change for this work
jj new main -m "feat: implement user authentication"

# Work on your files...
# (changes auto-tracked by jj, no need for git add)
```

### Making Progress

```bash
# Update commit message as work evolves
jj describe -m "feat: implement user authentication

- Add login endpoint
- Add session management
- Add password hashing with bcrypt"

# Check status anytime
jj status

# View your changes
jj diff

# Amend the current change if needed
jj describe -m "feat: implement user authentication (updated)"
```

### Completing Work

```bash
# Finalize commit message
jj describe -m "feat: implement user authentication"

# Close the beads issue
bd close bd-x3k9 --reason "Implemented and tested" --json

# Export beads changes (or let daemon handle it)
bd export -o .beads/issues.jsonl

# Create new change for beads update (optional, can be part of feature commit)
jj describe -m "feat: implement user authentication

Closes bd-x3k9"

# Create bookmark for PR
jj bookmark create feat/user-auth

# Push to remote
jj git push -b feat/user-auth
```

### Working on Multiple Features Simultaneously

One of jj's superpowers is seamless parallel work without branch juggling:

```bash
# Start feature A
jj new main -m "feat: add feature A"
jj bookmark create feature-a
# ... work on feature A ...
jj describe -m "feat: add feature A (WIP)"

# Switch to feature B (no stashing needed!)
jj new main -m "feat: add feature B"
jj bookmark create feature-b
# ... work on feature B ...

# Switch back to feature A
jj edit feature-a
# Continue working on A
# Your changes are preserved, no stashing

# View all your in-flight work
jj log
```

### Handling Discovered Issues

```bash
# While working on bd-abc, discover a bug
bd create "Fix authentication edge case" -t bug -p 1 --json
# Returns: bd-xyz

# Link as discovered work
bd dep add bd-xyz bd-abc --type discovered-from --json

# Create separate change for the bug fix
jj new main -m "fix: handle auth edge case"
bd update bd-xyz --status in_progress --json

# Fix the bug...
jj describe -m "fix: handle authentication edge case

Fixes bd-xyz
Discovered while working on bd-abc"

# Return to original feature
jj edit <feature-change-id>
```

---

## Multi-Agent Workflows

Beads is designed for AI-supervised development with multiple agents working in parallel. Jujutsu's model aligns perfectly with this workflow.

### Scenario: Three Agents Working Simultaneously

**Agent 1: Authentication System**
```bash
# Create change for auth work
jj new main -m "feat: authentication system"
jj bookmark create agent-1/auth

# Claim beads task
bd update bd-auth --status in_progress --json

# Work on authentication...
# ... implement JWT, sessions, etc ...

# Commit work
jj describe -m "feat: add JWT authentication with refresh tokens"
jj git push -b agent-1/auth
```

**Agent 2: Database Layer (working in parallel)**
```bash
# Create independent change
jj new main -m "feat: database layer"
jj bookmark create agent-2/database

# Claim beads task
bd update bd-db --status in_progress --json

# Work on database...
# ... add connection pooling, migrations, etc ...

# Commit work (no conflict with Agent 1)
jj describe -m "feat: add PostgreSQL connection pool and migrations"
jj git push -b agent-2/database
```

**Agent 3: REST API (working in parallel)**
```bash
# Create independent change
jj new main -m "feat: REST API"
jj bookmark create agent-3/api

# Claim beads task
bd update bd-api --status in_progress --json

# Work on API...
# ... implement endpoints ...

# Commit work (no conflict with others)
jj describe -m "feat: add user management REST API"
jj git push -b agent-3/api
```

### Orchestrator Agent Integrates Work

```bash
# Fetch all agent work
jj git fetch

# Review what was done
jj log --limit 20

# Create integration commit merging all three agent branches
jj new main agent-1/auth agent-2/database agent-3/api \
  -m "feat: integrate auth, database, and API components"

# Check for conflicts
jj resolve --list
# (jj allows working with conflicts present - can defer resolution)

# If no conflicts or after resolving
jj describe -m "feat: integrate authentication system

Merges:
- agent-1/auth: JWT authentication
- agent-2/database: PostgreSQL layer
- agent-3/api: User management API

Closes bd-auth, bd-db, bd-api"

# Update bookmark and push
jj bookmark move main -t @
jj git push -b main
```

### Benefits for Multi-Agent Workflows

1. **No Dirty Working Copy Errors:** Agents switch contexts freely
2. **Conflict-Tolerant:** Conflicts don't block agent progress
3. **Stable Change IDs:** Agents can reference work consistently
4. **Operation Undo:** Orchestrator can undo problematic merges
5. **Parallel Commits:** Agents push simultaneously without coordination

---

## Sync Branch Operations

The beads daemon automatically syncs `.beads/issues.jsonl` to a sync branch for real-time collaboration.

### How Sync Works with jj

The daemon creates temporary changes for sync operations:

```bash
# Daemon workflow (automatic, happens in background)
1. Detect changes in SQLite cache
2. Create new jj change:
   jj new beads-sync -m "bd sync: 2026-01-07 14:23:45"
3. Export to .beads/issues.jsonl
4. Describe change:
   jj describe -m "bd sync: 2026-01-07 14:23:45"
5. Move bookmark:
   jj bookmark move beads-sync -t @
6. Push:
   jj git push -b beads-sync
7. Return to user's work:
   jj edit <previous-change>
```

### Git Worktree vs jj Change Model

**Git Approach (Old):**
```bash
# Creates separate directory
.git/worktrees/beads-sync/
└── (isolated working directory)

# Requires careful management of two working copies
```

**jj Approach (New):**
```bash
# Uses change-based isolation (no separate directory)
# Works in-place with change ID tracking

# Benefits:
# - No separate directory to manage
# - Atomic operations
# - Conflicts don't block sync
# - Easy rollback with jj op undo
```

### Manual Sync (If Daemon is Stopped)

```bash
# Export current state
bd export -o .beads/issues.jsonl

# Create sync change
jj new beads-sync -m "bd sync: $(date '+%Y-%m-%d %H:%M:%S')"

# Describe change (jj auto-tracks the file changes)
jj describe -m "bd sync: $(date '+%Y-%m-%d %H:%M:%S')"

# Update bookmark
jj bookmark move beads-sync -t @

# Push
jj git push -b beads-sync

# Return to your work
jj edit @-  # Go to parent change
```

### Pulling Remote Sync Changes

```bash
# Fetch from remote
jj git fetch

# Check what changed
jj log -r beads-sync

# Import remote changes
bd import -i .beads/issues.jsonl

# Check for new or updated issues
bd ready --json
```

---

## Workspace Differences

### Git Worktrees

```bash
# Git creates separate directory
.git/worktrees/beads-sync/
└── (isolated working directory)
```

### JJ Changes

```bash
# JJ uses change model - works in-place
jj new -m "bd sync: timestamp"    # Create sync change
# ... make changes to .beads/ ...
jj describe -m "sync"              # Describe change
jj bookmark move beads-sync        # Move bookmark
jj git push -b beads-sync          # Push
jj edit <original>                 # Return to user's work
```

The abstraction makes these differences invisible to you.

## Command Mapping

For reference, here's how beads operations map to VCS commands:

| Beads Operation | Git Command | JJ Command |
|----------------|-------------|------------|
| Detect repo | `git rev-parse --git-dir` | Check for `.jj/` |
| Current branch | `git symbolic-ref HEAD` | `jj log -r @ --no-graph` |
| Has changes | `git status --porcelain` | `jj status` |
| Create branch | `git branch <name>` | `jj bookmark create <name>` |
| Commit | `git commit -m "msg"` | `jj describe -m "msg"` |
| Push | `git push` | `jj git push` |
| Fetch | `git fetch` | `jj git fetch` |
| Undo | `git reset` (limited) | `jj op undo` (full) |

## Troubleshooting

### JJ Not Detected

**Problem:** Beads still uses git even though jj is installed.

**Solution:**
```bash
# Verify jj is initialized
ls -la .jj
# If not present:
jj git init --colocate

# Force jj usage
export BD_VCS=jj
bd ready
```

### Colocated Mode Issues

**Problem:** Conflicts between git and jj state.

**Solution:**
```bash
# Sync jj with git
jj git import

# Or sync git with jj
jj git export

# Check status
jj status
git status
```

### Sync Branch Not Working

**Problem:** Sync daemon fails with jj.

**Solution:**
```bash
# Check daemon status
bd daemon status

# Restart daemon
bd daemon stop
bd daemon start

# Check logs
tail -f ~/.local/state/beads/daemon.log
```

### Performance Issues

**Problem:** JJ operations feel slow.

**JJ is typically faster than git, but if you see slowness:**
```bash
# Check operation log
jj op log --limit 20

# Clean up operation log if very large
jj op abandon --at 'root()..@-100'  # Keep last 100 operations
```

### Reverting to Git

**Problem:** Need to go back to git.

**Solution:**
```bash
# For colocated repos, just configure beads
echo 'vcs:
  preferred: git' > .beads/config.yaml

# Or use environment variable
export BD_VCS=git

# Beads now uses git, .jj remains but is ignored
```

## Advanced Usage

### Custom Bookmark Names

```yaml
# .beads/config.yaml
vcs:
  sync-branch: my-custom-sync  # Default: beads-sync
```

### Debugging VCS Detection

```bash
# See which VCS beads is using
bd daemon status | grep -i vcs

# Verbose output
BD_VCS_LOG=1 bd ready
```

### Mixed Teams (Some Git, Some JJ)

**Scenario:** Team members use different VCS, but same repo.

**Solution:**
- Use colocated mode
- Everyone's `.beads/` data is identical (JSONL format)
- Git users use `git`, jj users use `jj`
- Beads works seamlessly with both

```bash
# Git user workflow
git pull
bd ready
bd close bd-abc
git add .beads/
git commit -m "Close bd-abc"
git push

# JJ user workflow (same repo)
jj git fetch
bd ready
bd close bd-def
jj describe -m "Close bd-def"
jj git push
```

---

## FAQ

### Q: Do I need to uninstall Git to use jj?

**A:** No! In colocated mode, jj and Git work together. Git must remain installed for jj's Git backend to function.

### Q: Can I switch back to pure Git if I don't like jj?

**A:** Yes, absolutely. Just delete the `.jj` directory. Your Git history is completely unchanged. Beads will automatically detect and use Git.

```bash
rm -rf .jj
bd doctor  # Now shows VCS: git
```

### Q: Will my CI/CD pipelines work with jj?

**A:** Yes, when using colocated mode (recommended). From the CI's perspective, it's a normal Git repository. The CI doesn't need jj installed.

### Q: How does jj handle merge conflicts in .beads/issues.jsonl?

**A:** jj allows conflicted commits, so sync doesn't fail. You can resolve conflicts later:

```bash
# Sync continues even with conflicts
jj log  # Shows conflict indicators

# Resolve when convenient
jj resolve .beads/issues.jsonl
```

### Q: What if a team member doesn't want to use jj?

**A:** No problem! They can continue using Git exclusively. Colocated mode means both work seamlessly:

```bash
# Team member A uses jj
jj new main -m "Add feature"
jj git push

# Team member B uses git (sees the same commits)
git pull
git checkout -b fix/bug
git commit -m "Fix bug"
git push
```

Both workflows are completely compatible.

### Q: Does jj work with protected branches?

**A:** Yes. Protected branch rules are enforced by the Git remote (GitHub/GitLab), not the local VCS. jj respects these when pushing.

### Q: How do I visualize jj history?

**A:** Use `jj log` with various options:

```bash
# Graphical view
jj log

# Compact view
jj log --limit 20

# With diffs
jj log -p

# Custom template
jj log -T 'commit_id ++ " " ++ description ++ "\n"'
```

**GUI Options:**
- `jj log` has built-in terminal UI
- `gitk` or `gitg` work with colocated repos
- VS Code with jj extension (experimental)

### Q: What happens to Git hooks?

**A:** In colocated mode, Git hooks continue to work when using `jj git push`. jj has experimental hook support:

```bash
# jj hooks (beta)
.jj/hooks/pre-commit
.jj/hooks/post-commit
```

### Q: Can I use jj with stealth mode beads?

**A:** Yes! Stealth mode (`.beads/` not committed) works identically with jj:

```bash
bd init --stealth
jj new main -m "Personal task tracking"
# .beads/ is gitignored, works as expected
```

### Q: How do bookmarks differ from branches?

**A:** Conceptually similar, but with key differences:

| Aspect | Git Branches | jj Bookmarks |
|--------|--------------|--------------|
| Required | Yes (always "on" a branch) | No (can work without bookmarks) |
| Moves automatically | Yes (on commit) | No (manual with `jj bookmark move`) |
| Creating | Frequent | Only for pushing/sharing |
| Mental model | Container for commits | Pointer to commits |

### Q: What's the performance difference?

**A:** jj is generally fast, with some trade-offs:

- **Initial operations:** Slightly slower (jj imports/exports from Git)
- **Subsequent operations:** Fast (cached)
- **Large repos:** Similar to Git
- **Undo/redo:** Much faster than Git reflog operations

**Beads-specific:** Sync operations are slightly slower due to jj's import/export, but conflicts are handled much better.

### Q: Can I use bd commands exactly the same with jj?

**A:** Yes! All bd commands work identically:

```bash
bd init
bd create "Task" -p 1
bd ready --json
bd update bd-abc --status in_progress
bd close bd-abc --reason "Done"
```

The VCS abstraction layer makes jj transparent to beads users.

---

## Resources

### Official Documentation

- [Jujutsu Official Docs](https://jj-vcs.github.io/jj/)
- [Git Compatibility Guide](https://jj-vcs.github.io/jj/latest/git-compatibility/)
- [Working with GitHub](https://jj-vcs.github.io/jj/latest/github/)

### Community Resources

- [jj GitHub Repository](https://github.com/jj-vcs/jj)
- [Using jj in Colocated Repositories](https://cuffaro.com/2025-03-15-using-jujutsu-in-a-colocated-git-repository/)
- [jj Tutorial by Tony Finn](https://tonyfinn.com/blog/jj/)
- [jj Cheatsheet](https://github.com/bfirestone/jj-cheatsheet)

### Blog Posts and Articles

- [Jujutsu: The Future of Version Control](https://medium.com/@shrmtv/jujutsu-150945f97753)
- [Git and Jujutsu: The Next Evolution](https://www.infovision.com/blog/git-and-jujutsu-the-next-evolution-in-version-control-systems/)
- [What I've Learned from jj](https://zerowidth.com/2025/what-ive-learned-from-jj/)
- [Jujutsu: A Git-compatible VCS](https://tonyfinn.com/blog/jj/)

### Beads-Specific Documentation

- [Architecture Design](../.agents/research/architecture-design.md) - Full VCS abstraction design
- [Phase 1 Research Summary](../.agents/research/phase1-research-summary.md) - Migration analysis
- [VCS Interface Documentation](../internal/vcs/vcs.go) - VCS abstraction code
- [Git Integration Guide](GIT_INTEGRATION.md) - Original Git documentation

---

## Summary

- **JJ support is production-ready** and fully tested
- **Colocated mode is the safest migration path** - zero risk, fully reversible
- **Beads auto-detects** and works seamlessly with both git and jj
- **No workflow changes needed** - beads commands remain identical
- **Full team flexibility** - some members can use Git, others jj
- **Configuration available** via `.beads/config.yaml` or `BD_VCS` environment variable
- **Superior AI workflow support** - no dirty working copy errors, better conflict handling, powerful undo

### When to Use jj with Beads

**Highly Recommended For:**
- AI-supervised development workflows
- Multi-agent parallel development
- Frequent context switching between tasks
- Complex rebase/merge scenarios
- Teams wanting better undo capabilities

**Optional For:**
- Simple linear workflows
- Single-developer projects with minimal branching
- Teams heavily invested in Git-specific tools

### Getting Help

- **Documentation:** See [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
- **GitHub Issues:** [jj-beads issues](https://github.com/steveyegge/beads/issues)
- **Community:** [jj Discord](https://discord.gg/dkmfj3aGQN)

---

**Document Maintainer:** Atlas (Principal Software Engineer)
**Last Updated:** 2026-01-07
**Status:** Production Ready

---

**Happy tracking with jj-beads!**
