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

# Spec Jig

## Overview

This jig guides you through a structured process for maintaining a living system specification. Changes to the system start as spec updates, then flow to code. You identify what specs need to change, research the impact, design the changes, draft new spec text, and produce implementation tasks.

Each pass produces one or more files. If work is not captured in a file, it is lost when the session ends.

In this jig, the `{component}` placeholder expands to affected spec areas (typically spec filenames without the `.md` extension). In the Spec Draft pass, `{component}` expands to target spec filenames -- each draft in `05-spec-drafts/` maps 1:1 to a file in the system `specs/` directory.

## Pass 1: Problem Space (problem-space)

**Output:** `01-problem-space.md`

Clarify what needs to change in the system and why. Identify which aspects of the system are affected.

This is a conversation, not a questionnaire. Ask 2-3 focused questions, listen to the answers, then ask follow-up questions based on what you learn. Do not fire all questions at once.

**What to do:**

1. Read any source material the user has provided. Understand the motivation for the change before asking questions.
2. Clarify the user's goals. Ask "what should be true about the system after this change?" and "what problem does this solve?" Focus on the system-level impact, not implementation.
3. Define scope boundaries. What spec areas are in scope? What is explicitly out of scope? If the user hasn't thought about boundaries, propose them.
4. Identify constraints -- backwards compatibility requirements, interactions with other spec areas, things that must not change.
5. Capture success criteria. These are concrete statements about what the specs should describe after the work is complete. "The jig system spec defines a review pattern" not "improve the review process."
6. Save to `01-problem-space.md`. Advance status to `decompose`.

**What done looks like:**

- `01-problem-space.md` contains: a summary of what's changing and why, goals, non-goals, constraints, success criteria, and a preliminary list of spec areas that may be affected
- The user has confirmed the problem space is accurate

## Pass 2: Decompose (decompose)

**Output:** `02-components.md`

Identify which existing spec files are affected and what new spec files are needed. Define the scope of changes for each.

**What to do:**

1. Read `01-problem-space.md` to ground the decomposition in the agreed problem space.
2. Read the existing system specs that may be affected. For each spec file, understand its current content and scope.
3. Identify affected spec files. For each: what requirements does this change impose? What needs to be true after the change that isn't true now? State requirements in terms of what the spec should describe, not how the spec text should read.
4. Identify new spec files needed. For each: what is its scope, what requirements does it satisfy, and what is the intended filename?
5. Map dependencies between spec changes. Which changes must be made before others? Which spec files reference each other?
6. Ask the user: walk through each affected area together (guided), or complete the full breakdown for review (autonomous)? Default to autonomous if no user is present.
7. Save to `02-components.md`. Advance status to `research`.

**What done looks like:**

- `02-components.md` lists each affected spec file (existing and new) with: a one-line description of the change, concrete requirements for what the spec should describe after the change, and dependencies on other spec changes
- Every goal from `01-problem-space.md` maps to at least one spec change
- No spec change exists that isn't driven by a goal or requirement

### Review Criteria

After completing the decomposition, spawn a review sub-agent with:
- `02-components.md`
- `01-problem-space.md` (for scope validation)
- The existing spec files listed as affected

The reviewer checks:
- Every goal from `01-problem-space.md` maps to at least one spec area
- No spec area is listed that isn't justified by a goal or requirement
- Requirements describe what should be true, not how the text should change
- All relevant existing spec files are accounted for
- Dependencies between spec changes are correctly identified

Up to 3 review rounds. Save findings to `decompose-review.md`.

## Pass 3: Research (research)

**Output:** `03-research/{component}/findings.md` (one file per affected spec area)

Investigate each affected area to inform the design.

**What to do:**

