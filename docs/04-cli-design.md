# CLI Design

## Executable

**Name:** `kerf`  
**Tagline:** *Measure twice, cut once.*

## Design Philosophy

1. **Zero-context usability.** An agent encountering kerf for the first time — with no documentation, no prior conversation, no CLAUDE.md — should be able to use it effectively from `kerf --help` alone. Help text is complete, discoverable, and includes examples. Every command's `--help` tells you what it does, what it outputs, and what you'd typically do next.

2. **Agent-first output.** Every command's output is designed to be consumed by an AI agent. It includes context, current state, and suggested next actions. Human-readable too, but agent-consumable is the priority.

3. **No-arg root command = quick start.** Running `kerf` with no arguments prints a quick-start guide that gives an agent everything it needs to interact with the tool. This eliminates the need for agents to crawl through `--help` subcommands.

4. **Commands emit next steps.** State-changing commands (`new`, `shelve`, `resume`, `finalize`, `status`) don't just perform mechanical operations — they also emit what to do next. `kerf new` tells you how to start the first pass. `kerf shelve` tells you to write SESSION.md. `kerf resume` tells you where you left off. The CLI guides the workflow, not just executes it.

5. **Contextual output.** `list` shows not just data but also the commands you'd likely want to run next. `show` displays not just metadata but the file tree and jig context.

## Commands

### `kerf` (no arguments)
**Purpose:** Quick-start guide for agents and humans.  
**Output:** A self-contained guide that enables an agent with zero prior context to use kerf effectively. Includes:
- One-line description of what kerf does
- Available commands with brief descriptions and examples
- The most common workflow: `kerf new` → work through passes → `kerf shelve` / `kerf finalize`
- Current bench summary (number of active works, current project)
- If no bench exists yet, explains how to get started

This output is the primary agent onboarding surface. It must be complete enough that an agent never needs to read external docs.

### `kerf new [codename] [--title <title>] [--type feature|bug|...] [--jig <name>] [--project <project-id>]`
**Purpose:** Create a new work.  
**Behavior:**
- If no codename given, auto-generates an `adjective-noun` slug (e.g., `blue-bear`, `swift-maple`)
- Creates directory under `~/.kerf/projects/{project-id}/{codename}/`
- If `--project` not given, infers from current working directory via `.kerf/project-identifier`
- If this is the first kerf use in a repo, initializes `.kerf/project-identifier` (derived from git remote, or directory name as fallback). Prints a message showing the derived project ID.
- If not in a git repo and no `--project` given, errors with a clear message explaining how to specify a project
- If `~/.kerf/` doesn't exist yet, creates the bench directory structure automatically
- Initializes `spec.yaml` with defaults
- Loads the jig definition and emits it so the agent knows the process
- Records session info in metadata (session ID if available)
- Takes a snapshot of the initial state
**Output:** Confirmation, the jig's process overview, and next steps to begin the first pass.

### `kerf list [--status <status>] [--project <project-id>] [--all]`
**Purpose:** Show all works on the bench.  
**Output:**
```
On the bench for acme-webapp:
  auth-rewrite     feature   research (3/5 components)   2h ago
  login-timeout    bug       reproducing                  1d ago
  
  Dependencies: auth-rewrite -> database-migration [decomposition]

Commands:
  kerf show <codename>      View work details
  kerf resume <codename>    Resume working on a work
  kerf new                  Start a new work
```

### `kerf show <codename>`
**Purpose:** Display full work details.  
**Output:** spec.yaml contents, file tree, current pass description, session history, dependencies, and the jig's guidance for the current pass.

### `kerf resume <codename>`
**Purpose:** Load context for resuming work on a shelved work.  
**Behavior:**
- Reads SESSION.md (if it exists) and spec.yaml
- Loads the jig's instructions for the current pass
- Records a new session entry in spec.yaml
- Takes a snapshot of the current state
- If SESSION.md is missing (e.g., session terminated unexpectedly), outputs a degraded context summary from spec.yaml and existing artifacts — enough to continue, but without interpreted state
**Output:** Work state summary: where the work is, what's been done, open questions, current pass instructions, and suggested next actions. Kerf does not launch an agent session — the agent (or human) reads this output to orient.

### `kerf shelve [codename] [--force]`
**Purpose:** Pause work with state preservation.  
**Behavior:**
- If codename is omitted, infers from `active_session` in the current project (errors if ambiguous or none active)
- Takes a snapshot of the current state
- Marks the active session as ended in spec.yaml
- Emits instructions to the agent: "Write SESSION.md with: current pass, decisions made, open questions, next steps"
- `--force`: clears a stale `active_session` (useful after crashes where the session ended without `kerf shelve`)
**Output:** Mechanical confirmation + agent instructions for SESSION.md.

