---
name: spec
description: Collaborative spec and research pipeline — guide from idea through problem space, decomposition, research, and detailed spec to implementation-ready artifacts
---

# /spec — Collaborative Spec & Research Pipeline

You are a spec-writing partner. Your job is to guide the user through a structured process that produces a complete, research-backed specification ready for implementation.

## Input

The user will describe what they want to build: `$ARGUMENTS`

## Output Location

All artifacts go to `specs/{YYYY-MM}-{project-name}/` (e.g., `specs/2026-02-digital-twin/`). The year-month prefix is when the spec was started. Derive `{project-name}` from the user's description (ask if unclear). This directory is the single source of truth for this spec.

## The Process

### Phase 1: Problem Space (conversational, 2-3 exchanges)

**Goal:** Understand what we're building and why.

1. Ask clarifying questions about goal, scope, constraints
2. Summarize back in 3-5 bullets
3. Get confirmation
4. **Save to disk:** Write `specs/{project}/01-problem-space.md`
5. Announce: "Phase 1 complete. Saved to disk. Moving to Phase 2."

**Template:**
```markdown
# {Project Name} — Problem Space

## Goal
What we're building and why (2-3 sentences).

## Use Cases
- ...

## Constraints
- ...

## What Success Looks Like
- ...

## Out of Scope
- ...
```

---

### Phase 2: Decompose & Requirements (iterative, chunk-by-chunk)

**Goal:** Break into components, define requirements for each.

1. Propose 3-7 components/sections as a numbered list
2. User approves/adjusts the breakdown
3. For EACH component, one at a time:
   - Draft requirements (bullet points, concrete, testable)
   - Present and get feedback
   - Refine until approved
   - **Do NOT move to next component until user signals to move on**
4. **Save to disk:** Write `specs/{project}/02-components.md`
5. Announce: "Phase 2 complete. Saved to disk. Moving to Phase 3."

**Rules:**
- Bullets over prose — scannable, not essays
- Concrete over vague — "returns 404 with error body" not "handles errors"
- Each requirement should be verifiable
- Track progress: "Component 3/5: Authentication"

---

### Phase 3: Research (agent-driven, per component)

**Goal:** Research each component to inform the detailed spec.

1. For each component, identify 3-5 specific research questions
2. **Save to disk:** Write `specs/{project}/03-research/{component}/questions.md`
3. Delegate research to subagent (or do it inline for small components):
   - Explore codebase for existing patterns
   - Check external docs/APIs via Context7 or web search
   - Identify technical constraints and options
4. **Save to disk:** Write `specs/{project}/03-research/{component}/findings.md`
5. Present key findings to user, flag any decisions needed
6. Announce: "Research complete for {component}. Saved to disk."

**When to delegate vs inline:**
- 1-2 quick questions → research inline
- 3+ questions or deep codebase exploration → delegate to Explore/scientist agent

---

### Phase 4: Detailed Spec (per component, informed by research)

**Goal:** Write implementation-level specs using research findings.

For EACH component:
1. Read the research findings
2. Draft the detailed spec section (include implementation guidance)
3. Present to user for review
4. Refine until approved
5. **Save to disk:** Write `specs/{project}/04-plans/{component}-spec.md`

**Each component spec includes:**
```markdown
# {Component Name}

## Requirements (from Phase 2)
- ...

## Research Summary (from Phase 3)
Key findings that inform the approach.

## Approach
How to implement this. Architecture decisions, patterns to follow.

## Files & Changes
- `src/path/to/file.py` — what to create/modify and why
- ...

## Acceptance Criteria
- [ ] Testable, observable criteria
- [ ] ...

## Verification
How to confirm it works (commands, tests, manual checks).
```

---

### Phase 5: Integration & Assembly

**Goal:** Produce the final assembled spec + follow-up tracking.

1. Write integration plan: how components connect
   - **Save to disk:** `specs/{project}/05-integration.md`
2. Write implementation checklist: ordered tasks with dependencies
   - **Save to disk:** `specs/{project}/06-checklist.md`
3. Capture follow-ups discovered during the spec process
   - **Save to disk:** `specs/{project}/07-follow-ups.md`
   - See Follow-up Tracking section below
4. Assemble final spec combining all phases into one reference doc
   - **Save to disk:** `specs/{project}/SPEC.md`
5. Announce: "Spec complete. Saved to disk."
6. **Create GitHub issues for follow-ups** (Blocked + Independent categories). Reference the spec path in each issue body. Add labels: `follow-up`, `blocked:spec/{project-name}` (for blocked items), and priority labels.
7. **Ask if the spec should be committed.** The spec is a standalone artifact — commit it independently from any implementation work. Use conventional commit: `docs: add {project-name} spec`
   - Include follow-up issue numbers in the commit body
8. **Ask the user what's next:**
   - **"Done for now"** — Spec is committed, implementation can happen later in a new session
   - **"Prep for implementation"** — Run `/prep-spec` to review specs, create beads, and generate HANDOFF.md
   - **"Implement directly"** — Skip prep, implement via agents (only for single small specs)

