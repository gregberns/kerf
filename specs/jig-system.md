# Jig System

> Jig file format, resolution, versioning, customization, and management.

## Definition

A **jig** is a process template defining how an agent walks through a [work](works.md). It declares an ordered sequence of passes, expected output files, recommended status values, and agent instructions for each phase. Jigs are the repeatable guide that makes work structured and resumable.

Jigs cover the full software development lifecycle — from spec-writing through implementation. Some jigs guide agents through producing specification artifacts (plan, spec, bug). Others guide agents through executing implementation processes (breakdown, dispatch, implement, review). The format and mechanics are the same; only the content and purpose differ.

kerf ships with built-in jigs:

**Spec-writing jigs:**
- [`plan`](jig-plan.md) -- write a plan before changing code, for existing projects
- [`spec`](jig-spec.md) -- maintain a living spec that defines your system, for spec-first projects
- [`bug`](jig-bug.md) -- investigate and specify a fix for a defect

**Process jigs:**
- [`implementation`](jig-implementation.md) -- implement a spec: break down into tasks, dispatch to agents, execute with review gates
- [`retrofit`](jig-retrofit.md) -- reconcile code and specs when code changed without the spec workflow
- [`spike`](jig-spike.md) -- structured exploration when the approach is unknown

## File Format

Jigs are markdown files with YAML frontmatter. All machine-readable data (pass definitions, expected files, status values) lives in the frontmatter. Agent instructions live in the markdown body.

### Frontmatter Schema

```yaml
---
name: <string>            # Identifier used in `kerf new --jig <name>`
description: <string>     # One-line summary of this jig's purpose
version: <integer>        # Incremented on breaking changes to the jig
phase: <string>           # SDLC phase: planning, implementation, bug-fix, exploration
tools:                    # Optional. External tools this jig's passes use.
  - <string>              # Informational — agents know what's needed. kerf does not verify.
aliases:                  # Optional. List of alternative names that resolve to this jig.
  - <string>
composable: <boolean>     # Optional. Default false. If true, project config can select which passes to include.
status_values:            # Ordered list of recommended status strings
  - <string>
  - <string>
passes:                   # Ordered list of passes
  - name: <string>        # Human-readable pass name
    status: <string>      # Status value when this pass is active (must appear in status_values)
    output:               # Files produced by this pass
      - <string>          # May include `{component}` placeholders for dynamic paths
    tools:                # Optional. Tools used by this specific pass (inherits from jig-level if omitted)
      - <string>
  - name: <string>
    status: <string>
    output:
      - <string>
file_structure:           # Complete list of expected files in the work directory
  - <string>              # Includes spec.yaml, SESSION.md, and all pass outputs
---
```

**New fields:**

- **`phase`** — Which SDLC phase this jig applies to. Values: `planning` (plan, spec), `implementation` (implementation), `bug-fix` (bug), `exploration` (spike, retrofit). Used for display grouping in `kerf jig list` and for agent discovery. Not enforced — informational only.
- **`tools`** — External tools the jig depends on (e.g., `br`, `ntm`, `agent-mail`). Declared at the jig level and optionally overridden per-pass. Informational — lets agents and users know what needs to be available. kerf does not verify tool availability.
- **`composable`** — Whether a project can select which of this jig's passes to include. When `false` (default), all passes are used. When `true`, the project config specifies which passes are active. Composable jigs define the full set of available passes; the project selects a subset.

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

### Composable Passes

When a jig has `composable: true`, its passes can be selectively activated per-project. The jig file defines the full set of available passes; the project's configuration specifies which ones to include for that project.

Example: The `implementation` jig defines four passes (breakdown, dispatch, implement, review). A project using single-agent development might activate only breakdown and implement, skipping dispatch and review.

When passes are deactivated:
- They are omitted from `kerf show` output and status progression
- Their output files are not expected by `kerf square`
- The jig's markdown instructions for deactivated passes are not emitted by `kerf setup`
- The remaining passes retain their relative ordering

