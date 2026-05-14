# Plan 004 — Local Storage

## Intent

Add a project-level setting that controls where in-progress works are stored: on the bench (`~/.kerf/projects/`) as today, or locally in the git repository under `.kerf/works/`. Local storage gives teams auditability, visibility, and the ability to commit work-in-progress alongside code. The bench remains the universal index via symlinks.

## Why

The bench-only model works well for solo developers and projects where in-progress specs are private. But for teams, keeping works outside the repo has real costs:

1. **Auditability.** Work artifacts are invisible to code review and git history until finalization. There's no way to see the spec-writing process in progress.
2. **Team visibility.** Other developers can't see what's being spec'd unless they have access to the same bench. In multi-developer setups, each developer has their own `~/.kerf/` — works are siloed.
3. **Backup.** The bench is a single machine's filesystem. If the machine dies, all in-progress works are lost. Local storage means works are backed up with the repo.
4. **CI/tooling.** Downstream tools (linters, reviewers, CI checks) can't operate on works that don't exist in the repo.

The design preserves the bench as the universal index (via symlinks) so cross-project queries and `kerf list --all-projects` continue to work without changes.

## What Changes

### New: Repo config file (`.kerf/config.yaml`)

A **project-level** config file at `.kerf/config.yaml` in the repo root. This is committed to git and shared by the team. It is distinct from the global bench config at `~/.kerf/config.yaml`.

```yaml
# .kerf/config.yaml (in repo root, committed to git)

# Where in-progress works are stored.
# "bench" — works live at ~/.kerf/projects/{project-id}/{codename}/ (default)
# "local" — works live at .kerf/works/{codename}/ in the repo
# Default: bench
storage: local
```

**Resolution order for settings:** Repo config (`.kerf/config.yaml`) applies to the current project only. If a setting exists in both repo config and bench config (`~/.kerf/config.yaml`), repo config wins for that project. Bench config provides defaults for settings not present in repo config.

**Scope for v1:** The repo config file contains only the `storage` field. Other settings (`default_jig`, `spec_path`, `snapshots`, etc.) remain in bench config. Expanding repo config to cover more settings is a future enhancement.

### New: Local work path (`.kerf/works/`)

When `storage: local`, works live at `.kerf/works/{codename}/` in the repo. This path:

- Keeps works contained under `.kerf/`, which is already a kerf-owned directory (contains `project-identifier` and finalized artifacts at `.kerf/{codename}/`).
- Avoids collision with `project-identifier` and finalized work artifacts (which live at `.kerf/{codename}/`, not `.kerf/works/{codename}/`).
- Is predictable and greppable.

The internal structure of a work directory is identical regardless of storage mode. `spec.yaml`, `SESSION.md`, `.history/`, and jig artifacts all live in the same relative locations within the codename directory.

### New: Symlink for bench integration

When local storage is active, kerf creates a symlink:

```
~/.kerf/projects/{project-id}  -->  {repo-root}/.kerf/works/
```

This means:

- `kerf list --all-projects` still works — it scans the bench and follows symlinks transparently.
- Cross-project queries (`--project`) still work.
- The bench remains the universal index of all projects and works.
- Only one symlink per project, pointing to the repo's works directory (not per-work symlinks).

**Symlink lifecycle:**
- **Created by** `kerf localize` (migration command) or `kerf new` when `storage: local` and no symlink exists yet.
- **Validated on command invocation.** If the symlink target doesn't exist (e.g., repo was moved or deleted), kerf warns but doesn't error. Commands that need the work directory will error with a specific message: `Error: symlink at ~/.kerf/projects/{project-id} points to {target} which does not exist. Run 'kerf localize' from the repo to fix.`
- **Not auto-deleted.** If the user switches back to bench storage, the symlink is replaced with a real directory (by `kerf delocalize` or manual intervention). kerf does not silently delete symlinks.

### New: `kerf localize` command

One-shot migration from bench storage to local storage.

**Syntax:**

```
kerf localize [--project <project-id>]
```

**Behavior:**

1. Resolve the project ID (from `.kerf/project-identifier` or `--project`).
2. Verify `.kerf/config.yaml` doesn't already have `storage: local`. If it does, emit "Already using local storage" and exit.
3. Verify `~/.kerf/projects/{project-id}/` exists and is a real directory (not already a symlink).
4. Create `.kerf/works/` in the repo if it doesn't exist.
5. Move all work directories from `~/.kerf/projects/{project-id}/` to `.kerf/works/` in the repo. This includes all codename directories. `project.yaml` is handled separately (see §project.yaml below).
6. Replace `~/.kerf/projects/{project-id}/` with a symlink pointing to `{repo-root}/.kerf/works/`.
7. Write `storage: local` to `.kerf/config.yaml` in the repo (creating the file if needed, preserving existing content if present).
8. Emit confirmation and next steps.

