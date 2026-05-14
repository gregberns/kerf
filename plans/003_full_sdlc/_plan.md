# Plan 003 — Full SDLC Jig Coverage

> STATUS: OUTLINE — reviewed by 3 agents, updated with Gas Town / ntm research

## Problem Statement

kerf's jigs currently cover the spec-writing phase well (plan, spec, bug), but the software development lifecycle doesn't end at specs. Three gaps have emerged:

1. **No retrofit path.** Agents sometimes do work (write code, make changes) before going through the spec workflow. There's no jig for capturing what happened after the fact — syncing specs to reality and creating a plan that represents the change.

2. **No exploration/spike path.** Some problems require iterative experimentation before a spec makes sense. Example: selecting an embedding model that works on Apple Silicon GPU required multiple code iterations. The current jigs assume you know what you're building. There's no structured process for "figure out the right approach, then spec it."

3. **Implementation process is ad-hoc.** The spec-to-code handoff is managed by per-project skills and CLAUDE.md instructions, not by kerf. This means the process is inconsistent across projects — some have the beads/ntm/review workflow, others don't. Jumping between projects means different (or missing) implementation processes.

## Proposed Direction

Expand jigs to cover the full SDLC. Make kerf the single source of truth for "how work gets done" in a project, not just "how specs get written."

### Prior Art: Gas Town (MEOW) and ntm

Steve Yegge's Gas Town uses a layered workflow abstraction called MEOW (Molecular Expression of Work):

