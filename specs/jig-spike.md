# Spike Jig

> Built-in jig for structured exploration when the approach is unknown.

This spec defines the `spike` jig that ships with kerf. It guides agents through structured exploration — investigating options, iterating on approaches, and converging on an answer before committing to a spec or plan. The core principle is freedom during exploration, discipline at convergence. See [jig-system.md](jig-system.md) for jig file format, resolution, versioning, and the review pattern. See [jig-plan.md](jig-plan.md) for planning changes to existing codebases. See [jig-retrofit.md](jig-retrofit.md) for reconciling code and specs after the fact.

## When To Use

The `spike` jig applies when:

- The approach is unknown and must be discovered through experimentation
- Multiple options exist and need to be tried before one can be chosen (e.g., selecting a library, validating a technical constraint, benchmarking approaches)
- A problem requires iterative exploration before a spec makes sense
- An implementation or test session went sideways and the tangent needs to be captured

It does not apply to:

- Changes where the solution is already understood (use the [`plan`](jig-plan.md) or [`spec`](jig-spec.md) jig)
- Defects with a clear reproduction path (use the [`bug`](jig-bug.md) jig)
- Code that diverged from specs without exploration context (use the [`retrofit`](jig-retrofit.md) jig)

**Spike vs. Retrofit:** A spike has rich context — options tried, why things failed, what was learned. A retrofit is primarily "code diverged from spec, sync them." A spike that is already resolved may use the retrofit jig for the sync step.

## Two Entry Points

### Entry A — Intentional Spike

You know upfront that exploration is needed. You start a spike work before writing any code. The question or constraint is stated, and the agent explores freely — trying options, changing code, benchmarking, iterating. When a winning approach emerges, the agent transitions to convergence.

### Entry B — Accidental Spike (Mid-Work Tangent)

You were implementing or testing, it went sideways. The agent started debugging, you went on a tangent, tried multiple approaches. Eventually you figured it out. Now you need to capture what happened and get everything synced back up.

Entry B starts partway through the jig. The exploration already happened — the Frame and Explore passes capture what occurred retroactively, then Converge produces the structured exploration log from the agent's memory of what was tried.

## Status Progression

```
frame -> explore -> converge -> align -> squared
```

## Frontmatter

```yaml
---
name: spike
description: Structured exploration when the approach is unknown.
version: 1
phase: exploration
aliases: [explore, investigation]
tools: []
composable: false
status_values:
  - frame
  - explore
  - converge
  - align
  - squared
passes:
  - name: "Frame"
    status: frame
    output: ["01-frame.md"]
  - name: "Explore"
    status: explore
    output: ["02-explore.md"]
  - name: "Converge"
    status: converge
    output: ["03-exploration-log.md"]
  - name: "Align"
    status: align
    output: ["04-alignment.md"]
  - name: "Square"
    status: squared
    output: []
file_structure:
  - spec.yaml
  - SESSION.md
  - 01-frame.md
  - 02-explore.md
  - 03-exploration-log.md
  - 04-alignment.md
---
```

## Passes

### Pass 1: Frame (frame)

**Output:** `01-frame.md`

Define the question, constraint, or problem being explored. Establish what "answered" looks like so the exploration has a target.

#### Agent Instructions

You are framing an exploration. The output is a clear statement of what you are trying to figure out and how you will know when you have an answer.

**Entry A — Intentional spike:**

1. Ask the user (or read source material) to understand what needs to be explored. What is the question? What constraint must be validated? What decision must be made?
2. State the question or constraint precisely. "Which embedding model runs on Apple Silicon GPU with acceptable latency?" not "figure out embeddings."
3. Identify known constraints — hardware limitations, performance requirements, compatibility needs, time budget for the exploration.
4. Define what "answered" looks like. What would a successful exploration produce? This does not need to be as rigorous as plan-jig success criteria — a spike's success criteria are directional. Examples: "We know which library to use and have a working prototype" or "We have benchmark numbers for three approaches."
5. List candidate approaches if known. The agent may already have ideas about what to try. Listing them upfront gives the exploration direction, but the list is not binding — new options may emerge.
6. **Save to disk:** Write `01-frame.md`. Advance status to `explore`.

**Entry B — Accidental spike (mid-work tangent):**

The exploration already happened. You are reconstructing the frame after the fact.

