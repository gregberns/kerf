# Spec Jig

> Built-in jig for maintaining a living system specification where specs are the source of truth.

This spec defines the `spec` jig that ships with kerf. It is for teams that maintain a living specification — code that doesn't match the spec is wrong. Changes to the system start as spec updates, then flow to code. See [jig-system.md](jig-system.md) for jig file format, resolution, and versioning. See [jig-plan.md](jig-plan.md) for the plan-first alternative and [jig-bug.md](jig-bug.md) for defect investigation.

## When To Use

The `spec` jig applies when:

- The project maintains a living specification as source of truth
- The project is greenfield and specs should be written before code
- Changes must be designed as spec updates first, then implemented to match
- Multiple spec files may be affected by a single change

It does not apply to existing codebases where code is the source of truth (use the `plan` jig) or to defects (use the `bug` jig).

## Frontmatter

The `spec` jig file contains this YAML frontmatter:

```yaml
---
name: spec
description: Maintain a living spec that defines your system. Spec is always right.
version: 1
status_values:
  - problem-space
  - decompose
  - research
  - change-design
  - spec-draft
  - integration
  - tasks
  - ready
passes:
  - name: "Problem Space"
    status: problem-space
    output: ["01-problem-space.md"]
  - name: "Decompose"
    status: decompose
    output: ["02-components.md"]
  - name: "Research"
    status: research
    output: ["03-research/{component}/findings.md"]
  - name: "Change Design"
    status: change-design
    output: ["04-design/{component}-design.md"]
  - name: "Spec Draft"
    status: spec-draft
    output: ["05-spec-drafts/{component}.md", "05-changelog.md"]
  - name: "Integration"
    status: integration
    output: ["06-integration.md"]
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
  - 02-components.md
  - "03-research/{component}/findings.md"
  - "04-design/{component}-design.md"
  - "05-spec-drafts/{component}.md"
  - 05-changelog.md
  - 06-integration.md
  - 07-tasks.md
---
```

## Status Progression

```
problem-space -> decompose -> research -> change-design -> spec-draft -> integration -> tasks -> ready
```

The `ready` status indicates the work is complete and available for finalization. Statuses beyond `ready` (e.g., `implementing`, `done`) are orchestrator-defined and outside the jig's scope.

## Component Placeholder

In this jig, the `{component}` placeholder expands differently depending on the pass:

- In **Research** and **Change Design**, `{component}` expands to the names of affected spec areas as identified in Decompose. These are typically the filenames of existing spec files (without the `.md` extension) or descriptive names for new spec files. For example: `jig-system`, `commands`, `finalization`.
- In **Spec Draft**, `{component}` expands to **target spec filenames** — the names of the spec files that will be created or updated. Each draft in `05-spec-drafts/` maps 1:1 to a file in the system `specs/` directory. For example: `05-spec-drafts/jig-system.md` maps to `specs/jig-system.md`.

This is different from other jigs where `{component}` refers to feature components. Here it refers to spec files because the unit of work is a spec file, not a code module.

## Passes

### Pass 1: Problem Space (problem-space)

**Output:** `01-problem-space.md`

Clarify what needs to change in the system and why. Identify which aspects of the system are affected.

#### Agent Instructions

**What to do:**

1. Read any source material the user has provided. Understand the motivation for the change before asking questions.
2. Clarify the user's goals. Ask "what should be true about the system after this change?" and "what problem does this solve?" Focus on the system-level impact, not implementation.
3. Define scope boundaries. What spec areas are in scope? What is explicitly out of scope? If the user hasn't thought about boundaries, propose them.
4. Identify constraints — backwards compatibility requirements, interactions with other spec areas, things that must not change.
5. Capture success criteria. These are concrete statements about what the specs should describe after the work is complete. "The jig system spec defines a review pattern" not "improve the review process."
6. Save to `01-problem-space.md`.

**What done looks like:**

- `01-problem-space.md` contains: a summary of what's changing and why, goals, non-goals, constraints, success criteria, and a preliminary list of spec areas that may be affected
- The user has confirmed the problem space is accurate

Advance status to `decompose`.

### Pass 2: Decompose (decompose)

**Output:** `02-components.md`

Identify which existing spec files are affected and what new spec files are needed. Define the scope of changes for each.

#### Agent Instructions

**What to do:**

1. Read `01-problem-space.md` to ground the decomposition in the agreed problem space.
2. Read the existing system specs that may be affected. For each spec file, understand its current content and scope.
3. Identify affected spec files. For each: what requirements does this change impose? What needs to be true after the change that isn't true now? State requirements in terms of what the spec should describe, not how the spec text should read — that comes in Change Design.
4. Identify new spec files needed. For each: what is its scope, what requirements does it satisfy, and what is the intended filename?
5. Map dependencies between spec changes. Which changes must be made before others? Which spec files reference each other?
6. Ask the user: walk through each affected area together (guided), or complete the full breakdown for review (autonomous)? Default to autonomous if no user is present. In guided mode: present each affected area individually, get approval before proceeding. Track progress ("Area 3/5: commands.md").
7. Save to `02-components.md`.