| Gas Town Layer | What it is | kerf equivalent |
|---------------|------------|-----------------|
| **Formula** (TOML) | Composable workflow source with loops/gates | Jig (pass definitions with instructions) |
| **Protomolecule** | Template graph of beads with variable substitution | Assembled jig (project's selected passes) |
| **Molecule** | Instantiated workflow — live bead chain on an agent's hook | Work (active instance with pass artifacts) |
| **Beads** | Atomic tasks agents check off | Beads (via bd) |

Key Gas Town patterns to adopt:
- **Work as durable TODO list.** Workflows expressed as beads that agents check off. Agents are good at following TODO lists. State persists in Git — survives crashes, context exhaustion, restarts.
- **Recovery over prevention.** Don't try to gate agents from going off-script. Instead, make the work state durable and visible. If an agent fails, the next session picks up where it left off.
- **Templates with variable substitution.** Reusable workflow patterns (protomolecules) that get instantiated for specific tasks. Prevents ad-hoc workflow creation, enforces consistent process.

ntm already has a workflow execution engine:
- **`ntm pipeline`** — YAML workflow files with dependencies, conditionals, variable substitution, resume from saved state
- **`ntm assign`** — dependency-aware task dispatch with watch mode, integrates with beads
- **`ntm coordinator`** — session-level coordination daemon with conflict detection

**Implication for kerf:** kerf defines the process (jig passes). ntm executes it (pipeline orchestration). The dispatch pass can generate ntm pipeline YAML from the jig's pass definitions. kerf doesn't need to build a workflow engine — it needs to define workflows that ntm can execute.

---

## Section 1: New Jig Types

### 1A: Retrofit Jig (`jig-retrofit`)

**Scenario:** Code has changed without going through the spec workflow. Specs and code are out of sync.

**Entry:** `kerf new --jig retrofit` — creates a new work focused on reconciling code and specs.

**Two modes of entry:**

**Mode A — "Caught the agent" (context-rich):**
The session that did the work is still alive or recent. The agent has full context — what it changed, why, what it tried, what it learned. The retrofit captures all of this:
- What changed (diff/commits)
- Why it changed (agent explains intent)
- What was tried and rejected (learnings)
- Rationale for the final approach

**Mode B — "Found a divergence" (context-poor):**
Specs and code don't match and nobody remembers exactly why. Could be a quick fix from weeks ago, a different agent, etc. The retrofit:
- Captures what changed from the diff
- Offers light inference on why — but explicitly flags inferred rationale as inferred, not authoritative
- Does not fabricate intent. "This appears to handle X" not "This was changed to handle X"

**Outputs:**
- Retroactive plan documenting exactly what changed and why (or best inference of why)
- Updated specs synced to the current code reality
- Learnings captured (especially in Mode A — things tried, rejected approaches, constraints discovered)
- Squared result — specs and code verified in sync

**Creates:** A new work. The plan is the record of what happened; the spec updates are the sync.

**Triggering (reviewer feedback):** kerf can help detect when retrofit is needed. At `kerf list` or `kerf resume` time, if kerf detects repo changes (uncommitted changes, recent commits) that don't correspond to any active work, it can suggest: "Detected changes to X, Y, Z not tracked by any active work. Consider `kerf new --jig retrofit`." Lightweight, non-blocking.

### 1B: Spike/Exploration Jig (`jig-spike`)

**Scenario:** You don't know the answer yet. Need to try options, iterate, and figure out the right approach before a spec makes sense.

**Core principle:** Freedom during exploration, discipline at convergence. The jig is deliberately loose during investigation and only imposes structure when it's time to lock things down.

**Two entry points:**

**Entry A — Intentional spike:**
You know upfront that exploration is needed. Start a spike work, state the question/constraint, then explore freely — try options, change code, benchmark, iterate. No rigid iteration protocol. When you converge on an answer, transition to alignment: create a plan, sync specs and code.

**Entry B — Accidental spike (mid-work tangent):**
You were in the middle of something — testing, implementing — and it went sideways. The agent started debugging, you went on a tangent, tried multiple approaches. Eventually you figured it out. Now you need to capture what happened and get everything synced back up.

*Entry B overlaps with the retrofit jig. The difference: a spike has richer context (options tried, why things failed, what you learned). A retrofit is primarily "code diverged from spec, sync them." A spike that's already resolved may use the retrofit jig for the sync step.*

**Exploration phase (freeform):**
- State the question or constraint being investigated
- Try things, change code, no required structure
- The jig doesn't constrain how you explore

**Convergence phase (structured):**
- Identify the winning approach
- Produce exploration log — what was tried, what worked, what didn't, why
- Create or feed into a plan documenting the chosen approach
- Sync specs and code (may invoke retrofit mechanics)
- Square the result

**Exploration log:**
An artifact within the work that captures learnings from the spike. Not a rigid format — the point is to preserve the knowledge that would otherwise be lost when the session ends. Content like:
- What was the question / constraint
- Options tried and outcomes (pass/fail/partial, with notes)
- Why the winner was chosen
- Constraints discovered along the way (e.g., "doesn't run on Apple Silicon GPU")
- Anything surprising or non-obvious

Name TBD — "exploration log," "spike log," "decision log." The important thing is it gets captured in the work.

**Triggering (reviewer feedback):** Like retrofit, the accidental spike entry is hard for agents to self-diagnose. This is primarily user-initiated. The user recognizes "we've been debugging for an hour, let's capture this as a spike." kerf doesn't try to detect this automatically.

---

## Section 2: Composable Passes for Implementation

### Design Philosophy

**The problem:** Agents are terrible at following multi-step processes consistently. Instructions get copy-pasted across projects, go stale, and drift. Jumping between projects means different (or missing) processes.

**The solution:** A library of composable **passes** that can be assembled into jigs. Each pass encodes one well-defined process step with full agent-facing instructions. Projects compose the passes they need into an implementation jig. The "one work, one jig" invariant holds.

**Composability model (aligned with Gas Town):**
- **Pass library** = reusable process step templates (like Gas Town's protomolecule steps)
- **Assembled jig** = a project's selected passes composed into a jig (like a protomolecule)
- **Work** = an instance of that jig being executed (like a molecule)

This is consistent with how existing jigs already work — a jig has ordered passes, each with instructions and artifacts. The new thing is that passes can be drawn from a shared library rather than defined inline per jig.

### 2A: Breakdown Pass

**What it does:** Decompose a spec/plan into implementation tasks (beads).

**Process:**
- Read the spec(s) being implemented
- Break into discrete, ordered implementation units
- Identify dependencies between units
- Produce the task list (beads) ready for dispatch

**Tool:** beads (`bd`)

### 2B: Dispatch Pass

**What it does:** Spawn worker agents and dispatch implementation tasks to them.

**Process:**
- Set up a session (ntm, Kilroy, etc.)
- Dispatch beads/tasks to worker agents
- Monitor progress, poll for completion
- Handle failures and resets

**Tool:** ntm (or Kilroy, or whatever the project declares)

**ntm integration:** This pass can generate `ntm pipeline` YAML from the bead list, leveraging ntm's built-in dependency resolution, watch mode, and resume. The pass instructions encode how to use ntm correctly — the validated patterns for captures, polling, resets, file reservations.

### 2C: Implement Pass

**What it does:** Execute implementation tasks with mandatory review gates.

**Process:**
- Pick up next task/bead
- Implement it
- Review gate: verify output matches spec before proceeding
- Give feedback / iterate if needed
- Clear context, move to next task

**Key rules encoded:**
- One bead per prompt (no batching)
- Mandatory review gate (never skip)
- Clear context between beads
- Spec wins if code and spec disagree

### 2D: Review Pass

**What it does:** Review completed work against spec with a separate agent, pass feedback to the original agent.

**Process:**
- Spin up a review agent (or use the controller)
- Compare output to spec
- Pass feedback to the implementing agent if needed
- Approve or request changes

**Tool:** ntm / agent-mail

### Composability in Practice

A project assembles its implementation jig from available passes. For Greg's current process:

```
jig-implementation (assembled from pass library):
  Pass 1: Breakdown    (tool: bd)
  Pass 2: Dispatch     (tool: ntm)
  Pass 3: Implement    (tool: per-bead execution)
  Pass 4: Review       (tool: ntm / agent-mail)
```

A simpler project might use:

```
jig-implementation (simpler):
  Pass 1: Breakdown    (tool: bd)
  Pass 2: Implement    (tool: single agent, no dispatch)
```

The agent enters via `kerf new --jig implementation`. The jig's passes are ordered. The agent follows them in sequence — same as existing spec-writing jigs.

**The portability problem this solves:**
You refined the ntm dispatch process in one project's skill. You switched projects — the skill was gone. Passes fix this: the process lives in kerf's pass library, not in any one project. Update it once, every project that includes that pass gets the improvement.

---

## Section 3: Jig Metadata, Instructions & Discovery

### 3A: Jig Metadata Expansion

Currently jigs define passes and artifacts. Expand with:

- **`description`** — already exists in the spec; new jigs will use it. Confirm it's sufficient for the expanded jig types.
- **`phase`** — Where in the SDLC this jig applies (planning, implementation, testing, bug-fix, etc.). New field.
- **`tools`** — External tools this jig's passes use (bd, ntm, agent-mail, Kilroy, etc.). Informational — declared so agents know what they need. kerf does not verify tool availability (that's the agent's/user's job). New field.

**Note on `depends_on`:** With composable passes resolved as passes-within-a-jig (not separate jigs), jig-level `depends_on` is unnecessary. Pass ordering within a jig is already explicit. Work-level `depends_on` (work-to-work dependency) already exists in the spec and handles the SDLC chaining (spike-work → plan-work → impl-work). No new dependency mechanism needed.

### 3B: Jig Instructions — The Jig Contains the Process

The jig itself contains the full agent-facing instructions for how to execute the process. Not a pointer to external docs. Not a reference to a skill. The jig IS the single source of truth.

This is the core of the portability fix. Today the ntm process lives in a project-specific skill or CLAUDE.md section. When you switch projects, it's gone. When you improve it, only one project benefits.

With this change:
- The jig contains the complete instructions (how to use bd to create beads, how to spawn ntm workers, how to poll, how to do reviews, etc.)
- `kerf` serves these instructions to agents when the project activates the jig
- Update the jig once → every project that uses it gets the update

The existing jig format already has pass-level prompts. The new jigs extend this — some will have more substantial instructions (the ntm coordination patterns, for example). Same mechanism, bigger content.

### 3C: Jig Display & Discovery

- `kerf jig list` shows available jigs with descriptions, phases, tool requirements
- Agents can query available jigs and understand what process to follow for this project
- Jigs grouped/filtered by phase
- Shows which jigs the current project has activated vs. what's available in the library

**Live discovery (reviewer feedback):** Don't rely solely on CLAUDE.md for process instructions — it goes stale. The `kerf` no-args command and `kerf resume` should also emit the active jig chain for the project. The agent gets process information from a live source at the moment it needs it, not from a cached document.

---

## Section 4: Project-Level Jig Configuration

### 4A: Project Jig Selection

The project declares which jigs it uses from kerf's library. Not all projects need all jigs.

- Stored in kerf's project config (established via `kerf init`)
- The jig library is global — ships with kerf or lives in the bench
- The project picks which jigs are active for this project
- For composable jigs (like implementation), the project also selects which passes to include
- Example: Project A activates `plan`, `implementation` (with breakdown+dispatch+implement+review passes), `spike`. Project B activates `plan`, `implementation` (with breakdown+implement passes only).

**Note on architecture (reviewer feedback):** The current `config.yaml` is bench-wide, not per-project. Per-project jig activation requires either a per-project config mechanism or extending the project identifier file. This is a spec change to `architecture.md`.

### 4B: Agent Setup / AGENTS File Integration

**The key UX:** When an agent starts working in a project, kerf tells it the correct processes for this project.

**Mechanism:**
- `kerf setup` generates the agent-facing instructions from the project's active jigs
- kerf **emits** the content; the **agent applies** it to CLAUDE.md / AGENTS.md
- kerf does not own or directly write to CLAUDE.md — that file has project-specific content kerf shouldn't touch
- This follows the existing CLI pattern: kerf emits next-steps, the agent acts on them
- This is consistent with the existing kerf invariant: "kerf never launches or manages agent sessions. It reads/writes data and emits context."

**Relationship to `kerf init` (reviewer feedback):** `kerf init` is one-time project bootstrap. `kerf setup` is re-runnable — generates fresh agent config whenever jigs are updated. `kerf init` calls `kerf setup` as part of its flow. `kerf setup` can be run independently to refresh stale config.

**Update flow:**
- When jigs are updated in the library, run `kerf setup` again to get fresh instructions
- The agent diffs the new output against what's in CLAUDE.md and applies changes
- This is explicit and auditable — no silent background updates to agent config

**What the generated content includes:**
- Process instructions from each active jig (the full agent-facing process)
- Tool requirements (what needs to be installed/available)
- Jig sequencing (the composition chain for this project)
- References back to kerf commands for each phase

**Decided:** kerf generates, agent applies. Kerf never writes to CLAUDE.md directly.

---

## Section 5: Jig Lifecycle & Sequencing

### 5A: Jig Chaining / Workflows

Jigs naturally sequence. Work-level dependencies (already spec'd in `dependencies.md`) declare the chain:

```
spike-work → plan-work → impl-work
               ↑
          retrofit-work
```

This is **guidance, not a gate** — consistent with the existing kerf principle. Kerf shows the recommended chain and warns if you skip a step, but doesn't block. The agent/user decides.

Pass ordering within a jig is explicit — passes are numbered/ordered in the jig definition, same as existing jigs.

### 5B: Work Model — Separate Works, Linked by Dependencies

One work, one jig. A piece of work that flows through the full SDLC produces multiple linked works:

```
spike-work (jig-spike)
  → plan-work (jig-plan, depends_on: spike-work)
    → impl-work (jig-implementation, depends_on: plan-work)
```

**Why separate works, not one evolving work:**
- Consistent with the existing work/dependency model — no new concepts
- A spike and a plan are genuinely different artifacts with different outputs
- Clean boundaries — each work has one jig, one set of artifacts
- The dependency chain is already a first-class concept in kerf
- Keeps the "one work, one jig" invariant intact

Implementation passes (breakdown, dispatch, implement, review) are ordered passes within a single implementation work — not separate works. The work-level boundary is between conceptually different artifacts (spike vs. plan vs. implementation), not between process steps within one phase.

---

## Section 6: Process Compliance & Visibility

*New section — addresses the core agent UX problem identified by reviewers and informed by Gas Town patterns.*

### The Problem

Existing spec-writing jigs have natural compliance visibility: passes produce numbered artifact files. If a file is missing, `kerf square` catches it. The agent gets concrete feedback.

Process jigs (implementation passes) have no equivalent. If an agent skips the review gate, batches beads, or ignores the dispatch process, nothing in kerf flags it. "Guidance not gates" needs a companion: **visibility without enforcement.**

### The Approach: Recovery Over Prevention (Gas Town pattern)

Don't try to prevent agents from going off-script — make the work state durable and visible so deviations are detectable and recovery is cheap.

**Pass tracking in works:**
- When a work's jig has implementation passes, kerf tracks which passes have been started/completed
- `kerf show <codename>` reports pass status: "breakdown: done, dispatch: done, implement: 3/7 beads, review: 0/3"
- This gives the user and agent visibility into process compliance without blocking

**Beads as durable workflow state (Gas Town pattern):**
- The breakdown pass produces beads (via bd). These are the durable TODO list.
- Beads persist in Git. They survive agent crashes, context exhaustion, and session restarts.
- An agent that crashes mid-implementation can be restarted — it finds its place in the bead list and picks up where it left off.
- This is the same pattern as Gas Town's molecules: work state in the database, not in agent memory.

**Square for process jigs:**
- Extend `kerf square` to check process pass completion, not just artifact file existence
- "Has the breakdown pass produced beads? Has each bead been reviewed? Are there unreviewed completed beads?"
- Same posture as existing square: reports issues, doesn't block

---

## Impact on Existing Specs

**Modified specs:**
- `jig-system.md` — new metadata fields (phase, tools), pass library concept, composable pass assembly
- `works.md` — no structural changes (one work, one jig invariant holds), but document multi-work dependency patterns for SDLC flow
- `commands.md` — new `kerf setup` command (re-runnable), enhanced `kerf jig list` (phase filtering, tool display, active vs. available), enhanced `kerf show` (pass status for implementation works)
- `architecture.md` — per-project jig configuration (which jigs/passes are active). Currently config is bench-wide; needs per-project mechanism.
- `cli.md` — agent setup workflow, `kerf` no-args and `kerf resume` emit active jig chain
- `verification.md` — extend square to check process pass completion

**New spec files:**
- `jig-retrofit.md` — retrofit jig (two modes: context-rich, context-poor)
- `jig-spike.md` — spike/exploration jig (freeform exploration → structured convergence)
- `jig-implementation.md` — implementation jig assembled from composable passes (breakdown, dispatch, implement, review)

**Note:** Individual passes are defined within `jig-implementation.md` (or in a pass library spec), not as separate jig spec files. They are passes, not jigs.

**Existing spec principle changes:**
- `jig-system.md` currently says jigs are for "structured spec-writing workflows." This expands to cover the full SDLC. The spec must explicitly acknowledge this redefinition.
- The "not an orchestrator" principle (`cli.md`) holds: kerf stores process knowledge and emits it. Orchestration tools (ntm, Kilroy) execute it. kerf defines what to do; the orchestrator coordinates agents doing it.

**Future (not in this plan):**
- `jig-test.md` — structured testing beyond "run the suite" (TBD)
- Additional jigs/passes (deploy, release, etc.) — the framework supports adding new process steps
- Formula-like TOML source format for workflow composition (Gas Town pattern — interesting but premature for v1)
- Variable substitution in pass templates (protomolecule pattern — useful but not yet needed)

---

## Summary of Decisions

| Decision | Resolution |
|----------|-----------|
| Retrofit jig entry modes | Two: context-rich ("caught the agent") and context-poor ("found a divergence") |
| Retrofit triggering | kerf suggests retrofit when it detects untracked repo changes at `kerf list` / `kerf resume` time |
| Spike jig structure | Freeform exploration, structured convergence. Exploration log captures learnings. |
| Spike entry modes | Two: intentional (planned spike) and accidental (mid-work tangent) |
| Composability model | Composable pieces are **passes**, not jigs. Passes assemble into jigs. "One work, one jig" holds. |
| Implementation jig | A single `jig-implementation` with passes drawn from a library (breakdown, dispatch, implement, review). Projects select which passes to include. |
| Orchestration | Dispatch is a pass within the implementation jig. Tool (ntm, Kilroy) declared per-pass. |
| Jig instructions | Jig/pass contains the full agent-facing instructions. Single source of truth. |
| Agent config | kerf generates via `kerf setup` (re-runnable), agent applies to CLAUDE.md. kerf never writes directly. |
| `kerf setup` vs `kerf init` | `kerf init` is one-time bootstrap (calls setup). `kerf setup` is re-runnable for refreshing agent config. |
| Jig chaining | Work-level dependencies (existing mechanism). No new jig-level `depends_on` needed. |
| Work model | Separate works linked by dependencies. One work, one jig. Implementation passes are phases within a single work. |
| Process compliance | Recovery over prevention. Pass tracking in works. Beads as durable state. Extended `kerf square` for process passes. |
| Live discovery | `kerf` no-args and `kerf resume` emit active jig chain, not just CLAUDE.md |
| kerf's role boundary | kerf defines process, emits instructions. Orchestrators (ntm, Kilroy) execute. "Not an orchestrator" principle holds. |

---

## Open Items (to resolve during spec writing)

1. Exploration log naming — "exploration log," "spike log," or "decision log"
2. Per-project config mechanism — where exactly does the active jig/pass list live? (architecture.md change)
3. Pass library format — are passes defined inline in jig files, in a separate pass library, or both?
4. ntm pipeline integration specifics — how does the dispatch pass generate ntm pipeline YAML?
5. Square extension details — what exactly does process-pass squaring check?

---

## Next Steps

1. Prioritize which spec changes to tackle first
2. Break into individual spec change tasks
3. Implement spec changes following the plan → spec → code flow
