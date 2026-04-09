# Data Model

## Bench Structure

```
~/.kerf/                               # bench root (configurable)
  config.yaml                          # global configuration
  jigs/                               # user-level jig definitions
    feature.md                        # default: feature spec jig
    bug.md                            # default: bug investigation jig
    custom-jig.md                     # user-defined jigs
  archive/                            # archived works (hidden from `kerf list`)
    {project-id}/
      {codename}/
  projects/
    {project-id}/                     # e.g., "acme-webapp" (from .kerf/project-identifier)
      {codename}/                     # one directory per work
        spec.yaml                    # index file — source of truth
        SESSION.md                   # agent-written resumability state
        .history/                    # auto-versioned snapshots
          2026-04-07T14:30:00/       # timestamped snapshot directories
          2026-04-08T09:15:00/
        [jig-defined files]          # e.g., 01-problem-space.md, etc.
```

## `config.yaml` (bench-level)

```yaml
# ~/.kerf/config.yaml

# Default jig for new works
default_jig: feature

# Default project for commands run outside a repo (optional)
# Project ID is normally inferred from .kerf/project-identifier in the cwd's repo
# default_project: acme-webapp

# Snapshot settings
# Snapshots are taken automatically on kerf command invocations
# (new, resume, shelve, finalize, status changes) and optionally on interval.
snapshots:
  enabled: true
  # Optional: check elapsed time on each command invocation and snapshot if
  # interval has passed since the last snapshot (no background daemon needed)
  interval_enabled: false
  interval_seconds: 300
  max_snapshots: 100  # per work, oldest pruned

# Finalization defaults
finalize:
  # Where in the target repo works get placed
  repo_spec_path: ".kerf/{codename}/"
  # Branch naming: agent chooses based on work context (no fixed pattern)

# Future: sync configuration
# sync:
#   remote: https://kerf.example.com
#   auto_sync: false
#   sync_on_shelve: false
```

## `spec.yaml` (per-work index)

```yaml
# Source of truth for a work's metadata

codename: auth-rewrite        # immutable once created
title: "User Authentication Redesign"  # optional, human-friendly, changeable
type: feature
jig: feature
jig_version: 1                # recorded from jig at creation time
status: research
created: 2026-04-07T10:00:00Z
updated: 2026-04-08T14:30:00Z

# Project identity (from .kerf/project-identifier in repo)
project:
  id: acme-webapp

# Session tracking
# Session IDs are recorded when available (e.g., when human launches Claude
# with --session-id). When not available, sessions are recorded without an ID.
sessions:
  - id: 39142ac7-b54e-4726-bbb0-a6d41dfe9fba
    started: 2026-04-07T10:00:00Z
    ended: 2026-04-07T16:30:00Z
    notes: "Completed problem space and decomposition"
  - id: 5829f3a1-357e-4ee7-92b6-fff4a0e93251
    started: 2026-04-08T09:00:00Z
    ended: null  # active session
    notes: "Research pass, 3 of 5 components done"

# Current session (quick lookup)
# Stale detection: if active_session has no `ended` timestamp and the
# `started` timestamp is older than a configurable threshold (default 24h),
# kerf warns that the session may be stale. `kerf shelve --force` clears it.
active_session: 5829f3a1-357e-4ee7-92b6-fff4a0e93251

# Dependencies on other works
depends_on:
  - codename: database-migration
    project: acme-webapp    # explicit project; omit for same project
    relationship: must-complete-first
  - codename: auth-service-spec
    relationship: inform    # doesn't block, but should be read for context

# Implementation linkage (populated at finalize)
implementation:
  branch: null
  pr: null
  commits: []

# External references (future)
# external:
#   jira: PROJ-1234
#   linear: LIN-567

# Jig-defined status progression (cached from jig for quick reference)
status_values:
  - problem-space
  - decomposition
  - research
  - detailed-spec
  - review
  - ready
```

## Jig File Format

Jigs are markdown files with YAML frontmatter. All machine-readable data (pass definitions, expected files, statuses) lives in the frontmatter so kerf can parse it reliably. Agent instructions live in the markdown body.

