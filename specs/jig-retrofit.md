# Retrofit Jig

> Built-in jig for reconciling code and specs when code changed without the spec workflow.

Code has changed without going through the spec workflow. Specs and code are out of sync. This jig guides an agent through a structured reconciliation: capture what changed, document why it changed, update specs to match reality, and verify everything is in sync. See [jig-system.md](jig-system.md) for file format, resolution, and versioning. See [jig-plan.md](jig-plan.md), [jig-spec.md](jig-spec.md), and [jig-bug.md](jig-bug.md) for the other built-in jigs.

## When to Use

Use the `retrofit` jig when code has diverged from specs outside the normal spec workflow. This includes: an agent made changes without creating a plan first, a quick fix was applied weeks ago without updating specs, a different agent or human modified code and nobody updated the corresponding specs, or specs simply drifted out of date over time.

If the work involves designing something new, use the [`plan`](jig-plan.md) jig. If the work involves investigating a defect, use the [`bug`](jig-bug.md) jig. If the work involves iterative exploration where the approach is unknown, use the [`spike`](jig-spike.md) jig. A spike that has already converged on an approach may use the retrofit jig for the final sync step.

## Entry Modes

The retrofit jig supports two entry modes depending on how much context is available:

**Mode A -- "Caught the agent" (context-rich):** The session that did the work is still alive or recent. The agent has full context -- what it changed, why, what it tried, what it learned. The retrofit captures all of this directly from the agent.

**Mode B -- "Found a divergence" (context-poor):** Specs and code don't match and nobody remembers exactly why. Could be a quick fix from weeks ago, a different agent, or gradual drift. The retrofit captures what changed from diffs and offers light inference on why -- but explicitly flags inferred rationale as inferred, not authoritative.

The entry mode is determined in Pass 1 (Capture) and affects the instructions for Pass 2 (Rationale). Both modes converge to the same process for Passes 3 and 4.

## Status Progression

```
capture -> rationale -> spec-sync -> square
```

## Frontmatter

The `retrofit` jig file contains this YAML frontmatter:

```yaml
---
name: retrofit
description: Reconcile code and specs when code changed without the spec workflow.
version: 1
phase: exploration
tools: []
composable: false
status_values:
  - capture
  - rationale
  - spec-sync
  - square
passes:
  - name: "Capture"
    status: capture
    output: ["01-capture.md"]
  - name: "Rationale"
    status: rationale
    output: ["02-rationale.md"]
  - name: "Spec Sync"
    status: spec-sync
    output: ["03-spec-sync.md"]
  - name: "Square"
    status: square
    output: []
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-capture.md
  - 02-rationale.md
  - 03-spec-sync.md
---
```

## Passes

### Pass 1: Capture (capture)

**Output:** `01-capture.md`

Identify and document exactly what changed between code and specs. Establish the scope of the divergence and determine which entry mode applies.

#### Agent Instructions

**What to do:**

1. Determine the entry mode:
   - **Mode A (context-rich):** You are the agent that made the changes, or you have direct access to the session/context that did. You know what changed and why. Record the mode as "Mode A -- context-rich" in `01-capture.md`.
   - **Mode B (context-poor):** You are investigating a divergence discovered after the fact. You do not have direct context on why the changes were made. Record the mode as "Mode B -- context-poor" in `01-capture.md`.
2. Identify the affected specs. Read the project's `specs/` directory and `specs/_index.md` to understand which specs govern the changed code areas.
3. Capture the code changes:
   - Run `git diff` and/or `git log` to identify what changed. Record commit hashes, files modified, and a summary of each change.
   - If the changes are uncommitted, capture the working tree diff.
   - If the changes span multiple commits, list each commit with its hash, message, and summary of changes.
4. Capture the spec state. For each affected spec, record the current spec text for the sections that are now out of sync with code.
5. Identify the divergence points. For each change, state clearly: "Code does X. Spec says Y." Be precise and concrete -- quote the spec text, reference the code location (file, function, line range).
6. Assess the scope: how many specs are affected, how large is each divergence, are there cascading effects (e.g., a changed interface that affects multiple specs).
7. Save to `01-capture.md`.

