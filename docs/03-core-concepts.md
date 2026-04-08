# Core Concepts

## Work

A **work** is a collection of structured documents describing a unit of work. It lives in its own directory on the bench. A work has:

- A **codename** — a short, human-friendly identifier (e.g., `auth-rewrite`, `login-timeout-bug`)
- A **type** — what kind of work this is (feature, bug, migration, etc.)
- A **jig** — which process/workflow governs this work
- A **status** — where in the process this work currently is
- An **index file** (`spec.yaml`) — the source of truth for metadata
- **Artifact files** — the actual spec documents, produced by the agent during the process
- A **session file** (`SESSION.md`) — agent-written state for resumability

A work progresses through passes defined by its jig. At any point it can be shelved (paused) and resumed.

## Jig

A **jig** defines the process an agent follows when working on a work. Like a woodworking jig — a repeatable guide for precise cuts. It includes:

- **Passes** — the ordered steps of the process (e.g., rough cut: problem space -> decomposition -> research -> fine cut: detailed spec -> integration -> handoff)
- **File structure** — what files/directories get created at each pass
- **Agent instructions** — the prompts and guidance the agent uses at each pass
- **Status values** — the recommended status strings for each pass (defined as a list in the jig, emitted by the CLI so the agent follows conventions)

Jigs are markdown files with a defined structure. The tool ships with opinionated default jigs:
- `feature` — full spec process for new features or subsystems
- `bug` — reproduce, validate, locate, fix, verify
- (others TBD based on real usage)

Jigs can be customized per-user or per-project.

### Jig management
- `kerf jig list` — show available jigs
- `kerf jig show <name>` — display a jig's definition
- `kerf jig save <name> <path>` — save a jig definition
- `kerf jig load <name> <path-or-url>` — load a jig from a file or URL
- `kerf jig sync` — future: sync jigs from a remote source

## Session

A **session** links a work to a Claude Code conversation. When a user starts working on a work, the Claude session ID is recorded in the work's metadata. This enables:

- **Resume** — `kerf resume <codename>` can reopen the exact conversation where work left off
- **History** — the work tracks all sessions that have worked on it, with dates and notes
- **Handoff** — if a different person (or agent) needs to continue, the SESSION.md plus artifacts provide enough context to start a new session

Claude Code features used:
- `claude --resume <session-id>` — resume a specific conversation
- `claude --name "<codename>"` — name a session for discoverability
- Session storage: `~/.claude/projects/{path}/{uuid}.jsonl`

## Status

A work's **status** is a string indicating where it is in its lifecycle. Statuses are NOT a fixed enum — they're defined by the jig as a recommended list, but the system accepts any string.

This is important because:
- Different jigs have different passes/statuses
- An orchestrator should be able to assign whatever status makes sense to it
- The CLI emits the jig's status list in its output so agents follow conventions

Example status progression for a feature jig:
```
problem-space -> decomposition -> research -> detailed-spec -> review -> ready -> implementing -> done
```

Example for a bug jig:
```
triaging -> reproducing -> locating -> fixing -> verifying -> done
```

## Square

**Square** is verification — checking that a work is true. Like holding a carpenter's square to a piece: are the angles right? Does everything line up?

`kerf square <codename>` runs the jig's verification checks against the work. Is it complete? Are all passes done? Are dependencies satisfied? Is it ready for finalization?

## Dependencies

Works can declare dependencies on other works. This enables:

- **Visibility** — `kerf list` shows dependency status inline
- **Context loading** — when resuming a work, dependent works' current state can be loaded so the agent knows what's decided vs. in-flux
- **Ordering** — an external orchestrator can build a DAG and determine implementation order
- **Blocking** — finalization can warn if dependencies aren't complete

Dependencies are declared in `spec.yaml`:
```yaml
depends_on:
  - codename: database-migration
    relationship: must-complete-first
  - codename: auth-service-spec
    relationship: inform  # doesn't block, but should be read for context
```

## Bench

The **bench** is the root directory where all works live. Default: `~/.kerf/`.

Structure:
```
~/.kerf/
  config.yaml              # global configuration
  jigs/                    # user-level jig definitions
  projects/
    {repo-identifier}/     # one directory per linked repository
      {codename}/          # one directory per work
```

The bench is intentionally outside any git repo so that:
- All worktrees for the same repo share the same works
- No git ceremony is required for spec work
- Future sync mechanisms can operate independently of git

A root-level `~/.kerf/config.yaml` can contain cross-project settings, default jigs, sync configuration, and finalization procedures.
