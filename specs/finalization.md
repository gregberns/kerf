# Finalization

> The process of moving a completed work from the bench into the git repository.

Finalization is the only mechanism by which data crosses from the bench (`~/.kerf/`) into a git repository. It is a one-way, one-time operation per work: kerf performs a defined sequence of mechanical steps, then emits instructions for agent-driven follow-up.

For the `kerf finalize` command syntax, flags, and error messages, see [commands.md](commands.md). For the `implementation` fields in spec.yaml, see [works.md](works.md). For square verification details, see [verification.md](verification.md).

## Pre-Finalization Validation

Before performing any git operations, kerf validates that the work and the target repository are in a finalizable state.

### Square Check

kerf runs the same checks as `kerf square` (see [verification.md](verification.md)) on the work. If any check fails, finalization aborts with a report of what is not square. The agent or user must resolve the issues and retry.

### Uncommitted Changes Check

kerf checks the target repository's working tree and index for uncommitted changes. If any exist — staged or unstaged — finalization refuses to proceed.

```
Error: target repository has uncommitted changes. Commit or stash them before finalizing.
```

This prevents finalization from interleaving with in-progress work in the repository.

### Branch Existence Check

kerf verifies that the requested branch name does not already exist in the target repository. If it does, finalization aborts.

```
Error: branch '{branch-name}' already exists in the target repository.
```

## Git Operations

After validation passes, kerf performs the following steps in order. If any step fails, finalization aborts and reports the failure. No partial state is committed to the repository.

### 1. Snapshot

kerf takes a [snapshot](snapshots.md) of the current work state before making any changes.

### 2. Branch Creation

kerf creates a new git branch in the target repository from the repository's default branch (`main`, `master`, or whatever the repository uses). The branch name is specified by the `--branch` flag — the agent chooses the name based on the work's context, not the codename.

```
git checkout -b {branch-name}
```

### 3. Artifact Copying

kerf copies work artifacts from the bench into the target repository at the path defined by `finalize.repo_spec_path` in [config.yaml](architecture.md) (default: `.kerf/{codename}/`). The token `{codename}` in the path is replaced with the work's codename.

The copied artifacts include all files in the work directory except:

- `spec.yaml` (metadata stays in the bench)
- `SESSION.md` (session state stays in the bench)
- `.history/` (snapshot history stays in the bench)

For spec-first works, `05-spec-drafts/` is also excluded from this copy (see [Spec-First Finalization](#spec-first-finalization) below).

The destination directory is created if it does not exist.

### 4. Initial Commit

kerf creates a git commit containing the copied artifacts. The commit message follows the format:

```
kerf: finalize {codename}
```

### 5. Record Implementation Linkage

kerf updates `spec.yaml` in the bench:

- Sets `implementation.branch` to the branch name.
- Appends the commit hash to `implementation.commits`.

### 6. Status Update

kerf sets the work's `status` to `finalized` and updates the `updated` timestamp.

## Artifact Destination Path

The `finalize.repo_spec_path` setting in `config.yaml` controls where finalized artifacts land in the repository.

| Setting | Default | Example Result |
|---------|---------|----------------|
| `finalize.repo_spec_path` | `.kerf/{codename}/` | `.kerf/auth-rewrite/` |

The `{codename}` token is replaced with the work's codename. The path is relative to the repository root.

This setting can be overridden in `config.yaml`:

```yaml
finalize:
  repo_spec_path: "specs/{codename}/"
```

## Spec-First Finalization

For works with `jig: spec` in spec.yaml, finalization performs additional steps beyond the standard artifact copy. Detection is by jig name — only works using the built-in `spec` jig trigger this behavior. Custom jigs that produce `05-spec-drafts/` do not get spec-first finalization. This is an intentional v1 limitation; custom jigs that need similar behavior should use finalization hooks (future enhancement).

### Spec Draft Copying

After the standard artifact copy (step 3), kerf copies files from the work's `05-spec-drafts/` directory to `{repo_root}/{spec_path}/`, where `spec_path` is a config value (default: `specs/`). Filenames are preserved 1:1 — `05-spec-drafts/jig-system.md` becomes `{spec_path}/jig-system.md`.

- If `{repo_root}/{spec_path}/` does not exist, kerf creates it.
- If `05-spec-drafts/` is empty or missing, kerf warns but does not error. The standard artifact copy proceeds normally.
- `05-spec-drafts/` is **excluded** from the standard artifact copy to `repo_spec_path` (no duplication — spec files appear only in `spec_path`).

### `spec_path` vs `repo_spec_path`

These are distinct config values used for different purposes during finalization:

- **`repo_spec_path`** (`finalize.repo_spec_path` in config.yaml) — Where kerf copies work process artifacts: problem space, design documents, changelog, integration notes, tasks. These are the record of how the spec change was developed. For spec-first works, `05-spec-drafts/` is excluded from this copy.
- **`spec_path`** (`spec_path` in config.yaml, default: `specs/`) — Where kerf copies drafted spec files. These are the normative spec changes — the actual spec text that the system must conform to. Only used during spec-first finalization.

For spec-first works, the finalization commit includes both: the process record (in `repo_spec_path`) and the normative spec changes (in `spec_path`).

### Spec-First Output

When finalizing a spec-first work, the mechanical summary shows both destinations:

```
Finalizing {codename}...
  Square check: passed
  Branch created: {branch-name}
  Artifacts copied to: {repo-spec-path}
  Spec drafts applied to: {spec-path}
  Commit: {short-hash} — kerf: finalize {codename}
  Status: finalized
```

## Branch Naming

The `--branch` flag is required. kerf does not generate branch names. The agent chooses the branch name based on its understanding of the work — the feature being built, the bug being fixed, the team's branching conventions. The codename is an internal bench identifier and is not required to appear in the branch name.

## Post-Finalization Output

After the mechanical steps complete, kerf emits a summary of what it did followed by agent-driven follow-up instructions.

### Mechanical Summary

```
Finalizing {codename}...
  Square check: passed
  Branch created: {branch-name}
  Artifacts copied to: {repo-spec-path}
  Commit: {short-hash} — kerf: finalize {codename}
  Status: finalized
```

### Follow-Up Instructions

kerf emits suggested next steps for the agent or user to perform. These are instructions, not actions kerf takes:

```
Next steps:
  - Create a pull request for branch '{branch-name}'
  - Update implementation.pr in spec.yaml with the PR URL
  - Notify the team / link external systems
  - Run 'kerf archive {codename}' when implementation is complete
```

kerf does not create pull requests, send notifications, or perform any action beyond the mechanical steps listed above. Follow-up is the responsibility of the agent or user. The `implementation.pr` field in [spec.yaml](works.md) is not set by any kerf command — the agent or user updates it manually after creating the PR.

## Idempotency and Re-Finalization

Finalization is a one-time operation. Once a work's status is `finalized`, running `kerf finalize` on it again fails the square check (status is past the jig's ready state). To re-finalize, the user must manually reset the status and clean up the previously created branch.
