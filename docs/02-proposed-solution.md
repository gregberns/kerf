# Proposed Solution

## Overview

A CLI tool (`kerf`) that manages a **bench** — a persistent, auto-versioned store of specification documents that lives outside of git but maintains a link to the codebase. Agents read the codebase for context but write spec artifacts to the bench. The bench handles persistence, versioning, session tracking, and handoff to implementation.

The tool provides opinionated, built-in processes ("jigs") that guide agents through structured spec-writing workflows for different types of work (features, bugs, migrations, etc.). Jigs define the passes, file structures, and agent instructions — so engineers get a working process out of the box without having to define their own.

## Key Design Principles

### 1. Separate spec artifacts from the codebase
Works live in `~/.kerf/` (or a configurable location), not in the git repo. This means:
- No branches, commits, or PRs for work-in-progress specs
- All worktrees for the same repo share the same works
- Multiple works can be in flight without any git conflicts
- Works enter the codebase only at finalization, cleanly

### 2. Immediate, automatic persistence
Every file the agent writes is persisted immediately. Versioning happens automatically (timestamped snapshots). No save button, no commit step. If the agent writes to `02-components.md`, that change is captured.

### 3. Process-driven via jigs
Jigs define how an agent walks through a work. A "feature" jig has different passes than a "bug" jig. Jigs are opinionated out of the box but replaceable. The jig is the instruction set the agent follows — it defines both the process and the file structure.

### 4. CLI output is agent-friendly
When the CLI outputs information (list of works, work details, available commands), it's formatted so an agent can immediately understand context and next steps. Running the root command with no arguments shows a quick-start guide. Commands like `shelve` and `finalize` emit instructions that tell the agent what additional steps to take.

### 5. Clean handoff, not orchestration
This tool manages the spec-writing lifecycle. It does NOT orchestrate implementation. At finalization, works are packaged and placed into the codebase (on a branch, with tasks/beads generated). What happens next — CI, agent-driven implementation, human review — is the responsibility of other tools. The design should make it trivially easy for an orchestrator to integrate, but orchestration is out of scope.

### 6. Team-ready architecture, solo-first implementation
v1 is for a solo developer. But every design decision should be compatible with future multi-user scenarios: shared storage, access control, sync. Don't build team features yet, but don't paint yourself into a corner.

## How It Works (User Journey)

### Starting a new work
```
$ kerf new
```
Creates a new work with defaults. The agent and user begin iterating — discussing the problem space, breaking it into components, researching approaches. All artifacts are written to `~/.kerf/projects/{repo}/` as the conversation progresses.

### Working through the process
The loaded jig guides the agent through passes. After each pass, artifacts are saved to disk. The agent knows what pass it's in, what's been completed, and what comes next — all from reading the work's index file and existing artifacts.

### Shelving a work
```
$ kerf shelve
```
The CLI does its bookkeeping (snapshots, status update). It also emits instructions to the agent: "Write a SESSION.md summarizing current state, decisions made, open questions, and next steps." The agent follows these instructions, producing a high-signal resumability artifact.

### Resuming a work
```
$ kerf resume <codename>
```
The CLI looks up the associated Claude session ID and resumes it (`claude --resume <id>`). If the session is gone or stale, it starts a new session, loading the work's artifacts + SESSION.md as context. The agent reads these and is oriented within a couple of exchanges.

### Listing works
```
$ kerf list
```
Shows all works with codename, type, status, last updated. Also shows contextual next commands (e.g., "Run `kerf resume auth-rewrite` to continue"). This output is designed to be useful to both humans and agents.

### Finalizing a work
```
$ kerf finalize <codename>
```
The CLI emits a defined procedure (configurable at the `~/.kerf/` level): create a branch, copy work artifacts into the repo, generate tasks/beads, update the work's metadata with branch/PR info, change status. The agent follows these instructions. The work is now in git, ready for implementation.

### Viewing a work
```
$ kerf show <codename>
```
Displays the work's metadata, file tree, current pass, session history, and dependencies. Enough context for an agent or human to understand the state of the work at a glance.

## What This Is NOT

- **Not a project management tool.** No sprints, no story points, no burndown charts.
- **Not an orchestrator.** Does not schedule or manage implementation agents.
- **Not a database.** Uses the filesystem. Files are the source of truth.
- **Not a git replacement.** Works enter git at finalization. Before that, they're just files with automatic versioning.
- **Not prescriptive about methodology.** Jigs are opinionated defaults, not enforced processes. Users can define their own.
