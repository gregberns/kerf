# Jig System

> Jig file format, resolution, versioning, customization, and management.

## Definition

A **jig** is a process template defining how an agent walks through a [work](works.md). It declares an ordered sequence of passes, expected output files, recommended status values, and agent instructions for each phase. Jigs are the repeatable guide that makes spec-writing structured and resumable.

kerf ships with built-in jigs:

- [`plan`](jig-plan.md) -- write a plan before changing code, for existing projects
- [`spec`](jig-spec.md) -- maintain a living spec that defines your system, for spec-first projects
- [`bug`](jig-bug.md) -- investigate and specify a fix for a defect

## File Format

Jigs are markdown files with YAML frontmatter. All machine-readable data (pass definitions, expected files, status values) lives in the frontmatter. Agent instructions live in the markdown body.

### Frontmatter Schema

```yaml
---
name: <string>            # Identifier used in `kerf new --jig <name>`
description: <string>     # One-line summary of this jig's purpose
version: <integer>        # Incremented on breaking changes to the jig
aliases:                  # Optional. List of alternative names that resolve to this jig.
  - <string>
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
name: plan
description: Write a plan before changing code. For existing projects.
version: 1
aliases: [feature]
status_values:
  - problem-space
  - analyze
  - decompose
  - research
  - change-spec
  - integration
  - tasks
  - ready
passes:
  - name: "Problem Space"
    status: problem-space
    output: ["01-problem-space.md"]
  - name: "Analyze"
    status: analyze
    output: ["02-analysis.md"]
  - name: "Decompose"
    status: decompose
    output: ["03-components.md"]
  - name: "Research"
    status: research
    output: ["04-research/{component}/findings.md"]
  - name: "Change Spec"
    status: change-spec
    output: ["05-specs/{component}-spec.md"]
  - name: "Integration"
    status: integration
    output: ["06-integration.md", "SPEC.md"]
  - name: "Tasks"
    status: tasks
    output: ["07-tasks.md"]
  - name: "Ready"
    status: ready
    output: []
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-problem-space.md
  - 02-analysis.md
  - 03-components.md
  - "04-research/{component}/findings.md"
  - "05-specs/{component}-spec.md"
  - 06-integration.md
  - SPEC.md
  - 07-tasks.md
---

# Plan Jig

## Overview
This jig guides you through a structured process for planning
a change to an existing codebase. You describe what to change,
the jig walks you through analysis, decomposition, research,
and spec writing, and you end with an implementation-ready
task list.

## Pass 1: Problem Space (problem-space)
**Goal:** Clarify goals, scope, and constraints through 2-3
conversational exchanges with the user.

[Detailed agent instructions for this pass]

## Pass 3: Decompose (decompose)
**Goal:** Break the change into 3-7 components and define
concrete, testable requirements for each.

[Detailed agent instructions for this pass]

### Review Criteria

After completing the component breakdown, spawn a review
sub-agent with:
- The 03-components.md file
- The 01-problem-space.md for scope validation

The reviewer checks:
- Every goal from 01-problem-space.md maps to at least one
  component
- Requirements are concrete and testable — "returns 404 with
  error body" not "handles errors"
- Component boundaries are clean (minimal cross-dependencies)
- 3-7 components (flag if outside this range)

Up to 3 review rounds. After that, present artifacts + any
remaining findings to the user for approval.

## Pass 4-7: Research, Change Spec, Integration, Tasks
[Detailed agent instructions for each pass]

## Pass 8: Ready (ready)
Run `kerf square <codename>` to verify all expected artifacts
exist. The work is ready for implementation.

## Finalization
When this work moves to `ready`, run `kerf square <codename>`
to verify, then `kerf finalize <codename>` to package it for
implementation.
```

## Resolution Order

When resolving a jig by name, kerf checks in order:

1. **User-level jig by filename** -- `~/.kerf/jigs/{name}.md`
2. **Built-in jig by filename** -- shipped with the kerf binary, matched by `name` field
3. **Built-in jig by alias** -- scan built-in jigs' `aliases` fields for a match

The first match wins. This allows users to override any built-in jig by placing a file with the same name in `~/.kerf/jigs/`.

