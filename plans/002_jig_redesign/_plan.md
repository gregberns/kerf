# Plan 002: Jig Redesign — Spec-First and Plan-First Workflows

## Intent

Redesign kerf's built-in jigs to support two distinct project management patterns: **plan-first** (code is source of truth) and **spec-first** (specs are source of truth). Add a first-run onboarding flow that helps users choose between them. Formalize the review pattern (sub-agent review with iteration) as a jig-level concept. Rewrite the bug jig as a structured investigation workflow.

## Why

kerf currently ships with `feature` and `bug` jigs that were placeholders — the feature jig's pass sequence was a sketch, and the bug jig had minimal structure. Through building kerf itself using the spec-first pattern, and referencing a battle-tested `/spec` workflow (see source/spec-skill-reference.md), we've identified:

1. **Two valid patterns exist.** Most developers have existing codebases and want plan-first ("I have an idea, help me plan and execute it"). Some teams want spec-first ("the spec is the source of truth, code must match"). Both need first-class support.

2. **The review loop was undefined.** Jigs had no concept of sub-agent review, review iteration, or review limits. In practice, agents writing specs need independent review — self-review misses drift.

3. **Human involvement needs flexibility.** Sometimes the user wants to walk through each component (guided mode). Sometimes there's strong consensus and the agent can complete autonomously. This should be a per-pass choice, encoded in jig instructions.

4. **The first-run experience was missing.** No onboarding flow to help users understand and choose between the two patterns.

## Review History

**Round 1:** Three parallel sub-agent reviews (spec coverage, architecture, process consistency). 38 findings total. Key changes applied.

**Round 2:** Three parallel sub-agent reviews (spec coverage, architecture, completeness). ~30 findings. Key changes:

- `aliases` field fully specified: type, resolution order (file match first, then alias scan), collision rules, canonical name recording.
- Spec draft file naming fixed: drafts are named to match target spec files (e.g., `05-spec-drafts/jig-system.md` maps to `specs/jig-system.md`). Finalization is a direct copy.
- Autonomous + unresolved review: after max rounds with no human, advance with findings saved to `{pass}-review.md`. Don't block autonomous workflows.
- `ready` pass with `output: []`: jig-system.md updated to "each pass produces zero or more files."
- Spec-first finalization: `05-spec-drafts/` excluded from standard artifact copy (no duplication). Only copied to `spec_path`.
- Review findings saved to disk as `{pass}-review.md` for resumability.
- `commands.md` finalize section added to affected specs for `spec_path` behavior.
- jig-system.md full example updated to use `plan` jig.
- SESSION.md exclusion inconsistency between finalization.md and commands.md noted for reconciliation.

**Round 1 (original):** Key design changes from review:

- Spec-first jig no longer writes directly to `specs/` during the work. Spec changes are drafted in the work directory and applied to the system specs directory at finalization. This preserves the bench/repo boundary invariant.
- Bug jig truncated to end at Fix Spec → Ready. Implementation and verification passes removed — they exceeded kerf's stated scope as a spec-writing tool.
- `feature` retained as an alias for `plan` during migration. Existing works and configs using `feature` continue to resolve.
- Review semantics (`reviewable`, `max_review_rounds`) moved from pass frontmatter to jig markdown body. kerf does not track review state — review is purely agent-driven, guided by jig instructions.
- "Plan" pass in spec-first jig renamed to "Change Design" to avoid collision with the kerf plan concept.
- Added `works.md`, `verification.md`, and `finalization.md` to affected specs list.

## What Changes

### New spec files
- `specs/jig-plan.md` — the plan-first jig (replaces `jig-feature.md`)
- `specs/jig-spec.md` — the spec-first jig (new)

### Modified spec files
- `specs/jig-system.md` — add review pattern section (in jig body, not frontmatter). Update built-in jig list. Add resumability rule (save after every pass).
- `specs/jig-bug.md` — rewrite as structured investigation workflow (Report → Research → Reproduce → Root Cause → Fix Spec → Ready)
- `specs/commands.md` — update `kerf new` for first-run onboarding error flow. Update `--jig` flag default. Update `kerf config` output for new fields.
- `specs/architecture.md` — add `spec_path` config value. Change `default_jig` default to unset.
- `specs/works.md` — update built-in types (`plan`, `spec`, `bug`), update example status progressions.
- `specs/verification.md` — clarify that square checks work-directory artifacts only. Spec-first jig's drafted spec changes are in the work directory and subject to the same checks.
- `specs/finalization.md` — add spec-first finalization behavior: copy drafted spec changes from work directory to `spec_path` in addition to standard artifact copying.
- `specs/_index.md` — update spec map: remove jig-feature, add jig-plan and jig-spec.

