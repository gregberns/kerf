# Implementation Tasks — Plan 002

> Spec-change tasks for the jig redesign. Each task produces updated or new spec files.
> These are spec-writing tasks, not code changes.
> Revised after 3-agent review (completeness, dependencies, implementability).

## Dependency Graph

```
T0 (jig-system.md)
 └─► T1a (jig-plan.md)      ─┐
 └─► T1b (jig-spec.md)       ├─► T2 (supporting spec updates) ─► T3 (cross-spec review)
 └─► T1c (jig-bug.md)       ─┘
```

## Structural Conventions

All three jig specs (T1a, T1b, T1c) must follow the same per-pass markdown structure for consistency. Use the existing `jig-feature.md` as a format reference:

- **"When to Use" section** after the overview — describes who this jig is for and when to choose it
- **Per-pass sections** with this structure:
  ```markdown
  ## Pass N: {Name} ({status})

  **Output:** `{artifact-file.md}`

  {1-2 sentence summary of this pass's goal.}

  ### Agent Instructions

  **What to do:**
  1. {Numbered steps}
  2. ...

  **What done looks like:**
  - {Concrete criteria}

  ### Review Criteria (if reviewable)

  After completing this pass, spawn a review sub-agent with:
  - {files to provide}

  The reviewer checks:
  - {specific checks}

  Up to 3 review rounds. {Escalation behavior.}
  ```
- **Finalization section** at the end describing what happens after Ready
- Review criteria are inline within each reviewable pass (not a separate appendix)

---

## Tasks

### T0 — Update jig-system.md

**Source:** Plan Section 4 (Review Pattern), Section 5 (Aliases), Section 6 (Resumability), Section 10 (jig-system.md Updates)

**Changes to `specs/jig-system.md`:**

1. **Add `aliases` field to frontmatter schema:**
   ```yaml
   aliases:              # Optional. List of alternative names that resolve to this jig.
     - <string>
   ```
   Add after the `version` field in the schema block.

2. **Update resolution order** (§Resolution Order):
   Current: (1) user-level by filename, (2) built-in by filename.
   New: (1) user-level by filename, (2) built-in by filename, (3) built-in by alias.
   Add: "Aliases are only checked on built-in jigs. User-level filename always wins over any alias." Add collision rule: "If two built-in jigs claim the same alias, it is a build-time error."
   Add: "When a jig resolves via alias, spec.yaml records the canonical name (the jig's `name` field), not the alias."

3. **Add Review Pattern section** (new section between §Customization and §Design Principles):
   - Review is agent-driven, encoded in jig markdown body, not kerf runtime
   - Describe the review loop: complete artifacts → spawn sub-agent → findings to `{pass-name}-review.md` → original agent fixes → re-review → up to 3 rounds → escalate to human
   - Autonomous mode: if sub-agent approves, advance. If unresolved after max rounds, advance with findings saved, don't block.
   - "kerf does not orchestrate reviews. kerf tracks pass status. The jig's markdown body tells the agent how to conduct reviews."
   - Explain why not frontmatter: "Review semantics are process guidance, not machine-readable data."

4. **Add Resumability section** (new section after Review Pattern):
   - "Every pass MUST save its artifacts to disk before the pass status advances."
   - Compaction recovery: re-read work directory, numbered files show which passes are complete.

5. **Relax pass output constraint** (§Passes AND §Design Principles):
   - §Passes: Change "Each pass produces one or more files" to "Each pass produces zero or more files."
   - §Design Principles, principle #2: Update "Each pass produces a file" to "Each content pass produces a file. Terminal passes (e.g., `ready`) may produce no files."
   - Note: the `ready` pass in all built-in jigs produces no files — it is a terminal marker.

6. **Update built-in jig list** (opening section):
   - Replace `feature` and `bug` with `plan`, `spec`, and `bug`.
   - Update descriptions to match plan.

7. **Update full example** (§Full Example):
   - Replace the `feature` jig example with the `plan` jig's full frontmatter.
   - For the markdown body portion, show the overview + one fully abbreviated pass + one pass with review criteria as illustration. Use `[Detailed agent instructions for this pass]` placeholders for other passes. Do NOT paste the entire jig body into the example.