Non-composable jigs (the default) always use all defined passes. The spec-writing jigs (plan, spec, bug) are not composable — their passes have sequential dependencies that make selective activation impractical.

### Process Passes vs. Artifact Passes

Spec-writing jigs produce **artifact passes** — each pass creates a file (01-problem-space.md, 02-analysis.md, etc.). The numbered files on disk show exactly which passes are complete. `kerf square` checks for file existence.

Process jigs (implementation, retrofit) may include **process passes** — passes where the primary output is an action (dispatch beads to agents, run reviews) rather than a document. Process passes may produce tracking artifacts but their completion is measured differently:

- Artifact passes: complete when the output file exists on disk
- Process passes: complete when the process step has been executed (tracked via pass status in the work)

`kerf show` reports the status of both types. `kerf square` checks both — file existence for artifact passes, status completion for process passes. See [verification.md](verification.md).

## Status Values

A jig declares an ordered list of `status_values` representing the recommended progression through the work. These values are cached in the work's `spec.yaml` at creation time.

Status is an open string. The CLI emits the jig's status list so agents follow conventions, but accepts any string. When a status is set to a value not in the jig's recommended list, the CLI warns but does not error. This catches typos without blocking custom statuses from orchestrators.

Statuses beyond a spec-writing jig's final value (e.g., `implementing`, `done`) are orchestrator-defined and not part of the jig. For spec-writing jigs, kerf manages works through the jig's terminal status; what happens after finalization is the responsibility of other tools. Process jigs (implementation, retrofit) define their own status progressions that cover the full process lifecycle.

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

Jigs are customizable at three levels:

- **Per-user** -- place jig files in `~/.kerf/jigs/`. These override built-in jigs of the same name and are available across all projects.
- **Per-project jig selection** -- a project's configuration declares which jigs are active and, for composable jigs, which passes to include. See [architecture.md](architecture.md) for project configuration details.
- **Per-project defaults** -- a project's `config.yaml` can set `default_jig` to control which jig is used when `kerf new` is run without `--jig`. The jig itself is still resolved via the standard resolution order.

Users create custom jigs by copying and modifying a built-in jig (via the save/load commands) or by writing a new jig file from scratch following the format defined in this spec.

## Review Pattern

Certain passes in a jig include review instructions in the jig's markdown body. Review is agent-driven — kerf does not orchestrate reviews, spawn sub-agents, or track review state. The jig's markdown body tells the agent how to conduct reviews. kerf tracks only the pass status (a single string).

A pass that ships review criteria is reviewed before its status advances. The review step is part of the pass — the artifact is not considered complete until a reviewer has approved (or the work has been escalated with unresolved findings). The jig spec describes this pattern once; per-pass markdown sections list only the criteria and reference this section for the protocol.

### Reviewer primitives

Review requires a reviewer that reads the artifact with fresh eyes. Three primitives can satisfy that contract; the agent uses whichever the harness has, in this preference order:

1. **Harness sub-agent.** A sub-agent is dispatched with fresh context, the artifact paths, the prior-pass artifacts named in the criteria, and the criteria themselves. This is the default when the harness exposes an Agent-style tool.
2. **Parent-orchestrator review.** The orchestrator that dispatched the pass-execution agent reads the artifact and applies the criteria itself. Used when the harness does not expose sub-agent dispatch but a separate orchestrator role exists.
3. **Fresh-context re-read.** The same agent compacts its own context and re-reads the artifact alongside the criteria. Used when no other reviewer is available; weaker than the first two and labelled as such in findings.

The agent records which primitive it used at the top of the findings file so the human review trail is legible.

### Review loop

When following review instructions, the agent:

1. Completes the pass artifacts and saves them to disk.
2. Selects a reviewer primitive from the list above and hands it the pass artifacts, relevant prior-pass artifacts or specs for comparison, and the review criteria from the jig's markdown body.
3. The reviewer produces findings — specific and actionable, quoting specs and citing line numbers. Findings are returned to the agent as text; the agent owns the write.
4. The agent saves findings to `{pass-name}-review.md` in the work directory, prefixed with the primitive used. This supports resumability: if context is compacted, the review state is on disk.
5. The agent reads findings, applies fixes, and saves updated artifacts to disk.
6. The reviewer re-reviews against the updated artifacts.
7. This repeats for up to 3 rounds (configurable per jig in the markdown instructions).
8. After the final round, or if the reviewer finds no issues, the agent escalates to the human. The human receives the polished artifacts and any unresolved review findings (from `{pass-name}-review.md`). The human can approve (advance to the next pass), request more agent iteration, or intervene directly.

**Autonomous mode** (no human present): If the reviewer approves (no findings), the agent advances automatically via `kerf status <codename> <next-status>`. If the reviewer has unresolved findings after the maximum rounds, the agent advances anyway but saves unresolved findings to `{pass-name}-review.md` with an `## Unresolved` section. Autonomous workflows are not blocked. The findings persist on disk for later human review.

### Sub-agent file writes

Sub-agents (reviewers, researchers, any delegated worker) return their output as text to the parent agent. The parent owns the write step: it persists the returned text to the canonical file path under the work directory. This keeps the pattern portable across harnesses that restrict sub-agent file writes, and it keeps a single accountable writer per artifact. Per-pass instructions that delegate work name the file the parent will write into.

### `kerf review`

`kerf review <codename>` emits the canonical reviewer prompt for the work's current pass — the criteria from the resolved jig's markdown body, the artifact paths, and the prior-pass references named by the criteria. The command is harness-agnostic: it prints to stdout, and the harness pipes the output into whichever reviewer primitive it has from the list above. See [commands.md](commands.md) for the command surface; the per-pass criteria that `kerf review` emits live in this spec and in the per-jig markdown bodies.

**Why not frontmatter?** Review semantics are process guidance, not machine-readable data that kerf acts on. Putting `reviewable: true` in frontmatter implies kerf reads and uses it — it does not. The agent reads the markdown body. Keeping review instructions in the markdown body is consistent with this spec's principle: "All machine-readable data lives in the frontmatter. Agent instructions live in the markdown body."

## Pass-Directory Pre-Creation