**What done looks like:**

- `02-components.md` lists each affected spec file (existing and new) with: a one-line description of the change, concrete requirements for what the spec should describe after the change, and dependencies on other spec changes
- Every goal from `01-problem-space.md` maps to at least one spec change
- No spec change exists that isn't driven by a goal or requirement

Advance status to `research`.

#### Review Criteria

After completing the decomposition, spawn a review sub-agent with:
- The `02-components.md` file
- The `01-problem-space.md` for scope validation
- The existing spec files listed as affected

The reviewer checks:
- Every goal from `01-problem-space.md` maps to at least one spec area
- No spec area is listed that isn't justified by a goal or requirement
- Requirements describe what should be true, not how the text should change
- All relevant existing spec files are accounted for (no missing affected areas)
- Dependencies between spec changes are correctly identified

Up to 3 review rounds. After that, present artifacts and any remaining findings to the user for approval.

### Pass 3: Research (research)

**Output:** `03-research/{component}/findings.md` (one file per affected spec area)

Investigate each affected area to inform the design. The `{component}` placeholder expands to one directory per affected spec area from Pass 2.

#### Agent Instructions

**What to do:**

1. Read `02-components.md` to understand what each spec area needs.
2. For each affected area, identify 3-5 research questions. Examples: "What does the current spec say about finalization?" "How do other spec files cross-reference this one?" "Are there existing patterns in the spec corpus for this kind of structure?" "What does the source material say about this topic?"
3. Delegate research to a sub-agent with fresh context. Provide the sub-agent with the component requirements from `02-components.md` and access to the existing specs and any source material.
4. The sub-agent reads existing spec files, checks source material, identifies patterns and constraints, and reports findings.
5. Save findings per area to `03-research/{component}/findings.md`.
6. Present key findings to the user. Flag decisions needed — especially where existing specs have patterns that should be followed or where the proposed change conflicts with existing content.

**What done looks like:**

- For each affected area, `03-research/{component}/findings.md` contains: research questions, findings with evidence (spec references, source material quotes), patterns to follow, and risks or conflicts identified
- All research questions are addressed with evidence
- No unresolved blockers prevent writing a change design

Advance status to `change-design`.

### Pass 4: Change Design (change-design)

**Output:** `04-design/{component}-design.md` (one file per affected spec area)

Document the intended changes for each affected spec area. This is the design document — it explains the *intent* of each spec change, not the spec text itself.

#### Agent Instructions

**What to do:**

1. Read the research findings for the area you are designing (`03-research/{component}/findings.md`).
2. Read the existing spec file (if modifying an existing spec). Understand its current structure and content.
3. Document the **current state**: what the spec says now (or "new file" if creating a new spec).
4. Document the **target state**: what the spec should say after the change. Be specific about sections, content, and structure. This is not the final spec text — it is a description of the change.
5. Document the **rationale**: why this change is needed, which requirements it satisfies, and how the research findings informed the design.
6. Ask the user: guided or autonomous? Default to autonomous if no user is present. In guided mode: present each area's design individually, get approval before proceeding.
7. Save per area to `04-design/{component}-design.md`.

**What done looks like:**

- For each affected area, `04-design/{component}-design.md` contains: current state, target state, rationale, and requirements traceability
- Every requirement from `02-components.md` is addressed by a target state
- The target state is specific enough that a spec writer can produce the final text from it

Advance status to `spec-draft`.

#### Review Criteria

After completing all change designs, spawn a review sub-agent with:
- All files in `04-design/`
- The `02-components.md` requirements document
- The relevant `03-research/` findings
- The existing spec files being modified

The reviewer checks:
- Every requirement from `02-components.md` has a corresponding target state
- No target state exists that isn't backed by a requirement
- Current state accurately reflects what the spec says now
- Target state is specific enough to write spec text from
- Rationale references research findings where applicable
- No contradictions between different areas' target states

Up to 3 review rounds. After that, present artifacts and any remaining findings to the user for approval.

### Pass 5: Spec Draft (spec-draft)

**Output:** `05-spec-drafts/{component}.md` (one file per target spec file), `05-changelog.md`

Write the actual spec text as it should appear in the system specs. This is the most critical pass — the drafted text will become the normative specification at finalization.

#### Agent Instructions

**Drafts are named to match their target spec files.** Each file in `05-spec-drafts/` maps 1:1 to a file in the system `specs/` directory:

- For an **existing** spec file: the draft uses the same filename. Example: a draft updating `specs/jig-system.md` is saved as `05-spec-drafts/jig-system.md`. The draft contains the complete updated spec file — not a diff, not a patch, the full file as it should appear after the change.
- For a **new** spec file: the draft uses the intended filename. Example: if creating a new spec that will live at `specs/jig-spec.md`, the draft is `05-spec-drafts/jig-spec.md`.

This naming convention makes finalization a direct copy: each file in `05-spec-drafts/` is copied to the corresponding path in `spec_path`.

**What to do:**

1. Read the change design for the area you are drafting (`04-design/{component}-design.md`).
2. If modifying an existing spec: read the current spec file. Your draft must include the full updated file, incorporating all existing content that is not being changed alongside the new or modified content.
3. If creating a new spec: follow the conventions of existing spec files in the project (formatting, section structure, cross-reference style).
4. Write spec text that is normative — "the system does X", not "we chose X because Y." Design rationale belongs in the change design documents, not in the spec.
5. Ensure cross-references are correct. If the spec links to other spec files, verify those links will be valid after all changes are applied.
6. Save the draft to `05-spec-drafts/{target-filename}.md`.
7. After all drafts are written, produce the changelog (`05-changelog.md`). The changelog documents for each spec file:
   - **File**: the target spec filename
   - **Status**: new, modified, or removed
   - **Changes**: what was changed, added, or removed
   - **Driven by**: which Change Design document(s) motivated the change

**Changelog format:**

```markdown
# Spec Changelog

## {target-filename}.md
**Status:** modified
**Changes:**
- Added section on review pattern (§Review Pattern)
- Updated built-in jig list from feature/bug to plan/spec/bug
- Relaxed pass output constraint to "zero or more files"
**Driven by:** 04-design/jig-system-design.md

## {new-filename}.md
**Status:** new
**Changes:**
- Complete new spec file defining the spec-first jig
**Driven by:** 04-design/jig-spec-design.md
```

**What done looks like:**

- `05-spec-drafts/` contains one file per target spec file, named to match the target
- Each draft for an existing spec contains the full updated file (not a diff)
- Each draft for a new spec follows project conventions
- Spec text is normative (describes what the system does, not why decisions were made)
- Cross-references between spec files are valid
- `05-changelog.md` accounts for every draft with changes and traceability

Advance status to `integration`.

#### Review Criteria

After completing all spec drafts and the changelog, spawn a review sub-agent with:
- All files in `05-spec-drafts/`
- All files in `04-design/` (the change designs)
- The existing spec files being modified (for comparison)
- The `05-changelog.md`

**This is the most critical review.** The reviewer checks:
- Every target state from the change designs is accurately reflected in the drafted spec text
- No spec content was added that isn't backed by a change design
- No existing spec content was accidentally removed or altered beyond what the change design calls for
- Spec text is normative, not rationale or design discussion
- Cross-references between drafted specs are valid (and consistent with unchanged specs)
- Draft filenames match their target spec files exactly
- The changelog accurately describes all changes and traces each to a change design
- Formatting and structure are consistent with the project's existing spec files

Up to 3 review rounds. After that, present artifacts and any remaining findings to the user for approval.

### Pass 6: Integration (integration)

**Output:** `06-integration.md`

Cross-reference consistency check across all drafted spec changes and the existing system specs.

#### Agent Instructions

**What to do:**

1. Read all drafted specs in `05-spec-drafts/`.
2. Read all existing system specs — not just the ones being modified. Changes to one spec can introduce contradictions with specs that aren't being changed.
3. Check for contradictions. Does any drafted spec state something that conflicts with an unchanged spec? Do drafted specs conflict with each other?
4. Verify cross-references. For every link between spec files (`[text](file.md)`), verify the target exists and the linked content is accurate.
5. Check terminology consistency. Are the same concepts referred to by the same names across all specs?
6. Verify that the changelog in `05-changelog.md` is complete and accurate against the actual drafts.
7. Save review notes to `06-integration.md`.

**What done looks like:**

- `06-integration.md` contains: a list of all cross-reference checks performed, any contradictions found (with resolution), any consistency issues found (with resolution), and a final assessment of overall spec coherence
- All contradictions are resolved (either by updating drafts or documenting why the apparent contradiction is acceptable)
- All cross-references are valid

Advance status to `tasks`.

#### Review Criteria

After completing the integration check, spawn a review sub-agent with:
- The `06-integration.md` file
- All files in `05-spec-drafts/`
- All existing system spec files

The reviewer checks:
- The integration check examined all system specs, not just modified ones
- Cross-references are valid in both directions (nothing links to removed content, nothing was orphaned)
- Terminology is consistent across the spec corpus
- No contradictions remain unresolved
- The changelog matches the actual drafted changes

Up to 3 review rounds. After that, present artifacts and any remaining findings to the user for approval.