### Removed spec files
- `specs/jig-feature.md` — replaced by `specs/jig-plan.md`

### Migration
- The `feature` jig name resolves as an alias for `plan`. Existing works with `jig: feature` continue to function. `kerf jig list` shows `plan` with a note that `feature` is accepted.
- Bug jig version bumps to `2`. Existing bug works get a version mismatch warning (standard jig versioning behavior per jig-system.md).

## Detailed Design

### 1. Plan-First Jig (`plan`)

For existing codebases. Code is the source of truth. Specs describe planned changes and guide implementation.

**Frontmatter:**

```yaml
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
```

**Pass details:**

1. **Problem Space** — What are you changing and why? 2-3 conversational exchanges with the user. Capture goal, scope, constraints, what success looks like, what's out of scope. Save to `01-problem-space.md`.

2. **Analyze** — Agent reads relevant existing code and documents the current state. Maps the areas that will be affected by the change. Identifies existing patterns, conventions, and constraints. Save to `02-analysis.md`. This pass is unique to plan-first — spec-first doesn't need it because there's no existing codebase to map.

3. **Decompose** — Break the change into 3-7 components. Agent asks user: walk through each component together (guided), or complete the full breakdown for review (autonomous)? If no user is present, default to autonomous. In guided mode: present each component's requirements individually, get approval before proceeding to the next, track progress ("Component 3/5: Authentication"). In autonomous mode: complete all components, then present for review. Requirements must be concrete and testable — "returns 404 with error body" not "handles errors." Save to `03-components.md`. **Sub-agent review after completion** (see Review Pattern below).

4. **Research** — For each component, identify 3-5 specific research questions. Delegate research to a sub-agent (fresh context with the component requirements + codebase access). Sub-agent explores codebase for existing patterns, checks external docs/APIs, identifies technical constraints. Save findings per component to `04-research/{component}/findings.md`. Present key findings to user, flag decisions needed.

5. **Change Spec** — For each component (informed by research findings): write a change spec that includes requirements (from decompose), research summary, approach (how to implement), files & changes (what to create/modify), acceptance criteria (testable), and verification (how to confirm it works). Agent asks: guided or autonomous? (Default: autonomous if no user present.) Save per component to `05-specs/{component}-spec.md`. **Sub-agent review.**

6. **Integration** — How components connect to each other. Write integration plan (`06-integration.md`). Assemble all component specs + integration plan into a single reference document (`SPEC.md`). **Sub-agent review for cross-component consistency.**

7. **Tasks** — Break the spec into implementation tasks with dependencies. Each task specifies: what to build, which spec sections it implements, deliverables, acceptance criteria, tests. Define dependency graph and parallelization plan. Implementation-agnostic format (portable to any tracker). Save to `07-tasks.md`. **Sub-agent review for completeness and dependency correctness.**

8. **Ready** — Run `kerf square` to verify all expected artifacts exist. Work is ready for implementation. kerf's job ends here — implementation uses whatever tooling the team prefers.

**All artifacts live in the work directory.** The work is a self-contained change document. After implementation, the work can be archived — the code is the source of truth.

### 2. Spec-First Jig (`spec`)

For greenfield projects or teams maintaining a living system specification. Specs are the source of truth. Code that doesn't match the spec is wrong.

**Frontmatter:**

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

**Pass details:**

1. **Problem Space** — What needs to change in the system and why? What aspects of the system are affected? Save to `01-problem-space.md`.

2. **Decompose** — Identify which existing spec files are affected and what new spec files are needed. For each affected area: what are the requirements for the change? (What needs to be true after the change, not how the spec text should change — that comes in Change Design.) Save to `02-components.md`. **Sub-agent review.**

