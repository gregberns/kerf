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

### 2. Automatic persistence on interaction
Every file the agent writes goes directly to the bench (the filesystem is the database — no save step). Versioning snapshots are taken automatically whenever kerf commands run (new, resume, shelve, finalize, status changes) and optionally on a timed interval. Explicit snapshots are available via `kerf snapshot`. No commit step, no ceremony.

### 3. Process-driven via jigs
Jigs define how an agent walks through a work. A "feature" jig has different passes than a "bug" jig. Jigs are opinionated out of the box but replaceable. The jig is the instruction set the agent follows — it defines both the process and the file structure.

### 4. CLI output is agent-friendly
When the CLI outputs information (list of works, work details, available commands), it's formatted so an agent can immediately understand context and next steps. Running the root command with no arguments shows a quick-start guide sufficient for an agent to use the tool with zero prior context. State-changing commands emit next steps telling the agent what to do.

### 5. Kerf does not launch or manage agent sessions
Kerf is a data and workflow management tool. It never launches Claude, manages sessions, or orchestrates agents. The human launches Claude (or any agent), tells the agent about kerf, and the agent uses kerf commands to manage its workflow. `kerf resume` emits context for the agent — it does not launch a new Claude session. This keeps kerf tool-agnostic and avoids coupling to any specific agent runtime.

### 6. Clean handoff, not orchestration
This tool manages the spec-writing lifecycle. It does NOT orchestrate implementation. At finalization, works are packaged and placed into the codebase (on a branch, with tasks/beads generated). What happens next — CI, agent-driven implementation, human review — is the responsibility of other tools. The design should make it trivially easy for an orchestrator to integrate, but orchestration is out of scope.

### 7. Team-ready architecture, solo-first implementation
v1 is for a solo developer. But every design decision should be compatible with future multi-user scenarios: shared storage, access control, sync. Don't build team features yet, but don't paint yourself into a corner.

## How It Works (User Journey)

### Starting a new work
```
$ kerf new
```
Creates a new work with defaults. The agent and user begin iterating — discussing the problem space, breaking it into components, researching approaches. All artifacts are written to `~/.kerf/projects/{project-id}/` as the conversation progresses.

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
The CLI outputs the work's current state: SESSION.md contents, current pass and jig instructions, recent session history, and open questions. The agent reads this output and is oriented within a couple of exchanges. Kerf does not launch Claude — the human starts the agent session, and the agent (or human) runs `kerf resume` to load context.

### Listing works
```
$ kerf list
```
Shows all works with codename, type, status, last updated. Also shows contextual next commands (e.g., "Run `kerf resume auth-rewrite` to continue"). This output is designed to be useful to both humans and agents.

### Finalizing a work
```
$ kerf finalize <codename>
```
Kerf performs the mechanical steps: creates a git branch in the target repo, copies work artifacts into `.kerf/{codename}/`, creates an initial commit, and updates spec.yaml with branch info. It then emits instructions for agent-driven follow-up steps (create a PR, notify the team, etc.). The work is now in git, ready for implementation.

### Viewing a work
```
$ kerf show <codename>
```
Displays the work's metadata, file tree, current pass, session history, and dependencies. Enough context for an agent or human to understand the state of the work at a glance.

## Agent Discovery & Human-Agent Handoff

### How an agent discovers kerf
Kerf is a CLI tool. An agent learns about it the same way it learns about any tool: the human tells it, or it's documented in the project's CLAUDE.md (or equivalent). Kerf ships with a recommended CLAUDE.md snippet that projects can add:

```markdown
This project uses `kerf` for spec management. Run `kerf` with no arguments for a quick-start guide. 
Use `kerf list` to see active works, `kerf resume <codename>` to load context for an in-progress work.
```

If no CLAUDE.md exists, the human tells the agent: "Use kerf to manage this spec." The agent runs `kerf` and reads the quick-start output.

### The handoff protocol
1. **Human initiates:** The human runs `kerf new` (or tells the agent to). Kerf creates the work, outputs the jig overview and first-pass instructions.
2. **Agent works:** The agent follows jig instructions, writing artifacts to the work directory. It uses `kerf status` to advance through passes.
3. **Human steers:** The human can redirect at any time — "skip the research pass, we already know the approach." The agent uses `kerf status <codename> <new-status>` to jump ahead. Passes are guidance, not gates.
4. **Shelving:** When the session ends (planned or not), the agent runs `kerf shelve` and writes SESSION.md. If the session terminates unexpectedly, SESSION.md may not exist — the raw artifacts and spec.yaml still provide enough context for a new session, just with less interpreted state.
5. **Resuming:** The human starts a new agent session and runs `kerf resume <codename>` (or tells the agent to). The output provides full context: where the work is, what's been done, what's next.
6. **Finalizing:** The human reviews the spec (via `kerf show` or reading files directly), then tells the agent to run `kerf finalize`. Finalization is not unilateral — the human decides when a spec is ready.

## What This Is NOT

- **Not a project management tool.** No sprints, no story points, no burndown charts.
- **Not an orchestrator.** Does not schedule or manage implementation agents.
- **Not a database.** Uses the filesystem. Files are the source of truth.
- **Not a git replacement.** Works enter git at finalization. Before that, they're just files with automatic versioning.
- **Not prescriptive about methodology.** Jigs are opinionated defaults, not enforced processes. Users can define their own.
