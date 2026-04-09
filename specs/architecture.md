# Architecture

> Bench layout, project identity, and global configuration.

## The Bench

The **bench** is the root workspace directory where all kerf data lives. The default location is `~/.kerf/`.

The bench is outside any git repository. This separation ensures:

- All worktrees for the same repo share the same works
- No git operations (branches, commits, PRs) are required for spec work in progress
- Multiple works can be in flight simultaneously without git conflicts
- Works enter git only at [finalization](finalization.md)

### Bench Directory Structure

```
~/.kerf/
  config.yaml                    # global configuration
  jigs/                          # user-level jig definitions (see jig-system.md)
  archive/                       # archived works, hidden from `kerf list`
    {project-id}/
      {codename}/
  projects/
    {project-id}/                # one directory per project
      {codename}/                # one directory per work (see works.md)
```

- `config.yaml` — global configuration. See [Global Configuration](#global-configuration) below.
- `jigs/` — user-level jig overrides and custom jigs. See [jig-system.md](jig-system.md) for format and resolution order.
- `archive/` — works moved here are hidden from `kerf list` but otherwise retain their structure.
- `projects/` — the primary storage area. Each project has its own subdirectory keyed by project ID. Each work within a project has its own subdirectory keyed by codename. See [works.md](works.md) for work directory contents.

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

## Global Configuration

The file `~/.kerf/config.yaml` contains bench-wide settings. All fields are optional. kerf operates with sensible defaults when no config file exists.

### Schema

```yaml
# ~/.kerf/config.yaml

# Default jig assigned to new works when no --jig flag is provided.
# Must match a jig name resolvable via the jig resolution order (see jig-system.md).
# Default: "feature"
default_jig: feature

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

## Bench vs. Repo Boundary

The bench (`~/.kerf/`) and the repository are separate domains with a defined interface.

### What lives in the bench (outside git)

- All work directories and their contents (spec.yaml, SESSION.md, artifacts, snapshots)
- Global configuration (`config.yaml`)
- User-level jig definitions
- Archived works

### What lives in the repository (inside git)

- `.kerf/project-identifier` — the project ID file
- Finalized work artifacts (placed by `kerf finalize`)

### The interface between them

- **Project identity** links the two: `.kerf/project-identifier` in the repo maps to `~/.kerf/projects/{project-id}/` in the bench.
- **Finalization** is the only process that copies data from bench to repo. See [finalization.md](finalization.md).
- kerf reads the repository (e.g., to determine the current project, the default branch, or uncommitted changes) but never writes to it except during finalization.