**What "done" looks like:**

- `01-capture.md` contains: entry mode (A or B), list of affected specs, code changes with commit hashes/diffs, current spec text for affected sections, explicit divergence points ("code does X, spec says Y"), and scope assessment.
- The divergences are precise enough that another agent could read them and know exactly what is out of sync without investigating further.

### Pass 2: Rationale (rationale)

**Output:** `02-rationale.md`

Document why the code changed. This pass differs significantly between Mode A and Mode B.

#### Agent Instructions -- Mode A (context-rich)

**What to do:**

1. For each divergence identified in `01-capture.md`, explain the intent behind the change. Why was the code modified? What problem was it solving?
2. Document what was tried and rejected. If multiple approaches were attempted before settling on the current code, record them and explain why they were discarded. This is valuable knowledge that would otherwise be lost when the session ends.
3. Document constraints discovered during the work. Were there technical limitations, API behaviors, performance characteristics, or compatibility issues that influenced the approach? Record them.
4. Document learnings. What did the agent learn during this work that is not captured anywhere else? What would a future agent need to know about this area of the code?
5. For each divergence, state whether the code change is correct (the spec should be updated to match) or whether the code change was a mistake (the code should be reverted or fixed). If the latter, flag it clearly -- the Spec Sync pass will need to handle this differently.
6. Save to `02-rationale.md`.

**What "done" looks like:**

- `02-rationale.md` contains: for each divergence -- the intent/rationale, alternatives tried and why they were rejected, constraints discovered, learnings, and a disposition (spec should update to match code / code should be corrected).
- The rationale is authoritative -- it comes from the agent or session that did the work.

#### Agent Instructions -- Mode B (context-poor)

**What to do:**

1. For each divergence identified in `01-capture.md`, examine the code change and offer a light inference on why it was made. Use evidence from:
   - Commit messages (if the change was committed)
   - Code comments added as part of the change
   - The nature of the change itself (e.g., "adds a nil check" suggests a crash was encountered)
   - Related issues, PRs, or documentation if available
2. **Flag every inference as inferred.** Use explicit language: "This appears to..." / "The commit message suggests..." / "Inferred: ...". Never state inferred rationale as fact. Do not fabricate intent. "This appears to handle a nil pointer case" is acceptable. "This was changed to handle a nil pointer case" is not -- unless the commit message or a comment explicitly says so.
3. For inferences where you have low confidence, say so: "Low confidence -- the reason for this change is unclear from the available evidence."
4. Check git blame and git log for the affected lines to identify when and by whom (or which agent session) the change was made. Record this context.
5. For each divergence, state whether the code change appears correct (the spec should probably be updated) or whether it appears to be a mistake or regression. Flag this assessment as inferred when appropriate.
6. Save to `02-rationale.md`.

**What "done" looks like:**

- `02-rationale.md` contains: for each divergence -- inferred rationale (explicitly flagged as inferred), evidence supporting the inference, confidence level, git history context, and inferred disposition (spec should probably update / code may need correction).
- No inferred rationale is presented as authoritative fact. Every inference is explicitly labeled.

### Pass 3: Spec Sync (spec-sync)

**Output:** `03-spec-sync.md`

Update specs to match the current code reality. This pass produces both a sync plan and the actual spec changes.

#### Agent Instructions

**What to do:**

1. Read `01-capture.md` and `02-rationale.md` to understand the full picture: what diverged, why, and the disposition for each divergence.
2. For each divergence with disposition "spec should update to match code":
   - Draft the spec change. Write the new spec text that accurately describes the current code behavior.
   - Specs are normative: "the system does X", not "we changed X because Y". The rationale lives in `02-rationale.md` and the retroactive plan, not in the spec itself.
   - Ensure the spec change is consistent with the rest of the spec. Check for cross-references, terminology, and related sections that may also need updating.