3. **Research** — For each affected area, identify 3-5 research questions and investigate. Delegate to sub-agent. Save to `03-research/{component}/findings.md`.

4. **Change Design** — For each affected spec area: document current state (what the spec says now), target state (what it should say after), and the rationale (why this change). This is the design document — it explains the *intent* of each spec change. Agent asks: guided or autonomous? (Default: autonomous if no user present.) Save per component to `04-design/{component}-design.md`. **Sub-agent review.**

5. **Spec Draft** — Write the actual spec text as it should appear in the system specs. For each affected spec file, produce a draft in the work directory at `05-spec-drafts/{target-filename}.md`. **Drafts are named to match their target spec file** — e.g., a draft for `specs/jig-system.md` is saved as `05-spec-drafts/jig-system.md`. For new spec files, use the intended filename. This naming convention makes finalization a direct copy: each file in `05-spec-drafts/` maps 1:1 to a file in `spec_path`. Also produce a changelog (`05-changelog.md`) documenting for each spec file: what changed, what was added, what was removed, and which Change Design document drove the change. These drafts live in the work directory — they are applied to the system `specs/` directory at finalization (see Finalization below). **Sub-agent review — this is the most critical review, comparing drafted spec text against the change design.**

6. **Integration** — Cross-reference consistency check. Read the drafted spec changes alongside the existing system specs. Verify no contradictions introduced. Check that cross-references and links are valid. Save review notes to `06-integration.md`. **Sub-agent review.**

7. **Tasks** — Implementation tasks derived from the spec changes. Each task references the specific spec sections it implements. Tasks define what code changes are needed to make the codebase match the updated specs. Save to `07-tasks.md`. **Sub-agent review.**

8. **Ready** — Square check. All artifacts exist. Spec drafts are consistent. Tasks are defined. Ready for finalization and implementation.

**Artifacts live in the work directory until finalization.** The work directory holds everything: problem space, design rationale, drafted spec text, changelog, integration notes, and tasks. The system `specs/` directory is NOT modified during the work. This preserves the bench/repo boundary invariant — works live on the bench and enter git only at finalization.

**Finalization for spec-first works.** When `kerf finalize` runs on a spec-first work, it copies the drafted spec files from `05-spec-drafts/` to the configured `spec_path` in the repository. The `05-spec-drafts/` directory is **excluded** from the standard artifact copy (to avoid duplicating spec files in both locations). The finalization commit includes the spec changes (in `spec_path`) and the process artifacts (problem space, design, changelog, tasks — in `repo_spec_path`). See Finalization Changes below.

**Requires `spec_path` config.** The spec-first jig needs to know where system specs live. Set via `kerf config spec_path specs/` (path relative to repo root). Default: `specs/`. If `spec_path` does not exist at finalization time, kerf creates it. This is a global config value — projects with different spec locations should use `--project`-scoped config (out of scope for v1; document this limitation).

### 3. Bug Jig (`bug`)

For investigating and resolving defects. Structured investigation workflow that produces a fix specification.

**Frontmatter:**

```yaml
---
name: bug
description: Investigate and specify a fix for a defect.
version: 2
status_values:
  - reported
  - research
  - reproducing
  - root-cause
  - fix-spec
  - ready
passes:
  - name: "Report"
    status: reported
    output: ["01-report.md"]
  - name: "Research"
    status: research
    output: ["02-research.md"]
  - name: "Reproduce"
    status: reproducing
    output: ["03-reproduction.md"]
  - name: "Root Cause"
    status: root-cause
    output: ["04-root-cause.md"]
  - name: "Fix Spec"
    status: fix-spec
    output: ["05-fix-spec.md"]
  - name: "Ready"
    status: ready
    output: []
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-report.md
  - 02-research.md
  - 03-reproduction.md
  - 04-root-cause.md
  - 05-fix-spec.md
---
```

**Pass details:**

1. **Report** — Capture the bug report. Expected behavior, actual behavior, environment, steps to reproduce (if known), severity and impact. Save to `01-report.md`.

2. **Research** — Agent investigates the codebase. Identify 3-5 specific investigation questions (e.g., "What code handles this endpoint?", "When was this code last modified?", "Are there related error reports?"). Read relevant code, trace execution path, check git history for recent changes, look for related issues. Document questions and findings in `02-research.md`.

