# Works

> The core unit of kerf: a collection of structured documents describing a unit of specification work, living in its own directory on the [bench](architecture.md).

## What Is a Work

A work is a self-contained directory on the bench containing:

- An **index file** (`spec.yaml`) — the source of truth for all work metadata
- **Artifact files** — documents produced by the agent during the jig's passes (specs, plans, verification reports, etc.)
- A **session file** (`SESSION.md`) — agent-written state for resumability (see [sessions](sessions.md))
- A **history directory** (`.history/`) — auto-versioned snapshots (see [snapshots](snapshots.md))

A work progresses through passes defined by its [jig](jig-system.md). At any point it can be shelved (paused) and resumed. Some jigs include **process passes** where the primary output is an action (e.g., dispatching tasks to agents) rather than a document — see [jig-system.md](jig-system.md) §Process Passes vs. Artifact Passes.

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

- `plan` — planned change to an existing codebase (also accepts `feature` as an alias; see [jig-plan.md](jig-plan.md))
- `spec` — spec-first change where the specification is the source of truth (see [jig-spec.md](jig-spec.md))
- `bug` — bug investigation and fix specification (see [jig-bug.md](jig-bug.md))

Additional types may be defined by custom [jigs](jig-system.md). The type string has no inherent behavior in kerf; it exists for categorization and for selecting the appropriate jig.

## Status

A work's **status** is a string indicating where it is in its lifecycle.

### Open String

Status is **not a fixed enum**. The system accepts any string value. Each [jig](jig-system.md) defines a list of recommended status values corresponding to its passes. The CLI emits the jig's recommended values in its output so agents follow conventions.

### Recommended Values

The jig's `status_values` list defines the progression for that workflow. For example:

Plan jig:
```
problem-space -> analyze -> decompose -> research -> change-spec -> integration -> tasks -> ready
```

Spec jig:
```
problem-space -> decompose -> research -> change-design -> spec-draft -> integration -> tasks -> ready
```

Bug jig:
```
reported -> research -> reproducing -> root-cause -> fix-spec -> ready
```

Implementation jig:
```
breakdown -> dispatch -> implementing -> verify -> complete
```

Retrofit jig:
```
capture -> rationale -> spec-sync -> squared
```

Spike jig:
```
frame -> explore -> converge -> align -> squared
```

Spec-writing jigs (plan, spec, bug) manage works through `ready`; what happens after [finalization](finalization.md) is the responsibility of other tools. Process jigs (implementation, retrofit, spike) manage the full lifecycle through their own terminal status (`complete`, `squared`).

### Unrecognized Values

When a status is set to a value not in the jig's recommended list, the CLI **warns but does not error**. This catches typos (e.g., `reserach`) without blocking custom statuses from orchestrators.

## `spec.yaml` Schema

The `spec.yaml` file is the source of truth for a work's metadata. All fields:

```yaml
# Identity
codename: auth-rewrite                  # string, required, immutable once created
title: "User Authentication Redesign"   # string, optional, mutable
type: plan                              # string, required
project:                                # object, required
  id: acme-webapp                       # string — from .kerf/project-identifier in repo

# Jig
jig: plan                               # string, required — jig name used for this work
jig_version: 1                          # integer, required — recorded from jig at creation time
status: research                        # string, required — current lifecycle status
status_values:                          # list of strings, required — cached from jig
  - problem-space
  - analyze
  - decompose
  - research
  - change-spec
  - integration
  - tasks
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

# Areas — see coordination.md
areas:                                  # list of strings, optional (empty list default)
  - auth                                # references area names from areas.yaml
  - api-gateway

# Dependencies — see dependencies.md for full details
depends_on:                             # list of dependency objects, optional (empty list default)
  - codename: database-migration        # string, required — codename of dependency
    project: acme-webapp                # string, optional — omit for same project
    relationship: must-complete-first   # string, required

# Cross-work relationships — see coordination.md
related_to:                             # list of relationship objects, optional (empty list default)
  - codename: token-refresh             # string, required — codename of related work
    relationship: co-design             # string, required — e.g., co-design, informs, supersedes

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
| `type` | string | yes | — | yes | Work category (e.g., `plan`, `spec`, `bug`, `implementation`, `retrofit`, `spike`). |
| `project.id` | string | yes | — | no | Project identifier from `.kerf/project-identifier`. |
| `jig` | string | yes | — | no | Name of the [jig](jig-system.md) governing this work. |
| `jig_version` | integer | yes | — | no | Jig version recorded at creation time. |
| `status` | string | yes | first value in `status_values` | yes | Current lifecycle status. Open string. |
| `status_values` | list\<string\> | yes | — | no | Recommended statuses, cached from jig at creation. |
| `created` | RFC 3339 timestamp | yes | creation time | no | When the work was created. |
| `updated` | RFC 3339 timestamp | yes | creation time | yes | When metadata was last changed. |
| `sessions` | list\<session\> | no | `[]` | yes | Session history. See [sessions](sessions.md). |
| `active_session` | string \| null | no | `null` | yes | UUID of current session, `"anonymous"` if no ID available, or `null` when inactive. See [sessions](sessions.md). |
| `areas` | list\<string\> | no | `[]` | yes | Area names from `areas.yaml`. See [coordination](coordination.md). |
| `depends_on` | list\<dependency\> | no | `[]` | yes | Work dependencies. See [dependencies](dependencies.md). |
| `related_to` | list\<relationship\> | no | `[]` | yes | Cross-work relationships beyond dependencies (e.g., co-design). See [coordination](coordination.md). |
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

## SDLC Work Patterns

With the full set of jigs (spec-writing and process jigs), a piece of work may flow through the SDLC as multiple linked works. The dependency system (see [dependencies](dependencies.md)) connects them:

### Plan → Implementation

The most common pattern. A plan work reaches `ready`, is finalized, and an implementation work is created with a dependency on it:

```
plan-work (jig: plan, status: ready)
  → impl-work (jig: implementation, depends_on: plan-work)
```

The implementation work reads the plan work's finalized `SPEC.md` and `07-tasks.md` as inputs for the breakdown pass.

### Spike → Plan → Implementation

When exploration is needed before planning:

```
spike-work (jig: spike, status: squared)
  → plan-work (jig: plan, depends_on: spike-work)
    → impl-work (jig: implementation, depends_on: plan-work)
```

The spike work's exploration log informs the plan work's problem space.

### Retrofit (standalone)

Retrofit works typically stand alone — they reconcile an existing divergence:

```
retrofit-work (jig: retrofit, status: squared)
```

A retrofit work may depend on the original plan work it is reconciling against, using the `inform` relationship.

### Key Invariant

Each work has exactly one jig. The "one work, one jig" invariant holds across all patterns. SDLC progression is modeled as multiple linked works, not as one work evolving through multiple jigs.