3. For each divergence with disposition "code should be corrected":
   - Document this clearly in `03-spec-sync.md`. The spec is correct; the code needs to change.
   - Do not modify the spec for these items. Instead, note them as action items for follow-up (potentially a new work with the `bug` jig or a plan to correct the code).
4. For Mode B inferred dispositions: present the proposed spec changes to the user for review before applying them. Inferred rationale should not drive spec changes without human confirmation. Note in `03-spec-sync.md` which changes are based on inferred rationale and require user approval.
5. Apply the approved spec changes. Edit the actual spec files in `specs/`. Record each spec file modified and what was changed.
6. If any spec changes affect `specs/_index.md` (e.g., new specs added, specs renamed, scope changes), update the index.
7. Save the sync record to `03-spec-sync.md`. This file documents what spec changes were made (or proposed), which spec files were modified, and any items flagged for follow-up.

**What "done" looks like:**

- `03-spec-sync.md` contains: for each divergence -- the spec change made (or proposed), the spec file(s) modified, and any follow-up items.
- Actual spec files in `specs/` have been updated to reflect the current code reality (for approved changes).
- Items where code should be corrected are documented as follow-up action items, not papered over with spec changes.
- Mode B changes based on inferred rationale are clearly marked as requiring user approval.

#### Review Criteria

After completing this pass, spawn a review sub-agent (see [jig-system.md](jig-system.md) Review Pattern for the sub-agent delegation protocol) with:
- `03-spec-sync.md`
- `01-capture.md` (to verify all divergences are addressed)
- `02-rationale.md` (to verify spec changes match the documented rationale/disposition)
- The modified spec files

The reviewer checks:
- Every divergence from `01-capture.md` is addressed -- either the spec was updated or a follow-up item was created
- Spec changes accurately describe the current code behavior -- normative language, not narrative
- Spec changes are consistent with the rest of each spec (cross-references, terminology, related sections)
- Mode B inferred items are properly flagged for user approval
- No divergence was silently dropped or ignored

Up to 3 review rounds. After that, present artifacts and any remaining findings to the user for approval.

### Pass 4: Square (square)

**Output:** none

Verify that specs and code are in sync and all artifacts are complete. This is the terminal pass.

#### Agent Instructions

**What to do:**

1. Run `kerf square <codename>` to verify all expected artifacts exist.
2. If square fails, return to the appropriate pass to produce the missing artifacts.
3. Verify the sync is complete:
   - Re-read each divergence from `01-capture.md`.
   - Confirm that either the spec has been updated (check the actual spec file) or a follow-up item has been created.
   - Check that no new divergences have been introduced by the spec changes themselves.
4. Compile the retroactive plan. The combination of `01-capture.md` (what changed), `02-rationale.md` (why), and `03-spec-sync.md` (how specs were updated) constitutes the retroactive plan documenting the change. This is the permanent record.
5. Once square passes, the retrofit is complete.

**What "done" looks like:**

- `kerf square` reports SQUARE.
- All 3 artifact files exist and are populated.
- Every divergence is resolved: spec updated or follow-up item created.
- Specs and code are verified in sync.
- The retroactive plan (the three artifact files together) provides a complete record of what changed, why, and how specs were reconciled.

## Finalization

When the work reaches `square` status and `kerf square` passes, the work is eligible for [finalization](finalization.md). `kerf finalize <codename>` packages the retrofit record.

The retrofit produces a retroactive plan as its primary output -- the three artifact files together document exactly what changed, why (or best inference of why), and how specs were reconciled. For Mode A retrofits, the learnings captured in `02-rationale.md` (alternatives tried, constraints discovered) are particularly valuable and would otherwise be lost.

## File Structure

A work governed by the `retrofit` jig contains the following files:

```
{codename}/
  spec.yaml
  SESSION.md
  01-capture.md
  02-rationale.md
  03-spec-sync.md
```

`spec.yaml` and `SESSION.md` are defined in [works.md](works.md) and [sessions.md](sessions.md) respectively. All other files are pass outputs defined by this jig.