1. State what you were originally doing when the tangent started. What work were you in? What triggered the detour?
2. Identify the question that emerged. What did the tangent actually explore? This may differ from the original task.
3. Capture the constraints that were discovered during the tangent — these were not known upfront.
4. Define what "answered" looks like, now that you have the answer (or partial answer). This is retrospective — you are framing an exploration that already converged.
5. **Save to disk:** Write `01-frame.md`. Advance status to `explore`.

**What done looks like:**

- `01-frame.md` exists and contains:
  - The question or constraint being explored, stated precisely
  - Known constraints and limitations
  - What "answered" looks like (directional success criteria)
  - Candidate approaches (if known)
  - For Entry B: what triggered the tangent and what the original work was

### Pass 2: Explore (explore)

**Output:** `02-explore.md`

Freeform exploration. Try things. The jig does not constrain how you explore — this is the deliberately loose phase. The only requirement is that you keep notes on what you try.

#### Agent Instructions

You are exploring. There is no rigid structure here. Try things, change code, benchmark, prototype, read documentation, test assumptions. The point of a spike is that you do not yet know the right approach, so the jig gives you room to discover it.

**Entry A — Intentional spike:**

1. Read `01-frame.md` to ground yourself in the question and constraints.
2. Explore freely. Try the candidate approaches listed in the frame, or discover new ones. There is no required order or structure.
3. **Keep notes as you go.** Update `02-explore.md` periodically with what you are trying, what you observe, and whether each attempt succeeds, fails, or is inconclusive. These notes do not need to be polished — they are a working log.
   - What are you trying right now?
   - What happened? (error messages, benchmark numbers, observed behavior)
   - Does this approach look viable? Why or why not?
4. Do not prematurely commit to an approach. Try multiple options before deciding. If the first thing you try works, consider trying at least one alternative to validate the choice.
5. It is fine to change code, create throwaway prototypes, install dependencies, and generally make a mess. The Align pass will clean things up.
6. When you believe you have enough information to answer the question from `01-frame.md`, stop exploring. You do not need to exhaust all options — you need enough evidence to make a decision.
7. **Save to disk:** Write or update `02-explore.md` with your final exploration notes. Advance status to `converge`.

**Entry B — Accidental spike (mid-work tangent):**

The exploration already happened. You are capturing what occurred.

1. Read `01-frame.md` to recall the question that emerged.
2. Write `02-explore.md` as a retrospective log of what was tried during the tangent. Work from memory and from any code changes, git history, or artifacts that were produced.
3. Be honest about what you remember and what you are inferring. "I tried X and it failed with error Y" is better than "X was attempted." If you do not remember specifics, say so.
4. **Save to disk:** Write `02-explore.md`. Advance status to `converge`.

**What done looks like:**

- `02-explore.md` exists and contains notes on what was tried during exploration
- Each attempt is noted with its outcome (success, failure, partial, inconclusive)
- There is enough raw material for the Converge pass to produce a structured exploration log

### Pass 3: Converge (converge)

**Output:** `03-exploration-log.md`

The freeform phase is over. Now impose structure. Produce the exploration log — the key artifact of the spike jig. This is where the knowledge that would otherwise be lost when the session ends gets captured permanently.

#### Agent Instructions

You are converting raw exploration notes into a structured record of what was learned. This is the most important artifact the spike produces — it captures the knowledge for future agents and humans.

**What to do:**

1. Read `01-frame.md` (the question) and `02-explore.md` (the raw exploration notes).
2. Produce `03-exploration-log.md` with the following sections:

   **## Question**
   Restate the question or constraint from the frame. One or two sentences.

   **## Options Explored**
   For each approach that was tried:
   - **Name/description** of the approach
   - **Outcome:** pass, fail, or partial
   - **Evidence:** what happened — error messages, benchmark numbers, observed behavior, code references
   - **Notes:** anything non-obvious or surprising about this approach

   **## Decision**
   Which approach won and why. Reference the evidence from the options explored. If no clear winner emerged, state that and explain what additional information is needed.

   **## Constraints Discovered**
   Things learned during exploration that were not known at the start. These are valuable even if the spike did not produce a clear winner — they narrow the solution space for future work. Examples:
   - "Library X does not support Apple Silicon GPU acceleration"
   - "The API rate-limits to 100 requests/minute, not 1000 as documented"
   - "The existing parser cannot handle nested expressions without a rewrite"

   **## Surprises**
   Anything unexpected or non-obvious that came up during exploration. Things a future agent or developer would benefit from knowing.

