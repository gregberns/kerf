# Data Model

## Bench Structure

```
~/.kerf/                               # bench root (configurable)
  config.yaml                          # global configuration
  jigs/                               # user-level jig definitions
    feature.md                        # default: feature spec jig
    bug.md                            # default: bug investigation jig
    custom-jig.md                     # user-defined jigs
  procedures/                         # finalization and lifecycle procedures
    finalize-default.md               # default finalization procedure
  projects/
    {repo-identifier}/                # e.g., "github-myapp" or path-based
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

# Default project (if not inferred from cwd)
default_project: null

# How repo identifiers are derived
# Options: "path-based" (~/github/myapp -> github-myapp), "git-remote" (origin URL based)
repo_id_strategy: path-based

# Auto-snapshot settings
snapshots:
  enabled: true
  # Snapshot on every file write, or on interval
  strategy: on-write  # or "interval"
  interval_seconds: 300  # only used if strategy is "interval"
  max_snapshots: 100  # per work, oldest pruned

# Finalization defaults
finalize:
  procedure: finalize-default
  # Where in the target repo works get placed
  repo_spec_path: ".specs/"
  # Whether to generate beads/tasks
  generate_tasks: true
  # Branch naming pattern
  branch_pattern: "spec/{codename}"

# Future: sync configuration
# sync:
#   remote: https://kerf.example.com
#   auto_sync: false
#   sync_on_shelve: false
```

## `spec.yaml` (per-work index)

```yaml
# Source of truth for a work's metadata

codename: auth-rewrite
type: feature
jig: feature
status: research
created: 2026-04-07T10:00:00Z
updated: 2026-04-08T14:30:00Z

# Link to the target codebase
project:
  path: /Users/dev/github/myapp
  repo_id: github-myapp

# Session tracking
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
active_session: 5829f3a1-357e-4ee7-92b6-fff4a0e93251

# Dependencies on other works
depends_on:
  - codename: database-migration
    project: github-myapp  # same project, or could reference another
    relationship: must-complete-first
  - codename: auth-service-spec
    relationship: inform

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
  - implementing
  - done
```

## Jig File Format

Jigs are markdown files with YAML frontmatter:

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
---

# Feature Specification Jig

## Overview
This jig guides you through a structured process for specifying a new feature or subsystem.

## Passes

### Pass 1: Problem Space (rough cut)
**Status:** `problem-space`  
**Goal:** Clarify goals, scope, and constraints through 2-3 conversational exchanges with the user.  
**Output:** `01-problem-space.md`

**Instructions:**
[Detailed agent instructions for this pass — what questions to ask, what to capture, how to structure the output file]

### Pass 2: Decomposition
**Status:** `decomposition`  
**Goal:** Break the project into 3-7 components and define concrete, testable requirements for each.  
**Output:** `02-components.md`

**Instructions:**
[Detailed agent instructions]

### Pass 3: Research
**Status:** `research`  
**Goal:** For each component, identify 3-5 research questions and explore existing patterns, external APIs, and technical constraints.  
**Output:** `03-research/{component}/findings.md`

**Instructions:**
[Detailed agent instructions]

### Pass 4: Detailed Spec (fine cut)
**Status:** `detailed-spec`  
**Goal:** Write implementation-level specifications informed by research.  
**Output:** `04-plans/{component}-spec.md`

**Instructions:**
[Detailed agent instructions]

### Pass 5: Integration & Review
**Status:** `review`  
**Goal:** Assemble the full spec, create implementation checklist, identify follow-ups.  
**Output:** `05-integration.md`, `06-checklist.md`, `SPEC.md` (assembled)

**Instructions:**
[Detailed agent instructions]

### Finalization
**Status:** `ready`  
When this work moves to `ready`, run `kerf square <codename>` to verify, then `kerf finalize <codename>` to package it for implementation.

## File Structure
```
{codename}/
  spec.yaml
  SESSION.md
  01-problem-space.md
  02-components.md
  03-research/
    {component}/
      findings.md
  04-plans/
    {component}-spec.md
  05-integration.md
  06-checklist.md
  SPEC.md
```
```

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
  2026-04-07T14:30:00/
    spec.yaml
    01-problem-space.md
    02-components.md
  2026-04-08T09:15:00/
    spec.yaml
    01-problem-space.md
    02-components.md
    03-research/
      auth-flow/
        findings.md
```

Each snapshot is a full copy of the work directory (excluding `.history/` itself). This is simple, wasteful of disk space, but trivially correct and easy to diff. Could optimize later with deduplication if storage becomes a concern.

## Repo Identifier Strategy

The repo identifier links works to their target codebase. Two strategies:

### Path-based (default)
Derive from the filesystem path: `/Users/dev/github/myapp` -> `github-myapp`
- Simple, predictable
- Breaks if the repo moves

### Git-remote-based
Derive from the git remote URL: `git@github.com:user/myapp.git` -> `github.com-user-myapp`
- Survives repo moves
- Requires git remote to be configured
- May not work for repos without remotes

v1 uses path-based. Can add git-remote-based later.