### Pass 7: Tasks (tasks)

**Output:** `07-tasks.md`

Break the spec changes into implementation tasks. Each task defines what code changes are needed to make the codebase match the updated specs.

#### Agent Instructions

**What to do:**

1. Read all drafted specs in `05-spec-drafts/` and the changelog in `05-changelog.md`.
2. For each spec change, identify what code changes are needed to make the codebase consistent with the updated spec.
3. Define implementation tasks. Each task specifies:
   - What to build or change
   - Which spec sections it implements (with file and section references)
   - Deliverables (files created or modified)
   - Acceptance criteria (how to verify the code matches the spec)
   - Dependencies on other tasks
4. Define the dependency graph. Which tasks must complete before others? Which can run in parallel?
5. Keep the format implementation-agnostic — tasks should be portable to any tracker or execution system.
6. Save to `07-tasks.md`.

**What done looks like:**

- `07-tasks.md` contains: a task list with spec traceability, a dependency graph, and a parallelization plan
- Every spec change from the changelog has at least one corresponding task
- Every task traces back to a specific spec section
- Dependencies are correct (no circular dependencies, no missing prerequisites)
- Tasks are concrete enough for an implementing agent to execute without additional design decisions

Advance status to `ready`.

#### Review Criteria

After completing the task breakdown, spawn a review sub-agent with:
- The `07-tasks.md` file
- The `05-changelog.md` for completeness checking
- All files in `05-spec-drafts/` for spec traceability

The reviewer checks:
- Every changelog entry has at least one implementing task
- Every task traces to a specific spec section
- Dependencies form a valid DAG (no cycles, correct ordering)
- Acceptance criteria are concrete and testable
- The parallelization plan is realistic (no undeclared dependencies between parallel tasks)
- Task granularity is appropriate (not too coarse, not too fine)

Up to 3 review rounds. After that, present artifacts and any remaining findings to the user for approval.

### Pass 8: Ready (ready)

**Output:** (none)

Run `kerf square <codename>` to verify all expected artifacts exist. The work is ready for finalization and implementation.

#### Agent Instructions

**What to do:**

1. Run `kerf square <codename>` to verify structural completeness. All expected artifacts must exist on disk.
2. If square fails, identify and fix the missing artifacts.
3. Confirm the work is ready: spec drafts are consistent, changelog is complete, tasks are defined, integration check passed.

**What done looks like:**

- `kerf square` passes with no errors
- All spec drafts, the changelog, integration notes, and task list are complete and on disk
- The work is ready for `kerf finalize`

## File Structure

A work governed by the `spec` jig contains these files:

```
{codename}/
  spec.yaml
  SESSION.md
  01-problem-space.md
  02-components.md
  03-research/{component}/findings.md
  04-design/{component}-design.md
  05-spec-drafts/{component}.md
  05-changelog.md
  06-integration.md
  07-tasks.md
```

`spec.yaml` and `SESSION.md` are managed by kerf (see [works.md](works.md) and [sessions.md](sessions.md)). All other files are produced by the passes defined above.

The `{component}` placeholder expands to one entry per affected spec area. For example, a work affecting specs `jig-system`, `commands`, and `finalization` produces:

```
03-research/jig-system/findings.md
03-research/commands/findings.md
03-research/finalization/findings.md
04-design/jig-system-design.md
04-design/commands-design.md
04-design/finalization-design.md
05-spec-drafts/jig-system.md
05-spec-drafts/commands.md
05-spec-drafts/finalization.md
```

Note that in `05-spec-drafts/`, the filenames match the target spec files directly. `05-spec-drafts/jig-system.md` will be copied to `specs/jig-system.md` at finalization.

## Finalization

When the work reaches `ready` status and `kerf square` passes, the work is eligible for [finalization](finalization.md). Spec-first works have a **dual-destination finalization**:

1. **Process artifacts** (problem space, components, research, designs, changelog, integration notes, tasks) are copied to `repo_spec_path` — the standard artifact destination for all works.
2. **Spec drafts** (`05-spec-drafts/`) are copied to `spec_path` (default: `specs/`) — the system specs directory. Each file in `05-spec-drafts/` maps 1:1 to a file in `spec_path`, preserving filenames.

The `05-spec-drafts/` directory is **excluded** from the standard artifact copy to avoid duplicating spec files in both locations.

The finalization commit includes both the process record (in `repo_spec_path`) and the normative spec changes (in `spec_path`). The commit message remains `kerf: finalize {codename}`.

**Requires `spec_path` config.** The spec-first jig needs to know where system specs live. Set via `kerf config spec_path specs/` (path relative to repo root). Default: `specs/`. If `spec_path` does not exist at finalization time, kerf creates it.

See [finalization.md](finalization.md) for full finalization behavior and [commands.md](commands.md) for the `kerf finalize` command.