8. **Update cross-references:**
   - Replace `jig-feature.md` references with `jig-plan.md`.
   - Add reference to `jig-spec.md`.

**Review:** Sub-agent review against the plan's Sections 4, 5, 6, 10 to verify all changes are captured accurately. Specifically check that Design Principle #2 is updated (not just §Passes).

---

### T1a — Write jig-plan.md

**Source:** Plan Section 1 (Plan-First Jig)
**Depends on:** T0 (aliases field, review pattern, resumability must be defined)

**Create `specs/jig-plan.md`:**

Complete plan-first jig definition including:

1. **Frontmatter** — copy exactly from plan Section 1 (name, description, version, aliases, status_values, 8 passes with outputs, file_structure).

2. **Markdown body** — full agent instructions for each pass. Follow the structural conventions defined above (When to Use, per-pass Agent Instructions with What-to-do/What-done-looks-like, inline Review Criteria):
   - **When to Use section** — who this jig is for (existing codebases, plan-before-code workflow)
   - **Per-pass sections** (8 total): Problem Space, Analyze, Decompose, Research, Change Spec, Integration, Tasks, Ready
   - Each pass section includes: goal, detailed instructions, guided vs autonomous mode (where applicable), artifact template/format, what "done" looks like
   - **Inline review criteria** for reviewable passes (Decompose, Change Spec, Integration, Tasks): what the review sub-agent checks, what files to provide, escalation behavior
   - **Finalization section**: standard finalization, work is self-contained

3. **Key patterns to encode in instructions:**
   - One component at a time in guided mode, track progress ("Component 3/5")
   - Research delegation to sub-agent with 3-5 specific questions
   - Concrete over vague — "returns 404 with error body" not "handles errors"
   - Save to disk after completing each pass (resumability)
   - Autonomous default when no user present
   - Follow the source material (plans/002_jig_redesign/source/spec-skill-reference.md) for instruction depth and tone

**Stage deletion of `specs/jig-feature.md`** — remove the file. T3 will verify no dangling references remain.

**Review:** Sub-agent review comparing the new spec against plan Section 1 and the source material. Check: every pass detail from the plan is captured, instructions are complete enough for a new agent to follow, review criteria are specific and actionable, structural conventions are followed consistently.

---

### T1b — Write jig-spec.md

**Source:** Plan Section 2 (Spec-First Jig)
**Depends on:** T0

**Create `specs/jig-spec.md`:**

Complete spec-first jig definition including:

1. **Frontmatter** — from plan Section 2 (name, description, version, status_values, 8 passes, file_structure). **Important:** In the Spec Draft pass, `{component}` expands to target spec filenames (e.g., `jig-system`, `commands`), NOT to decomposed feature components. This is different from other jigs' use of `{component}`.

2. **Markdown body** — full agent instructions. Follow structural conventions (When to Use, per-pass sections, inline review criteria):
   - **When to Use** — who this is for (greenfield projects, teams maintaining living specs, spec-is-source-of-truth)
   - **Per-pass sections** (8 total): Problem Space, Decompose, Research, Change Design, Spec Draft, Integration, Tasks, Ready
   - **Spec Draft pass** is the most critical section. Must clearly explain:
     - Drafts are named to match target spec files: `05-spec-drafts/jig-system.md` maps to `specs/jig-system.md`
     - For NEW spec files: use the intended filename (e.g., if creating `specs/jig-spec.md`, the draft is `05-spec-drafts/jig-spec.md`)
     - Changelog (`05-changelog.md`) format: for each spec file, list what changed/added/removed and which Change Design document drove the change
   - **Review criteria** for reviewable passes (Decompose, Change Design, Spec Draft, Integration, Tasks)
   - **Finalization section**: explain dual-destination finalization — process artifacts to `repo_spec_path`, spec drafts to `spec_path`. Reference finalization.md.
   - Explain `spec_path` config requirement

3. **Key patterns:**
   - Decompose identifies WHICH specs are affected and WHAT requirements (scope of change)
   - Change Design specifies HOW each spec changes (current state → target state, rationale)
   - Drafts stay in work directory until finalization (bench/repo boundary)
   - Integration checks ALL system specs, not just modified ones

**Review:** Sub-agent review comparing against plan Section 2. Verify: Spec Draft naming convention is unambiguous with worked examples for both existing and new files, changelog format is specified, finalization behavior is consistent with plan Section 9.