3. Be concrete throughout. Quote error messages. Include benchmark numbers. Reference specific files, libraries, or APIs. The exploration log is evidence-based, not opinion-based.
4. **Save to disk:** Write `03-exploration-log.md`. Advance status to `align`.

**What done looks like:**

- `03-exploration-log.md` exists and contains all sections listed above
- Each option explored has a clear outcome with evidence
- The decision (or lack of decision) is stated with rationale
- Constraints discovered and surprises are captured
- A future agent reading this document understands what was tried, what worked, and why

### Pass 4: Align (align)

**Output:** `04-alignment.md`

Sync specs and code based on what the spike discovered. Create or feed into a plan work for the chosen approach. Clean up any mess from the exploration phase.

#### Agent Instructions

You are transitioning from exploration to structured work. The spike answered a question — now that answer needs to flow into the project's specs and plans.

**What to do:**

1. Read `03-exploration-log.md` to understand the decision and constraints discovered.
2. Assess what needs to happen next. Common outcomes:
   - **The spike produced a clear winner.** Create a plan work (`kerf new --jig plan`) that uses the spike's findings as input. Record the dependency: the new plan work depends on this spike work.
   - **The spike narrowed options but did not decide.** Document what is still unknown and what the next spike or investigation should focus on.
   - **The spike revealed the original approach was wrong.** Document the pivot and what the new direction is.
3. Check for code changes made during exploration:
   - If prototype code was written that should be kept: note it in `04-alignment.md` so the plan work knows what exists.
   - If throwaway code was written: note that it should be cleaned up. Do not clean it up yourself in this pass unless it is trivial — the plan/implementation work handles that.
   - If specs were affected by what was learned: note which specs need updating. Do not update specs directly from the spike — that is the plan work's job (or the retrofit jig's job if code already changed).
4. Write `04-alignment.md` with:
   - **Next steps:** what work follows this spike (plan, another spike, retrofit, etc.)
   - **Dependencies created:** any new works and their relationship to this spike
   - **Code state:** what code was changed during exploration, what to keep, what to discard
   - **Spec impact:** which specs need updating based on the spike's findings
5. If creating a follow-on work, use `kerf new` with the appropriate jig and set the dependency on this spike work.
6. **Save to disk:** Write `04-alignment.md`. Advance status to `squared`.

**What done looks like:**

- `04-alignment.md` exists and contains next steps, dependencies, code state, and spec impact
- Follow-on works have been created (or the document explains why none are needed)
- The spike's findings have a clear path into the project's spec/plan workflow
- No orphaned knowledge — everything learned in the spike is either in the exploration log or feeding into a downstream work

### Pass 5: Square (squared)

**Output:** (none)

The work is complete. This is a terminal pass — it produces no files.

#### Agent Instructions

**What to do:**

1. Run `kerf square <codename>` to verify all expected artifacts exist on disk.
2. If square fails, identify which artifacts are missing and return to the appropriate pass to produce them.
3. If square passes, the spike is complete. The exploration log is preserved, the findings have been aligned into downstream work, and the spike can be archived.

**What done looks like:**

- `kerf square` passes with no errors
- All 4 artifact files exist on disk (`01-frame.md`, `02-explore.md`, `03-exploration-log.md`, `04-alignment.md`)
- The spike's knowledge is captured and connected to downstream work

## File Structure

A work governed by the `spike` jig contains these files:

```
{codename}/
  spec.yaml
  SESSION.md
  01-frame.md
  02-explore.md
  03-exploration-log.md
  04-alignment.md
```

`spec.yaml` and `SESSION.md` are managed by kerf (see [works.md](works.md) and [sessions.md](sessions.md)). All other files are produced by the passes defined above.

## Finalization

When the work reaches `squared` status and `kerf square` passes, the work is eligible for [finalization](finalization.md). `kerf finalize <codename>` packages the work for archival.

The spike's primary lasting artifact is `03-exploration-log.md` — the structured record of what was explored and decided. Downstream plan or implementation works reference the spike work via dependencies and build on its findings. After downstream work is complete, the spike work can be archived — its knowledge lives in the exploration log and in the specs/plans it informed.
