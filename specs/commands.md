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
kerf new [codename] [--title <title>] [--type <type>] [--jig <name>] [--area <name>...] [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | No | Auto-generated `adjective-noun` slug | Immutable identifier for the work. Must match `[a-z0-9]+(-[a-z0-9]+)*`. |
| `--title` | No | `null` | Human-friendly title for the work. |
| `--type` | No | Matches jig name | Work type (e.g., `feature`, `bug`). |
| `--jig` | No | `default_jig` from config.yaml (required if `default_jig` unset) | Jig to use for this work. Resolved via jig resolution order (see [jig-system.md](jig-system.md)). |
| `--area` | No | `[]` | One or more area names to associate with the work. May be repeated (e.g., `--area auth --area api`). Each name must exist in `areas.yaml`. |
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
6. **Initialize `spec.yaml`** with: codename, title, type, project ID, jig name, jig version, initial status (first value in the jig's `status_values`), `created` and `updated` timestamps, empty `sessions` list, empty `depends_on` list, empty `related_to` list, null `implementation` fields, the jig's `status_values` list, and the `areas` list from `--area` flags (empty if none provided).
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

### Errors

| Condition | Message |
|-----------|---------|
| Not in a git repo and no `--project` flag | `Error: not in a git repository. Use --project <project-id> to specify a project.` |
| Codename already exists in project | `Error: work '{codename}' already exists in project '{project-id}'.` |
| Codename format invalid | `Error: codename must be lowercase alphanumeric and hyphens (matching [a-z0-9]+(-[a-z0-9]+)*).` |
| Jig not found | `Error: jig '{name}' not found. Run 'kerf jig list' to see available jigs.` |
| Area name not in `areas.yaml` | `Error: area '{name}' not found. Run 'kerf areas list' to see defined areas, or 'kerf areas add <name>' to create one.` |
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
kerf list [--status <status>] [--project <project-id>] [--all]
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--status` | No | — | Filter to works with this status. |
| `--project` | No | Inferred from cwd | Show works for this project. |
| `--all` | No | `false` | Include archived works. |

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
kerf show <codename>
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to display. |

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
- **Jig context**: the pass corresponding to the current status, with the jig's agent instructions for that pass.
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
- **Commands block**: contextually relevant next actions:

```
Commands:
  kerf resume <codename>                 Resume working
  kerf status <codename> <next-status>   Advance status
  kerf square <codename>                 Verify completeness
  kerf shelve <codename>                 Pause work
```

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
kerf status <codename> [new-status]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `codename` | Yes | The work to query or update. |
| `new-status` | No | The status value to set. If omitted, displays current status. |

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
7. Load the jig's agent instructions for the pass corresponding to the new status.

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
      Tools: bd, ntm
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
    Tools: bd, ntm

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

Bootstrap kerf in a project. Creates the project identifier, prompts the user to select active jigs, creates `project.yaml`, optionally sets the default workflow, and runs `kerf setup` to generate agent instructions.

This is the entry point for adopting kerf in any project. The user runs `kerf init` (or tells their AI agent to run it), and the output contains everything needed to complete the setup.

### Syntax

```
kerf init [--jig <plan|spec>]
```

### Arguments and Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--jig` | No | None | Set the default workflow. Must be `plan` or `spec`. If omitted and `default_jig` is not configured, a note is printed instructing the user to choose. |

### Behavior

1. Verify the current directory is inside a git repository. Error if not.
2. Ensure the bench (`~/.kerf/`) exists. Create if missing.
3. Resolve the project ID (same derivation as `kerf new` — from git remote or directory name).
4. If `.kerf/project-identifier` does not exist, create it and print the derived ID.
5. If `--jig` is provided, set `default_jig` in config and print confirmation.
6. If `--jig` is not provided and `default_jig` is not set, print a note with the two options (`kerf config default_jig plan` and `kerf config default_jig spec`).
7. **Prompt the user to select active jigs** for the project. Present the available jigs (from the jig library) and allow the user to choose which ones to activate. For composable jigs (e.g., `implementation`), also prompt for which passes to include.
8. **Auto-detect `bead_filter`.** Reads the bead store using kerf's existing bead-read path (the same one `kerf next` uses). If the bead tool is unavailable, silently skip auto-detect — the built-in default (`label: "work:{codename}"`) applies. Otherwise:
   1. Collect existing work codenames from `~/.kerf/projects/{project-id}/` (or the bench equivalent under local storage).
   2. If zero codenames exist, skip auto-detect; init proceeds without setting `bead_filter`.
   3. List label prefixes that appear in the bead store with at least 3 beads. For each prefix `P:`, compute `match_score = (beads matching some codename via "P:{codename}") / (total beads with prefix "P:")`. Pick the highest `match_score` above 0.5.
   4. If a candidate is selected, prompt:
      ```
      Detected: 87% of beads use `subsystem:*` labels.
      Set project-wide bead_filter to `subsystem:{codename}`? [Y/n]
      ```
   5. If no candidate scores above 0.5, fall back to a manual prompt offering the top 5 prefixes by raw count plus a "type your own" option (or skip).
   6. The chosen filter is written into `project.yaml`. It is always editable later via the standard config files. See [coordination.md](coordination.md#bead-attachment).
   7. If the bead store is empty or unreachable, skip detection silently — the built-in default applies.
   8. If kerf is invoked non-interactively (stdin not a TTY), auto-detect runs but does not prompt; if a confident candidate exists it is written, otherwise no `bead_filter` is set.
9. **Create `project.yaml`** with the selected jig and pass configuration, plus the chosen `bead_filter` (if any). If `.kerf/config.yaml` already declares `storage: local`, write it to `{repo}/.kerf/project.yaml` and create the bench symlink at `~/.kerf/projects/{project-id}` pointing at `{repo}/.kerf/works/`. Otherwise write it to `~/.kerf/projects/{project-id}/project.yaml`. See [architecture.md](architecture.md) for the `project.yaml` schema and storage modes.
10. **Run `kerf setup`** to generate agent-facing instructions from the project's active jigs. The setup output is included in the init output (see Output below).

### Output

The output includes:

1. Project initialization status (created or already exists).
2. Default jig status (set, or instructions to set it).
3. Active jig selection summary (which jigs and passes were selected).
4. Bead-filter detection summary: the detected filter (if any), or a note that the built-in default applies.
5. `project.yaml` creation confirmation.
6. The full output of `kerf setup` (see [`kerf setup`](#kerf-setup)), which includes:
   - Agent setup instructions: process instructions from each active jig, tool requirements, jig sequencing, references to kerf commands
   - What to add to `.gitignore` (`.kerf/` but commit `.kerf/project-identifier`)
   - Agent-agnostic instructions to add to the agent's configuration file (CLAUDE.md, .cursorrules, etc.)
7. A verification step the agent can run to confirm the setup works.

The instructions are agent-agnostic. kerf does not know or reference any specific AI tool. The agent reading the output determines where to put the instructions based on its own configuration conventions.

### Errors

| Condition | Message |
|-----------|---------|
| Not in a git repository | `Error: not in a git repository. kerf requires a git repo.` |
| `--jig` value not `plan` or `spec` | `Error: --jig must be 'plan' or 'spec', got '{value}'.` |

---

## `kerf localize`

### Purpose

Migrate a project from bench storage to local storage. Moves all in-progress works from `~/.kerf/projects/{project-id}/` into `{repo}/.kerf/works/`, moves `project.yaml` (and `areas.yaml`, if present) into the repo, writes `storage: local` to `{repo}/.kerf/config.yaml`, and replaces the bench project directory with a symlink pointing at the repo's works directory. See [architecture.md](architecture.md#storage-modes).

### Syntax

```
kerf localize [--project <project-id>]
```

### Arguments and Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
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

The output is a clearly delimited block of agent-facing instructions containing:

- **Process instructions** from each active jig: the full agent-facing process for each jig's passes.
- **Tool requirements**: which external tools are needed (e.g., `bd` for bead management, `ntm` for orchestration), as declared by the active jigs.
- **Jig sequencing**: the composition chain for this project — which jigs are available and in what SDLC order.
- **References to kerf commands**: the kerf commands relevant to each phase (e.g., `kerf new --jig plan` for planning, `kerf new --jig implementation` for implementation).

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
     - `work_no_attached_beads` — fires when a work's resolved `bead_filter` matches zero beads (attached_count == 0). Suggested action: edit the work's `spec.yaml` or the project filter.
     - `work_beads_done_status_open` — fires when a work has attached_count > 0, every attached bead is closed, and the work's jig status is not terminal. Suggested action: advance status (`kerf status <codename> <next-stage>`) or `kerf shelve <codename>`.

     The two detectors are mutually exclusive by construction (the attached-count guard on `work_beads_done_status_open` ensures a zero-bead work is reported only by `work_no_attached_beads`).
   - **Warning detectors** — project-level state checks. v1 detectors:
     - Unmatched beads: any beads in the store that match no work's filter. Surfaced once as a single warning item.
     - Filter literal yields zero matches: when the project-wide `bead_filter`'s literal prefix matches nothing in the bead store, surface a warning suggesting a case-mismatch check (e.g., `Subsystem:` vs `subsystem:`). Matching is case sensitive — see [coordination.md](coordination.md#bead-attachment).
4. **Exclude** items per kind:
   - `bead` items are excluded when their target work is blocked by an unmet `must-complete-first` dependency, archived, or finalized. Dependency gating remains strict on jig status — see [dependencies.md](dependencies.md). The bead-done-but-status-stale case is intentionally surfaced as a `cleanup` item rather than auto-clearing dependency gates.
   - `cleanup` items are excluded only when their target work is archived or finalized. A blocked work still surfaces its cleanup items — those items are how the user resolves the block.
   - `warning` items are project-level and not filtered by work state.
5. **Score** each kind separately. Beads rank against beads using the factors described in [coordination.md](coordination.md#computed-priority) — dependency fan-out, completion momentum, rework, area focus. Cleanup items do not enter bead scoring; they sort after all beads, ordered by their parent work's would-be bead score (descending), so that a stale-status work near the top of the queue is visible without leapfrogging genuinely-blocking new work. Warning items are not ranked.
6. **Filter** by the kind selection from the flags above.
7. **Render** the feed.

### Default kind selection

Without flags, the feed includes `bead` and `cleanup` items. Beads are ranked first; cleanup items follow after all beads, ordered by their parent work's would-be bead score. `warning` items, when present, are rendered as a header block above the ranked list — they are project-wide, not ranked entries.

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

When unmatched beads are present, the feed prepends a warning block:

```
warning: 12 beads match no work — check bead_filter in project.yaml
```

If no items exist, the output says so.

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

Filter flags apply to JSON output identically.

### Help text

`kerf next --help` is part of the spec — the agent's contract. A fresh agent running it once must come away knowing the full loop. The help text covers, in this fixed order:

1. **What it returns** — a ranked feed of things to act on right now.
2. **The item kinds** — one line per kind: `bead` = work on this; `cleanup` = resolve this on a work; `warning` = project-level issue, fix config.
3. **The default action loop** — read the top item, do it, re-run `kerf next`.
4. **The filter flags** with concrete examples: `--only=bead`, `--include=warning`, `--kinds=bead,cleanup`.
5. **Machine output** — `--format=json` for scripts.
6. **How scoring works** in one sentence, with a pointer to [coordination.md](coordination.md#computed-priority) for detail.

Changes to this help text require a spec change.

### Errors

| Condition | Message |
|-----------|---------|
| No project resolvable | `Error: cannot determine project. Use --project <project-id> or run from inside a git repo with .kerf/project-identifier.` |
| Unknown kind in `--only`/`--include`/`--kinds` | `Error: unknown item kind '{value}'. Known kinds: {list of kinds from the current build}.` |
| Unknown value in `--format` | `Error: unknown format '{value}'. Supported: text, json.` |

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
