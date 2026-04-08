# CLI Design

## Executable

**Name:** `kerf`  
**Tagline:** *Measure twice, cut once.*

## Design Philosophy

1. **Agent-first output.** Every command's output is designed to be consumed by an AI agent. It includes context, current state, and suggested next actions. Human-readable too, but agent-consumable is the priority.

2. **No-arg root command = quick start.** Running `kerf` with no arguments prints a quick-start guide that gives an agent everything it needs to interact with the tool. This eliminates the need for agents to crawl through `--help` subcommands.

3. **Commands emit instructions.** Commands like `shelve` and `finalize` don't just perform mechanical operations — they also emit instructions telling the agent what additional steps it should take (write SESSION.md, update status, sync, etc.). This makes the CLI a collaborative partner with the agent, not just a passive tool.

4. **Contextual output.** `list` shows not just data but also the commands you'd likely want to run next. `show` displays not just metadata but the file tree and jig context.

## Commands

### `kerf` (no arguments)
**Purpose:** Quick-start guide for agents and humans.  
**Output:** Overview of the tool, available commands with brief descriptions, example workflows, and the current bench summary (number of active works, etc.).

### `kerf new [codename] [--type feature|bug|...] [--jig <name>] [--project /path/to/repo]`
**Purpose:** Create a new work.  
**Behavior:**
- If no codename given, generates a placeholder or prompts
- Creates directory under `~/.kerf/projects/{repo-id}/{codename}/`
- Initializes `spec.yaml` with defaults
- If `--project` not given, infers from current working directory
- Loads the jig definition and emits it so the agent knows the process
- Records the current Claude session ID in metadata
**Output:** Confirmation, the jig's process overview, and instructions to begin the first pass.

### `kerf list [--status <status>] [--project <path>] [--all]`
**Purpose:** Show all works on the bench.  
**Output:**
```
On the bench for github/myapp:
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
**Purpose:** Resume work on a shelved work.  
**Behavior:**
- Looks up the latest session ID from spec.yaml
- Attempts `claude --resume <session-id>`
- If session is unavailable, starts a new session with context loaded
- Updates session tracking in spec.yaml
**Output:** Session resumed or new session started with context summary.

### `kerf shelve [codename]`
**Purpose:** Pause work with state preservation.  
**Behavior:**
- Snapshots current state
- Updates spec.yaml status
- Emits instructions to the agent: "Write SESSION.md with: current pass, decisions made, open questions, next steps"
- Records session end in metadata
**Output:** Mechanical confirmation + agent instructions for SESSION.md.

### `kerf finalize <codename>`
**Purpose:** Complete the work and hand off to implementation.  
**Behavior:**
- Validates the work is in a finalizable state (jig-defined)
- Executes the finalization procedure (defined at ~/.kerf/ or project level):
  - Create a git branch in the target repo
  - Copy work artifacts into the repo (location configurable)
  - Generate tasks/beads from the work
  - Update spec.yaml with branch name, PR info, commit refs
  - Change status to the jig's "ready" equivalent
- Emits post-finalization instructions (e.g., "Create a PR", "Notify the team")
**Output:** Step-by-step results of the finalization procedure + next actions.

### `kerf square <codename>`
**Purpose:** Check if a work is square — verification against jig requirements.  
**Behavior:**
- Checks all passes are complete
- Validates expected files exist
- Checks dependencies are satisfied
- Reports what's true and what's off
**Output:** Pass/fail with details on what's square and what needs attention.

### `kerf status <codename> [new-status]`
**Purpose:** Get or set a work's status.  
**Behavior:**
- With no new-status: shows current status and the jig's defined status progression
- With new-status: updates spec.yaml
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

### `kerf snapshot <codename>`
**Purpose:** Manually trigger a versioning snapshot of the current work state. (Auto-snapshots happen on writes, but this allows explicit named snapshots.)

### `kerf history <codename>`
**Purpose:** Show the version history of a work — timestamped snapshots with diffs available.

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
