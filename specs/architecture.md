# Architecture

> Bench layout, project identity, storage modes, and configuration.

## The Bench

The **bench** is the root workspace directory where kerf indexes every project. The default location is `~/.kerf/`. Whether a project's work directories live on the bench or in the repo depends on its [storage mode](#storage-modes); in both cases the bench remains the universal index of all projects.

The bench itself is outside any git repository. For bench-mode projects this means:

- All worktrees for the same repo share the same works
- No git operations (branches, commits, PRs) are required for spec work in progress
- Multiple works can be in flight simultaneously without git conflicts
- Works enter git only at [finalization](finalization.md)

For projects using local storage, work directories live in the repo and the bench holds a symlink to them — see [Storage Modes](#storage-modes).

### Bench Directory Structure

```
~/.kerf/
  config.yaml                    # global configuration
  jigs/                          # user-level jig definitions (see jig-system.md)
  archive/                       # archived works, hidden from `kerf list`
    {project-id}/
      {codename}/
  projects/
    {project-id}/                # one entry per project — directory (bench mode)
                                 # or symlink to repo's .kerf/works/ (local mode)
      project.yaml               # per-project jig configuration (bench mode only)
      areas.yaml                 # area definitions (bench mode only)
      {codename}/                # one directory per work (see works.md)
```

- `config.yaml` — global configuration. See [Global Configuration](#global-configuration) below.
- `jigs/` — user-level jig overrides and custom jigs. See [jig-system.md](jig-system.md) for format and resolution order.
- `archive/` — works moved here are hidden from `kerf list` but otherwise retain their structure. Archived works always live on the bench regardless of storage mode.
- `projects/{project-id}/` — entry point for a project. In bench mode this is a real directory containing the project's works, `project.yaml`, and `areas.yaml`. In local mode this is a symlink to the repo's `.kerf/works/` directory; `project.yaml` and `areas.yaml` instead live in the repo at `.kerf/project.yaml` and `.kerf/areas.yaml`. See [Storage Modes](#storage-modes).
- `project.yaml` — per-project configuration. Declares which jigs are active and how composable jigs are configured. See [Project Configuration](#project-configuration) below.
- `areas.yaml` — area definitions for this project. Areas are named regions of the system used for overlap detection and work coordination. See [coordination.md](coordination.md).

The filesystem is the database. Files are the source of truth. There is no separate datastore.

## Project Identity

Each project (git repository) is identified by a **project ID** — a stable slug stored in a file at `.kerf/project-identifier` in the repository root. This file is committed to git.

The project ID determines the subdirectory under `~/.kerf/projects/` where works for that project are stored.

### Format

The project ID is a lowercase slug containing only alphanumeric characters and hyphens. Example: `acme-webapp`.

### Derivation

On first `kerf` use in a repository, if `.kerf/project-identifier` does not exist, kerf derives the project ID:

1. Parse the git remote `origin` URL. Extract the `{owner}/{repo}` path. Slugify to `{owner}-{repo}` (e.g., `github.com/acme/webapp` → `acme-webapp`).
2. If no remote is configured, fall back to the repository's root directory name.
3. Write the result to `.kerf/project-identifier`.

### Properties

- **Stable across moves and renames.** The project ID is stored in the repo, not derived from the filesystem path at runtime.
- **Worktree-friendly.** `.kerf/project-identifier` is committed to git, so all worktrees and checkouts of the same repo resolve to the same project ID.
- **User-overridable.** The user can edit `.kerf/project-identifier` at any time to change the project ID.
- **Cross-project lookup.** Commands that accept a `--project` flag use the project ID to locate works in other projects. See [commands.md](commands.md).

### Collision Handling

If a derived project ID matches a project ID already present in the bench but associated with a different git remote, kerf warns the user and requires manual resolution. It does not automatically rename or merge.

### Monorepos

Multiple logical projects within a single git repository share the same `.kerf/project-identifier` and therefore the same project ID. For v1, monorepo users who need separate project IDs must manually edit `.kerf/project-identifier` per-checkout or use worktrees with different identifier files.

## Storage Modes

Each project chooses where in-progress works are stored.

- **bench** (default) — works live at `~/.kerf/projects/{project-id}/{codename}/`. The bench owns the work directories.
- **local** — works live at `{repo-root}/.kerf/works/{codename}/`. The repo owns the work directories. The bench keeps a symlink at `~/.kerf/projects/{project-id}` pointing at `{repo-root}/.kerf/works/` so that bench-scoped queries (e.g., `kerf list --all-projects`) still see the project transparently.

The internal structure of a work directory is identical in both modes — `spec.yaml`, `SESSION.md`, `.history/`, and jig artifacts live in the same relative locations within the codename directory.

### Repo Configuration File

Local storage is opted into by writing a file at `{repo-root}/.kerf/config.yaml`:

```yaml
# .kerf/config.yaml — committed to git
storage: local   # or "bench" (default)
```

This file is the **repo configuration**. It is separate from the global bench configuration at `~/.kerf/config.yaml`. For v1 it holds only the `storage` field; future settings may follow. When a setting appears in both the repo config and the bench config, the repo config wins for that project.

### File Locations by Mode

| File | Bench mode | Local mode |
|------|------------|------------|
| Work directory | `~/.kerf/projects/{id}/{codename}/` | `{repo}/.kerf/works/{codename}/` |
| `project.yaml` | `~/.kerf/projects/{id}/project.yaml` | `{repo}/.kerf/project.yaml` |
| `areas.yaml` | `~/.kerf/projects/{id}/areas.yaml` | `{repo}/.kerf/areas.yaml` |
| Repo config | (not used) | `{repo}/.kerf/config.yaml` |
| Bench symlink | (not used) | `~/.kerf/projects/{id}` → `{repo}/.kerf/works/` |
| Archive | `~/.kerf/archive/{id}/{codename}/` | `~/.kerf/archive/{id}/{codename}/` |

### Migration

A project switches from bench to local mode with `kerf localize` (see [commands.md](commands.md)). The reverse migration is a manual procedure in v1.

### Symlink Lifecycle

The bench symlink is created by `kerf localize` and re-created if missing by `kerf init` or `kerf new` when local storage is active. If the symlink target is missing (e.g., the repo was moved or deleted), kerf emits a warning and commands that need the work directory error with a message pointing the user back to the repo.

Symlinks rely on `os.Symlink`. On platforms where symlink creation fails, kerf surfaces a clear error rather than silently falling back; local storage is effectively unavailable on those platforms in v1.

### Git Worktrees

`.kerf/` lives in the working tree, not in `.git/`. With local storage and multiple worktrees, each worktree has its own `.kerf/works/`; the bench symlink can point to only one of them. Worktree users who need cross-worktree visibility via the bench should stay on bench storage in v1.

## Global Configuration

The file `~/.kerf/config.yaml` contains bench-wide settings. All fields are optional. kerf operates with sensible defaults when no config file exists.

### Schema

```yaml
# ~/.kerf/config.yaml

# Default jig assigned to new works when no --jig flag is provided.
# Must match a jig name resolvable via the jig resolution order (see jig-system.md).
# Default: unset. When unset, `kerf new` without --jig emits an onboarding
# error with instructions to choose a workflow. See commands.md.
# default_jig: plan

# Path relative to repo root where system specs live.
# Used by the spec-first jig at finalization to copy drafted spec files
# to the repository. Only meaningful for spec-first projects.
# If the directory does not exist at finalization time, kerf creates it.
# Default: "specs/"
# Note: Per-project config scoping is not supported. Users working across
# multiple projects with different spec paths should set spec_path before
# running `kerf finalize`.
# spec_path: specs/

# Default project for commands run outside a git repository.
# When inside a repo, the project is always inferred from .kerf/project-identifier.
# When outside a repo and no --project flag is given, this value is used.
# Optional — if absent and no project can be inferred, kerf errors.
# default_project: acme-webapp

# Snapshot settings.
# See snapshots.md for snapshot structure and trigger details.
snapshots:
  # Whether automatic snapshots are enabled.
  # Default: true
  enabled: true

  # Interval-based snapshots: on each command invocation, if more than
  # interval_seconds have elapsed since the last snapshot, take a new one.
  # No background daemon — the check happens only when kerf runs.
  # Default: false
  interval_enabled: false

  # Seconds between interval snapshots.
  # Default: 300
  interval_seconds: 300

  # Maximum snapshots retained per work. When exceeded, the oldest are pruned.
  # Default: 100
  max_snapshots: 100

# Session settings.
# See sessions.md for session tracking details.
sessions:
  # Hours before an active session is considered stale.
  # Default: 24
  stale_threshold_hours: 24

# Finalization defaults.
# See finalization.md for the full finalization process.
finalize:
  # Path within the target repo where finalized work artifacts are placed.
  # The token {codename} is replaced with the work's codename.
  # Default: ".kerf/{codename}/"
  repo_spec_path: ".kerf/{codename}/"
```

### Semantics

- **Missing file.** If `config.yaml` does not exist, kerf uses defaults for all settings. It does not create the file automatically.
- **Unknown keys.** kerf ignores unrecognized keys without error. This supports forward compatibility.
- **Overrides.** Individual settings can be overridden by CLI flags where applicable. CLI flags take precedence over `config.yaml` values.

### `spec_path` vs `finalize.repo_spec_path`

These are distinct settings with different purposes:

- **`spec_path`** — where the project's normative system specs live (e.g., `specs/`). Used by the spec-first jig at finalization to copy drafted spec changes into the repository. Only relevant for spec-first projects.
- **`finalize.repo_spec_path`** — where kerf copies work process artifacts (problem space, analysis, components, tasks, etc.) at finalization. Used by all jigs. Default: `.kerf/{codename}/`.

For a spec-first project, both are used during finalization: process artifacts go to `repo_spec_path`, drafted spec files go to `spec_path`. See [finalization.md](finalization.md).

## Project Configuration

The file `~/.kerf/projects/{project-id}/project.yaml` contains per-project settings. It is optional — projects without it use all available jigs with default settings.

### Schema

```yaml
# ~/.kerf/projects/{project-id}/project.yaml

# Jigs active for this project.
# When set, only these jigs are available for `kerf new` in this project.
# When absent, all jigs (built-in and user-level) are available.
# jigs:
#   - plan
#   - implementation
#   - spike

# Composable jig pass configuration.
# For jigs with `composable: true`, specify which passes to include.
# Passes not listed are deactivated. Order follows the jig's definition.
# passes:
#   implementation:           # jig name
#     - breakdown             # pass names to include
#     - dispatch
#     - implement
#     - review

# Tool declarations for process jigs.
# Declares which tools are used for each role in this project.
# Informational — emitted in `kerf setup` output so agents know what to use.
# tools:
#   orchestrator: ntm         # agent orchestration tool
#   tasks: bd                 # task/bead management tool

# Queue scoring weights for `kerf next`.
# Any field omitted falls back to its default. See coordination.md.
# queue:
#   fan_out: 10.0
#   momentum: 5.0
#   creation: 0.1

# Project-wide bead attachment filter for `kerf next` and related views.
# Resolution order: per-work bead_filter (in spec.yaml) → this project filter →
# built-in default `label: "work:{codename}"`. First hit wins.
# See coordination.md#bead-attachment.
# bead_filter:
#   label: "subsystem:{codename}"
```

### Semantics

- **Missing file.** If `project.yaml` does not exist, all jigs are available and composable jigs use all passes. This is the default for new projects.
- **Created by `kerf init`.** When `kerf init` runs in a project, it creates `project.yaml` with the user's jig selections. The user is prompted to choose active jigs and configure composable passes.
- **Updated by `kerf setup`.** Running `kerf setup` re-reads `project.yaml` to generate fresh agent config. It does not modify `project.yaml` — that is the user's configuration.
- **Unknown keys.** kerf ignores unrecognized keys without error.
- **Relationship to `config.yaml`.** `project.yaml` contains project-specific jig configuration. `config.yaml` contains bench-wide defaults (default_jig, snapshot settings, etc.). `project.yaml` settings take precedence over `config.yaml` for the given project.

### Interaction with `kerf jig list`

When inside a project with a `project.yaml`:
- `kerf jig list` shows which jigs are active for the current project vs. available but not activated
- For composable jigs, shows which passes are active
- Shows tool declarations

When no `project.yaml` exists, `kerf jig list` shows all available jigs without activation status.

## Bench vs. Repo Boundary

The bench (`~/.kerf/`) and the repository are separate domains with a defined interface. What sits on each side depends on the storage mode.

### Always on the bench

- Global configuration (`~/.kerf/config.yaml`)
- User-level jig definitions
- Archived works (`~/.kerf/archive/`)
- The project index (`~/.kerf/projects/{project-id}/`), either as a real directory (bench mode) or a symlink into the repo (local mode)

### In the repo (inside git)

- `.kerf/project-identifier` — the project ID file
- `.kerf/config.yaml` — repo configuration, when present (declares storage mode)
- Finalized work artifacts placed by `kerf finalize`
- In local mode: `.kerf/works/`, `.kerf/project.yaml`, `.kerf/areas.yaml`

### Storage-mode placement

- **Bench mode**: work directories, `project.yaml`, and `areas.yaml` live under `~/.kerf/projects/{project-id}/`.
- **Local mode**: work directories live in `{repo}/.kerf/works/`; `project.yaml` and `areas.yaml` live in `{repo}/.kerf/`.

### The interface between them

- **Project identity** links the two: `.kerf/project-identifier` in the repo maps to `~/.kerf/projects/{project-id}/` on the bench (a directory or symlink, depending on mode).
- **Finalization** copies data into the repo (typically into `.kerf/{codename}/`). See [finalization.md](finalization.md). In local mode, work artifacts already live in the repo at `.kerf/works/{codename}/`; finalization still copies them to the finalized location so it remains a permanent record.
- kerf reads the repository to determine project identity, storage mode, and branch state. It writes to the repo during finalization and during `kerf localize`; in local mode it also writes work-directory contents under `.kerf/works/` during normal operation.

For how areas and cross-work relationships support coordination across concurrent work items, see [coordination.md](coordination.md).
