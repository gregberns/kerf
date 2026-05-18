# Commands

> Complete command reference for the kerf CLI. Every command with its syntax, behavior, output, and error conditions.

For CLI design principles and output conventions, see [cli.md](cli.md). For data model details referenced throughout, see [works.md](works.md) (spec.yaml schema), [architecture.md](architecture.md) (bench layout, config.yaml), [jig-system.md](jig-system.md) (jig format and resolution), [sessions.md](sessions.md) (session tracking), and [snapshots.md](snapshots.md) (versioning).

## Global Flags

These flags are accepted by all commands:

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Display help for the command. |
| `--project <project-id>` | Override project inference. Uses this project ID instead of reading `.kerf/project-identifier` from the current working directory. |

When `--project` is not provided, kerf infers the project ID from `.kerf/project-identifier` in the nearest git repository root above the current working directory. If the current directory is not inside a git repository and no `--project` flag is given, kerf uses `default_project` from `config.yaml` (see [architecture.md](architecture.md)). If none of these resolve, commands that require a project ID error.

---

## `kerf` (no arguments)

### Purpose

Quick-start guide for agents and humans. The primary onboarding surface — an agent with zero prior context can use kerf effectively after reading this output.

### Syntax

```
kerf
```

### Behavior

1. If the bench (`~/.kerf/`) does not exist, kerf outputs a getting-started message explaining that no bench exists yet and that `kerf new` will create one.
2. If the bench exists, kerf assembles a summary of the current state.

### Output

The output includes all of the following:

- One-line description of what kerf does.
- Available commands with brief descriptions and usage examples.
- The standard workflow: `kerf new` -> work through passes -> `kerf shelve` / `kerf finalize`.
- Bench summary: number of active works in the current project (if inside a repo), total active works across all projects.
- If no bench exists, instructions for getting started (`kerf new`).

### Errors

None. This command always succeeds.

---

## `kerf new`

### Purpose

Create a new [work](works.md) on the bench.

### Syntax

```
kerf new [codename] [--title <title>] [--type <type>] [--jig <name>] [--area <name>...] [--bead-filter <expr>] [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | No | Auto-generated `adjective-noun` slug | Immutable identifier for the work. Must match `[a-z0-9]+(-[a-z0-9]+)*`. |
| `--title` | No | `null` | Human-friendly title for the work. |
| `--type` | No | Matches jig name | Work type (e.g., `feature`, `bug`). |
| `--jig` | No | `default_jig` from config.yaml (required if `default_jig` unset) | Jig to use for this work. Resolved via jig resolution order (see [jig-system.md](jig-system.md)). |
| `--area` | No | `[]` | One or more area names to associate with the work. May be repeated (e.g., `--area auth --area api`). Each name must exist in `areas.yaml`. |
| `--bead-filter` | No | — | A single bead-filter clause to write into the new work's `spec.yaml` as its per-work `bead_filter`. Accepts either `label=<value>` or `id_prefix=<value>` (e.g., `--bead-filter 'label=subsystem:bridge'`). One-shot: not repeatable on `kerf new` — to compose a multi-clause `any:` union, create the work with one clause and then add additional clauses via `kerf work edit --bead-filter-add` (which is repeatable). See [coordination.md](coordination.md#bead-attachment). |
| `--project` | No | Inferred from `.kerf/project-identifier` | Project to create the work under. |

### Behavior

1. **Resolve project identity.**
   - If `--project` is given, use it.
   - Otherwise, look for `.kerf/project-identifier` in the current repo.
   - If this is the first kerf use in a repo (no `.kerf/project-identifier` exists), derive the project ID from the git remote (or directory name as fallback), write it to `.kerf/project-identifier`, and print a message showing the derived project ID.
   - If not in a git repo and no `--project` given, error.
2. **Create the bench** if `~/.kerf/` does not exist. Create the `projects/` subdirectory and any needed project directory.
3. **Resolve codename.** If no codename argument is provided, auto-generate an `adjective-noun` slug (e.g., `blue-bear`, `swift-maple`). Validate the codename format. Error if a work with this codename already exists in the project.
4. **Resolve jig.** Look up the jig via the resolution order (see [jig-system.md](jig-system.md)). Error if the jig is not found.
5. **Create the work directory** at the location dictated by the project's storage mode: `~/.kerf/projects/{project-id}/{codename}/` in bench mode, or `{repo}/.kerf/works/{codename}/` in local mode. If local mode is active and the bench symlink at `~/.kerf/projects/{project-id}` does not yet exist, kerf creates it pointing at `{repo}/.kerf/works/`. See [architecture.md](architecture.md#storage-modes).
6. **Initialize `spec.yaml`** with: codename, title, type, project ID, jig name, jig version, initial status (first value in the jig's `status_values`), `created` and `updated` timestamps, empty `sessions` list, empty `depends_on` list, empty `related_to` list, null `implementation` fields, the jig's `status_values` list, the `areas` list from `--area` flags (empty if none provided), an empty `pinned_beads` list, and a `bead_filter` key. The `bead_filter` key is **always present** in the emitted `spec.yaml`: when `--bead-filter` is given, its value is built from the supplied clause (written as a direct, non-union clause); when omitted, the key is emitted with an empty value (`bead_filter:`) so the work is identifiable as `unwired` rather than silently absent. Multi-clause `any:` unions are composed post-creation via `kerf work edit --bead-filter-add` (see [coordination.md](coordination.md#bead-attachment)). For filter resolution, "absent key" and "present-but-empty key" are equivalent — only the latter is canonical for new works (the schema detail lives in [works.md](works.md)). Drift baseline is not advanced (see [coordination.md](coordination.md#baseline-advancement)).
7. **Check area overlap.** If `--area` flags were provided, scan other active (non-archived) works in the project for overlapping areas. If any other work shares an area, emit an overlap warning in the output (see Output below). This is informational — it does not block creation.
8. **Record session.** Append a session entry to `sessions` with the current timestamp and `ended: null`. Set `active_session`.
9. **Take a snapshot** of the initial state (see [snapshots.md](snapshots.md)).

### Output

- Confirmation: work created, codename, project ID, jig name.
- If areas were assigned and other works share those areas, an overlap warning:
  ```
  Area overlap:
    auth — also touched by: token-refresh (status: research), session-mgmt (status: tasks)
  ```
- The jig's process overview: list of passes with descriptions.
- Agent instructions for the first pass (from the jig's markdown body).
- Next steps block: how to begin the first pass, where to write artifacts.
- A fenced final block naming the work's storage location, so an agent that scrolls to the end of the output can find the canonical paths without re-deriving them:

  ```
  working directory: {bench-or-local-path-to-the-work}
  repo-side files:   .kerf/project-identifier (committed); add agent instructions to your config file (CLAUDE.md, AGENTS.md, etc.)
  ```

  The `working directory:` line resolves to `~/.kerf/projects/{project-id}/{codename}/` in bench mode or `{repo}/.kerf/works/{codename}/` in local mode. The block always appears as the last lines of `kerf new` output.

### Errors

| Condition | Message |
|-----------|---------|
| Not in a git repo and no `--project` flag | `Error: not in a git repository. Use --project <project-id> to specify a project.` |
| Codename already exists in project | `Error: work '{codename}' already exists in project '{project-id}'.` |
| Codename format invalid | `Error: codename must be lowercase alphanumeric and hyphens (matching [a-z0-9]+(-[a-z0-9]+)*).` |
| Jig not found | `Error: jig '{name}' not found. Run 'kerf jig list' to see available jigs.` |
| Area name not in `areas.yaml` | `Error: area '{name}' not found. Run 'kerf areas list' to see defined areas, or 'kerf areas add <name>' to create one.` |
| `--bead-filter` value does not parse as `label=<value>` or `id_prefix=<value>` | `Error: --bead-filter expects 'label=<value>' or 'id_prefix=<value>', got '{value}'.` |
| `default_jig` unset and no `--jig` flag | See First-Run Onboarding below. |

### First-Run Onboarding

When `default_jig` is not configured and no `--jig` flag is provided, `kerf new` fails with:

```
Error: No default workflow configured.

How do you want to use kerf?

  kerf config default_jig plan
    Write a plan before changing code. Best for existing projects.
    You describe what to change → kerf guides you through planning →
    you get an implementation-ready spec and task list.

  kerf config default_jig spec
    Maintain a living spec that defines your system. Best for new projects.
    The spec is always right. Code that doesn't match the spec is wrong.
    Changes start as spec updates, then flow to code.

Or specify for just this work:  kerf new my-feature --jig plan
```

This is not interactive. It is an error with actionable instructions. An agent can parse the output and run the appropriate `kerf config` command. A human can read and choose. After the user sets `default_jig` (or uses `--jig`), subsequent `kerf new` commands work without this message.

---

## `kerf list`

### Purpose

Show all works on the bench.

### Syntax

```
kerf list [--status <status>] [--project <project-id>] [--all] [--created-by <self|all>]
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--status` | No | — | Filter to works with this status. |
| `--project` | No | Inferred from cwd | Show works for this project. |
| `--all` | No | `false` | Include archived works. |
| `--created-by <self\|all>` | No | `all` | Filter by work creator. `self` restricts the list to works created by the current agent's session identity; `all` (the default) lists every work. When multi-agent works are present and `--created-by all` is in effect, each row carries an attribution marker so the active agent can tell its own works apart from others. <!-- TBD: open question 4 from plan 019 — whether per-session attribution lives on spec.yaml or needs a sessions.md schema extension. --> |

### Behavior

1. Resolve the project ID (from `--project` flag or cwd inference).
2. Read all work directories under `~/.kerf/projects/{project-id}/`. For each, read `spec.yaml` to get codename, type, status, and `updated` timestamp.
3. If `--all` is set, also read works from `~/.kerf/archive/{project-id}/`.
4. If `--status` is set, filter to works matching that status.
5. Sort works by `updated` timestamp, most recent first.
6. Read dependency information from each work's `spec.yaml`.

### Output

```
On the bench for {project-id}:
  {codename}     {type}   {status}   {relative-time}
  {codename}     {type}   {status}   {relative-time}

  Dependencies: {codename} -> {dep-codename} [{dep-status}]

Commands:
  kerf show <codename>      View work details
  kerf resume <codename>    Resume working on a work
  kerf new                  Start a new work
```

- Each work is listed with its codename, type, current status, and time since last update.
- If any works have dependencies, a Dependencies section shows them with the dependency's current status.
- Archived works (when `--all` is set) are marked with `[archived]`.
- A Commands block suggests likely next actions.
- If no works exist, output says so and suggests `kerf new`.
- When `kerf list` is run inside a git repo that has uncommitted changes and at least one active (non-complete, non-archived) work exists for the project, the output may include an informational hint at the end suggesting `kerf new --jig retrofit`. The hint is best-effort and non-blocking: if git is unavailable or the check fails for any reason, no hint is shown. See [jig-retrofit.md](jig-retrofit.md).

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

---

## `kerf show`

### Purpose

Display full details for a work.

### Syntax

```
kerf show <codename> [--compact]
```

### Arguments and Flags

| Argument/Flag | Required | Description |
|---------------|----------|-------------|
| `codename` | Yes | The work to display. |
| `--compact` | No | One-line-per-section summary: status, next-pass name, file count, last-session marker. Skips the full jig instructions, file tree, attached-beads listing, and session history. Useful when an agent only needs to know "where is this work and what's next." |

### Behavior

1. Resolve the project ID.
2. Read `spec.yaml` from the work directory.
3. Read SESSION.md if present.
4. Load the jig definition for the work.
5. List the files in the work directory.
6. Read dependency status for each entry in `depends_on`.

### Output

The output includes:

- **Metadata**: codename, title, type, status, project ID, jig name and version, created and updated timestamps.
- **Bead filter**: the work's `bead_filter` slot, always rendered as a single line. The literal value is shown when present (e.g., `bead_filter: label=subsystem:bridge`); when the key is absent or empty, the line reads `bead_filter: (none)` so an agent can tell `unwired` apart from a populated filter at a glance.
- **Jig context**: the pass corresponding to the current status, with the jig's agent instructions for that pass. The pass-list rendering uses a stable per-pass line of the form `Pass N: <name> → Output: NN-<filename>.md` so an agent can locate the canonical output path for each pass without re-deriving the filename convention. The convention itself (`NN-<short-name>.md` for content passes) lives in [jig-system.md](jig-system.md).
- **File tree**: all files in the work directory (excluding `.history/`).
- **Session history**: the `sessions` list from `spec.yaml`, with active session highlighted.
- **Dependencies**: each dependency's codename, project, relationship, and current status.
- **SESSION.md contents**: the full text of SESSION.md, if present.
- **Pass status** (for implementation works): when the work uses a composable jig (e.g., `jig-implementation`), show the completion status of each pass. Example:
  ```
  Pass status:
    breakdown:  done
    dispatch:   done
    implement:  3/7 beads complete
    review:     0/3 beads reviewed
  ```
- **Bead status** (when available): when the work has beads (produced by the breakdown pass), show a summary of bead state:
  ```
  Beads: 7 total, 3 closed, 4 open, 1 with unresolved review feedback
  ```
  If no beads exist, this section is omitted.
- **Attached beads** (when the work has any beads attached by filter or pin): a per-bead listing, sourced from the resolved `bead_filter` (see [coordination.md](coordination.md#bead-attachment)) composed with the work's `pinned_beads` list (see [coordination.md](coordination.md#pin-layer)). Open beads come first, then closed beads. Each line carries the bead ID, status, title, and any drift markers computed against the cached baseline (see [coordination.md](coordination.md#drift-detection)). Pinned beads are annotated `(pinned)`.

  ```
  Attached beads (4 open / 3 closed):
    hk-cb-042  open    wire retry into adapter
    hk-cb-051  open    extract header parser
    hk-cb-099  open    investigate flaky timeout                   (pinned)
    hk-cb-101  open    add idempotency guard                       ! closed externally since last triage
    hk-cb-030  closed  scaffold adapter
    hk-cb-031  closed  swap stub for real client
    hk-cb-040  closed  add retry envelope                          ! reopened externally since last triage
  ```

  Drift markers correspond to the categories in [coordination.md](coordination.md#drift-categories): `! closed externally since last triage`, `! reopened externally since last triage`, `! deleted externally since last triage`, `! new since last triage`. When no baseline exists, drift markers are omitted. When the work has zero attached beads, this section is omitted (the `Bead status` line above already covers the empty case).
- **Commands block**: contextually relevant next actions:

```
Commands:
  kerf resume <codename>                 Resume working
  kerf status <codename> <next-status>   Advance status
  kerf square <codename>                 Verify completeness
  kerf shelve <codename>                 Pause work