When status advances into a pass (via `kerf status <codename> <next-status>` or `kerf new` landing on the jig's first status), kerf reads the resolved jig's pass list and pre-creates the directory prefix of each output path declared for the new pass. The behavior is idempotent — directories that already exist are left alone.

Where a per-pass template file ships alongside the jig, kerf also copies the template into the output location if the output file does not yet exist. Templates are skeletons (headings and `<TODO: ...>` markers), not boilerplate prose; copying them gives the agent a structured starting point instead of an empty file. Existing files are never overwritten.

`{component}` placeholders in output paths are resolved only when the components are already known from a prior pass (typically pass 2). Until then, only the static directory prefix (e.g., `03-research/`) is created; per-component subdirectories are created on demand as components are enumerated.

## Surfacing Pass Filenames

`kerf show` prints one line per pass identifying the canonical output filename. The format is:

```
Pass N: <pass name> → Output: NN-<filename>.md
```

For passes with multiple outputs or `{component}`-templated paths, the line lists the template form (e.g., `03-research/{component}/findings.md`). New jigs follow the `NN-<short-name>.md` convention for content passes so this line renders consistently. See [commands.md](commands.md) for the full `kerf show` surface.

## Resumability

Every pass MUST save its artifacts to disk before the pass status advances. This is non-negotiable. It ensures:

- Works are resumable across sessions (`kerf resume` re-reads artifacts from disk).
- Context compaction does not lose work (artifacts are on disk, not just in context).
- Sub-agent reviews have files to read (not just context window contents).

If an agent is compacted mid-pass, it re-reads the work directory to restore context and continues from where it left off. The numbered file structure (`01-`, `02-`, ...) shows exactly which passes are complete.

## Validation-test requirement

Every pass that produces a **normative planning artifact** MUST list, in its "What done looks like" checklist, two tracked test-item IDs — one **scenario-level** and one **exploratory** — and the artifact's downstream beads MUST be gated on those items closing.

A normative planning artifact is one that downstream implementation reads as authoritative: a plan's Change Spec or Tasks output, a spec jig's Spec Draft or Tasks output, a bug jig's Fix Spec, and an implementation jig's Breakdown and Verify outputs. See the per-jig specs ([jig-plan.md](jig-plan.md), [jig-spec.md](jig-spec.md), [jig-bug.md](jig-bug.md), [jig-implementation.md](jig-implementation.md)) for the exact passes affected.

The two checklist items take this shape inside the existing "What done looks like" block:

- `Scenario-test item filed with ID <id>` — one end-to-end test against a runnable substrate (twin substrate, real binary, or equivalent).
- `Exploratory-test item filed with ID <id>` — one operator-facing exercise of the live surface.

Recommended title conventions (not enforced): `scenario: <codename> — <brief>` and `explore: <codename> — <brief>`.

The mechanism is **tracker-agnostic**. `<id>` may be any stable identifier the project's tracker emits. As one example, a project using `br` would file the items with `br create` and paste the returned issue IDs into the checklist. Other trackers work the same way.

For the implementation jig, the requirement has two touch points: the **creation point** (Pass 1, Breakdown) where the IDs are filed and recorded, and the **closure-check point** (Pass 4, Verify) where the items are confirmed closed before the work advances.

### What this does not guarantee

This requirement does not structurally guarantee the filed scenario test exercises an integration surface. A planning agent can satisfy the "scenario-test item filed" checkbox by writing a unit-against-fake test — the harmonik incident `hk-37zy8` (handler-pause goroutine, unit-tested and reviewer-approved, never wired into the composition root) is the cautionary case. The mechanism's power is "force the agent to name the integration-surface gap at planning time, before code is written," not "structurally guarantee a working integration test exists."

The requirement also does not detect missing exploratory coverage on a live binary; it only detects that two IDs were filed in the artifact. Any downstream detector (see [commands.md](commands.md) § `kerf doctor`) is read-only against the artifact file and does not query the tracker.

### Retrofit and spike exclusion

The `retrofit` and `spike` jigs are excluded from this requirement. Rationale: retrofit reconciles existing code against drifted specs (the integration surface already exists and is being documented after the fact), and spike is structured exploration where the approach itself is unknown (filing scenario/exploratory tests against an undefined surface is premature). Both jigs may still file test items when appropriate; they are not gated on doing so.

## Design Principles

These principles govern the jig system and the design of individual jigs:

1. **Opinionated but not rigid.** Passes are guidance, not gates. An agent can skip passes when directed.
2. **Each content pass produces a file.** Terminal passes (e.g., `ready`) may produce no files. Process passes may produce tracking artifacts rather than primary documents. For content passes, file output is critical for persistence and resumability.
3. **Requirements before implementation.** Passes that capture what is needed come before passes that capture how to build it.
4. **Concrete over vague.** "Supports up to 10,000 concurrent sessions" not "is scalable."
5. **The jig teaches the agent.** A new agent with no context reads the jig file and knows exactly what to do. This applies equally to spec-writing jigs and process jigs — the jig contains the full instructions for how to use the tools and follow the process.
6. **Recovery over prevention.** Process jigs make work state durable and visible rather than trying to gate agents from deviating. Beads persist in Git. Pass status is tracked. If an agent crashes or goes off-script, the next session finds its place and picks up. Visibility without enforcement.
7. **Portable process.** Jig instructions are the single source of truth for how a process works. They live in kerf's jig library, not in per-project CLAUDE.md files or skills. Update a jig once, every project using it gets the improvement.
8. **kerf defines, tools execute.** kerf stores process knowledge and emits it. Orchestration tools (ntm, Kilroy) execute the process. kerf never launches or manages agent sessions.