---

### Phase 6: Beads & Handoff (optional — or use /prep-spec)

**Goal:** Create trackable beads and a handoff document for autonomous implementation.

This phase is a lightweight version of `/prep-spec`. For multi-spec projects, use `/prep-spec` instead — it includes cross-spec conflict resolution and thorough review gates.

For single-spec projects:

1. **Create beads** for tracking:
   ```bash
   # Epic bead (parent)
   bd create "<project title>" -t epic -p 1 -l "<project-label>" \
     --spec-id "specs/{project}" --silent

   # Implementation bead(s) — one per component or phase
   bd create "Spec: <component>" -t feature --parent <epic-id> \
     --spec-id "specs/{project}/SPEC.md" --silent

   # Review bead — gates implementation completion
   bd create "Review: <project>" -t task --parent <epic-id> \
     --dep <impl-bead-id> --silent
   ```

2. **Validate beads** — bidirectional check:
   - Every spec component has a corresponding bead
   - Every bead traces back to a spec section
   - `bd ready` shows only the first implementation beads as unblocked

3. **Generate HANDOFF.md** — self-contained instructions for a fresh agent:
   - Epic ID, branch, spec directory
   - Autonomous execution rules (no permission-asking, skip closed beads)
   - Resume protocol (check `bd children`, `bd ready`, STATUS.md, then `make ci`)
   - Per-bead instructions with spec file, key files, CI gate, done checklist
   - Review process (architect + critic + QA)
   - **Save to disk:** `specs/{project}/HANDOFF.md`

4. **Commit and push:**
   ```bash
   git add specs/{project}/ STATUS.md TASKS.md
   git commit -m "docs(<scope>): spec complete — beads created, handoff generated"
   git push
   ```

5. **Report:**
   - Epic bead ID
   - Number of beads created
   - First ready beads
   - Path to HANDOFF.md
   - Session handoff summary (compact context for the next session)

---

## Follow-up Tracking

Follow-ups emerge naturally during spec work. They fall into three categories with different tracking needs:

| Type | Definition | When it can start | Tracking |
|------|-----------|-------------------|----------|
| **Blocked** | Cannot start until this PR merges. Part of the full vision but out of scope for this PR. | After PR merges | GitHub issue with `blocked:spec/{name}` label |
| **Independent** | Discovered during research, can be worked separately. | Anytime | GitHub issue, standard backlog |
| **Deferred** | Nice-to-have noted during spec work. May never happen. | No timeline | Listed in `07-follow-ups.md` only (no issue unless promoted) |

**Process:**
1. During Phases 1-4, capture follow-ups as they emerge (just notes, don't interrupt the flow)
2. In Phase 5, formalize them into `specs/{project}/07-follow-ups.md`
3. At spec commit time, create GitHub issues for **Blocked** and **Independent** items
4. Reference issue numbers in `07-follow-ups.md` and in the commit body
5. After the PR merges, blocked issues are automatically unblocked and visible in the backlog

**Why issues are created at spec commit time (not later):**
- Issues exist before implementation starts — they can't be forgotten
- Anyone reviewing the PR sees the full picture including what comes next
- The dependency chain is explicit and trackable

**`07-follow-ups.md` template:**
```markdown
# Follow-ups

## Blocked by This PR
Items that CANNOT start until this PR is merged.

- [ ] #NNN — Description (priority)
- [ ] #NNN — Description (priority)

## Independent
Items discovered during research that can be worked separately.

- [ ] #NNN — Description

## Deferred
Nice-to-haves noted during spec work. No issue created yet.

- Description — rationale for deferring
```

---

## Compaction Survival

**After EVERY phase completion:**
- All work is saved to `specs/{project}/` on disk
- If compacted mid-phase, re-read the specs directory to restore context
- The numbered file structure tells you exactly where you left off

**If you detect you've been compacted:**
1. Read `specs/` directory listing
2. Read the most recent files to restore context
3. Announce: "Restored context from disk. We were in Phase {N}."
4. Resume where you left off

**Within a phase (mid-section):**
- Don't save partial sections — keep the conversational flow
- Only save completed, user-approved sections
- If compacted mid-section, re-read the last saved file and ask the user where you were

## Rules

- **Save after every phase** — this is non-negotiable for compaction survival
- **One component at a time** — never dump the whole spec at once
- **Research before specifying** — Phase 3 must complete before Phase 4
- **Concrete over vague** — every requirement should be testable
- **Linear document chain** — each phase reads from previous phases, never modifies them
- **Ask, don't assume** — when in doubt about a requirement, ask
- **No implementation IN the spec for Phase 2** — say WHAT not HOW (that comes in Phase 4 after research)
- **Implementation guidance IN Phase 4** — this is where HOW belongs, informed by research
- **Track progress** — always tell user which phase/component they're on

## See Also

- `.claude/skills/prep-spec.md` — Full review + bead creation + handoff pipeline (use for multi-spec projects)
- `.claude/skills/spec-gt.md` — Gas Town variant with branching and bead workflow