---

### T1c — Rewrite jig-bug.md

**Source:** Plan Section 3 (Bug Jig)
**Depends on:** T0

**Rewrite `specs/jig-bug.md`:**

1. **Frontmatter** — from plan Section 3. Version bumped to 2. 6 passes, 6 status values.

2. **Markdown body** — full agent instructions. Follow structural conventions:
   - **When to Use** — for investigating and specifying fixes for defects
   - **Per-pass sections** (6 total): Report, Research, Reproduce, Root Cause, Fix Spec, Ready
   - **Research pass** must structure investigation around 3-5 explicit questions (e.g., "What code handles this path?", "When was this last modified?", "Are there related issues?")
   - **Inline review criteria** for reviewable passes (Root Cause, Fix Spec)
   - **Ready section**: kerf's job ends here, implementation and verification are external

3. **Cross-references:** Update to reference `jig-plan.md` and `jig-spec.md` instead of `jig-feature.md`.

**Review:** Sub-agent review. Walk through a concrete bug ("login fails when password contains special characters") and verify each pass produces useful output and the pass ordering makes sense.

---

### T2 — Update supporting specs

**Source:** Plan Sections 7-11 and affected specs table
**Depends on:** T1a, T1b, T1c (need final jig names, status values, and pass details)

Update all supporting spec files. These can be done in parallel by different workers since they touch different files, but they all depend on the jig specs being finalized.

#### T2a — Update commands.md