**Output:**

```
Localized project '{project-id}' to {repo-root}/.kerf/works/
Moved {n} works: {codename-1}, {codename-2}, ...
Symlink: ~/.kerf/projects/{project-id} -> {repo-root}/.kerf/works/

Next steps:
  git add .kerf/config.yaml .kerf/works/
  git commit -m "kerf: enable local storage"
```

**Errors:**

| Condition | Message |
|-----------|---------|
| Not in a git repo and no `--project` | `Error: not in a git repository. Use --project <project-id> to specify a project.` |
| Already local | `Already using local storage for project '{project-id}'.` |
| No bench directory for project | `Error: no works found on bench for project '{project-id}'.` (Warn, still set up local storage.) |
| Move failure (permissions, etc.) | `Error: failed to move {codename}: {reason}. Localization aborted — no changes made.` |

**Atomicity:** If any work move fails, the entire operation is aborted and rolled back (works moved back to bench, symlink not created, config not written). This prevents half-migrated states.

### `kerf delocalize` — not in v1

Inverse migration (local back to bench) is explicitly deferred. The user can do it manually if needed: move works back to `~/.kerf/projects/{project-id}/`, remove the symlink, delete `storage: local` from `.kerf/config.yaml`. Documenting the manual steps in `kerf localize --help` is sufficient for v1.

### project.yaml location

When `storage: local`, `project.yaml` moves to `.kerf/project.yaml` in the repo alongside `config.yaml` and `project-identifier`. This makes it team-shared — all developers see the same jig configuration.

**Migration:** `kerf localize` moves `project.yaml` from `~/.kerf/projects/{project-id}/project.yaml` to `{repo-root}/.kerf/project.yaml`.

**Resolution order:** When loading project.yaml, kerf checks:
1. `{repo-root}/.kerf/project.yaml` (if `storage: local`)
2. `~/.kerf/projects/{project-id}/project.yaml` (bench location)

The first one found wins. If both exist (e.g., mid-migration), the repo copy takes precedence when `storage: local`.

### Archive stays on bench

Archived works (`~/.kerf/archive/`) remain on the bench regardless of storage mode. Archives are "out of sight" by design — cluttering the repo with archived works defeats the purpose of archiving.

`kerf archive` moves the work from `.kerf/works/{codename}/` (local) or `~/.kerf/projects/{project-id}/{codename}/` (bench) to `~/.kerf/archive/{project-id}/{codename}/`. The behavior is the same in both modes — the destination is always the bench archive.

### Finalization when local

When storage is local, finalization behavior changes slightly:

- **Spec-first finalization** (copy `05-spec-drafts/` to `spec_path`): unchanged. Works the same regardless of storage mode.
- **Process artifact copying** (copy work artifacts to `finalize.repo_spec_path`): When local, the source is `.kerf/works/{codename}/` instead of `~/.kerf/projects/{project-id}/{codename}/`. The destination (`finalize.repo_spec_path`, default `.kerf/{codename}/`) is the same.
- **Key difference:** With local storage, work artifacts are already in the repo (at `.kerf/works/{codename}/`). Finalization still copies them to the finalization path (`.kerf/{codename}/`) because the finalized location is the permanent record, while `.kerf/works/` is for in-progress works. After finalization, the work directory in `.kerf/works/` can be archived or deleted — the finalized copy persists.

No changes to the finalization commit, branch creation, or post-finalization flow.

### The key abstraction: work directory resolution

Most code paths in kerf flow through a single point that resolves a work's directory path. Today this always returns `~/.kerf/projects/{project-id}/{codename}/`. The change is:

- If `storage: local` in repo config: return `{repo-root}/.kerf/works/{codename}/`
- If `storage: bench` or unset: return `~/.kerf/projects/{project-id}/{codename}/`

This is the primary code change. Commands like `kerf new`, `kerf show`, `kerf resume`, `kerf status`, `kerf square`, `kerf finalize`, `kerf list`, `kerf archive`, `kerf delete`, `kerf snapshot`, and `kerf history` all use this resolved path. They don't need to know which storage mode is active.

Similarly, the project directory resolution (where `project.yaml` and work directories live) needs the same branching:

- If local: `{repo-root}/.kerf/works/` (works) and `{repo-root}/.kerf/` (project.yaml)
- If bench: `~/.kerf/projects/{project-id}/` (both)

## Specs Affected

