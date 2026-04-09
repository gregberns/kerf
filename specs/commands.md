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
kerf new [codename] [--title <title>] [--type <type>] [--jig <name>] [--project <project-id>]
```

### Arguments and Flags

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `codename` | No | Auto-generated `adjective-noun` slug | Immutable identifier for the work. Must match `[a-z0-9]+(-[a-z0-9]+)*`. |
| `--title` | No | `null` | Human-friendly title for the work. |
| `--type` | No | Matches jig name | Work type (e.g., `feature`, `bug`). |
| `--jig` | No | `default_jig` from config.yaml (required if `default_jig` unset) | Jig to use for this work. Resolved via jig resolution order (see [jig-system.md](jig-system.md)). |
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
5. **Create the work directory** at `~/.kerf/projects/{project-id}/{codename}/`.
6. **Initialize `spec.yaml`** with: codename, title, type, project ID, jig name, jig version, initial status (first value in the jig's `status_values`), `created` and `updated` timestamps, empty `sessions` list, empty `depends_on` list, null `implementation` fields, and the jig's `status_values` list.
7. **Record session.** Append a session entry to `sessions` with the current timestamp and `ended: null`. Set `active_session`.
8. **Take a snapshot** of the initial state (see [snapshots.md](snapshots.md)).

### Output

- Confirmation: work created, codename, project ID, jig name.
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
- **Next steps**: suggested actions based on the current pass and SESSION.md content.

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

Show available jigs.

### Syntax

```
kerf jig list
```

### Behavior

1. Enumerate jigs from all resolution sources in order: user-level (`~/.kerf/jigs/`), then built-in defaults.
2. For each jig, read its frontmatter to extract name, description, and version.
3. If a user-level jig has the same name as a built-in jig, only the user-level jig appears (it overrides the built-in).

### Output

```
Available jigs:
  plan (also: feature)    Write a plan before changing code. ...    v1    built-in
  spec                    Maintain a living spec that defines ...   v1    built-in
  bug                     Investigate and specify a fix for ...     v2    built-in

Commands:
  kerf jig show <name>    View full jig definition
```

If a jig has aliases, they appear in parentheses after the canonical name. User-level jigs that override a built-in show `user` as the source.

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
2. Move the work directory from `~/.kerf/projects/{project-id}/{codename}/` to `~/.kerf/archive/{project-id}/{codename}/`.
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