3. **Reproduce** — Build a minimal reproduction case. This could be a test case, a script, CLI commands, or specific input data that triggers the bug. Document the reproduction steps and observed output in `03-reproduction.md`. If reproduction fails, document what was tried and escalate to user.

4. **Root Cause** — Based on research and reproduction, identify the root cause. Document: what's broken, why it's broken, when it was introduced (if determinable), blast radius (what else might be affected). Save to `04-root-cause.md`. **Sub-agent review — verify root cause analysis is sound and blast radius assessment is complete.**

5. **Fix Spec** — Specify the fix. What needs to change, approach, risks, tests to add or modify, acceptance criteria. If spec-first project: what spec changes are needed. Save to `05-fix-spec.md`. **Sub-agent review.**

6. **Ready** — Run `kerf square` to verify all artifacts exist. Bug investigation is complete. The fix spec is ready for implementation. kerf's job ends here — implementation and verification use whatever tooling the team prefers.

**Consistent with kerf's scope.** The bug jig produces a fix specification. It does not track implementation or verification — those are the responsibility of the implementation tooling. This matches the plan and spec jigs, which also end at Ready.

### 4. Review Pattern (jig-system.md changes)

Add a review pattern section to jig-system.md. Review is part of the jig's agent instructions (markdown body), not the frontmatter schema.

**Review behavior (encoded in jig instructions, not kerf runtime):**

Certain passes in each jig include review instructions in the jig's markdown body. When following these instructions, the agent:

1. Completes the pass artifacts and saves to disk
2. Spawns a review sub-agent with fresh context, providing:
   - The pass artifacts (the files just produced)
   - The relevant prior-pass artifacts or specs for comparison
   - The review criteria from the jig's markdown body
3. Sub-agent produces findings (specific, actionable — quote specs, cite line numbers)
4. Findings are saved to `{pass-name}-review.md` in the work directory (supports resumability — if context is compacted, the review state is on disk)
5. Original agent reads findings, applies fixes, and saves artifacts to disk
6. Sub-agent re-reviews (against updated artifacts)
7. Repeat up to 3 rounds (configurable per jig in the markdown instructions)
8. After the final round, or if the sub-agent finds no issues: escalate to the human
   - Human receives the polished artifacts AND any unresolved review findings (from `{pass-name}-review.md`)
   - Human can approve (advance to next pass), request more agent iteration, or intervene directly
9. **Autonomous mode** (no human present):
   - If the sub-agent approves (no findings): advance automatically via `kerf status <codename> <next-status>`
   - If the sub-agent has unresolved findings after max rounds: advance anyway, but save unresolved findings to `{pass-name}-review.md` with a `## Unresolved` section. Do not block autonomous workflows. The findings persist on disk for later human review.

**kerf's role:** kerf does not orchestrate reviews. kerf tracks the pass status (a single string). The jig's markdown body tells the agent how to conduct reviews — this is guidance, same as all other jig instructions. kerf never spawns sub-agents.

**Why not frontmatter?** Review semantics are process guidance, not machine-readable data that kerf acts on. Putting `reviewable: true` in frontmatter implies kerf reads and uses it. It does not — the agent reads the markdown body. Keeping review instructions in the markdown body is consistent with jig-system.md's principle: "All machine-readable data lives in the frontmatter. Agent instructions live in the markdown body."

**Each jig's markdown body includes per-pass review criteria.** Example for the plan jig's Change Spec pass:

```markdown
## Review: Change Spec

After completing all component specs, spawn a review sub-agent with:
- All files in 05-specs/
- The 03-components.md requirements document
- The relevant 04-research/ findings

The reviewer checks:
- Every requirement from 03-components.md has a corresponding spec section
- No spec content exists that isn't backed by a requirement
- Acceptance criteria are concrete and testable
- Files & Changes sections reference real paths in the codebase
- Verification steps are runnable

Up to 3 review rounds. After that, present artifacts + any remaining
findings to the user for approval.
```

### 5. Aliases (jig-system.md addition)

Add an `aliases` field to the jig frontmatter schema:

```yaml
aliases:              # Optional. List of alternative names that resolve to this jig.
  - <string>
```

