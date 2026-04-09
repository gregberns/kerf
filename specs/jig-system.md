# Jig System

> Jig file format, resolution, versioning, customization, and management.

## Definition

A **jig** is a process template defining how an agent walks through a [work](works.md). It declares an ordered sequence of passes, expected output files, recommended status values, and agent instructions for each phase. Jigs are the repeatable guide that makes spec-writing structured and resumable.

kerf ships with built-in jigs:

- [`feature`](jig-feature.md) -- full specification process for new features and subsystems
- [`bug`](jig-bug.md) -- structured investigation and resolution of defects

## File Format

Jigs are markdown files with YAML frontmatter. All machine-readable data (pass definitions, expected files, status values) lives in the frontmatter. Agent instructions live in the markdown body.

### Frontmatter Schema

```yaml
---
name: <string>            # Identifier used in `kerf new --jig <name>`
description: <string>     # One-line summary of this jig's purpose
version: <integer>        # Incremented on breaking changes to the jig
status_values:            # Ordered list of recommended status strings
  - <string>
  - <string>
passes:                   # Ordered list of passes
  - name: <string>        # Human-readable pass name
    status: <string>      # Status value when this pass is active (must appear in status_values)
    output:               # Files produced by this pass
      - <string>          # May include `{component}` placeholders for dynamic paths
  - name: <string>
    status: <string>
    output:
      - <string>
file_structure:           # Complete list of expected files in the work directory
  - <string>              # Includes spec.yaml, SESSION.md, and all pass outputs
---
```

### Markdown Body

The markdown body contains agent instructions organized by pass. A new agent with no prior context reads the jig file and knows exactly what to do at each pass, what questions to ask, what files to produce, and what "done" looks like.

The body structure is:

- A title and overview section
- One section per pass, containing detailed instructions for the agent
- A finalization section describing how to close out the work

### Full Example

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
This jig guides you through a structured process for specifying
a new feature or subsystem.

## Pass 1: Problem Space (rough cut)
**Goal:** Clarify goals, scope, and constraints through 2-3
conversational exchanges with the user.

[Detailed agent instructions for this pass]

## Pass 2: Decomposition
**Goal:** Break the project into 3-7 components and define
concrete, testable requirements for each.

[Detailed agent instructions]

...

## Finalization
When this work moves to `ready`, run `kerf square <codename>`
to verify, then `kerf finalize <codename>` to package it for
implementation.
```

## Resolution Order

When resolving a jig by name, kerf checks in order:

1. **User-level jig** -- `~/.kerf/jigs/{name}.md`
2. **Built-in defaults** -- shipped with the kerf binary

The first match wins. This allows users to override any built-in jig by placing a file with the same name in `~/.kerf/jigs/`.

## Versioning

The jig `version` field is an integer. It is recorded in the work's `spec.yaml` as `jig_version` at creation time.

On any subsequent kerf command that loads a work, kerf compares the resolved jig's current `version` against the recorded `jig_version`. If they differ, kerf emits a warning. It does not block the operation. The agent or user decides whether to continue with the new jig version or investigate the changes.

## Passes

Passes are the ordered phases of a jig. Each pass has:

- A **name** for display
- A **status** string that maps to a value in `status_values`
- An **output** list of files the pass produces

Passes are guidance, not gates. An agent can skip a pass if the user directs it to (e.g., "we already know the root cause, skip to fix spec"). Each pass produces one or more files -- if work is not captured in a file, it is lost when the session ends.

## Status Values

A jig declares an ordered list of `status_values` representing the recommended progression through the work. These values are cached in the work's `spec.yaml` at creation time.

Status is an open string. The CLI emits the jig's status list so agents follow conventions, but accepts any string. When a status is set to a value not in the jig's recommended list, the CLI warns but does not error. This catches typos without blocking custom statuses from orchestrators.

Statuses beyond the jig's final value (e.g., `implementing`, `done`) are orchestrator-defined and not part of the jig. kerf manages specs through the jig's terminal status; what happens after finalization is the responsibility of other tools.

## File Structure

The `file_structure` field lists all expected files in a work directory governed by this jig. This includes `spec.yaml`, `SESSION.md`, and all pass outputs.

[Verification](verification.md) uses `file_structure` to check that expected artifacts exist on disk.

Output paths may contain `{component}` placeholders. These expand to one directory per component as identified during the work.

## Management Commands

kerf provides commands for managing jigs:

- **list** -- show available jigs (both user-level and built-in)
- **show** -- display a jig's full definition
- **save** -- export a jig definition to a file
- **load** -- import a jig from a file or URL
- **sync** -- sync jigs from a remote source (future; see [future.md](future.md))

See [commands.md](commands.md) for full command syntax.

## Customization

Jigs are customizable at two levels:

- **Per-user** -- place jig files in `~/.kerf/jigs/`. These override built-in jigs of the same name and are available across all projects.
- **Per-project** -- a project's `config.yaml` can set `default_jig` to control which jig is used when `kerf new` is run without `--jig`. The jig itself is still resolved via the standard resolution order.

Users create custom jigs by copying and modifying a built-in jig (via the save/load commands) or by writing a new jig file from scratch following the format defined in this spec.

## Design Principles

These principles govern the jig system and the design of individual jigs:

1. **Opinionated but not rigid.** Passes are guidance, not gates. An agent can skip passes when directed.
2. **Each pass produces a file.** This is critical for persistence and resumability.
3. **Requirements before implementation.** Passes that capture what is needed come before passes that capture how to build it.
4. **Concrete over vague.** "Supports up to 10,000 concurrent sessions" not "is scalable."
5. **The jig teaches the agent.** A new agent with no context reads the jig file and knows exactly what to do.