Aliases are only checked on built-in jigs. User-level jigs do not support aliases. A user-level jig filename always takes priority over any built-in alias.

**Collision rules:** If two built-in jigs claim the same alias, it is a build-time error.

**Canonical name recording:** When a jig resolves via alias, `spec.yaml` records the canonical name (the jig's `name` field), not the alias. Example: `kerf new --jig feature` resolves to the `plan` jig, so `spec.yaml` gets `jig: plan`. This ensures jig version checks and resolution work correctly on subsequent commands.

## Versioning

The jig `version` field is an integer. It is recorded in the work's `spec.yaml` as `jig_version` at creation time.

On any subsequent kerf command that loads a work, kerf compares the resolved jig's current `version` against the recorded `jig_version`. If they differ, kerf emits a warning. It does not block the operation. The agent or user decides whether to continue with the new jig version or investigate the changes.

## Passes

Passes are the ordered phases of a jig. Each pass has:

- A **name** for display
- A **status** string that maps to a value in `status_values`
- An **output** list of files the pass produces

Passes are guidance, not gates. An agent can skip a pass if the user directs it to (e.g., "we already know the root cause, skip to fix spec"). Each pass produces zero or more files -- if work is not captured in a file, it is lost when the session ends.

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

## Review Pattern

Certain passes in a jig include review instructions in the jig's markdown body. Review is agent-driven — kerf does not orchestrate reviews, spawn sub-agents, or track review state. The jig's markdown body tells the agent how to conduct reviews. kerf tracks only the pass status (a single string).

When following review instructions, the agent:

1. Completes the pass artifacts and saves them to disk.
2. Spawns a review sub-agent with fresh context, providing the pass artifacts, relevant prior-pass artifacts or specs for comparison, and the review criteria from the jig's markdown body.
3. The sub-agent produces findings — specific and actionable, quoting specs and citing line numbers.
4. Findings are saved to `{pass-name}-review.md` in the work directory. This supports resumability: if context is compacted, the review state is on disk.
5. The original agent reads findings, applies fixes, and saves updated artifacts to disk.
6. The sub-agent re-reviews against the updated artifacts.
7. This repeats for up to 3 rounds (configurable per jig in the markdown instructions).
8. After the final round, or if the sub-agent finds no issues, the agent escalates to the human. The human receives the polished artifacts and any unresolved review findings (from `{pass-name}-review.md`). The human can approve (advance to the next pass), request more agent iteration, or intervene directly.

**Autonomous mode** (no human present): If the sub-agent approves (no findings), the agent advances automatically via `kerf status <codename> <next-status>`. If the sub-agent has unresolved findings after the maximum rounds, the agent advances anyway but saves unresolved findings to `{pass-name}-review.md` with an `## Unresolved` section. Autonomous workflows are not blocked. The findings persist on disk for later human review.

**Why not frontmatter?** Review semantics are process guidance, not machine-readable data that kerf acts on. Putting `reviewable: true` in frontmatter implies kerf reads and uses it — it does not. The agent reads the markdown body. Keeping review instructions in the markdown body is consistent with this spec's principle: "All machine-readable data lives in the frontmatter. Agent instructions live in the markdown body."

## Resumability

Every pass MUST save its artifacts to disk before the pass status advances. This is non-negotiable. It ensures:

- Works are resumable across sessions (`kerf resume` re-reads artifacts from disk).
- Context compaction does not lose work (artifacts are on disk, not just in context).
- Sub-agent reviews have files to read (not just context window contents).

If an agent is compacted mid-pass, it re-reads the work directory to restore context and continues from where it left off. The numbered file structure (`01-`, `02-`, ...) shows exactly which passes are complete.

## Design Principles

These principles govern the jig system and the design of individual jigs:

1. **Opinionated but not rigid.** Passes are guidance, not gates. An agent can skip passes when directed.
2. **Each content pass produces a file.** Terminal passes (e.g., `ready`) may produce no files. For content passes, this is critical for persistence and resumability.
3. **Requirements before implementation.** Passes that capture what is needed come before passes that capture how to build it.
4. **Concrete over vague.** "Supports up to 10,000 concurrent sessions" not "is scalable."
5. **The jig teaches the agent.** A new agent with no context reads the jig file and knows exactly what to do.
