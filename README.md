# bd - Beads

**Distributed, VCS-backed graph issue tracker for AI agents.**

Works with both git and Jujutsu (jj), including colocated repositories.

[![License](https://img.shields.io/github/license/steveyegge/beads)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/steveyegge/beads)](https://goreportcard.com/report/github.com/steveyegge/beads)
[![Release](https://img.shields.io/github/v/release/steveyegge/beads)](https://github.com/steveyegge/beads/releases)
[![npm version](https://img.shields.io/npm/v/@beads/bd)](https://www.npmjs.com/package/@beads/bd)
[![PyPI](https://img.shields.io/pypi/v/beads-mcp)](https://pypi.org/project/beads-mcp/)

Beads provides a persistent, structured memory for coding agents. It replaces messy markdown plans with a dependency-aware graph, allowing agents to handle long-horizon tasks without losing context.

## ⚡ Quick Start

```bash
# Install (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash

# Initialize (Humans run this once)
bd init

# Tell your agent
echo "Use 'bd' for task tracking" >> AGENTS.md

```

## 🛠 Features

* **VCS as Database:** Issues stored as JSONL in `.beads/`. Versioned, branched, and merged like code. Works with both git and jj.
* **Agent-Optimized:** JSON output, dependency tracking, and auto-ready task detection.
* **Zero Conflict:** Hash-based IDs (`bd-a1b2`) prevent merge collisions in multi-agent/multi-branch workflows.
* **Invisible Infrastructure:** SQLite local cache for speed; background daemon for auto-sync.
* **Compaction:** Semantic "memory decay" summarizes old closed tasks to save context window.
* **JJ Support:** First-class support for Jujutsu VCS with colocated git+jj repositories.

## 📖 Essential Commands

| Command | Action |
| --- | --- |
| `bd ready` | List tasks with no open blockers. |
| `bd create "Title" -p 0` | Create a P0 task. |
| `bd dep add <child> <parent>` | Link tasks (blocks, related, parent-child). |
| `bd show <id>` | View task details and audit trail. |

## 🔗 Hierarchy & Workflow

Beads supports hierarchical IDs for epics:

* `bd-a3f8` (Epic)
* `bd-a3f8.1` (Task)
* `bd-a3f8.1.1` (Sub-task)

**Stealth Mode:** Run `bd init --stealth` to use Beads locally without committing files to the main repo. Perfect for personal use on shared projects.

## 📦 Installation

* **npm:** `npm install -g @beads/bd`
* **Homebrew:** `brew install steveyegge/beads/bd`
* **Go:** `go install github.com/steveyegge/beads/cmd/bd@latest`

**Requirements:** Linux (glibc 2.32+), macOS, or Windows.

## 🌐 Community Tools

See [docs/COMMUNITY_TOOLS.md](docs/COMMUNITY_TOOLS.md) for a curated list of community-built UIs, extensions, and integrations—including terminal interfaces, web UIs, editor extensions, and native apps.

## 🔄 JJ Integration

Beads supports both git and Jujutsu (jj) through a VCS abstraction layer. It automatically detects your VCS type and works seamlessly with:

- **Git-only repositories** - Traditional git workflow
- **JJ-only repositories** - Pure jj workflow
- **Colocated repositories** - Both git and jj together (`jj git init --colocate`)

### Getting Started with JJ

```bash
# Option 1: New jj repository
jj git init --colocate
bd init

# Option 2: Add jj to existing git repository
cd your-git-repo
jj git init --colocate
# bd continues to work - now with jj superpowers!
```

### Configuration

Beads auto-detects your VCS, but you can configure preferences in `.beads/config.yaml`:

```yaml
vcs:
  preferred: jj    # or "git" or "auto"
  fallback: git    # fallback if preferred unavailable
```

Or use environment variable: `export BD_VCS=jj`

See [docs/JJ_MIGRATION.md](docs/JJ_MIGRATION.md) for migration guide and advanced usage.

## 📝 Documentation

* [Installing](docs/INSTALLING.md) | [Agent Workflow](AGENT_INSTRUCTIONS.md) | [Sync Branch Mode](docs/PROTECTED_BRANCHES.md) | [JJ Migration](docs/JJ_MIGRATION.md) | [Troubleshooting](docs/TROUBLESHOOTING.md) | [FAQ](docs/FAQ.md)
* [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/steveyegge/beads)