**Changes:**
1. `kerf new` section: add first-run onboarding error when `default_jig` is unset and no `--jig` flag. Copy error message exactly from plan Section 7. Update `--jig` flag default from `feature` to `(required if default_jig unset)`.
2. `kerf finalize` section: add spec-first finalization step (detect `jig: spec` in spec.yaml, copy `05-spec-drafts/` to `spec_path`, show dual-destination output). Add behavior for empty/missing `05-spec-drafts/` (warn, don't error). **Cross-reference with T2d** — finalization behavior must be consistent between commands.md and finalization.md.
3. `kerf config` section: add `spec_path` to known keys list and output format.
4. `kerf jig list` section: update example output to show `plan (also: feature)`, `spec`, `bug`.

#### T2b — Update architecture.md

**Changes:**
1. Config schema: add `spec_path` field (string, default `specs/`, relative to repo root). Document: only meaningful for spec-first, kerf creates if missing at finalization. Add v1 limitation: "Per-project config scoping is not supported. Users working across multiple projects with different spec paths should set `spec_path` before running `kerf finalize`."
2. Config schema: change `default_jig` default from `feature` to unset. Document onboarding behavior when unset.
3. Add note distinguishing `spec_path` (where system specs live) from `finalize.repo_spec_path` (where work artifacts go).

#### T2c — Update works.md

**Changes:**
1. §Type: update built-in types from `feature`/`bug` to `plan`/`spec`/`bug` with updated descriptions.
2. §Recommended Values: replace example status progressions with the new jig status values (plan jig and bug jig progressions).

#### T2d — Update finalization.md

**Changes:**
1. §Artifact Copying: add `SESSION.md` to the exclusion list (reconciles existing inconsistency with commands.md — commands.md already excludes it, finalization.md doesn't).
2. Add new section "Spec-First Finalization" after §Artifact Copying: for works with `jig: spec`, copy `05-spec-drafts/` to `spec_path`, exclude `05-spec-drafts/` from standard artifact copy. Document: detection by jig name (intentional v1 limitation — custom jigs don't get this behavior), empty/missing dir warning, `spec_path` directory creation.
3. Document `spec_path` vs `repo_spec_path` distinction.
4. **Cross-reference with T2a** — finalization behavior must be consistent between finalization.md and commands.md.

#### T2e — Update verification.md + _index.md

**Changes:**
1. `specs/verification.md`: Add clarifying note that square checks work-directory artifacts only. No special behavior for spec-first works — drafted specs live in the work directory and are checked like any other file.
2. `specs/_index.md`: §Spec Map → Jigs section: remove `jig-feature.md`, add `jig-plan.md` and `jig-spec.md` with descriptions.

(Combined from the original T2e + T2f — both are trivially small single-sentence changes.)

**Review (all T2 tasks):** Sub-agent review of all modified specs together. Cross-reference check: do all internal links resolve? Do status value examples match the actual jig definitions? Do commands reference the correct config keys? Is finalization behavior consistent between commands.md and finalization.md?

---

### T3 — Cross-spec consistency review

**Depends on:** T2

Final consistency pass across ALL specs (not just modified ones). This is the Integration step from the spec-first jig's own workflow.

**Checks (produce a pass/fail with evidence for each):**

1. **Internal links:** For every `[text](file.md)` link in every spec file, verify the target file exists. List any broken links.

2. **Status values:** Extract `status_values` from each jig's frontmatter. Verify the same values appear in works.md §Recommended Values examples. Verify `kerf status` behavior in commands.md references valid status progressions.

3. **Config keys:** List all config keys mentioned in architecture.md's schema. Verify every key appears in commands.md's `kerf config` known keys list. Verify finalization.md references `spec_path` correctly.

4. **Alias consistency:** Verify `feature` → `plan` alias is described in: jig-system.md (resolution), jig-plan.md (frontmatter), commands.md (`kerf jig list` output), works.md (type description if applicable).

5. **File structure vs pass details:** For each jig, compare the `file_structure` frontmatter against the `output` fields of all passes. Every output file should appear in `file_structure`. Every `file_structure` entry (except `spec.yaml` and `SESSION.md`) should be an output of some pass.

6. **Finalization exclusion list:** Verify finalization.md and commands.md both exclude the same files (`spec.yaml`, `SESSION.md`, `.history/`).

7. **Component placeholder compatibility:** For each jig, list all `{component}` patterns in `file_structure`. Verify that verification.md's expansion logic (detect components from directory structure) would correctly expand each pattern. Specifically check that the spec jig's `{component}` (which means target-spec-filename) is compatible.

8. **No dangling references:** Search all spec files for any remaining reference to `jig-feature.md`. Should be zero.

**Output:** Produce `plans/002_jig_redesign/t3-review.md` listing each check, its result (pass/fail), evidence, and any fixes applied. Commit fixes directly to the spec files.

---

## Parallelization Plan

| Phase | Tasks | Workers | Depends On |
|-------|-------|---------|------------|
| 1 | T0 | 1 | — |
| 2 | T1a, T1b, T1c | 3 | Phase 1 |
| 3 | T2a, T2b+T2c, T2d+T2e | 2-3 | Phase 2 |
| 4 | T3 | 1 | Phase 3 |

**Notes:**
- T0 is the foundation — aliases, review pattern, resumability must be specced before jig files reference them.
- T1a/T1b/T1c are independent (different files) and can run in parallel.
- T1a (jig-plan.md) is the largest task — 8 passes with full instructions and review criteria. T1b and T1c are smaller; workers finishing early can pick up T2 tasks once all T1 tasks are done.
- T2 tasks touch different files and can run in parallel. Suggested grouping for 3 workers: (T2a), (T2b + T2c), (T2d + T2e). T2a is the largest (commands.md has the most changes).
- T3 is a serial review pass — one agent reads everything and checks cross-references.
- T2a and T2d both describe finalization changes to different files — they must cross-reference to stay consistent. T3 check #6 catches any drift.

## File Ownership

| Task | Files owned (write) | Files referenced (read) |
|------|-------------------|----------------------|
| T0 | `specs/jig-system.md` | Plan sections 4, 5, 6, 10 |
| T1a | `specs/jig-plan.md` (new), delete `specs/jig-feature.md` | `specs/jig-system.md`, plan section 1, source/spec-skill-reference.md |
| T1b | `specs/jig-spec.md` (new) | `specs/jig-system.md`, plan section 2 |
| T1c | `specs/jig-bug.md` | `specs/jig-system.md`, plan section 3 |
| T2a | `specs/commands.md` | Plan sections 7, 9. All jig specs for status values. |
| T2b | `specs/architecture.md` | Plan section 8 |
| T2c | `specs/works.md` | All jig specs for types and status values |
| T2d | `specs/finalization.md` | Plan section 9 |
| T2e | `specs/verification.md`, `specs/_index.md` | Jig specs for names, descriptions, `file_structure` patterns |
| T3 | All spec files (read + fix) | All of the above |