**Resolution order** (updated from current spec): When resolving a jig by name, kerf checks:

1. **User-level jig by filename** — `~/.kerf/jigs/{name}.md`
2. **Built-in jig by filename** — shipped with binary, matched by `name` field
3. **Built-in jig by alias** — scan built-in jigs' `aliases` fields for a match

The first match wins. This means a user-level jig named `feature` takes priority over the built-in `plan` jig's `feature` alias. Aliases are only checked on built-in jigs — user-level jigs do not support aliases.

**Collision rules:** If two built-in jigs claim the same alias, that is a build-time error (won't happen with kerf's own built-ins, but relevant for validation). User-level filename always wins over any alias.

**Canonical name recording:** When a jig resolves via alias, spec.yaml records the **canonical name** (the jig's `name` field), not the alias. Example: `kerf new --jig feature` resolves to the `plan` jig → spec.yaml gets `jig: plan`. This ensures jig version checks and resolution work correctly on subsequent commands.

**Display:** `kerf jig list` shows canonical names. If a jig has aliases, they appear in parentheses: `plan (also: feature)`.

### 6. Resumability (jig-system.md addition)

Add a cross-cutting rule: **every pass MUST save its artifacts to disk before the pass status advances.** This is non-negotiable. It ensures:

- Works are resumable across sessions (`kerf resume` re-reads artifacts from disk)
- Context compaction does not lose work (artifacts are on disk, not just in context)
- Sub-agent reviews have files to read (not just context window contents)

If an agent is compacted mid-pass, it re-reads the work directory to restore context and continues from where it left off. The numbered file structure (`01-`, `02-`, ...) shows exactly which passes are complete.

### 7. First-Run Onboarding (commands.md changes)

Update `kerf new` behavior when `default_jig` is not configured:

**When `default_jig` is unset AND no `--jig` flag is provided**, `kerf new` fails with:

```
Error: No default workflow configured.

How do you want to use kerf?

  kerf config default_jig plan
    Write a plan before changing code. Best for existing projects.
    You describe what to change → kerf guides you through planning →
    you get an implementation-ready spec and task list.

  kerf config default_jig spec
    Maintain a living spec that defines your system. Best for new projects.
    The spec is always right. Code that doesn't match the spec is wrong.
    Changes start as spec updates, then flow to code.

Or specify for just this work:  kerf new my-feature --jig plan
```

This is not interactive. It is an error with actionable instructions. An agent can parse the output and run the appropriate `kerf config` command. A human can read and choose.

**`default_jig` defaults to unset** in a fresh config. After the user sets it (or uses `--jig`), subsequent `kerf new` commands work without the onboarding message.

### 8. Config Changes (architecture.md)

Add to config.yaml schema:

- **`spec_path`** — Path relative to repo root where system specs live. Used by the spec-first jig at finalization to know where to copy drafted spec files. Default: `specs/`. Only meaningful for spec-first projects. If the directory does not exist at finalization time, kerf creates it. Per-project scoping is out of scope for v1 — users with multiple projects at different spec paths should set this before running `kerf finalize`.

- **`default_jig`** — Changed from defaulting to `feature` to defaulting to unset. When unset, `kerf new` without `--jig` emits the onboarding error message (see Section 6).

### 9. Finalization Changes (finalization.md, commands.md)

Update finalization behavior for spec-first works:

Standard finalization (per existing spec) copies work artifacts into the repo at `finalize.repo_spec_path`. This continues to work for plan-first and bug works.

For spec-first works (detected by `jig: spec` in spec.yaml), finalization additionally:
1. Reads the `spec_path` config value (default: `specs/`)
2. If `{repo_root}/{spec_path}/` does not exist, creates it
3. Copies files from the work's `05-spec-drafts/` directory to `{repo_root}/{spec_path}/`, preserving filenames (1:1 mapping — `05-spec-drafts/jig-system.md` → `specs/jig-system.md`)
4. **Excludes** `05-spec-drafts/` from the standard artifact copy (so spec files appear only in `spec_path`, not duplicated in `repo_spec_path`)
5. If `05-spec-drafts/` is empty or missing, kerf warns but does not error (the standard artifact copy proceeds normally)
6. The finalization output shows both destinations: "Artifacts copied to {repo_spec_path}" and "Spec drafts applied to {spec_path}"

The finalization commit includes both the process record (in `repo_spec_path`) and the normative spec changes (in `spec_path`). The commit message remains `kerf: finalize {codename}`.

**Detection is by jig name, not directory presence.** Only works with `jig: spec` in spec.yaml trigger the `spec_path` copy. Custom jigs that produce `05-spec-drafts/` do not get this behavior — this is an intentional v1 limitation. Custom jigs that need similar behavior should use finalization hooks (future enhancement).

**`spec_path` vs `finalize.repo_spec_path`:** These are distinct. `repo_spec_path` is where kerf copies work process artifacts (the work directory contents, minus `05-spec-drafts/`). `spec_path` is where kerf copies drafted spec files (the normative spec changes). For a spec-first project, both are used during finalization.

**SESSION.md exclusion:** Reconcile the existing inconsistency between finalization.md and commands.md. Both should state that finalization excludes `spec.yaml`, `SESSION.md`, and `.history/` from artifact copying. (commands.md is correct; finalization.md omits `SESSION.md`.)

### 10. jig-system.md Updates

In addition to the review pattern (Section 4), aliases (Section 5), and resumability (Section 6), update jig-system.md:

- **"Each pass produces one or more files"** → change to "Each pass produces zero or more files." The `ready` pass in all three jigs has `output: []` — it is a terminal marker, not a content-producing pass.
- **Built-in jig list** — update from `feature` and `bug` to `plan`, `spec`, and `bug`.
- **Full example** — update from the defunct `feature` jig to use the `plan` jig's frontmatter and a representative subset of the markdown body.
- **Cross-references** — update references to `jig-feature.md` → `jig-plan.md`, add reference to `jig-spec.md`.

### 11. Explicitly Out of Scope

- **Follow-up tracking** — The source material (spec-skill-reference.md) includes a follow-up tracking system (Blocked/Independent/Deferred categories). This is intentionally omitted. Follow-ups are tracked externally if needed.
- **Per-project config scoping** — `spec_path` is a global config value. Projects with different spec locations must reconfigure when switching projects. Per-project config is a future enhancement.
- **Jig orchestration by kerf** — kerf does not spawn agents, manage sub-agents, or orchestrate reviews. All agent coordination is encoded in jig instructions and executed by the agent tooling, not kerf.

## Spec Files Affected

| Spec file | Change type | What changes |
|-----------|-------------|-------------|
| `specs/jig-plan.md` | **New** | Complete plan-first jig definition with frontmatter and full agent instructions |
| `specs/jig-spec.md` | **New** | Complete spec-first jig definition with frontmatter and full agent instructions |
| `specs/jig-bug.md` | **Rewrite** | Investigation workflow (6 passes). Version bumped to 2. |
| `specs/jig-system.md` | **Modify** | Add review pattern section (markdown body guidance). Add resumability rule. Update built-in jig list to plan/spec/bug. Add `aliases` frontmatter field. Relax "one or more files" to "zero or more." Update full example to `plan` jig. Update cross-references. |
| `specs/commands.md` | **Modify** | Update `kerf new` for onboarding error flow. Update `--jig` default description. Add `spec_path` to `kerf config` known keys. Update `kerf finalize` behavior for spec-first works (spec_path copy step, dual-destination output). |
| `specs/architecture.md` | **Modify** | Add `spec_path` config field. Change `default_jig` default to unset. Document `spec_path` vs `repo_spec_path` distinction. |
| `specs/works.md` | **Modify** | Update built-in types to `plan`, `spec`, `bug`. Update example status progressions. |
| `specs/verification.md` | **Modify** | Clarify that square checks work-directory artifacts only. No changes needed for spec-first (drafted specs are in work dir). |
| `specs/finalization.md` | **Modify** | Add spec-first finalization behavior: copy `05-spec-drafts/` to `spec_path`. Document `spec_path` vs `repo_spec_path`. |
| `specs/_index.md` | **Modify** | Update spec map: remove jig-feature, add jig-plan and jig-spec. |
| `specs/jig-feature.md` | **Remove** | Replaced by jig-plan.md. `feature` retained as alias in jig resolution. |