```markdown
---
name: feature
description: Full specification process for new features and subsystems
version: 1
status_values:
  - problem-space
  - decomposition
  - research
  - detailed-spec
  - review
  - ready
passes:
  - name: "Problem Space"
    status: problem-space
    output: ["01-problem-space.md"]
  - name: "Decomposition"
    status: decomposition
    output: ["02-components.md"]
  - name: "Research"
    status: research
    output: ["03-research/{component}/findings.md"]
  - name: "Detailed Spec"
    status: detailed-spec
    output: ["04-plans/{component}-spec.md"]
  - name: "Integration & Review"
    status: review
    output: ["05-integration.md", "06-checklist.md", "SPEC.md"]
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-problem-space.md
  - 02-components.md
  - "03-research/{component}/findings.md"
  - "04-plans/{component}-spec.md"
  - 05-integration.md
  - 06-checklist.md
  - SPEC.md
---

# Feature Specification Jig

## Overview
This jig guides you through a structured process for specifying a new feature or subsystem.

## Pass 1: Problem Space (rough cut)
**Goal:** Clarify goals, scope, and constraints through 2-3 conversational exchanges with the user.

[Detailed agent instructions for this pass — what questions to ask, what to capture, how to structure the output file]

## Pass 2: Decomposition
**Goal:** Break the project into 3-7 components and define concrete, testable requirements for each.

[Detailed agent instructions]

## Pass 3: Research
**Goal:** For each component, identify 3-5 research questions and explore existing patterns, external APIs, and technical constraints.

[Detailed agent instructions]

## Pass 4: Detailed Spec (fine cut)
**Goal:** Write implementation-level specifications informed by research.

[Detailed agent instructions]

## Pass 5: Integration & Review
**Goal:** Assemble the full spec, create implementation checklist, identify follow-ups.

[Detailed agent instructions]

## Finalization
When this work moves to `ready`, run `kerf square <codename>` to verify, then `kerf finalize <codename>` to package it for implementation.
```

### Jig Resolution Order
When resolving which jig to use, kerf checks in order:
1. User-level jig (`~/.kerf/jigs/{name}.md`)
2. Built-in defaults (shipped with kerf)

The jig version is recorded in `spec.yaml` at creation time (`jig_version`). If the resolved jig's version differs from the recorded version on a subsequent command, kerf warns that the jig has changed since the work was created. This lets the agent or user decide whether to continue with the new version or investigate the changes.

## SESSION.md Format

Written by the agent when shelving or at the end of a session:

```markdown
# Session State

## Current Pass
Research — component 3 of 5 (notification-service)

## Decisions Made
- Using event-driven architecture for auth events (see 04-plans/auth-events-spec.md)
- Rejected OAuth proxy approach due to latency concerns (see 03-research/auth-flow/findings.md)

## Open Questions
- Need to determine if existing session store can handle new token format
- Waiting on database-migration work for schema decisions

## Next Steps
- Complete research for notification-service and user-preferences components
- Begin detailed spec for auth-flow component (research complete)

## Context for New Sessions
If starting a fresh session (not resuming), read these files first:
1. spec.yaml — current status and metadata
2. This file — SESSION.md
3. 02-components.md — the decomposition
4. 03-research/ — completed research
```

## Snapshot Structure

```
.history/
  2026-04-07T14:30:00/            # auto-snapshot (from kerf new)
    spec.yaml
    01-problem-space.md
    02-components.md
  2026-04-08T09:15:00/            # auto-snapshot (from kerf status)
    spec.yaml
    01-problem-space.md
    02-components.md
    03-research/
      auth-flow/
        findings.md
  2026-04-08T16:00:00--before-research/  # named snapshot (from kerf snapshot --name)
    ...
```

Each snapshot is a full copy of the work directory (excluding `.history/` itself). This is simple, wasteful of disk space, but trivially correct and easy to diff. Could optimize later with deduplication if storage becomes a concern.

### When snapshots are taken
Snapshots happen automatically on kerf command invocations that change or read significant state: `new`, `resume`, `shelve`, `finalize`, `status` (on change). Explicit snapshots via `kerf snapshot` are always available. An optional interval-based strategy can be enabled for long sessions.

Kerf does NOT detect agent file writes in real-time. The agent writes directly to the work directory; kerf snapshots the state when it next runs. This is an honest trade-off: no background daemons, no filesystem watchers, no ceremony — but snapshots are only as fresh as the last kerf interaction.

## Project Identity

Each project (git repo) is identified by a **project ID** — a stable slug stored in `.kerf/project-identifier` in the repo root (committed to git).

### Derivation (on first `kerf` use in a repo)
1. Parse git remote origin → extract `user/repo` → slugify to `user-repo` (e.g., `acme-webapp`)
2. No remote? Fall back to directory name
3. Write the result to `.kerf/project-identifier`
4. If the ID already exists in the bench for a different repo, warn the user

### Properties
- **Stable across moves/renames** — the ID is in the repo, not derived from the path
- **Worktree-friendly** — `.kerf/project-identifier` is committed, so all checkouts see the same ID
- **User-overridable** — the user can change the project ID at any time
- **Cross-project lookup** — `--project <project-id>` on commands like `kerf show`, `kerf list`