```

#### `--compact` output

Under `--compact`, the output collapses to four lines plus the bead-filter slot:

```
{codename}  status: {current-status} → next: {next-pass-name}
bead_filter: {value or (none)}
files:       {n} in work directory
last session: {relative-time} ({active|ended})
```

The compact form omits jig instructions, file tree, attached-beads listing, and session history. Errors and command-line resolution behave identically to the full form.

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |

---

## `kerf resume`

### Purpose

Load context for resuming work on a shelved work. kerf does not launch an agent session — the agent (or human) reads this output to orient.

### Syntax

```
kerf resume <codename>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to resume. |

### Behavior

1. Resolve the project ID.
2. Read `spec.yaml` for the target work.
3. If `active_session` is non-null, error — the work has an active session. Direct the user to `kerf shelve` or `kerf shelve --force` first. See [sessions.md](sessions.md) for stale session handling.
4. Record a new session entry in `sessions` with the current timestamp and `ended: null`. Set `active_session` to the new session's ID (or `"anonymous"` if no session ID is available).
5. Update the `updated` timestamp in `spec.yaml`.
6. Take a [snapshot](snapshots.md) of the current state.
7. Load the jig definition and determine the current pass from the work's status.
8. Read SESSION.md if present. If absent, operate in degraded mode.

### Output

The resume context block contains:

- **Work metadata**: codename, title, type, status, project ID.
- **SESSION.md contents**: the full text of SESSION.md, if present. If absent, a notice: `SESSION.md not found — resuming without interpreted session state.`
- **Current pass**: the jig pass corresponding to the current status, with the jig's full agent instructions for that pass.
- **Session history**: previous sessions from `spec.yaml`.
- **Dependency status**: current status of each work in `depends_on`.
- **File listing**: files present in the work directory.
- **Area overlap**: when the work has areas and other active works share them, list the shared areas and the overlapping codenames. Same format as `kerf show`.
- **Next steps**: suggested actions based on the current pass and SESSION.md content.
- **Retrofit hint** (optional): when the repo has uncommitted changes, `kerf resume` may append an informational suggestion to consider `kerf new --jig retrofit`. The hint is non-blocking and silently skipped if git is unavailable. See [jig-retrofit.md](jig-retrofit.md).

### Degraded Mode

When SESSION.md is missing, kerf substitutes a context summary assembled from `spec.yaml` and existing artifact files. The agent can continue working but lacks the interpreted state (decisions, open questions, next steps) that SESSION.md provides. See [sessions.md](sessions.md) for details.

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Active session exists | `Error: work '{codename}' has an active session (started {timestamp}). Run 'kerf shelve {codename}' or 'kerf shelve --force {codename}' to end it before resuming.` |

---

## `kerf shelve`

### Purpose

Pause work with state preservation.

### Syntax

```
kerf shelve [codename] [--force]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | No | Inferred from `active_session` in current project | The work to shelve. |
| `--force` | No | `false` | Clear a stale `active_session` without emitting SESSION.md instructions. |

### Behavior (normal shelve)

1. **Resolve the target work.**
   - If `codename` is provided, use it.
   - If omitted, scan all works in the current project for one with a non-null `active_session`. Error if zero or more than one match.
2. Take a [snapshot](snapshots.md) of the current work state.
3. Set the `ended` timestamp on the active session entry in `spec.yaml` to the current time.
4. Set `active_session` to `null`.
5. Update the `updated` timestamp in `spec.yaml`.
6. Emit instructions directing the agent to write SESSION.md.

### Behavior (`--force`)

1. Resolve the target work (codename required when using `--force`).
2. Set the `ended` timestamp on the active session entry in `spec.yaml` to the current time.
3. Set `active_session` to `null`.
4. Take a [snapshot](snapshots.md).
5. Do **not** emit SESSION.md instructions (the original agent is no longer present).

### Output

**Normal shelve:**

```
Work {codename} shelved.

Before ending this session, write SESSION.md in the work directory with:
- Current pass and progress within it
- Decisions made during this session
- Open questions
- Suggested next steps
- Reading order for a new session picking this up

Path: ~/.kerf/projects/{project-id}/{codename}/SESSION.md
```

**Force shelve:**

```
Work {codename} force-shelved. Stale session cleared.
```

### Errors

| Condition | Message |
|-----------|---------|
| Codename omitted, no active session found in project | `Error: no active session found in project '{project-id}'. Specify a codename.` |
| Codename omitted, multiple active sessions in project | `Error: multiple active sessions in project '{project-id}': {list}. Specify a codename.` |
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Work has no active session (normal shelve) | `Error: work '{codename}' has no active session to shelve.` |

---

## `kerf finalize`

### Purpose

Complete a work and hand off to implementation. Copies work artifacts from the bench into the git repository and creates a branch with an initial commit. See [finalization.md](finalization.md) for the full finalization process.

### Syntax

```
kerf finalize <codename> --branch <name>
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to finalize. |
| `--branch` | Yes | — | Git branch name to create in the target repository. The agent chooses the name based on work context. |

### Behavior

1. Resolve the project ID and target repository path.
2. **Pre-flight checks:**
   - Run `kerf square` checks on the work (see [verification.md](verification.md)). If square fails, report the issues and abort.
   - Check the target repository for uncommitted changes. If any exist, refuse to finalize.
   - Verify the `--branch` name does not already exist in the target repository. If it does, abort.
3. Take a [snapshot](snapshots.md) of the current work state.
4. **Create the git branch** in the target repository using the `--branch` name.
5. **Copy work artifacts** into the target repository at the path specified by `finalize.repo_spec_path` in config.yaml (default: `.kerf/{codename}/`). The token `{codename}` in the path is replaced with the work's codename. Excludes `spec.yaml`, `SESSION.md`, and `.history/`. See [finalization.md](finalization.md) for details.
6. **Spec-first finalization** (only for works with `jig: spec` in spec.yaml):
   - Read the `spec_path` config value (default: `specs/`).
   - If `{repo_root}/{spec_path}/` does not exist, create it.
   - Copy files from the work's `05-spec-drafts/` to `{repo_root}/{spec_path}/`, preserving filenames (1:1 mapping — `05-spec-drafts/jig-system.md` → `specs/jig-system.md`).
   - Exclude `05-spec-drafts/` from the standard artifact copy in step 5 (so spec files appear only in `spec_path`, not duplicated in `repo_spec_path`).
   - If `05-spec-drafts/` is empty or missing, warn but do not error — the standard artifact copy proceeds normally.
   - Detection is by jig name in spec.yaml, not by directory presence. Custom jigs that produce `05-spec-drafts/` do not get this behavior.
7. **Create an initial commit** in the target repository containing the copied artifacts.
8. **Update `spec.yaml`**: set `implementation.branch` to the branch name, append the commit hash to `implementation.commits`.
9. **Set status** to `finalized`.
10. Update the `updated` timestamp in `spec.yaml`.

### Output

Step-by-step results of the mechanical operations:

```
Finalizing {codename}...
  Square check: passed
  Branch created: {branch-name}
  Artifacts copied to: {repo-spec-path}
  Commit: {short-hash} — {commit-message}
  Status: finalized

Next steps:
  - Create a pull request for branch '{branch-name}'
  - Notify the team / link external systems
  - Run 'kerf archive {codename}' when implementation is complete
```

For spec-first works (`jig: spec`), the output additionally shows:

```
  Spec drafts applied to: {spec-path}
```

If `05-spec-drafts/` is empty or missing:

```
  Warning: 05-spec-drafts/ is empty or missing — no spec drafts to apply.
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Square check fails | `Error: work '{codename}' is not square. {details}. Fix the issues and try again.` |
| Uncommitted changes in target repo | `Error: target repository has uncommitted changes. Commit or stash them before finalizing.` |
| Branch already exists | `Error: branch '{branch-name}' already exists in the target repository.` |
| `--branch` not provided | `Error: --branch is required. Specify the branch name for the finalized work.` |

---

## `kerf review`

### Purpose

Emit the canonical reviewer prompt for the work's current pass. `kerf review` is the harness-agnostic surface for the jig's review gate: it prints the review criteria, the artifact paths the reviewer is asked to read, and references to prior-pass output. The calling harness then dispatches its own reviewer primitive — a sub-agent via the Agent tool, the parent orchestrator's own attention, or a fresh-context re-read of the artifact — against this prompt. See [jig-system.md](jig-system.md) for the three review-primitive fallback paths in preference order.

### Syntax

```
kerf review <codename> [--pass <name>] [--format <format>] [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to review. |
| `--pass <name>` | No | Current pass | Render the reviewer prompt for a specific pass instead of the current one. Useful when the agent wants to re-trigger a review on a previously-completed pass. |
| `--format <format>` | No | `text` | Output format. `text` (default) or `json`. |
| `--project <project-id>` | No | Inferred from cwd | The project containing the work. |

### Behavior

1. Resolve the project ID.
2. Read the target work's `spec.yaml`. Error if the work does not exist.
3. Resolve the pass — either `--pass` if given, or the pass corresponding to the work's current status.
4. Load the jig's review block for that pass: the `Done when reviewer approves on:` criteria list (the single normative block replacing the prior split between "What done looks like" and "Review Criteria" — see [jig-spec.md](jig-spec.md) for the spec-jig instance and [jig-system.md](jig-system.md) for the convention).
5. Render the prompt as stdout text (or a JSON record under `--format=json`). `kerf review` does not dispatch the reviewer itself. <!-- TBD: open question 1 from plan 020 — whether kerf review should optionally auto-dispatch when an Agent tool is detected; spec follows the plan's print-only default. -->

### Output (text)

```
Reviewer prompt for {codename} — pass: {pass-name}

Artifacts to read:
  {path/to/current-pass-output}
  {path/to/prior-pass-output (if referenced)}

Done when the reviewer approves on:
  - {criterion-1}
  - {criterion-2}
  - {criterion-N}

The reviewer returns either:
  - "Approved" — the pass is ready to advance via 'kerf status {codename} <next>'
  - "Changes requested: <list>" — the agent addresses each item and re-requests review
```

### Output (`--format=json`)

```
{
  "codename":  "{codename}",
  "pass":      "{pass-name}",
  "artifacts": ["..."],
  "criteria":  ["..."]
}
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Unknown pass name in `--pass` | `Error: pass '{value}' is not declared in jig '{jig-name}'. Known passes: {list}.` |
| Jig has no review block for the resolved pass | `Error: jig '{jig-name}' declares no review criteria for pass '{pass-name}'.` |

---

## `kerf preview`

### Purpose

Read-only render of a future pass's instructions without advancing status. `kerf preview` is the look-ahead surface for the spec jig (and any other multi-pass jig): the agent inspects what the next pass will require before committing to the transition.

### Syntax

```
kerf preview <codename> <status> [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to preview against. |
| `status` | Yes | — | The status whose pass instructions should be rendered. May be any value in the jig's `status_values` list. |
| `--project` | No | Inferred from cwd | The project containing the work. |

### Behavior

1. Resolve the project ID.
2. Read the target work's `spec.yaml`. Error if the work does not exist.
3. Resolve the jig and look up the pass corresponding to `status`. Error if `status` is not in the jig's `status_values`.
4. Render the pass instructions using the same renderer as [`kerf show`](#kerf-show), scoped to the named pass. The output is marked read-only in the header so an agent does not mistake the preview for a transition confirmation.

### Output

```
Preview for {codename} — pass: {pass-name} (read-only, status unchanged)

{full jig instructions for that pass}

Output: NN-{filename}.md
```

The status on disk is not touched. Re-running `kerf preview` is always idempotent.

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Status not in jig's status_values | `Error: status '{value}' is not declared in jig '{jig-name}'. Known statuses: {list}.` |

---

## `kerf square`

### Purpose

Structural verification — check if a work is square against its [jig](jig-system.md) requirements. Square is a structural check, not a semantic one. It verifies that expected artifacts exist and the workflow was followed, but does not verify content quality. See [verification.md](verification.md) for the full verification specification.

### Syntax

```
kerf square <codename>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to verify. |

### Behavior

1. Resolve the project ID.
2. Read `spec.yaml` for the target work.
3. Load the jig definition.
4. Run the following checks:
   - **Status check**: Is the status at or past the jig's terminal status (`ready` or equivalent)? Determined by position in the jig's `status_values` list.
   - **File check**: Do all expected files defined in the jig's `file_structure` exist on disk in the work directory?
   - **Dependency check**: Are all `must-complete-first` dependency works in a complete status (at or past `ready`)?
5. Compile results.

### Output

```
Square check for {codename}:

  Status:        {pass|fail} — {current-status} (expected: {ready-equivalent} or later)
  Files:         {pass|fail} — {n}/{total} expected files present
    Missing:     {list of missing files, if any}
  Dependencies:  {pass|fail} — {n}/{total} blocking dependencies complete
    Incomplete:  {list of incomplete deps with their status, if any}

Result: {SQUARE | NOT SQUARE}
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |

---

## `kerf status`

### Purpose

Get or set a work's status.

### Syntax

```
kerf status <codename> [new-status] [--quiet]
```

### Arguments and Flags

| Argument/Flag | Required | Description |
|---------------|----------|-------------|
| `codename` | Yes | The work to query or update. |
| `new-status` | No | The status value to set. If omitted, displays current status. |
| `--quiet` | No | On a write, suppress the full jig-instructions block and emit only the single-line transition confirmation. Intended for scripted transitions and chains of status advances. Has no effect on read mode. |

### Behavior (read — no new-status)

1. Resolve the project ID.
2. Read `spec.yaml`.
3. Display the current status and the jig's status progression.

### Behavior (write — new-status provided)

1. Resolve the project ID.
2. Read `spec.yaml`.
3. If the new status is not in the jig's `status_values` list, emit a warning (but proceed).
4. Update `status` in `spec.yaml` to the new value.
5. Update the `updated` timestamp.
6. Take a [snapshot](snapshots.md).
7. **Pre-create the next pass's output directory.** Look up the resolved jig's pass list, find the pass corresponding to the new status, and `mkdir -p` any directory prefix in the pass's declared output paths that does not yet exist. The operation is idempotent — re-running on an existing directory does nothing. When the pass produces per-component output (e.g., pass-3 research, where one directory per affected spec area is needed), only the top-level pass directory is created from the status advance; component subdirectories are created when their names become known (typically from the prior pass's output). If a per-pass template ships with the jig, copy the template into place when the target file does not already exist. See [jig-system.md](jig-system.md) for the directory and template conventions.
8. Load the jig's agent instructions for the pass corresponding to the new status. Under `--quiet`, the instructions are suppressed and only the transition confirmation is printed.

### Output (read)

```
Work: {codename}
Status: {current-status}

Status progression ({jig-name} jig):
  {status-1} -> {status-2} -> ... -> {status-n}
                               ^^ current
```

### Output (write)

```
Status updated: {old-status} -> {new-status}

{jig instructions for the new pass, if any}

Next steps:
  {pass-specific guidance from the jig}
```

If the new status is not in the jig's recommended list:

```
Warning: '{new-status}' is not in the {jig-name} jig's recommended statuses.
Recommended: {status-1}, {status-2}, ..., {status-n}
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |

---

## `kerf jig list`

### Purpose

Show available jigs with phase, tool, and activation information.

### Syntax

```
kerf jig list [--phase <phase>]
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--phase` | No | — | Filter to jigs matching this SDLC phase (e.g., `planning`, `implementation`, `bug-fix`). |

### Behavior

1. Enumerate jigs from all resolution sources in order: user-level (`~/.kerf/jigs/`), then built-in defaults.
2. For each jig, read its frontmatter to extract name, description, version, phase, and tools.
3. If a user-level jig has the same name as a built-in jig, only the user-level jig appears (it overrides the built-in).
4. If `--phase` is provided, filter to jigs whose `phase` field matches the given value.
5. If inside a project with a `project.yaml` (see [architecture.md](architecture.md)), determine which jigs are active for the current project vs. available but not activated.
6. For composable jigs (jigs with `composable: true`), determine which passes are active from the project's `project.yaml` pass configuration.

### Output

**When inside a project with `project.yaml`:**

```
Jigs for {project-id}:

  Active:
    plan (also: feature)    Write a plan before changing code. ...    v1    planning       built-in
    implementation          Spec-to-code with composable passes ...   v1    implementation built-in
      Passes: breakdown, dispatch, implement, review
      Tools: br, ntm
    spike                   Explore before you spec ...               v1    planning       built-in
      Tools: —

  Available (not active):
    spec                    Maintain a living spec that defines ...   v1    planning       built-in
    bug                     Investigate and specify a fix for ...     v2    bug-fix        built-in
    retrofit                Sync specs to code after the fact ...     v1    planning       built-in

Commands:
  kerf jig show <name>    View full jig definition
```

**When no `project.yaml` exists:**

```
Available jigs:
  plan (also: feature)    Write a plan before changing code. ...    v1    planning       built-in
  spec                    Maintain a living spec that defines ...   v1    planning       built-in
  bug                     Investigate and specify a fix for ...     v2    bug-fix        built-in
  implementation          Spec-to-code with composable passes ...   v1    implementation built-in
    Passes: breakdown, dispatch, implement, review
    Tools: br, ntm

Commands:
  kerf jig show <name>    View full jig definition
```

If a jig has aliases, they appear in parentheses after the canonical name. User-level jigs that override a built-in show `user` as the source. Each jig's phase is displayed. For jigs that declare tools, the tools are listed below the jig entry. For composable jigs, active passes are listed below the jig entry.

### Errors

None. Outputs an empty list if no jigs exist.

---

## `kerf jig show`

### Purpose

Display a jig's full definition — passes, file structure, status values, and agent instructions.

### Syntax

```
kerf jig show <name>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | The jig to display. |

### Behavior

1. Resolve the jig via the resolution order (see [jig-system.md](jig-system.md)).
2. Parse the jig file: YAML frontmatter and markdown body.

### Output

The full jig definition:

- **Metadata**: name, description, version.
- **Status values**: the recommended status progression.
- **Passes**: each pass with its name, associated status, expected output files, and the agent instructions from the markdown body.
- **File structure**: the complete expected file listing.

### Errors

| Condition | Message |
|-----------|---------|
| Jig not found | `Error: jig '{name}' not found. Run 'kerf jig list' to see available jigs.` |

---

## `kerf jig save`

### Purpose

Save or create a jig definition in the user's jigs directory.

### Syntax

```
kerf jig save <name> [--from <path>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `name` | Yes | — | Name for the jig. |
| `--from` | No | — | Path to a jig file to copy. If omitted, saves the currently resolved jig (e.g., a built-in) to the user directory for customization. |

### Behavior

1. If `--from` is provided, read and validate the file at that path as a jig definition. Copy it to `~/.kerf/jigs/{name}.md`.
2. If `--from` is omitted, resolve the jig by `name` via the resolution order. Copy it to `~/.kerf/jigs/{name}.md`. This "promotes" a built-in jig to a user-level jig for customization.
3. If `~/.kerf/jigs/` does not exist, create it.
4. If a user-level jig with this name already exists, overwrite it.

### Output

```
Jig '{name}' saved to ~/.kerf/jigs/{name}.md
```

### Errors

| Condition | Message |
|-----------|---------|
| `--from` path does not exist | `Error: file not found: {path}` |
| `--from` file is not a valid jig | `Error: {path} is not a valid jig definition. {details}` |
| No `--from` and jig not found | `Error: jig '{name}' not found. Use --from <path> to create a new jig.` |

---

## `kerf jig load`

### Purpose

Load a jig definition from an external source into the user's jigs directory.

### Syntax

```
kerf jig load <name> <path-or-url>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Name to assign to the loaded jig. |
| `path-or-url` | Yes | Local file path or URL to load the jig from. |

### Behavior

1. Fetch the jig definition from the given path or URL.
2. Validate the fetched content as a jig definition (valid markdown with required YAML frontmatter fields).
3. Write it to `~/.kerf/jigs/{name}.md`.
4. If a user-level jig with this name already exists, overwrite it.

### Output

```
Jig '{name}' loaded from {path-or-url} to ~/.kerf/jigs/{name}.md
```

### Errors

| Condition | Message |
|-----------|---------|
| Path or URL not accessible | `Error: cannot read from {path-or-url}: {details}` |
| Content is not a valid jig | `Error: content from {path-or-url} is not a valid jig definition. {details}` |

---

## `kerf jig sync`

### Purpose

Sync jigs from a remote source (team-shared jigs). This command is reserved for future implementation.

### Syntax

```
kerf jig sync
```

### Behavior

Outputs a message indicating this feature is not yet available.

### Output

```
Jig sync is not yet available.
```

### Errors

None.

---

## `kerf config`

### Purpose

View or modify bench configuration. Configuration is stored in `~/.kerf/config.yaml`. See [architecture.md](architecture.md) for the full config schema.

### Syntax

```
kerf config [key] [value]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `key` | No | Configuration key to read or set, using dot notation (e.g., `default_jig`, `snapshots.enabled`, `finalize.repo_spec_path`). |
| `value` | No | Value to set. If omitted with a key, displays the current value. |

### Behavior (no arguments — display all)

1. Read `~/.kerf/config.yaml`. If the file does not exist, display all defaults.
2. Display all configuration values with their current settings and defaults.

### Behavior (key only — read)

1. Read `~/.kerf/config.yaml`.
2. Display the value for the given key. If the key is not set, display the default value.

### Behavior (key and value — write)

1. Read `~/.kerf/config.yaml` (create with empty content if it does not exist).
2. Set the key to the given value.
3. Write the updated config file.

### Output (no arguments)

```
kerf configuration (~/.kerf/config.yaml):
  default_jig:               {value}
  default_project:           {value}
  spec_path:                 {value}
  snapshots.enabled:         {value}
  snapshots.interval_enabled: {value}
  snapshots.interval_seconds: {value}
  snapshots.max_snapshots:   {value}
  sessions.stale_threshold_hours: {value}
  finalize.repo_spec_path:   {value}
```

### Output (key only)

```
{key}: {value}
```

### Output (key and value)

```
Set {key} = {value}
```

### Errors

| Condition | Message |
|-----------|---------|
| Unknown key | `Error: unknown configuration key '{key}'.` |
| Invalid value for key | `Error: invalid value for '{key}': {details}` |

---

## `kerf snapshot`

### Purpose

Manually trigger a versioning snapshot of the current work state. See [snapshots.md](snapshots.md) for snapshot structure, automatic triggers, and pruning.

### Syntax

```
kerf snapshot <codename> [--name <label>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to snapshot. |
| `--name` | No | — | Human-readable label for the snapshot (e.g., `before-research`, `post-review`). Must be a lowercase slug: alphanumeric and hyphens. |

### Behavior

1. Resolve the project ID.
2. Read the work directory.
3. Create a snapshot directory in `.history/`:
   - Without `--name`: `{ISO 8601 timestamp}/`
   - With `--name`: `{ISO 8601 timestamp}--{label}/`
4. Copy all files from the work directory into the snapshot, excluding `.history/` itself.
5. If the snapshot count exceeds `max_snapshots` (see [architecture.md](architecture.md)), prune the oldest snapshots.

Explicit snapshots are always taken regardless of the `snapshots.enabled` configuration setting.

### Output

```
Snapshot created: .history/{snapshot-directory-name}/
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Invalid label format | `Error: snapshot name must be lowercase alphanumeric and hyphens.` |

---

## `kerf history`

### Purpose

Show the version history of a work — timestamped snapshots with summary information.

### Syntax

```
kerf history <codename>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to show history for. |

### Behavior

1. Resolve the project ID.
2. List all subdirectories of `{work-dir}/.history/`, sorted chronologically (newest first).
3. For each snapshot, read its `spec.yaml` to extract the status at that point in time.

### Output

```
History for {codename}:
  {timestamp}                     {status}
  {timestamp}--{label}            {status}
  {timestamp}                     {status}
  ...

Commands:
  kerf restore {codename} {snapshot}    Restore to a previous snapshot
```

Each entry shows the snapshot directory name and the status recorded in that snapshot's `spec.yaml`.

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| No snapshots exist | `No snapshots found for work '{codename}'.` |

---

## `kerf restore`

### Purpose

Restore a work to a previous snapshot state. See [snapshots.md](snapshots.md) for the full restore sequence and session data preservation rules.

### Syntax

```
kerf restore <codename> <snapshot>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to restore. |
| `snapshot` | Yes | The snapshot directory name (timestamp or timestamp--label) to restore from. |

### Behavior

1. Resolve the project ID.
2. Verify the snapshot directory exists in `{work-dir}/.history/`.
3. Take a snapshot of the **current** work state (so the restore is reversible).
4. Copy the snapshot's files over the current work directory, replacing existing files.
5. **Preserve session data**: after the copy, overwrite the `sessions` list and `active_session` field in `spec.yaml` with the values from before the restore. Session history is never rolled back. See [snapshots.md](snapshots.md) for details.
6. If `active_session` is non-null at restore time, emit a warning.

### Output

```
Restored {codename} to snapshot {snapshot}.
Pre-restore state saved to: .history/{pre-restore-snapshot}/
```

If an active session exists:

```
Warning: active session in progress. Restored spec.yaml reflects the
snapshot's status and metadata, but session tracking is preserved from
the current state.
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Snapshot not found | `Error: snapshot '{snapshot}' not found in work '{codename}'. Run 'kerf history {codename}' to see available snapshots.` |

---

## `kerf archive`

### Purpose

Move a work off the active bench into archive storage. Archived works do not appear in `kerf list` unless `--all` is used.

### Syntax

```
kerf archive <codename>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to archive. |

### Behavior

1. Resolve the project ID.
2. Move the work directory from its current location (the bench or the repo's `.kerf/works/`, depending on storage mode) to `~/.kerf/archive/{project-id}/{codename}/`. Archived works always live on the bench regardless of the project's storage mode.
3. Create the archive project directory if it does not exist.

### Output

```
Work '{codename}' archived.
To un-archive, move the directory back:
  mv ~/.kerf/archive/{project-id}/{codename}/ ~/.kerf/projects/{project-id}/{codename}/
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Work already archived | `Error: work '{codename}' is already archived.` |

---

## `kerf delete`

### Purpose

Permanently remove a work from the bench. This is irreversible.

### Syntax

```
kerf delete <codename> [--yes]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to delete. |
| `--yes` | No | `false` | Skip confirmation prompt. |

### Behavior

1. Resolve the project ID.
2. Read `spec.yaml` to assemble a work summary.
3. If `--yes` is not set, print the work summary and prompt for confirmation.
4. If confirmed (or `--yes` is set), remove the entire work directory.
5. If the work is archived, remove it from the archive directory instead.

Deletion does not affect any finalized copies of the work that exist in the target git repository.

### Output

Before confirmation (when `--yes` is not set):

```
About to permanently delete:
  Codename:  {codename}
  Title:     {title}
  Status:    {status}
  Created:   {created}
  Snapshots: {count}

This cannot be undone. Continue? [y/N]
```

After deletion:

```
Work '{codename}' deleted.
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Confirmation declined | Operation cancelled. No output. |

---

## `kerf init`

### Purpose

Bootstrap kerf in a project. Creates the project identifier, records the active jigs, creates `project.yaml`, optionally sets the default workflow, and runs `kerf setup` to generate agent instructions.

This is the entry point for adopting kerf in any project. The user (or their agent) runs `kerf init`, and the output contains everything needed to complete the setup. `kerf init` is non-interactive: it never prompts and never blocks waiting for input.

### Syntax

```
kerf init [--jig <plan|spec>] [--force] [--yes] [--no] [--bead-filter <expr>]
```

### Arguments and Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--jig` | No | None | Set the default workflow. Must be `plan` or `spec`. If omitted and `default_jig` is not configured, a note is printed instructing the user to choose. |
| `--force` | No | `false` | Re-run init even when `project.yaml` already exists. See "Re-running on an existing project" below. Controls overwrite only — does not affect bead-filter resolution. |
| `--yes` | No | `false` | Accept the bead-filter detector's suggestion when one clears the confidence threshold. When the detector has no confident suggestion, `--yes` leaves `bead_filter` unset (silent). |
| `--no` | No | `false` | Skip bead-filter detection entirely. `bead_filter` is left unset. |
| `--bead-filter` | No | — | Set the project-wide `bead_filter` to an explicit literal value, bypassing the detector. Accepts the same clause forms as `kerf new --bead-filter`. Takes precedence over `--yes` and `--no`. |

### Behavior

1. Verify the current directory is inside a git repository. Error if not.
2. Ensure the bench (`~/.kerf/`) exists. Create if missing.
3. Resolve the project ID (same derivation as `kerf new` — from git remote or directory name).
4. **Detect existing `project.yaml`.** Look for `project.yaml` at the resolved location (either `{repo}/.kerf/project.yaml` under local storage, or `~/.kerf/projects/{project-id}/project.yaml` under bench storage). If it exists, follow the re-run rule in "Re-running on an existing project" below before continuing.
5. If `.kerf/project-identifier` does not exist, create it and print the derived ID.
6. If `--jig` is provided, set `default_jig` in config and print confirmation.
7. If `--jig` is not provided and `default_jig` is not set, print a note with the two options (`kerf config default_jig plan` and `kerf config default_jig spec`).
8. **Resolve active jigs.** Record the default jig (from `--jig` or `default_jig`) and any other jigs the project will use. Jig selection is not interactive — the default-jig resolution drives the active set, and additional jigs are added later by editing `project.yaml` directly. For composable jigs (e.g., `implementation`), pass selection follows the jig's default pass list; users override by editing `project.yaml`.
9. **Resolve `bead_filter`.** Flag precedence: `--bead-filter <expr>` wins outright; otherwise `--no` skips detection; otherwise `--yes` accepts the detector's suggestion if confident; otherwise the default behavior is identical to `--yes` (detector runs, confident suggestion is written, low-confidence suggestion is dropped). Detection runs as follows:
   1. Read the bead store using kerf's existing bead-read path (the same one `kerf next` uses). If the bead tool is unavailable, the bead store is empty, or no work codenames yet exist in the project, the detector returns no suggestion.
   2. List label prefixes that appear in the bead store. For each prefix `P:`, compute `match_score = (beads matching some codename via "P:{codename}") / (total beads with prefix "P:")`.
   3. A prefix is *confident* when both an absolute-count floor and a score floor are met. The detector returns the highest-scoring confident prefix, or no suggestion otherwise. <!-- TBD: open question 2 from plan 016 — exact floor values; in the silent-on-tiny-corpus direction the plan recommends. -->
   4. When the detector returns a confident suggestion and the flags allow it, the suggestion is written to `project.yaml` as the project-wide `bead_filter`. When the detector returns no suggestion, `bead_filter` is left unset; the state-change summary names the post-init command (`kerf config bead_filter ...`) for setting it later.
   5. The chosen filter is editable later via the standard config files. See [coordination.md](coordination.md#bead-attachment).
10. **Create `project.yaml`** with the selected jig and pass configuration, plus the resolved `bead_filter` (if any). If `.kerf/config.yaml` already declares `storage: local`, write it to `{repo}/.kerf/project.yaml` and create the bench symlink at `~/.kerf/projects/{project-id}` pointing at `{repo}/.kerf/works/`. Otherwise write it to `~/.kerf/projects/{project-id}/project.yaml`. See [architecture.md](architecture.md) for the `project.yaml` schema and storage modes.
11. **Run `kerf setup`** to generate agent-facing instructions from the project's active jigs. `kerf init` does not emit its own inline instruction block — `kerf setup` is the single source. The setup output is included in the init output (see Output below).
12. **Emit the state-change summary** as the last block of output (see Output below).

### Re-running on an existing project

`kerf init` is idempotent. When run in a project that already has `project.yaml`, the default behaviour is **skip with informative output**: kerf detects the existing file, prints a summary of the current configuration, and does **not** overwrite it. This preserves any user-edited fields — most importantly a hand-set `bead_filter` — and avoids re-prompting the agent through interactive jig selection on every accidental re-run.

The detection and dispatch rules:

1. **Existing `project.yaml`, no `--force`** — kerf prints the resolved path, the active jigs (with their passes for composable jigs), and the current `bead_filter` (or "built-in default" if unset). It then **skips** steps 8–10 (jig prompts, bead-filter auto-detection, `project.yaml` write). Steps that are safe to re-apply still run:
   - `.kerf/project-identifier` is created if missing (step 5).
   - `--jig` still updates `default_jig` in `~/.kerf/config.yaml` if provided (step 6).
   - `kerf setup` still runs (step 11) so the agent gets fresh, current instructions.
   - Exit status is 0. The output ends with a hint: `Use 'kerf init --force' to overwrite project.yaml, or edit it directly.`

2. **Existing `project.yaml`, with `--force`** — kerf prints a warning naming the file it is about to overwrite, then re-runs the full init flow (steps 8–10) as if no `project.yaml` were present:
   - Active-jig resolution runs again from `--jig` / `default_jig`.
   - Bead-filter resolution runs again under the flag precedence above. A previously-set `bead_filter` is preserved verbatim unless `--bead-filter`, `--yes` (with a new confident suggestion), or `--no` is supplied — `--force` alone does not silently discard a user-set filter.
   - The resulting `project.yaml` overwrites the existing file.
   - `--force` is the only way to overwrite via `kerf init`. There is no `--merge` mode in v1; users who want to add a single field edit `project.yaml` directly.

3. **No existing `project.yaml`** — `--force` is a no-op; behaviour is identical to a fresh init.

The `.kerf/project-identifier` file is not considered part of "already initialised" for this rule — kerf will recreate it if missing even on a skip-path re-run, because its absence breaks project resolution for every other command.

### Output

The output includes, in order:

1. Project initialization status (created or already exists).
2. Default jig status (set, or instructions to set it).
3. Active jig summary (which jigs and passes are recorded in `project.yaml`).
4. Bead-filter resolution summary: the resolved filter (if any), the detector's finding (confident, low-confidence, or no suggestion), and the source (`--bead-filter`, `--yes`, `--no`, or default).
5. The full output of `kerf setup` (see [`kerf setup`](#kerf-setup)) — the single source of the agent-setup instruction block.
6. A verification step the agent can run to confirm the setup works.
7. **State-change summary block.** One fenced block at the end of normal output, with one line per artifact init touched. Each line names the artifact and reports `created`, `updated`, or `unchanged`. The summary is the diffable signal that what init claims matches what landed on disk; an artifact that init did not write is not listed.

   ```
   State changes:
     .kerf/project-identifier   created
     project.yaml               created
     default_jig                updated (set to 'spec')
     bead_filter                unchanged (detector returned no confident suggestion; run 'kerf config bead_filter <expr>' to set)
   ```

   The summary covers `.kerf/project-identifier`, `project.yaml`, `default_jig`, and `bead_filter`. Artifacts whose persistence is not yet wired (see open question 3 in plan 016) are not advertised on this block — init only reports what it actually writes. <!-- TBD: open question 3 from plan 016 — whether default_jig lands in project.yaml or stays in config.yaml. -->

The instructions are agent-agnostic. kerf does not know or reference any specific AI tool. The agent reading the output determines where to put the instructions based on its own configuration conventions.

### Errors

| Condition | Message |
|-----------|---------|
| Not in a git repository | `Error: not in a git repository. kerf requires a git repo.` |
| `--jig` value not `plan` or `spec` | `Error: --jig must be 'plan' or 'spec', got '{value}'.` |
| `--yes` and `--no` both given | `Error: --yes and --no are mutually exclusive.` |
| `--bead-filter` value does not parse | `Error: --bead-filter expects 'label=<value>' or 'id_prefix=<value>', got '{value}'.` |
| Existing `project.yaml` detected, no `--force` | Not an error. kerf prints `project.yaml already exists at {path} — skipping re-initialisation. Use 'kerf init --force' to overwrite, or edit the file directly.` and exits 0 after running the safe-to-repeat steps (project-identifier creation, `--jig` update, `kerf setup`). |
| `--force` passed but `project.yaml` could not be read for the pre-overwrite summary | `Error: --force requested but existing project.yaml at {path} is unreadable: {details}. Move or delete the file manually before re-running.` |

---

## `kerf localize`

### Purpose

Migrate a project from bench storage to local storage. Moves all in-progress works from `~/.kerf/projects/{project-id}/` into `{repo}/.kerf/works/`, moves `project.yaml` (and `areas.yaml`, if present) into the repo, writes `storage: local` to `{repo}/.kerf/config.yaml`, and replaces the bench project directory with a symlink pointing at the repo's works directory. See [architecture.md](architecture.md#storage-modes).

### Syntax

```
kerf localize [--check] [--project <project-id>]
```

### Arguments and Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--check` | No | `false` | Dry-run preview. Print the moves that would happen — work directories, `project.yaml`, `areas.yaml`, the symlink swap, and the `storage: local` write — without changing anything on disk. Equivalent alias: `--dry-run`. |
| `--project` | No | inferred from `.kerf/project-identifier` | Project ID to localize. Must be run from inside the target repo. |

### Behavior

1. Resolve the repo root from the current working directory. Error if not inside a git repository.
2. Resolve the project ID.
3. If `{repo}/.kerf/config.yaml` already declares `storage: local`, emit `Already using local storage for project '{id}'.` and exit.
4. Verify `~/.kerf/projects/{project-id}/` exists and is a real directory (not already a symlink).
5. Verify `{repo}/.kerf/works/` either does not exist or is empty.
6. Create `{repo}/.kerf/works/`.
7. Move each work directory from `~/.kerf/projects/{project-id}/` to `{repo}/.kerf/works/`.
8. If `~/.kerf/projects/{project-id}/project.yaml` exists, move it to `{repo}/.kerf/project.yaml`. Same for `areas.yaml`.
9. Remove the now-empty `~/.kerf/projects/{project-id}/` and replace it with a symlink to `{repo}/.kerf/works/`.
10. Write `storage: local` to `{repo}/.kerf/config.yaml`.
11. Print a summary including next-step git commands.

### `--check` (preview)

When `--check` is given, kerf runs steps 1–5 (resolution and pre-flight verification) and then prints the move plan without mutating the filesystem. The output names each work directory that would move, the destination path, the bench → symlink swap, and the `storage: local` write target. Steps 6–11 are skipped. Exit status is 0 when the move would succeed, non-zero with the same Errors table when the pre-flight checks fail (so an agent can use `--check` as a guard before the real run).

### Atomicity

If any move fails, kerf attempts to roll back the operation: move files back to the bench, remove the partial works directory, and avoid writing the symlink and repo config. The user receives the failure reason and the original state is preserved on a best-effort basis.

### Output

```
Localized project '{project-id}' to {repo-root}/.kerf/works/
Moved {n} works: {codename-1}, {codename-2}, ...
Symlink: ~/.kerf/projects/{project-id} -> {repo-root}/.kerf/works/

Next steps:
  git add .kerf/config.yaml .kerf/works/
  git commit -m "kerf: enable local storage"

Tip: To exclude snapshots from git, add to .gitignore:
  .kerf/works/*/.history/
```

### Errors

| Condition | Message |
|-----------|---------|
| Not in a git repository | `Error: not in a git repository. Use --project <project-id> to specify a project.` |
| Already local | `Already using local storage for project '{project-id}'.` |
| Bench path is already a symlink | `Error: {path} is already a symlink; project may already be localized.` |
| `.kerf/works/` exists and is non-empty | `Error: {path} already exists and is not empty; aborting.` |
| Move failure | `Error: moving {codename}: {reason}. Localization aborted — no changes made.` |

### Reverse migration

There is no `kerf delocalize` command in v1. To revert manually:

1. Move work directories from `{repo}/.kerf/works/` back to `~/.kerf/projects/{project-id}/`.
2. Move `{repo}/.kerf/project.yaml` (and `areas.yaml`) back into `~/.kerf/projects/{project-id}/`.
3. Remove the bench symlink at `~/.kerf/projects/{project-id}`.
4. Remove the `storage:` field (or set to `bench`) in `{repo}/.kerf/config.yaml`.

---

## `kerf setup`

### Purpose

Generate agent-facing instructions from the project's active jigs. The agent applies the output to its configuration file (CLAUDE.md, AGENTS.md, etc.) — kerf does not write to these files directly.

`kerf setup` is re-runnable. It generates fresh instructions whenever jigs are updated. `kerf init` calls `kerf setup` as part of its flow; `kerf setup` can also be run independently to refresh stale agent configuration.

### Syntax

```
kerf setup
```

### Behavior

1. Resolve the project ID from `.kerf/project-identifier` in the current repository.
2. Read `project.yaml` from `~/.kerf/projects/{project-id}/project.yaml` to determine active jigs and pass configurations. See [architecture.md](architecture.md) for the `project.yaml` schema.
3. For each active jig, load the jig definition and extract:
   - Process instructions (the full agent-facing instructions from each pass)
   - Tool requirements (declared in the jig's `tools` field)
   - Jig sequencing (the recommended order of jigs for the project's SDLC)
   - References to kerf commands used by the jig
4. For composable jigs, include only the passes that are active per `project.yaml`.
5. Assemble the complete agent-facing instruction block.

**When no `project.yaml` exists:** kerf emits default instructions — all jigs are presented as available, basic kerf usage is included, and no project-specific jig or pass filtering is applied.

### Output

`kerf setup` is the single source of the `AGENT SETUP INSTRUCTIONS` block. `kerf init` calls into this command and does not emit its own inline copy.

The output is a clearly delimited block of agent-facing instructions containing:

- **Process instructions** from each active jig: the full agent-facing process for each jig's passes.
- **Tool requirements**: which external tools are needed (e.g., `br` for bead management, `ntm` for orchestration), as declared by the active jigs.
- **Jig sequencing**: the composition chain for this project — which jigs are available and in what SDLC order.
- **References to kerf commands**: the kerf commands relevant to each phase (e.g., `kerf new --jig plan` for planning, `kerf new --jig implementation` for implementation). The block names the daily-driver commands explicitly, with one-line descriptions: `kerf next` (ranked feed of things to do), `kerf triage` (drift report on the bead store), `kerf pin` (attach a specific bead to a work), `kerf map` (portfolio view across areas), `kerf areas` (define and list areas), and `kerf work edit` (mutate a work's bead-filter).
- **`.gitignore` pattern**: the exact two-line pattern to add — `.kerf/` on the first line and `!.kerf/project-identifier` on the second — so the bench-side state stays out of git while project identity is committed.
- **Bench location** (one-line placeholder; expanded by `specs/architecture.md`'s "Where state lives" cheat-sheet — see plan 017): names `~/.kerf/projects/<project-id>/` as the bench path and the repo-side files agents should touch. <!-- TBD: plan 017 fills this slot once the architecture cheat-sheet lands. -->

When no `project.yaml` exists:

```
No project.yaml found — showing default instructions.
All available jigs can be used with `kerf new --jig <name>`.

{default kerf usage instructions}
{list of all available jigs with descriptions}
```

### Errors

| Condition | Message |
|-----------|---------|
| Not in a git repository | `Error: not in a git repository. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |
| No `.kerf/project-identifier` found | `Error: project not initialized. Run 'kerf init' first.` |

---

## `kerf doctor`

### Purpose

Health check for the current project. Reports green / yellow / red findings on `project.yaml` shape, storage drift between the repo's `.kerf/` and the bench at `~/.kerf/projects/<project-id>/`, symlink integrity (in local mode), per-work `bead_filter` coverage, and archive orphans. `kerf doctor` is read-only by default: it surfaces findings and names the command that would fix each. It does not mutate any state.

### Syntax

```
kerf doctor [--format <format>] [--detector <id>] [--quiet] [--strict] [--project <project-id>]
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--format <format>` | No | `text` | Output format. `text` (default) or `json`. |
| `--detector <id>` | No | — | Repeatable. Run only the named detectors (e.g., `--detector storage-drift`). When omitted, all detectors run. |
| `--quiet` | No | `false` | Suppress green findings; emit only yellow and red. |
| `--strict` | No | `false` | Exit non-zero when any red finding is present. Without `--strict`, the command exits 0 regardless of findings. <!-- TBD: open question 4 from plan 017 — exact exit-code defaults. --> |
| `--project <project-id>` | No | Inferred from cwd | Project to inspect. |

### Behavior

1. Resolve the project ID and the active storage mode (bench or local) for the project. See [architecture.md](architecture.md#storage-modes).
2. Run the detectors selected by `--detector` (or all detectors when unspecified):
   - **`project-yaml`** — checks `project.yaml` exists at the location dictated by the active storage mode, parses cleanly, declares at least one jig, and (when applicable) names a `default_jig`. Reports against whatever schema `project.yaml` carries; the schema itself lives in [architecture.md](architecture.md).
   - **`storage-drift`** — presence-level only. Reports drift when a work directory, `project.yaml`, or `areas.yaml` lives in the non-canonical location for the active mode (a `.kerf/works/<codename>/` directory under bench mode, or a real `~/.kerf/projects/<project-id>/<codename>/` directory under local mode), when a work appears in both locations, or when `project.yaml` / `areas.yaml` exist in both at once. Content-level (hash) drift is out of scope in v1.
   - **`symlink-integrity`** (local mode only) — checks that `~/.kerf/projects/<project-id>` is a symlink, that the target exists, and that the target matches the resolver's expected path. Reports broken, missing, or real-directory-where-symlink-expected cases.
   - **`bead-filter-coverage`** — reports each active work whose `bead_filter` resolves to zero beads, labelled by the rank-label vocabulary documented under [`kerf next`](#kerf-next) (`empty`, `unwired`, `broken`). The hint for an unwired work names the filter-bootstrap entry point (see [plan 019]).
   - **`archive-orphans`** — reports any `~/.kerf/archive/<project-id>/<codename>/` entry whose codename also appears as a live work in the project.
3. Aggregate findings, assign severity (`green` for healthy, `yellow` for warnings without immediate damage, `red` for findings that block normal use), and render.

Each finding names the canonical fix command in its hint line. `kerf doctor` does not run those commands itself.

### Output (default: compact text)

```
kerf doctor — project: {project-id} ({mode} mode)

[green]  project-identifier: {project-id}
[green]  project.yaml: present, default_jig={jig}, {n} jigs declared
[yellow] storage drift: 1 finding
         - work '{codename}' exists on bench but not in .kerf/works/
           hint: kerf localize --check  (preview what reconcile would do)
[green]  bench symlink: ~/.kerf/projects/{project-id} -> {repo}/.kerf/works
[red]    bead_filter coverage: 2 of 6 works unwired
         - works without bead_filter: {codename-1}, {codename-2}
           hint: see 'kerf next' warning rows for per-work next steps
[green]  archive: 1 entry, no live collisions
```

The header line names the project and active storage mode. Each detector reports one row with its severity tag, summary, and (when not green) an indented finding list followed by a `hint:` line naming the fix command.

When all detectors return green, the body collapses to one summary line per detector and the final line reads `All checks green.`

### Output (`--format=json`)

```
{
  "project_id": "{project-id}",
  "storage_mode": "{bench|local}",
  "findings": [
    {
      "detector":  "{detector-id}",
      "severity":  "{green|yellow|red}",
      "summary":   "{one-line summary}",
      "items":     [ { "target": "{codename or path}", "detail": "..." } ],
      "hint":      "{fix command}"
    }
  ]
}
```

Detector ids are stable. Future detectors append to the list; consumers should treat unknown detectors as informational.

### Exit codes

| Condition | Exit |
|-----------|------|
| No red findings, `--strict` absent or present | 0 |
| Red findings present, `--strict` not given | 0 |
| Red findings present, `--strict` given | 1 |
| Bead store unreadable, project not initialised, or other infrastructure error | 1 |

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |
| Unknown detector id in `--detector` | `Error: unknown detector '{value}'. Known detectors: {list}.` |
| Unknown value in `--format` | `Error: unknown format '{value}'. Supported: text, json.` |

---

## `kerf map`

### Purpose

Show all active work items with their areas, status, and bead progress. Provides a portfolio-level view of what the project is doing across all areas.

### Syntax

```
kerf map [--project <project-id>]
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--project` | No | Inferred from cwd | Show the map for this project. |

### Behavior

1. Resolve the project ID.
2. Read all active (non-archived) work directories under `~/.kerf/projects/{project-id}/`. For each, read `spec.yaml` to get codename, title, type, status, areas, and `depends_on`.
3. Read `areas.yaml` for the project to get area definitions.
4. If bead integration is available (the project uses a jig that declares bead tooling), query bead status per work.
5. Group works by area. Works with no areas appear under an "unassigned" group. Works with multiple areas appear under each area.

### Output

```
Map for {project-id}:

  auth:
    token-refresh     plan   research    3/7 beads
    session-mgmt      plan   tasks       —

  api-gateway:
    token-refresh     plan   research    3/7 beads
    rate-limiter      spec   decompose   —

  unassigned:
    quick-fix         bug    reported    —

Dependencies:
  token-refresh -> database-migration [ready]
  session-mgmt -> token-refresh [research]

Commands:
  kerf show <codename>      View work details
  kerf next                 See suggested work ordering
  kerf areas list           View all areas
```

- Works are listed under each area they touch. A work touching multiple areas appears in each group.
- Bead progress (e.g., `3/7 beads`) is shown when bead data is available; `—` otherwise.
- The Dependencies section shows cross-work dependencies with the dependency's current status.
- If no works exist, output says so and suggests `kerf new`.

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

---

## `kerf next`

### Purpose

A ranked feed of things to act on right now. `kerf next` is the agent's primary pull signal: it runs at the top of every cycle, returns one ordered list, and the agent acts on the top item. The feed mixes several kinds of items — beads to work on, cleanup tasks owed on a work, and project-level warnings — so a single command surfaces everything the agent should resolve next.

The output is a view over current state, not stored data. Running it ten times with no state changes returns the same list.

### Syntax

```
kerf next [--only <kind>] [--include <kind>] [--kinds <list>] [--format <format>] [--project <project-id>]
```

### Item kinds

Every item carries a `kind` and a `target`:

| kind | target | meaning |
|------|--------|---------|
| `bead` | a bead | A ready-to-work bead. Do this bead. |
| `cleanup` | a work | Something is owed on this work — walk the jig, advance status, or shelve. |
| `warning` | project-level | A project-wide issue (typically a misconfiguration). Fix config, not code. |

`pr` and additional kinds may be added in later versions. The item shape is designed to absorb them without changing existing kinds.

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--only <kind>` | No | — | Show only items of this kind. Repeatable. Example: `--only=bead`. |
| `--include <kind>` | No | — | Add a non-default kind to the feed. Repeatable. Example: `--include=warning`. |
| `--kinds <list>` | No | — | Comma-separated list of kinds to show, replacing the default set. Example: `--kinds=bead,cleanup`. |
| `--format <format>` | No | `text` | Output format. `text` (compact, default) or `json`. |
| `--project <project-id>` | No | Inferred from cwd | Show next actions for this project. |

`--only=bead` is shorthand for the bead-only feed (today's behavior). See **Flag precedence** below for combined-flag rules.

### Behavior

1. Resolve the project ID.
2. Read all active works and their `spec.yaml` files; read project state (filter config, areas, dependencies).
3. **Assemble candidate items** from each source:
   - **Beads** — from the bead store, filtered per work via the resolved `bead_filter` (see [coordination.md](coordination.md#bead-attachment)). Beads that are blocked, in progress, or closed are excluded. Each ready bead becomes a `bead` item attributed to every work it matches.
   - **Cleanup detectors** — pure functions over current state, run on every invocation. v1 detectors:
     - `work_no_attached_beads` — fires when a work's resolved `bead_filter` matches zero beads (attached_count == 0). The cleanup item carries one of three **rank labels** to distinguish the three failure modes that previously collapsed under `clean`:
       - `empty` — `bead_filter` declared and syntactically valid, but matches zero open beads. Likely benign; the work is wired but its beads have not been created yet.
       - `unwired` — no `bead_filter` key on `spec.yaml`, or key present with empty value. The agent needs to bootstrap or edit. Suggested action: `kerf work edit <codename> --bead-filter-add '<clause>'` or the project-wide bootstrap entry point. <!-- TBD: open question 1 from plan 019 — whether bootstrap is a new top-level verb or a flag on kerf work edit. -->
       - `broken` — `bead_filter` declared but malformed, or referencing a clause shape that does not parse. Suggested action: edit the filter to a valid clause. <!-- TBD: open question 5 from plan 019 — whether the existing parser can distinguish "malformed" from "valid but zero-match"; if not, broken collapses into empty. -->
     - `work_beads_done_status_open` — fires when a work has attached_count > 0, every attached bead is closed, and the work's jig status is not terminal. Suggested action: advance status (`kerf status <codename> <next-stage>`) or `kerf shelve <codename>`.

     The two detectors are mutually exclusive by construction (the attached-count guard on `work_beads_done_status_open` ensures a zero-bead work is reported only by `work_no_attached_beads`).

   - **Near-match advisor.** For each work tagged `empty` whose codename is one prefix-swap away from a heavily-populated label family in the bead store (e.g., `codename:foo` ↔ bare `foo`, `subsystem:foo` ↔ `foo`), the warning row appends ` — try: kerf work edit <codename> --bead-filter '<proposed>'`. The advisor only emits a suggestion when exactly one alternate clause would lift the work out of `empty`; ambiguous or zero-candidate cases stay silent.
   - **Warning detectors** — project-level state checks. v1 detectors:
     - `untriaged_beads`: count of beads in the store that are (a) open status (not blocked, in progress, or closed — same readiness filter applied to candidate beads above), (b) match no work's resolved `bead_filter`, and (c) not pinned. Surfaced once as a single warning item. When a drift baseline is recorded, the count is rendered as the `untriaged` segment of the drift summary line below rather than as a separate warning block.
     - Filter literal yields zero matches: when the project-wide `bead_filter`'s literal prefix matches nothing in the bead store, surface a warning suggesting a case-mismatch check (e.g., `Subsystem:` vs `subsystem:`). Matching is case sensitive — see [coordination.md](coordination.md#bead-attachment).
     - `multi_matched`: count of beads matching more than one work's filter and not resolved by a pin. See [coordination.md](coordination.md#drift-categories).
     - `external_drift`: aggregated count of beads classified as `external_close`, `external_reopen`, `external_delete`, or `external_new` since the last drift baseline. See [coordination.md](coordination.md#drift-detection).
4. **Exclude** items per kind:
   - `bead` items are excluded when their target work is blocked by an unmet `must-complete-first` dependency, archived, or finalized. Dependency gating remains strict on jig status — see [dependencies.md](dependencies.md). The bead-done-but-status-stale case is intentionally surfaced as a `cleanup` item rather than auto-clearing dependency gates.
   - `cleanup` items are excluded only when their target work is archived or finalized. A blocked work still surfaces its cleanup items — those items are how the user resolves the block.
   - `warning` items are project-level and not filtered by work state.
5. **Score** each kind separately. Beads rank against beads using the factors described in [coordination.md](coordination.md#computed-priority) — dependency fan-out, completion momentum, rework, area focus. Cleanup items do not enter bead scoring; they sort after all beads, ordered by their parent work's would-be bead score (descending), so that a stale-status work near the top of the queue is visible without leapfrogging genuinely-blocking new work. Warning items are not ranked.
6. **Filter** by the kind selection from the flags above.
7. **Render** the feed.

### Default kind selection

Without flags, the feed includes `bead` and `cleanup` items. Beads are ranked first; cleanup items follow after all beads, ordered by their parent work's would-be bead score. `warning` items, when present, render as a short stanza **after** the ranked payload and the drift footer — not as a header block above it. The payload-first ordering is so an agent reading the top of the output sees actionable work first, not diagnostics.

### Flag precedence

`--kinds`, `--only`, and `--include` compose as follows:

1. `--kinds=a,b` sets the base set, replacing the default. With no `--kinds`, the base set is all kinds.
2. `--only=X` is repeatable and acts as intersection — it restricts the working set to the listed kinds. Multiple `--only` flags union among themselves first (so `--only=bead --only=cleanup` keeps both), then intersect with the base set.
3. `--include=X` is repeatable and acts as union — each adds a kind back into the set.
4. Repeated identical flags are idempotent (union semantics, no last-wins).
5. `--only=warning` produces a feed containing only the warning header block; no ranked items are emitted.

### Output (default: compact text)

```
$ kerf next
1. bead   hk-cb-042  "wire retry into adapter"        work: bridge
2. bead   hk-cb-051  "extract header parser"          work: bridge
3. clean  bridge      all beads closed; walk jig or shelve

run with --format=json for machine output, --help for filters
```

Each line shows rank, kind, target identifier, title or reason, and (for `bead` and `cleanup`) the owning work codename. The footer points at machine output and the filter help.

Text output is for humans (and for an agent reading it as prose at the top of a cycle). It is not a parsing contract: column positions, spacing, and exact phrasing may change between versions. Agents or scripts that need stable structured output use `--format=json`.

When drift counters are non-zero, the feed appends a one-line drift summary **after** the ranked payload and before any warning stanza:

```
drift: 6 untriaged · 2 multi-matched · 1 external — run 'kerf triage'
```

Each segment is omitted when its count is zero; the whole line is omitted when all three counts are zero. The segments correspond to the `untriaged_beads`, `multi_matched`, and `external_drift` detectors above. The summary line is the agent's pull signal that `kerf triage` has new work to surface — it appears whether or not the agent is invoking `kerf triage` directly.

When untriaged beads are present and the drift-summary line is rendered, the older "warning: N beads match no work" line is omitted from the same invocation (the drift-summary `untriaged` segment covers it). When no baseline has been recorded and the drift-summary line is suppressed, the legacy warning is still emitted:

```
warning: 12 beads match no work — check bead_filter in project.yaml
```

After the drift footer, the warning stanza (when any work-level cleanup items are present) renders compactly — one row per affected work, prefixed with the rank label (`empty`, `unwired`, `broken`) and the codename, followed by the near-match advisor line when applicable:

```
unwired: phase-3-dot — try: kerf work edit phase-3-dot --bead-filter 'label=phase-3-dot'
empty:   bridge      — try: kerf work edit bridge --bead-filter 'label=subsystem:bridge'
broken:  scratch     — bead_filter clause does not parse; edit spec.yaml
```

If no items exist, the output says so.

#### Storage-drift footer

When `kerf doctor` would report any non-green storage finding for the current project (see [`kerf doctor`](#kerf-doctor)), `kerf next` appends a one-line footer below the ranked list and any drift summary:

```
note: {n} storage finding{s} — run 'kerf doctor' for details
```

The footer is on by default. It is suppressed when `kerf config doctor.footer false` is set or when the environment variable `KERF_DOCTOR_FOOTER=0` is set at invocation time. The footer is independent of the bead-store `drift_summary` line above — bead-store drift and storage-layout drift are distinct surfaces.

### Output (`--format=json`)

`--format=json` emits the full item stream — one record per item, in rank order, including `warning` items. No other formats in v1. Field names are snake_case. The item shape:

```
{
  "kind":          "<one of the build's known kinds>",
  "score":         <number>,
  "title":         "<human one-liner>",
  "action":        "<suggested command or next step>",
  "reason":        "<why this surfaced>",
  "work_codename": "<codename or null>",
  "bead_id":       "<id or null>"
}
```

The set of valid `kind` values comes from the current build (v1: `bead`, `cleanup`, `warning`). Future kinds (e.g., `pr`) are additive. Consumers should treat unknown kinds as informational rather than erroring.

Top-level output shape depends on whether a drift baseline is recorded for the project (see [coordination.md](coordination.md#baseline-advancement)):

- **No baseline recorded** — output is a bare JSON array of items in rank order. An empty feed emits `[]`.
- **Baseline recorded** — output is a JSON object with two top-level fields:
  ```
  {
    "drift_summary": {
      "untriaged":      <number>,
      "multi_matched":  <number>,
      "external_drift": <number>
    },
    "items": [ <item>, ... ]
  }
  ```
  The `drift_summary` field is always present (and always an object with these three integer counters) when a baseline exists, even when all three counters are zero. The `items` array uses the same item shape and ordering as the bare-array form. Counter semantics match the text headline segments documented above.

Filter flags apply to JSON output identically and do not change the top-level shape.

### Help text

`kerf next --help` is part of the spec — the agent's contract. A fresh agent running it once must come away knowing the full loop. The help text covers, in this fixed order:

1. **What it returns** — a ranked feed of things to act on right now.
2. **The item kinds** — one line per kind: `bead` = work on this; `cleanup` = resolve this on a work; `warning` = project-level issue, fix config.
3. **The default action loop** — read the top item, do it, re-run `kerf next`.
4. **The filter flags** with concrete examples: `--only=bead`, `--include=warning`, `--kinds=bead,cleanup`.
5. **Machine output** — `--format=json` for scripts.
6. **How scoring works** in one sentence, with a pointer to [coordination.md](coordination.md#computed-priority) for detail.

Changes to this help text require a spec change.

### Warning kinds

`kerf next` is a read-only view that surfaces problems it encounters during feed assembly rather than failing the command. Each warning is a structured entry with three fields:

- `title` — short single-line label suitable for the warning banner.
- `action` — the suggested next command (or a hyphen if no command applies).
- `reason` — a one- or two-line explanation, including the offending codename and any underlying error string.

Detectors run during feed assembly; the feed (the actionable-work list) is still emitted alongside any warnings unless the warning kind is documented as fatal. The full set of warning kinds is defined here. The mirror table in [coordination.md](coordination.md#feed-warning-rules) lists each kind with its fatality and severity.

#### `corrupt_spec`

- **When it fires:** during step 2 (read all active works and their `spec.yaml` files), a per-work `spec.yaml` cannot be parsed — malformed YAML, an invalid `created_at` / status timestamp, a missing required field, or any other schema violation that prevents the work from being loaded.
- **Effect on the feed:** the offending work is excluded from feed assembly. It is not silently dropped; the corrupt-spec warning replaces the silent skip that earlier versions performed.
- **Fields:**
  - `title`: `Corrupt spec: {codename}`
  - `action`: `kerf show {codename}` (so the operator can see the parse error in context).
  - `reason`: `Could not parse spec.yaml for '{codename}': {parse-error}. Work excluded from this feed.`
- **Message shape (one warning per corrupt spec):**

  ```
  Warning: corrupt spec for '{codename}'.
    {parse-error}
    Work excluded from this feed. Run 'kerf show {codename}' to inspect.
  ```

#### `no_project_yaml`

- **When it fires:** `kerf next` runs in a directory that resolves to a project id (via git remote, directory name, or `.kerf/project-identifier`) but no `project.yaml` exists at either the local-storage path (`{repo}/.kerf/project.yaml`) or the bench path (`~/.kerf/projects/{project-id}/project.yaml`). This is distinct from "no project resolvable" (which remains a hard error per the Errors table); the project is identifiable but never initialised.
- **Effect on the feed:** fatal for this invocation. kerf does not assemble a feed without `project.yaml`, because jig configuration and `bead_filter` come from that file. Exit status is non-zero (see Errors).
- **Fields:**
  - `title`: `No project.yaml for '{project-id}'`
  - `action`: `kerf init`
  - `reason`: `Project '{project-id}' has no project.yaml. Run 'kerf init' to create one before using 'kerf next'.`
- **Message shape:**

  ```
  Warning: no project.yaml for '{project-id}'.
    Run 'kerf init' to initialise this project.
  ```

When multiple warnings fire on the same invocation, they are printed in the order they were detected, before the feed listing. The feed listing is omitted entirely when a fatal warning fires.

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |
| Unknown kind in `--only`/`--include`/`--kinds` | `Error: unknown item kind '{value}'. Known kinds: {list of kinds from the current build}.` |
| Unknown value in `--format` | `Error: unknown format '{value}'. Supported: text, json.` |
| `no_project_yaml` warning fires | The warning above is printed and the command exits non-zero with `Error: no project.yaml — run 'kerf init'.` |

---

## `kerf triage`

### Purpose

A single drift report for the project's bead store. `kerf triage` is the agent's closed loop for reconciling kerf's view of the bead store with what is actually there: beads added, relabeled, closed, reopened, or deleted by other tools since the last acknowledged baseline. The output sections, exit codes, and `--ack`/`--resolved` flags compose so an agent can loop `until kerf triage --resolved; do <act>; done` and terminate.

Triage reads bead state and the drift baseline at `.kerf/sync-cache.json` (see [coordination.md](coordination.md#drift-detection)). It does not mutate the baseline except on `--ack`.

### Syntax

```
kerf triage [--resolved] [--ack] [--kind <kind>...] [--top <n>] [--group-by <field>] [--format <format>] [--project <project-id>]
```

### Item kinds

Every triage item carries a `kind` and a `target`:

| kind | target | meaning |
|------|--------|---------|
| `untriaged` | a bead | A bead matches no work's filter and is not pinned. Suggested action: a `kerf new`, `kerf work edit --bead-filter-add`, or `kerf pin` command. |
| `multi_matched` | a bead | A bead matches more than one work's filter and is not pinned to disambiguate. Suggested action: `kerf pin <codename> <bead-id>` or narrow a filter via `kerf work edit --bead-filter-remove`. |
| `external_drift` | a bead | A bead's status changed externally since the last acknowledged baseline. Sub-kinds: `external_close`, `external_reopen`, `external_delete`, `external_new`. Suggested action: investigate, then `kerf triage --ack`. |

The `--kind` flag selects which kinds are shown; see Flags below.

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--resolved` | No | `false` | Exit-code mode (see Exit codes). With `--resolved`, kerf still emits the report so an agent loop can capture the latest state; the load-bearing piece is the exit code. |
| `--ack` | No | `false` | Acknowledge the current bead-store snapshot as the new drift baseline. Writes `.kerf/sync-cache.json`. No other state changes. Mutually exclusive with `--resolved`. |
| `--kind <kind>` | No | All kinds | Repeatable. Show only items of this kind. Accepts `untriaged`, `multi_matched`, `external_drift` (or any `external_*` sub-kind). |
| `--top <n>` | No | Unlimited | Truncate each section to the top `n` items after sorting. The section header reports both shown and total counts. `external_drift` is not truncated by `--top` unless `--top` is given on the command line explicitly — its findings are the smallest section and the most actionable. |
| `--group-by <field>` | No | — | Group `untriaged` items by the first label whose prefix matches the tier-1 cohort-defining list (see Suggester routing). Currently the only accepted value is `codename-label`. Ties between groups break lexicographically. The grouped section emits one header per group with items nested under it. |
| `--format <format>` | No | `text` | Output format. `text` (default) or `json`. |
| `--project <project-id>` | No | Inferred from cwd | Show triage for this project. |

`--resolved` and `--ack` cannot be combined.

### Behavior

1. Resolve the project ID. If `project.yaml` is absent, exit 1 with the `not_initialized` kind (see Errors).
2. Read the current bead-store snapshot via the configured bead tool.
3. Read the drift baseline at `.kerf/sync-cache.json`. A missing or empty file is treated as an empty baseline.
4. Compute filter resolution for every active work (see [coordination.md](coordination.md#resolution-order)) composed with the pin layer (see [coordination.md](coordination.md#pin-layer)). Classify each current bead:
   - Pinned to a work → attached to that work only; not eligible for `multi_matched`.
   - Matches no work's filter and not pinned → `untriaged`.
   - Matches more than one work's filter and not pinned → `multi_matched`.
5. Compute drift categories by diffing current snapshot against baseline (see [coordination.md](coordination.md#drift-categories)).
6. If `--kind` is given, filter the item list to the selected kinds. When the resulting set is empty, skip the full report header and emit a single line: `No {kind} items.` (or, with multiple `--kind` flags, `No items in selected kinds: {list}.`) The command exits normally.
7. Render the report — but only when `--ack` is absent. With `--ack`, the render step is skipped; stdout sees only the single-line baseline confirmation in step 8.
8. **If `--ack` was given**: capture the current snapshot, write it to `.kerf/sync-cache.json`, replacing the previous baseline. Print one line: `Baseline advanced to {timestamp}.` Under `--ack --format=json`, emit a one-record summary `{ "baseline_advanced_at": "{timestamp}", "items_captured": {n} }` in place of the item stream. <!-- TBD: open question 4 from plan 018 — silent vs. summary record under --ack --format=json; spec follows the plan's leaning toward the summary record. -->
9. **If `--resolved` was given**: compute the exit code per the table below and exit.

`kerf triage` does not mutate works, `spec.yaml` files, or beads — only `.kerf/sync-cache.json` (on `--ack`).

### Output (default: compact text)

The text report is structured by section. Sections with zero items are omitted.

```
Triage for {project-id} (baseline: 2026-05-13T09:14:00Z, 5 days ago):
Beads scanned: 163 open · 168 total.

Untriaged beads (3):
  hk-cb-077  open  "audit log rotation"      labels: subsystem:audit
    suggest: kerf new audit --bead-filter 'label=subsystem:audit'
  hk-cb-078  open  "rotate audit secrets"    labels: subsystem:audit
    suggest: kerf work edit audit --bead-filter-add 'label=subsystem:audit'
  hk-cb-091  open  "investigate timeout"     labels: -
    suggest: kerf pin <codename> hk-cb-091

Multi-matched beads (1):
  hk-cb-064  open  "shared header parser"    matches: bridge, gateway
    suggest: kerf pin bridge hk-cb-064

External changes since last triage (2):
  hk-cb-040  closed  "add retry envelope"     was open at baseline
  hk-cb-101  deleted "old idempotency probe"  present at baseline, gone now

Per-work bead health:
  bridge   filter: label=subsystem:bridge   beads: 4 open / 3 closed
  gateway  filter: label=subsystem:gateway  beads: 0 open / 0 closed   (no attached beads)

Next:
  Address surfaced items, then run 'kerf triage --ack' to advance the baseline.
  Re-run 'kerf triage --resolved' to confirm the project is clean.
```

Each per-bead suggestion is a templated, ready-to-paste command. Agents copy literally rather than synthesizing. The chosen template for each kind:

- `untriaged` — `kerf new <suggested-codename> --bead-filter '<clause derived from the bead's labels>'` for new buckets; otherwise `kerf work edit <existing-codename> --bead-filter-add '<clause>'` when a closely-named work exists; otherwise `kerf pin <codename> <bead-id>`.
- `multi_matched` — `kerf pin <codename> <bead-id>` for the lexicographically-earliest matching codename (single deterministic suggestion).
- `external_drift` — informational; the remediation is `kerf triage --ack` after the agent investigates.

#### Count reconciliation and `--top` rendering

The header line `Beads scanned: N open · M total` is always present. The two numbers are accurate against different status filters — `open` excludes closed beads and is the population the per-section item counts apply to; `total` is the unfiltered population. Each section header that follows reports a single count against its kind's natural filter (e.g., `Untriaged beads (3):` counts open beads matching no work).

When `--top N` is given, each truncated section's header reports both counts, e.g., `Untriaged beads (showing 20 of 168):`. Truncation happens after sorting and never reorders items.

#### Suggester routing

The `untriaged` suggester ranks a bead's labels into two tiers before proposing a `kerf new`:

- **Tier-1 (cohort-defining)** — `codename:` and `spec:`. Labels in this tier may seed a new work; the suggester emits `kerf new <derived-codename> --bead-filter 'label=<tier-1-label>'`.
- **Tier-2 (cross-cutting)** — every other prefix, including `axis:`, `tag:`, `kind:`, `scope:`, `subsystem:`, `area:`, and any prefix not in the tier-1 allow-list. The tier-1 list is a small explicit allow-list, not a tier-2 deny-list: an unfamiliar prefix falls to tier-2 by default. Tier-2 labels never seed a `kerf new`.

When all of a bead's labels are tier-2, the suggester falls back to `kerf pin <codename> <bead-id>` against the lexicographically-earliest active work, or — when no active work exists — emits a one-line `no auto-suggestion; investigate manually` note. <!-- TBD: open question 1 from plan 018 — whether the tier-1 list becomes per-project configurable. -->

#### Archive-aware suggestions

Before emitting `kerf new <codename>` for an `untriaged` bead, the suggester checks the archive index at `~/.kerf/archive/<project-id>/`. When the proposed codename already exists in the archive, the `suggest:` line is replaced with:

```
suggest: codename '{codename}' is archived — consider 'kerf restore {codename}' to unarchive,
         or 'kerf pin <codename> {bead-id}' to attach this bead to a different live work.
```

The archive scan is performed once per `kerf triage` invocation and cached for the run.

When no items exist, the output says so and prints the baseline timestamp:

```
Triage for {project-id} (baseline: 2026-05-15T11:02:00Z, 0 days ago):
  No untriaged, multi-matched, or externally-changed beads. Project is clean.
```

#### Storage-drift footer

The same footer documented under [`kerf next`](#storage-drift-footer) is appended to `kerf triage` text output when `kerf doctor` would report any non-green storage finding. The footer respects the same suppression switches (`kerf config doctor.footer false` or `KERF_DOCTOR_FOOTER=0`). Storage-layout drift is distinct from the bead-store drift reported in the body of `kerf triage`.

### Output (`--format=json`)

`--format=json` emits one record per surfaced item, in section order (untriaged → multi_matched → external_drift). Field names are snake_case. The item shape:

```
{
  "kind":          "<untriaged | multi_matched | external_drift>",
  "sub_kind":      "<external_close | external_reopen | external_delete | external_new | null>",
  "bead_id":       "<id>",
  "title":         "<bead title>",
  "status":        "<bead status>",
  "labels":        ["..."],
  "work_codenames": ["..."],
  "suggest":       "<ready-to-paste command or null>",
  "reason":        "<one-line explanation>"
}
```

`sub_kind` is populated only for `external_drift` items; `null` otherwise. `work_codenames` carries the matching works for `multi_matched`; for `untriaged` it is `[]`; for `external_drift` it reflects the bead's current filter-resolved attachment (post-pin).

The report's baseline timestamp and per-work bead-health summary are emitted as a header object preceding the item stream:

```
{
  "baseline_captured_at": "2026-05-13T09:14:00Z",
  "works": [
    { "codename": "bridge",  "filter": "label=subsystem:bridge",  "open": 4, "closed": 3 },
    { "codename": "gateway", "filter": "label=subsystem:gateway", "open": 0, "closed": 0 }
  ],
  "items": [ /* records as above */ ]
}
```

### Exit codes

`kerf triage` (without `--resolved`) exits 0 on a successful report, 1 on error. With `--resolved`, the exit code reflects drift state:

| Condition | Exit |
|-----------|------|
| Untriaged == 0, multi_matched == 0, external_drift == 0 | 0 |
| Bead store unreadable, project not initialized (`kind: not_initialized`), or other error | 1 |
| Non-zero drift, drift count decreased compared to the previous `--resolved` run | 3 |
| Non-zero drift, no progress since the previous `--resolved` run | 2 |

Exit 3 is the "made progress" signal so that a loop of the form `until kerf triage --resolved; do <act>; done` terminates: an agent that sees two consecutive exit-3 runs with identical drift sets should break out and ask for human help rather than spin. Exit 2 indicates the same drift set the agent already saw — repeating the act-then-retry cycle without changing approach will not converge.

The "previous run" comparison is in-memory across a single agent loop; kerf does not persist exit-3 history.

### Help text

`kerf triage --help` covers, in fixed order: what triage returns (drift report), the three item kinds with one-line meanings, the `--resolved` exit-code semantics including the stuck-loop guidance, `--ack` as the only baseline-advancement command, and a **Baseline lifecycle** paragraph that walks through the loop end-to-end. The lifecycle paragraph names each phase explicitly:

- First run on a fresh project shows `baseline: never` and the full current state — every untriaged or multi-matched bead is surfaced.
- Subsequent runs without `--ack` show drift accumulating since the previous baseline.
- After the agent investigates and resolves items, `kerf triage --ack` advances the baseline to the current bead-store snapshot.
- The `--resolved` exit-code loop (`until kerf triage --resolved; do <act>; done`) terminates when drift returns to zero.

A recipe line (`First run on a large project: kerf triage --top 20 --group-by codename-label`) is included so the agent has a concrete entry point.

Changes to the help text require a spec change.

### Errors

| Condition | Message | Exit |
|-----------|---------|------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` | 1 |
| `project.yaml` absent | `Error: project not initialized. Run 'kerf init' first.` (`kind: not_initialized` in JSON output) | 1 |
| Bead store unreadable | `Error: cannot read bead store: {detail}.` | 1 |
| `--resolved` and `--ack` both given | `Error: --resolved and --ack are mutually exclusive.` | 1 |
| Unknown value in `--kind` | `Error: unknown triage kind '{value}'. Known kinds: untriaged, multi_matched, external_drift.` | 1 |
| Unknown value in `--format` | `Error: unknown format '{value}'. Supported: text, json.` | 1 |

A non-zero exit from a not-yet-initialized project is *not* "drift exists" — the agent reads the `kind: not_initialized` signal and runs `kerf init` before retrying.

---

## `kerf pin`

### Purpose

Attach a specific bead to a specific work by ID, regardless of filter outcome. `kerf pin` is the escape hatch for the case where a bead cannot reasonably be caught by any filter clause, or where a multi-matched bead needs to be resolved to a single owner. See [coordination.md](coordination.md#pin-layer).

Pins are a single-owner layer: pinning a bead to one work removes it from every other work's pin list as part of the same operation. Filter resolution is unchanged — pins ride on top.

### Syntax

```
kerf pin <codename> <bead-id> [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work the bead is being pinned to. |
| `bead-id` | Yes | — | The bead ID to pin. |
| `--project` | No | Inferred from cwd | The project containing the work. |

### Behavior

1. Resolve the project ID.
2. Read the target work's `spec.yaml`. Error if the work does not exist.
3. If the bead ID is already in the target work's `pinned_beads`, exit 0 with a no-op message.
4. Scan every other active (non-archived) work's `spec.yaml` for the bead ID in `pinned_beads`. For each match, remove the bead ID from that work's list and update its `updated` timestamp. This enforces the single-owner invariant.
5. Append the bead ID to the target work's `pinned_beads` list.
6. Update the target work's `updated` timestamp.
7. Take a [snapshot](snapshots.md) of the target work (and of any other work whose `pinned_beads` list changed).

The drift baseline is not advanced (see [coordination.md](coordination.md#baseline-advancement)).

### Output

```
Pinned hk-cb-091 to bridge.
```

If the pin moved the bead from another work:

```
Pinned hk-cb-091 to bridge (removed from gateway).
```

If the bead was already pinned to the target work:

```
hk-cb-091 is already pinned to bridge. No change.
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Bead ID format invalid | `Error: bead ID '{value}' is not a valid identifier.` |
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

kerf does not validate that the bead ID exists in the bead store — pin is a kerf-side declaration. A pin pointing at a missing bead surfaces through normal drift detection as the bead being absent.

---

## `kerf work edit`

### Purpose

Edit a work's bead-attachment configuration in place. `kerf work edit` mutates the work's `spec.yaml` (resolved to `~/.kerf/projects/{project-id}/{codename}/spec.yaml` in bench mode or `{repo}/.kerf/works/{codename}/spec.yaml` in local mode) — primarily the `bead_filter` — without forcing the user to hand-edit YAML. The `--help` output names the resolved path so an agent does not have to derive it. It is the primary remediation path when a work's filter is too narrow (the `work_no_attached_beads` cleanup item) or too broad (a bead surfaces as `multi_matched`).

### Syntax

```
kerf work edit <codename> [--bead-filter-add <clause>...] [--bead-filter-remove <clause>...] [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to edit. |
| `--bead-filter-add <clause>` | No | — | Repeatable. Add a clause to the work's `bead_filter`. Accepts `label=<value>` or `id_prefix=<value>`. |
| `--bead-filter-remove <clause>` | No | — | Repeatable. Remove a clause from the work's `bead_filter`. Accepts `label=<value>` or `id_prefix=<value>`. The value must match an existing clause exactly. |
| `--project` | No | Inferred from cwd | The project containing the work. |

At least one of `--bead-filter-add` or `--bead-filter-remove` is required.

### Behavior

1. Resolve the project ID.
2. Read the target work's `spec.yaml`. Error if the work does not exist.
3. Read the current `bead_filter` value. The starting state:
   - Missing → empty clause set.
   - Direct clause (e.g., `label: "subsystem:bridge"`) → single-clause set.
   - `any:` union → the listed clauses.
4. Apply removals first, then additions. Removals match clauses by exact type and value. A removal that finds no matching clause is a warning, not an error.
5. Re-emit `bead_filter` in canonical form:
   - Zero clauses remain → remove the `bead_filter` key entirely; the work falls back to project filter then built-in default (see [coordination.md](coordination.md#resolution-order)).
   - One clause → direct clause form.
   - Two or more clauses → `any:` union form.
6. Preserve user comments and field ordering in `spec.yaml` where the YAML library permits round-trip.
7. Update the work's `updated` timestamp and take a [snapshot](snapshots.md).

The drift baseline is not advanced (see [coordination.md](coordination.md#baseline-advancement)).

### Output

```
Updated bead_filter for {codename}:
  + label=subsystem:audit
  - label=subsystem:gateway

Now matches: 5 (3 open / 2 closed). Previously: 2 (2 open / 0 closed).
```

The match-count delta is informational and breaks out open vs. closed beads on both sides so an agent can tell whether the broader filter is picking up live work or just historical clutter. When the filter is removed entirely (zero clauses remain), the output notes the fallback to the project filter or built-in default.

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| Neither `--bead-filter-add` nor `--bead-filter-remove` given | `Error: at least one of --bead-filter-add or --bead-filter-remove is required.` |
| Clause value does not parse as `label=<value>` or `id_prefix=<value>` | `Error: clause '{value}' does not parse. Expected 'label=<value>' or 'id_prefix=<value>'.` |
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

A `--bead-filter-remove` clause that matches no existing clause emits a warning but does not error:

```
Warning: --bead-filter-remove 'label=subsystem:legacy' did not match any existing clause. No change for that clause.
```

---

## `kerf work show`

### Purpose

Print a single work's `spec.yaml` field-by-field, without forcing the agent to read or parse YAML. The output is a flat, human-readable rendering of the work's metadata, filter, sessions, dependencies, and areas — the same data `kerf show` summarises, scoped tightly to one work and skipping the jig-pass, file-tree, SESSION.md, and bead-listing sections.

### Syntax

```
kerf work show <codename> [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | Yes | — | The work to dump. |
| `--project` | No | Inferred from cwd | The project containing the work. |

### Behavior

1. Resolve the project ID.
2. Read the target work's `spec.yaml`. Error if the work does not exist.
3. Emit each top-level field on its own line, in the order they appear in `spec.yaml`. List-valued fields render as one entry per indented line. The `bead_filter` slot is always rendered (literal value when present, `(none)` when absent or empty).

### Output

```
codename:       bridge
title:          Adapter retry envelope
type:           feature
status:         decompose
project_id:     harmonik
jig:            spec (v1)
created:        2026-05-12T14:03:00Z
updated:        2026-05-18T09:21:00Z
bead_filter:    label=subsystem:bridge
areas:          api-gateway, auth
depends_on:     (none)
pinned_beads:   hk-cb-099
active_session: 2026-05-18T08:00:00Z
sessions:
  - started: 2026-05-12T14:03:00Z   ended: 2026-05-12T17:40:00Z
  - started: 2026-05-18T08:00:00Z   ended: null
```

### Errors

| Condition | Message |
|-----------|---------|
| Work not found | `Error: work '{codename}' not found in project '{project-id}'.` |
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

---

## `kerf bootstrap-filters`

### Purpose

One-shot helper that proposes a `bead_filter` for every work whose resolved filter currently matches zero beads, then applies the accepted proposals. `kerf bootstrap-filters` is the remediation entry point for the `unwired` and `empty` rank labels surfaced by [`kerf next`](#kerf-next): instead of running four `kerf work edit` invocations by hand, the agent runs one command and confirms the diff.

The sampler is convention-aware: it recognises both prefixed (`codename:foo`) and bare (`foo`) label families and proposes the dominant pattern per work — not a single project-wide assumption.

### Syntax

```
kerf bootstrap-filters [--apply] [--yes] [--codename <name>...] [--format <format>] [--project <project-id>]
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--apply` | No | `false` | Mutate `spec.yaml` for each accepted proposal. Without `--apply`, the command is a dry-run preview that prints the proposals and exits without changes. |
| `--yes` | No | `false` | Skip the confirmation prompt and apply all proposals. Implies `--apply`. |
| `--codename <name>` | No | All eligible works | Repeatable. Restrict bootstrap to the named works. |
| `--format <format>` | No | `text` | Output format. `text` (default) or `json`. |
| `--project <project-id>` | No | Inferred from cwd | Project to bootstrap. |

### Behavior

1. Resolve the project ID and the active works.
2. Identify eligible works: those whose resolved `bead_filter` matches zero open beads (rank label `empty` or `unwired`). Works tagged `broken` are eligible only when the malformed clause can be parsed enough to extract a codename candidate; otherwise they are listed under "not auto-fixable" and skipped.
3. For each eligible work, the sampler:
   1. Builds a candidate label set — the work's codename, the codename combined with common prefixes (`codename:`, `subsystem:`, `area:`, `kind:`), and the bare codename slug.
   2. Counts open-bead matches for each candidate against the bead store.
   3. If exactly one candidate dominates (≥ 80% of total candidate matches with an absolute-count floor), proposes that single clause.
   4. If two or more candidates carry non-trivial counts and none dominates, proposes an `any:` union of the qualifying clauses.
   5. If no candidate yields any matches, leaves the work in the "no proposal" bucket with a one-line note that no label resembles its codename.
4. Render the proposal block. Under `--apply` (with `--yes`, or after the operator confirms), each accepted proposal is written via the same `kerf work edit --bead-filter-add` path. Without `--apply`, no changes occur.
5. Print a summary: works proposed, works applied, works skipped, works left without a proposal.

### Output (default: text, dry-run)

```
Bootstrap proposals for {project-id}:

  bridge       proposes: label=subsystem:bridge       (currently empty, would match 7 open beads)
  phase-3-dot  proposes: label=phase-3-dot            (currently unwired, would match 12 open beads)
  scratch      no proposal — no label resembles 'scratch' in the bead store

Dry-run: no changes made. Re-run with --apply to write the proposals to spec.yaml.
```

Under `--apply`, the summary line names each work that was written and the resulting match count (open / closed) in the same shape as `kerf work edit`.

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |
| Bead store unreadable | `Error: cannot read bead store: {detail}.` |
| Unknown codename in `--codename` | `Error: work '{value}' not found in project '{project-id}'.` |
| `--yes` given without `--apply` | `--yes` implies `--apply`; no error. |

---

## `kerf areas list`

### Purpose

Show all defined areas for the current project.

### Syntax

```
kerf areas list [--project <project-id>]
```

### Behavior

1. Resolve the project ID.
2. Read `areas.yaml` from `~/.kerf/projects/{project-id}/areas.yaml`.
3. For each area, count how many active works reference it.

### Output

```
Areas for {project-id}:

  auth             "Authentication and session management"     2 works
  api-gateway      "Public API surface and rate limiting"      1 work
  storage          "Database and cache layers"                 0 works

Commands:
  kerf areas add <name> --description "..."    Define a new area
  kerf areas remove <name>                     Remove an area
  kerf map                                     View works by area
```

If no areas are defined, output says so and suggests `kerf areas add`.

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

---

## `kerf areas add`

### Purpose

Define a new area for the current project.

### Syntax

```
kerf areas add <name> [--description <text>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `name` | Yes | — | Area name. Must be a lowercase slug: alphanumeric and hyphens. |
| `--description` | No | `""` | Human-readable description of what this area covers. |

### Behavior

1. Resolve the project ID.
2. Read `areas.yaml` (create it if it does not exist).
3. Validate the name format. Error if an area with this name already exists.
4. Append the new area to `areas.yaml`.

### Output

```
Area '{name}' added to project '{project-id}'.
```

### Errors

| Condition | Message |
|-----------|---------|
| Area already exists | `Error: area '{name}' already exists in project '{project-id}'.` |
| Invalid name format | `Error: area name must be lowercase alphanumeric and hyphens (matching [a-z0-9]+(-[a-z0-9]+)*).` |
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |

---

## `kerf areas remove`

### Purpose

Remove an area definition from the current project.

### Syntax

```
kerf areas remove <name>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | The area to remove. |

### Behavior

1. Resolve the project ID.
2. Read `areas.yaml`. Error if the area does not exist.
3. Check if any active works reference this area. If so, emit a warning listing those works (but proceed with removal).
4. Remove the area from `areas.yaml`.
5. The area name is not automatically removed from works that reference it. Works retain stale area references until manually updated.

### Output

```
Area '{name}' removed from project '{project-id}'.
```

If active works reference the area:

```
Warning: the following works still reference area '{name}':
  {codename-1}, {codename-2}
Area '{name}' removed from project '{project-id}'.
```

### Errors

| Condition | Message |
|-----------|---------|
| Area not found | `Error: area '{name}' not found in project '{project-id}'. Run 'kerf areas list' to see defined areas.` |
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |
