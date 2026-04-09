# Works

> The core unit of kerf: a collection of structured documents describing a unit of specification work, living in its own directory on the [bench](architecture.md).

## What Is a Work

A work is a self-contained directory on the bench containing:

- An **index file** (`spec.yaml`) — the source of truth for all work metadata
- **Artifact files** — specification documents produced by the agent during the jig's passes
- A **session file** (`SESSION.md`) — agent-written state for resumability (see [sessions](sessions.md))
- A **history directory** (`.history/`) — auto-versioned snapshots (see [snapshots](snapshots.md))

A work progresses through passes defined by its [jig](jig-system.md). At any point it can be shelved (paused) and resumed.

## Work Directory Structure

```
{codename}/
  spec.yaml                          # index file — source of truth for metadata
  SESSION.md                         # agent-written resumability state
  .history/                          # auto-versioned snapshots
    2026-04-07T14:30:00/
    2026-04-08T09:15:00/
  [jig-defined artifact files]       # e.g., 01-problem-space.md, 02-components.md, ...
```

Each work directory lives at `~/.kerf/projects/{project-id}/{codename}/` on the bench. See [architecture](architecture.md) for the full bench layout.

The jig determines which artifact files exist within the work directory. The files above (`spec.yaml`, `SESSION.md`, `.history/`) are present in every work regardless of jig.

## Codename

A work's **codename** is its primary identifier.

### Format

Codenames must be valid directory names: **lowercase alphanumeric characters and hyphens only** (matching the pattern `[a-z0-9]+(-[a-z0-9]+)*`).

### Generation

When the user does not provide a codename at creation time, kerf auto-generates one using an `adjective-noun` pattern (e.g., `blue-bear`, `swift-maple`). User-chosen codenames are also accepted (e.g., `auth-rewrite`), provided they meet the format requirements.

### Immutability

A codename is **immutable once created**. It is used as:

- The work's directory name on the bench
- The identifier in [dependency](dependencies.md) references
- The identifier in [session](sessions.md) associations
- The argument to CLI [commands](commands.md)

Codenames cannot be renamed. If a different codename is needed, the work must be recreated.

## Title

A work's **title** is an optional, human-friendly description (e.g., "User Authentication Redesign"). Unlike codenames, titles are **mutable** and can be changed at any time. Titles have no uniqueness constraint and are not used as identifiers.

## Type

A work's **type** indicates what kind of work it is. Types are strings. The built-in types are:

- `feature` — new feature or subsystem
- `bug` — bug investigation and fix specification

Additional types may be defined by custom [jigs](jig-system.md). The type string has no inherent behavior in kerf; it exists for categorization and for selecting the appropriate jig.

## Status

A work's **status** is a string indicating where it is in its lifecycle.

### Open String

Status is **not a fixed enum**. The system accepts any string value. Each [jig](jig-system.md) defines a list of recommended status values corresponding to its passes. The CLI emits the jig's recommended values in its output so agents follow conventions.

### Recommended Values

The jig's `status_values` list defines the progression for that workflow. For example:

Feature jig:
```
problem-space -> decomposition -> research -> detailed-spec -> review -> ready
```

Bug jig:
```
triaging -> reproducing -> locating -> specifying-fix -> ready
```

Statuses beyond `ready` (e.g., `implementing`, `done`) are orchestrator-defined. kerf manages specifications through `ready`; what happens after [finalization](finalization.md) is the responsibility of other tools.

### Unrecognized Values

When a status is set to a value not in the jig's recommended list, the CLI **warns but does not error**. This catches typos (e.g., `reserach`) without blocking custom statuses from orchestrators.

## `spec.yaml` Schema

The `spec.yaml` file is the source of truth for a work's metadata. All fields:

```yaml
# Identity
codename: auth-rewrite                  # string, required, immutable once created
title: "User Authentication Redesign"   # string, optional, mutable
type: feature                           # string, required
project:                                # object, required
  id: acme-webapp                       # string — from .kerf/project-identifier in repo

# Jig
jig: feature                            # string, required — jig name used for this work
jig_version: 1                          # integer, required — recorded from jig at creation time
status: research                        # string, required — current lifecycle status
status_values:                          # list of strings, required — cached from jig
  - problem-space
  - decomposition
  - research
  - detailed-spec
  - review
  - ready

# Timestamps
created: 2026-04-07T10:00:00Z          # RFC 3339, required, set at creation
updated: 2026-04-08T14:30:00Z          # RFC 3339, required, updated on any metadata change

# Sessions — see sessions.md for full details
sessions:                               # list of session objects, optional (empty list default)
  - id: 39142ac7-b54e-4726-bbb0-a6d41dfe9fba   # string or null — session UUID, best-effort
    started: 2026-04-07T10:00:00Z               # RFC 3339, required
    ended: 2026-04-07T16:30:00Z                 # RFC 3339 or null — null if active
    notes: "Completed problem space"             # string, optional

active_session: 5829f3a1-357e-4ee7-92b6-fff4a0e93251  # string or null — UUID, "anonymous", or null

# Dependencies — see dependencies.md for full details
depends_on:                             # list of dependency objects, optional (empty list default)
  - codename: database-migration        # string, required — codename of dependency
    project: acme-webapp                # string, optional — omit for same project
    relationship: must-complete-first   # string, required

# Implementation linkage — see finalization.md
implementation:                         # object, optional
  branch: null                          # string or null — set by kerf finalize
  pr: null                              # string or null — populated manually after PR creation
  commits: []                           # list of strings — set by kerf finalize
```

### Field Reference

| Field | Type | Required | Default | Mutable | Description |
|-------|------|----------|---------|---------|-------------|
| `codename` | string | yes | auto-generated | **no** | Primary identifier. Lowercase alphanumeric and hyphens. |
| `title` | string | no | `null` | yes | Human-friendly description. |
| `type` | string | yes | — | yes | Work category (e.g., `feature`, `bug`). |
| `project.id` | string | yes | — | no | Project identifier from `.kerf/project-identifier`. |
| `jig` | string | yes | — | no | Name of the [jig](jig-system.md) governing this work. |
| `jig_version` | integer | yes | — | no | Jig version recorded at creation time. |
| `status` | string | yes | first value in `status_values` | yes | Current lifecycle status. Open string. |
| `status_values` | list\<string\> | yes | — | no | Recommended statuses, cached from jig at creation. |
| `created` | RFC 3339 timestamp | yes | creation time | no | When the work was created. |
| `updated` | RFC 3339 timestamp | yes | creation time | yes | When metadata was last changed. |
| `sessions` | list\<session\> | no | `[]` | yes | Session history. See [sessions](sessions.md). |
| `active_session` | string \| null | no | `null` | yes | UUID of current session, `"anonymous"` if no ID available, or `null` when inactive. See [sessions](sessions.md). |
| `depends_on` | list\<dependency\> | no | `[]` | yes | Work dependencies. See [dependencies](dependencies.md). |
| `implementation` | object | no | `{branch: null, pr: null, commits: []}` | yes | Populated at [finalization](finalization.md). |

### Immutability Rules

The following fields are set at creation time and never change:

- `codename`
- `project.id`
- `jig`
- `jig_version`
- `status_values`
- `created`

All other fields may be updated during the work's lifecycle.

### Timestamps

All timestamps are RFC 3339 format in UTC (e.g., `2026-04-07T10:00:00Z`). The `updated` field is set whenever any metadata in `spec.yaml` changes.
