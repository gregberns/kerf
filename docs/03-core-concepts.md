# Core Concepts

## Work

A **work** is a collection of structured documents describing a unit of work. It lives in its own directory on the bench. A work has:

- A **codename** — a short, immutable identifier. Auto-generated as an `adjective-noun` slug (e.g., `blue-bear`, `swift-maple`) if not provided, or user-chosen (e.g., `auth-rewrite`). Codenames are immutable once created — they are used as directory names, dependency references, and session associations. Codenames must be valid directory names: lowercase alphanumeric and hyphens only.
- A **title** — an optional human-friendly description (e.g., "User Authentication Redesign"). Unlike codenames, titles can be changed at any time.
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

A **session** links a work to an agent conversation. When a user starts working on a work, the session ID can be recorded in the work's metadata. This enables:

- **History** — the work tracks all sessions that have worked on it, with dates and notes
- **Handoff** — if a different person (or agent) needs to continue, the SESSION.md plus artifacts provide enough context to start a new session
- **Resume** — `kerf resume <codename>` outputs the work's current state (SESSION.md, current pass, jig instructions) so the agent can orient itself. Kerf does not launch or manage agent sessions — the human starts the session, and the agent uses kerf to load context.

Session ID recording is best-effort. When the human launches Claude via `claude --session-id <uuid>`, kerf can record that UUID. When the agent is already running and the session ID isn't discoverable, kerf records the session without an ID. The primary resumability mechanism is SESSION.md and the work's artifacts, not the session ID.

## Status

A work's **status** is a string indicating where it is in its lifecycle. Statuses are NOT a fixed enum — they're defined by the jig as a recommended list, but the system accepts any string.

This is important because:
- Different jigs have different passes/statuses
- An orchestrator should be able to assign whatever status makes sense to it
- The CLI emits the jig's status list in its output so agents follow conventions

When `kerf status` sets a value not in the jig's recommended list, it warns (but does not error). This catches typos like `reserach` without blocking custom statuses from orchestrators.

Example status progression for a feature jig:
```
problem-space -> decomposition -> research -> detailed-spec -> review -> ready
```

Note: statuses beyond `ready` (e.g., `implementing`, `done`) are orchestrator-defined, not part of the built-in jig. Kerf manages specs through `ready`; what happens after finalization is the responsibility of other tools.

Example for a bug jig:
```
triaging -> reproducing -> locating -> specifying-fix -> ready
```

## Square

**Square** is verification — checking that a work is true. Like holding a carpenter's square to a piece: are the angles right? Does everything line up?

`kerf square <codename>` runs structural verification checks against the work:
- Is the status at or past the jig's "ready" equivalent?
- Do all expected files from the jig exist on disk?
- Are dependency works in a complete status?

Square is a structural check, not a semantic one. It verifies that the expected artifacts exist and the workflow was followed, but it cannot verify content quality. That's the human's (or a review agent's) job.

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
    project: acme-webapp  # explicit project; omit for same project
    relationship: must-complete-first
  - codename: auth-service-spec
    relationship: inform  # doesn't block, but should be read for context
```

When `project` is omitted, the dependency is in the same project. Cross-project dependencies use the project ID (see Project Identity in data model).

## Bench

The **bench** is the root directory where all works live. Default: `~/.kerf/`.

Structure:
```
~/.kerf/
  config.yaml              # global configuration
  jigs/                    # user-level jig definitions
  projects/
    {project-id}/          # one directory per project (e.g., acme-webapp)
      {codename}/          # one directory per work
```

The bench is intentionally outside any git repo so that:
- All worktrees for the same repo share the same works
- No git ceremony is required for spec work
- Future sync mechanisms can operate independently of git

Each project is identified by a **project ID** stored in `.kerf/project-identifier` in the repo root (committed to git). Derived from the git remote on first use (e.g., `acme-webapp`), user-overridable.

A root-level `~/.kerf/config.yaml` can contain cross-project settings, default jigs, sync configuration, and finalization procedures.