1. Read `02-components.md` to understand what each spec area needs.
2. For each affected area, identify 3-5 research questions. Examples: "What does the current spec say about finalization?" "How do other spec files cross-reference this one?" "Are there existing patterns in the spec corpus for this kind of structure?"
3. Delegate research to a sub-agent with fresh context. Provide the component requirements from `02-components.md` and access to the existing specs and any source material.
4. Save findings per area to `03-research/{component}/findings.md`.
5. Present key findings to the user. Flag decisions needed -- especially where existing specs have patterns that should be followed or where the proposed change conflicts with existing content.

**What done looks like:**

- For each affected area, findings file contains: research questions, findings with evidence, patterns to follow, and risks or conflicts identified
- All research questions are addressed with evidence
- No unresolved blockers prevent writing a change design

Advance status to `change-design`.

## Pass 4: Change Design (change-design)

**Output:** `04-design/{component}-design.md` (one file per affected spec area)

Document the intended changes for each affected spec area. This is the design document -- it explains the intent of each spec change, not the spec text itself.

**What to do:**

1. Read the research findings for the area you are designing.
2. Read the existing spec file (if modifying an existing spec). Understand its current structure and content.
3. Document the **current state**: what the spec says now (or "new file" if creating a new spec).
4. Document the **target state**: what the spec should say after the change. Be specific about sections, content, and structure.
5. Document the **rationale**: why this change is needed, which requirements it satisfies, and how the research findings informed the design.
6. Ask the user: guided or autonomous? Default to autonomous if no user is present.
7. Save per area to `04-design/{component}-design.md`. Advance status to `spec-draft`.

**What done looks like:**

- For each affected area, design file contains: current state, target state, rationale, and requirements traceability
- Every requirement from `02-components.md` is addressed by a target state
- The target state is specific enough that a spec writer can produce the final text from it

### Review Criteria

After completing all change designs, spawn a review sub-agent with:
- All files in `04-design/`
- `02-components.md` (requirements)
- The relevant `03-research/` findings
- The existing spec files being modified

The reviewer checks:
- Every requirement has a corresponding target state
- No target state exists that isn't backed by a requirement
- Current state accurately reflects what the spec says now
- Target state is specific enough to write spec text from
- No contradictions between different areas' target states

Up to 3 review rounds. Save findings to `change-design-review.md`.

## Pass 5: Spec Draft (spec-draft)

**Output:** `05-spec-drafts/{component}.md` (one file per target spec file), `05-changelog.md`

Write the actual spec text as it should appear in the system specs. This is the most critical pass -- the drafted text will become the normative specification at finalization.

**Drafts are named to match their target spec files.** Each file in `05-spec-drafts/` maps 1:1 to a file in the system `specs/` directory:

- For an **existing** spec file: the draft contains the complete updated spec file -- not a diff, not a patch, the full file as it should appear after the change.
- For a **new** spec file: the draft uses the intended filename and follows the conventions of existing spec files.

**What to do:**

1. Read the change design for the area you are drafting.
2. If modifying an existing spec: read the current spec file. Your draft must include the full updated file, incorporating all existing content that is not being changed alongside the new or modified content.
3. If creating a new spec: follow the conventions of existing spec files in the project.
4. Write spec text that is normative -- "the system does X", not "we chose X because Y." Design rationale belongs in the change design documents, not in the spec.
5. Ensure cross-references are correct. If the spec links to other spec files, verify those links will be valid after all changes are applied.
6. Save the draft to `05-spec-drafts/{target-filename}.md`.
7. After all drafts are written, produce the changelog (`05-changelog.md`). The changelog documents for each spec file: the target filename, status (new/modified/removed), what was changed, and which change design motivated the change.

**What done looks like:**

- `05-spec-drafts/` contains one file per target spec file, named to match the target
- Each draft for an existing spec contains the full updated file (not a diff)
- Spec text is normative (describes what the system does, not why decisions were made)
- Cross-references between spec files are valid
- `05-changelog.md` accounts for every draft with changes and traceability

Advance status to `integration`.

### Review Criteria

**This is the most critical review.** After completing all spec drafts, spawn a review sub-agent with:
- All files in `05-spec-drafts/`
- All files in `04-design/` (the change designs)
- The existing spec files being modified (for comparison)
- `05-changelog.md`

