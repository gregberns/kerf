# Snapshots

> Versioning via `.history/` — automatic and explicit snapshots of work state.

## Overview

A **snapshot** is a timestamped, full copy of a [work](works.md) directory's contents at a point in time. Snapshots live in the `.history/` directory within each work. They provide undo capability and an audit trail of how a work evolved.

kerf takes snapshots on command invocation, not via filesystem watchers or background daemons. Snapshots are only as fresh as the last kerf interaction.

## The `.history/` Directory

Each work directory contains a `.history/` directory:

```
{codename}/
  spec.yaml
  SESSION.md
  .history/
    2026-04-07T14:30:00/
      spec.yaml
      01-problem-space.md
    2026-04-08T09:15:00/
      spec.yaml
      01-problem-space.md
      02-components.md
      03-research/
        auth-flow/
          findings.md
    2026-04-08T16:00:00--before-research/
      spec.yaml
      01-problem-space.md
      02-components.md
  [jig-defined artifact files]
```

The `.history/` directory is created automatically when the first snapshot is taken (during `kerf new`).

## Snapshot Structure

Each snapshot is a subdirectory of `.history/` named with an RFC 3339 timestamp.

### Directory Naming

- **Automatic snapshots**: `{RFC 3339 timestamp}/` — e.g., `2026-04-08T09:15:00/`
- **Named snapshots**: `{RFC 3339 timestamp}--{label}/` — e.g., `2026-04-08T16:00:00--before-research/`

The label in a named snapshot is a lowercase slug (alphanumeric and hyphens). The double-hyphen `--` separates the timestamp from the label.

### Contents

Each snapshot contains a **full copy** of the work directory at the time of the snapshot.

**Included:**

- `spec.yaml`
- `SESSION.md` (if present)
- All jig-defined artifact files and subdirectories

**Excluded:**

- `.history/` itself — snapshots do not contain nested snapshots

The snapshot preserves the directory structure of the work. Subdirectories (e.g., `03-research/auth-flow/`) are copied recursively.

## Automatic Snapshot Triggers

When automatic snapshots are enabled (see [architecture.md](architecture.md) for `snapshots.enabled` configuration), kerf takes a snapshot on the following command invocations:

| Trigger | When |
|---------|------|
| `kerf new` | After creating the work and initializing `spec.yaml` |
| `kerf resume` | Before emitting resume context |
| `kerf shelve` | Before marking the session as ended |
| `kerf shelve --force` | After clearing the stale session |
| `kerf finalize` | Before beginning the finalization process |
| `kerf status` (with change) | After updating the status in `spec.yaml` |

`kerf status` without a new status value (read-only) does not trigger a snapshot.

Automatic snapshots are silent. kerf does not emit snapshot confirmation in its output for automatic triggers.

## Explicit Snapshots

`kerf snapshot` takes a snapshot on demand. See [commands.md](commands.md) for command syntax.

Explicit snapshots are always taken regardless of the `snapshots.enabled` configuration. They are the same format and structure as automatic snapshots. When a `--name` label is provided, the snapshot directory uses the named format (`{timestamp}--{label}/`).

## Interval-Based Snapshots

An optional interval strategy provides additional snapshots during long sessions without requiring a daemon.

When enabled (see [architecture.md](architecture.md) for `snapshots.interval_enabled` and `snapshots.interval_seconds` configuration), kerf checks the timestamp of the most recent snapshot in `.history/` on every command invocation. If more than `interval_seconds` have elapsed since that snapshot, kerf takes a new automatic snapshot before executing the command.

The interval check runs on **any** kerf command invocation for the affected work, not only the commands listed in [Automatic Snapshot Triggers](#automatic-snapshot-triggers). This means read-only commands like `kerf show` can trigger an interval snapshot.

If no snapshots exist in `.history/`, the interval check treats the elapsed time as exceeding the threshold and takes a snapshot.

## Snapshot Pruning

The `max_snapshots` setting (see [architecture.md](architecture.md)) limits the number of snapshots retained per work. When a new snapshot would cause the count to exceed `max_snapshots`, kerf deletes the oldest snapshots until the count is within the limit.

Pruning happens immediately after a new snapshot is created. Oldest snapshots (by directory name, which sorts chronologically) are removed first.

Named and unnamed snapshots are treated equally for pruning purposes.

## Restoring from a Snapshot

`kerf restore` replaces the current work state with the contents of a previous snapshot. See [commands.md](commands.md) for command syntax.

### Restore Sequence

1. kerf takes a snapshot of the **current** work state (so the restore is reversible).
2. kerf copies the snapshot's files over the current work directory, replacing existing files.
3. kerf preserves the current `active_session` and `sessions` entries in `spec.yaml` — session data from the snapshot is discarded, and the current session data is written back after the copy.
4. kerf emits confirmation including the pre-restore snapshot path.

### Session Data Preservation

Restoring replaces artifact files and `spec.yaml` field values (status, updated timestamp, etc.) but does **not** roll back session history. The `sessions` list and `active_session` field always reflect the true session history, not the state at the time of the snapshot. After the snapshot's files are copied, kerf overwrites the session-related fields in `spec.yaml` with the values that were current before the restore.

### Active Session Warning

If `active_session` is non-null at restore time, kerf emits a warning:

```
Warning: active session in progress. Restored spec.yaml reflects the
snapshot's status and metadata, but session tracking is preserved from
the current state.
```

This warns the agent that the restored `spec.yaml` status may not match the session's expectations.