| Spec file | Change type | What changes |
|-----------|-------------|-------------|
| `specs/architecture.md` | **Modify** | Add repo config file (`.kerf/config.yaml`) with `storage` field. Add `.kerf/works/` directory to repo layout. Add symlink behavior. Update bench vs. repo boundary section — works can now live in either location. Add resolution order for repo config vs. bench config. |
| `specs/works.md` | **Modify** | Update "each work directory lives at `~/.kerf/projects/{project-id}/{codename}/`" to include the local variant. Add a section on storage modes. Work directory contents are unchanged. |
| `specs/commands.md` | **Modify** | Add `kerf localize` command specification. Update `kerf new` behavior (create symlink if needed when `storage: local`). Minor: note that `kerf archive` always targets the bench. |
| `specs/finalization.md` | **Modify** | Note that the artifact source path depends on storage mode. Clarify that finalization still copies to `repo_spec_path` even when works are already in the repo. |
| `specs/cli.md` | **No change** | CLI principles and output philosophy are unaffected. |

## Edge Cases and Open Questions

### Git worktrees with local storage

When a repo uses git worktrees, `.kerf/` is per-worktree (it's in the working tree, not `.git/`). This means:

- Each worktree has its own `.kerf/works/` directory with its own works.
- The symlink from the bench points to one worktree's `.kerf/works/`. Works in other worktrees are invisible to `kerf list --all-projects` via the bench.
- **Mitigation for v1:** Document this as a known limitation. Worktree users who need cross-worktree visibility should use bench storage. Alternatively, they can create additional symlinks manually.
- **Future:** Support multiple symlinks per project (one per worktree), or scan known worktree locations.

### Stale symlinks

The symlink at `~/.kerf/projects/{project-id}` can become stale if:

- The repo is moved to a different path.
- The repo is deleted.
- The `.kerf/works/` directory is removed.

**Handling:** On every command that accesses the project directory, kerf checks if the path is a symlink and if the target exists. If the target is missing, kerf emits a warning: `Warning: project symlink points to {target} which does not exist.` Commands that need the work directory error with instructions to run `kerf localize` from the repo.

### `.gitignore` considerations

When using local storage, users may want to selectively ignore certain files within `.kerf/works/`:

- `.history/` directories (snapshots can be large and noisy in git)
- `SESSION.md` (per-session state, not always useful in git)

**Recommendation in v1:** `kerf localize` does NOT automatically modify `.gitignore`. The next-steps output suggests adding a `.gitignore` entry if desired:

```
Tip: To exclude snapshots from git, add to .gitignore:
  .kerf/works/*/.history/
```

### Multiple repos sharing a project ID

If two repos have the same project ID (collision), the symlink can only point to one. This is the same collision problem described in `architecture.md` §Collision Handling, now with an additional consequence: the symlink points to one repo, making the other's local works invisible via the bench.

**Handling:** Same as existing — kerf warns on collision and requires manual resolution.

### Switching between storage modes

A project might start on bench, localize, then want to go back. For v1, `kerf delocalize` is not implemented, but the manual process should be straightforward and documented in `kerf localize --help`.

### `kerf init` and local storage

`kerf init` currently sets up `project.yaml` on the bench. When `storage: local` is already configured (e.g., `.kerf/config.yaml` exists with `storage: local` before `kerf init` runs), `kerf init` should write `project.yaml` to `.kerf/project.yaml` in the repo instead of the bench.

### Interaction with `kerf list --all-projects`

`kerf list --all-projects` scans `~/.kerf/projects/`. With symlinks, this transparently follows into repo-local works directories. No code change needed — the filesystem handles it. The only issue is stale symlinks (covered above).

## Implementation Notes

1. **Read `.kerf/config.yaml` from repo root.** Add a new config source that reads the repo-level config file. This is loaded alongside the bench config, with repo config taking precedence for overlapping keys.

2. **Work directory resolution is the core abstraction.** The function that resolves `(project-id, codename) -> filesystem path` is the primary change point. Today it always returns `~/.kerf/projects/{project-id}/{codename}/`. After this change, it checks the storage mode and returns the appropriate path.

3. **Project directory resolution.** Similar to work directory resolution, but for the project-level path (where `project.yaml` lives and where work directories are enumerated).

4. **Symlink management.** A small utility that creates, validates, and reports on the bench symlink. Used by `kerf localize` and by `kerf new` (to ensure the symlink exists when `storage: local`).

5. **`kerf localize` is the only migration command.** It handles the full transition: move works, create symlink, write config. All other commands just read the storage mode and resolve paths accordingly.

6. **Test strategy.** The work directory resolution function is the critical test surface. Test both modes (bench and local) for all commands that access works. Test symlink creation, validation, and stale symlink handling. Test `kerf localize` end-to-end including rollback on failure.