The reviewer checks:
- Every target state from the change designs is accurately reflected in the drafted spec text
- No spec content was added that isn't backed by a change design
- No existing spec content was accidentally removed or altered beyond what the change design calls for
- Spec text is normative, not rationale or design discussion
- Cross-references between drafted specs are valid
- Draft filenames match their target spec files exactly
- The changelog accurately describes all changes
- Formatting and structure are consistent with the project's existing spec files

Up to 3 review rounds. Save findings to `spec-draft-review.md`.

## Pass 6: Integration (integration)

**Output:** `06-integration.md`

Cross-reference consistency check across all drafted spec changes and the existing system specs.

**What to do:**

1. Read all drafted specs in `05-spec-drafts/`.
2. Read all existing system specs -- not just the ones being modified. Changes to one spec can introduce contradictions with specs that aren't being changed.
3. Check for contradictions. Does any drafted spec state something that conflicts with an unchanged spec? Do drafted specs conflict with each other?
4. Verify cross-references. For every link between spec files, verify the target exists and the linked content is accurate.
5. Check terminology consistency. Are the same concepts referred to by the same names across all specs?
6. Verify that `05-changelog.md` is complete and accurate against the actual drafts.
7. Save review notes to `06-integration.md`. Advance status to `tasks`.

**What done looks like:**

- `06-integration.md` contains: cross-reference checks performed, any contradictions found (with resolution), consistency issues found (with resolution), and a final assessment of overall spec coherence
- All contradictions are resolved
- All cross-references are valid

### Review Criteria

After completing the integration check, spawn a review sub-agent with:
- `06-integration.md`
- All files in `05-spec-drafts/`
- All existing system spec files

The reviewer checks:
- The integration check examined all system specs, not just modified ones
- Cross-references are valid in both directions
- Terminology is consistent across the spec corpus
- No contradictions remain unresolved

Up to 3 review rounds. Save findings to `integration-review.md`.

## Pass 7: Tasks (tasks)

**Output:** `07-tasks.md`

Break the spec changes into implementation tasks. Each task defines what code changes are needed to make the codebase match the updated specs.

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
5. Save to `07-tasks.md`. Advance status to `ready`.

**What done looks like:**

- `07-tasks.md` contains: a task list with spec traceability, a dependency graph, and a parallelization plan
- Every spec change from the changelog has at least one corresponding task
- Every task traces back to a specific spec section
- Dependencies are correct (no circular dependencies, no missing prerequisites)
- Tasks are concrete enough for an implementing agent to execute without additional design decisions

### Review Criteria

After completing the task breakdown, spawn a review sub-agent with:
- `07-tasks.md`
- `05-changelog.md`
- All files in `05-spec-drafts/`

The reviewer checks:
- Every changelog entry has at least one implementing task
- Every task traces to a specific spec section
- Dependencies form a valid DAG
- Acceptance criteria are concrete and testable
- Task granularity is appropriate

Up to 3 review rounds. Save findings to `tasks-review.md`.

## Pass 8: Ready (ready)

**Output:** (none)

Run `kerf square <codename>` to verify all expected artifacts exist. The work is ready for finalization and implementation.

**What to do:**

1. Run `kerf square <codename>` to verify structural completeness.
2. If square fails, identify and fix the missing artifacts.
3. Confirm the work is ready: spec drafts are consistent, changelog is complete, tasks are defined, integration check passed.

**What done looks like:**

- `kerf square` passes with no errors
- All spec drafts, the changelog, integration notes, and task list are complete and on disk
- The work is ready for `kerf finalize`

## Finalization

When this work moves to `ready`, run `kerf square <codename>` to verify, then `kerf finalize <codename>` to package it for implementation. Spec-first works have dual-destination finalization: process artifacts go to `repo_spec_path` and spec drafts are copied to `spec_path`.