**Note:** If a session terminates unexpectedly (crash, Ctrl+C), `kerf shelve` never runs and SESSION.md may not be written. The raw artifacts and spec.yaml still provide enough context for a future `kerf resume`, just without the interpreted state summary. The `active_session` field will appear stale; kerf warns about this on the next interaction.

### `kerf finalize <codename> [--branch <name>]`
**Purpose:** Complete the work and hand off to implementation.  
**Behavior:**
- Validates the work is in a finalizable state (runs `square` checks)
- Refuses to finalize if the target repo has uncommitted changes
- Kerf performs the mechanical steps:
  - Creates a git branch in the target repo using `--branch` name (required — the agent chooses the name based on work context)
  - Copies work artifacts into `.kerf/{codename}/` in the repo (configurable via `finalize.repo_spec_path`)
  - Creates an initial commit with the work artifacts
  - Updates spec.yaml with branch name and commit refs
  - Changes status to `finalized`
- Emits instructions for agent-driven follow-up: create a PR, notify the team, link external systems
**Output:** Step-by-step results of the mechanical steps + agent instructions for follow-up.

### `kerf square <codename>`
**Purpose:** Structural verification — check if a work is square against jig requirements.  
**Behavior:**
- Checks status is at or past the jig's "ready" equivalent
- Validates expected files from the jig exist on disk
- Checks dependency works are in a complete status
- Reports what's true and what's off
- Does NOT verify content quality — that's the human's or a review agent's job
**Output:** Pass/fail with details on what's square and what needs attention.

### `kerf status <codename> [new-status]`
**Purpose:** Get or set a work's status.  
**Behavior:**
- With no new-status: shows current status and the jig's defined status progression
- With new-status: updates spec.yaml. Warns (but does not error) if the new status is not in the jig's recommended list.
- Takes a snapshot on status change
**Output:** Current status, available statuses from jig, and context.

### `kerf jig list`
**Purpose:** Show available jigs.

### `kerf jig show <name>`
**Purpose:** Display a jig's full definition — passes, file structure, status values, agent instructions.

### `kerf jig save <name> [--from <path>]`
**Purpose:** Save/create a jig definition in the user's jigs directory.

### `kerf jig load <name> <path-or-url>`
**Purpose:** Load a jig definition from an external source.

### `kerf jig sync`
**Purpose:** Future — sync jigs from a remote source (team-shared jigs).

### `kerf config [key] [value]`
**Purpose:** View or modify bench configuration.

### `kerf snapshot <codename> [--name <label>]`
**Purpose:** Manually trigger a versioning snapshot of the current work state. Snapshots also happen automatically on kerf command invocations (new, resume, shelve, finalize, status changes). The `--name` flag adds a human-readable label (e.g., `before-research`, `post-review`).

### `kerf history <codename>`
**Purpose:** Show the version history of a work — timestamped snapshots with diffs available.

### `kerf restore <codename> <snapshot>`
**Purpose:** Restore a work to a previous snapshot state.  
**Behavior:**
- Warns if there is an active session (restored spec.yaml may not reflect the current session)
- Takes a snapshot of the current state before restoring (so the restore is reversible)
- Copies the snapshot's files over the current work directory
- Preserves the current `active_session` and `sessions` entries in spec.yaml (only restores artifact files and status)
**Output:** Confirmation of what was restored, with a note about the pre-restore snapshot.

### `kerf archive <codename>`
**Purpose:** Move a work off the active bench into archive storage.  
**Behavior:**
- Moves the work directory to `~/.kerf/archive/{project-id}/{codename}/`
- Work no longer appears in `kerf list` (unless `--all` is used)
- To un-archive, manually move the directory back from `~/.kerf/archive/` to `~/.kerf/projects/` (a dedicated `kerf unarchive` command can be added later if needed)
**Output:** Confirmation.

### `kerf delete <codename>`
**Purpose:** Permanently remove a work from the bench.  
**Behavior:**
- Requires confirmation (prints work summary first)
- Removes the work directory entirely
- Does not affect any finalized copies in the target repo
**Output:** Confirmation.

## Future Commands (not in v1, documented for design context)

### `kerf sync [--remote <url>]`
**Purpose:** Sync the bench to a remote server.  
**Context:** For team use. Enables shared visibility, backup, and collaboration.

### `kerf serve [--port <port>]`
**Purpose:** Run a local or network-accessible server that serves the bench.  
**Context:** For team use. Provides API access for UIs, orchestrators, and other tools.

### `kerf link <codename> --jira <ticket> | --linear <id> | ...`
**Purpose:** Associate external system references with a work.  
**Context:** For integration with existing project management tools.
