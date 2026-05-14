# The Factory Line — SDLC as Manufacturing Process

> Thinking document. Not a proposal. Exploring the shape of the process.

---

## The Central Metaphor

A manufacturing line has a specific structure: raw material enters, passes through stations that progressively transform it, and finished goods exit. Each station has clear input requirements, a defined transformation, and quality criteria for output. Work flows forward. Defects flow backward — but only to the station that can fix them, not to the beginning of the line.

Software development has the same structure, but we rarely formalize it. The question is: what does the line look like, and what are the rules that make it work?

---

## The Stations

Here is the line, laid out end to end. Each station takes a specific form of work-in-progress, transforms it, and passes it forward.

```
  INTAKE       DESIGN        DECOMPOSE       QUEUE        EXECUTE       VERIFY       ARCHIVE
    |            |              |              |             |             |             |
    v            v              v              v             v             v             v
 [idea]  ->  [spec]  ->  [task graph]  ->  [ranked   ->  [code]  ->  [verified  ->  [done]
                                            backlog]                   code]
```

But this is deceptively linear. The real topology has branches, merges, and feedback loops. Let me describe each station, then the connections between them.

### Station 1: Intake

**Input:** An unstructured idea. A bug report. A user complaint. A "what if we..." conversation. A discovered technical issue during implementation.

**Transformation:** The idea is given just enough structure to be placed. It gets a codename, a type, an initial area tag. It does NOT get fully specified — that is the next station's job. Intake is triage, not design.

**Output:** A work item stub — enough to answer: "What is this about? What part of the system does it touch? Does it overlap with anything already on the line?"

**Quality gate:** The overlap check. Before the work proceeds, the system asks: does this share area tags with in-flight work? If yes, that's a signal, not a blocker. The decision is: proceed independently, link as co-design, or merge into the existing work.

**Who:** Human initiates (has the idea). System performs the overlap check. Human decides on the overlap resolution.

**Kerf today:** `kerf new`. Creates a work, assigns a jig. Missing: area tags, overlap detection.

### Station 2: Design

**Input:** A work item stub from Intake.

**Transformation:** The idea becomes a specification. This is the jig's spec-writing passes: problem space, analysis, decomposition, research, spec drafting, integration review. The output is a document that says exactly what must be built and how it relates to the rest of the system.

**Output:** A finalized spec with acceptance criteria.

**Quality gate:** `kerf square` — structural verification. The spec must have the required artifacts for its jig. But also: do the area tags still reflect what this work actually touches? Has the scope shifted during design? If so, re-run the overlap check from Station 1.

**Who:** Agent does the spec-writing work. Human reviews at the jig's review points. System runs square.

**Kerf today:** The jig passes (plan, spec, bug jigs). This station works well for individual works. The gap is cross-work coherence — two specs that touch the same area aren't checked against each other.

### Station 3: Decompose

**Input:** A finalized spec.

**Transformation:** The spec is broken into atomic tasks (beads) with a dependency graph. Each bead has a description, acceptance criteria (traced to the spec), and dependencies on other beads.

**Output:** A task graph — a DAG of beads ready for scheduling.

**Quality gate:** The decomposition must be complete (every spec section covered), the dependency graph must be a DAG (no cycles), and each bead must be independently implementable by a single agent in a single session.

**Who:** Agent performs the decomposition. The `bd` tool validates the DAG structure. Human may review the breakdown for sanity.

**Kerf today:** The implementation jig's Breakdown pass. The gap Greg identified: getting the task graph into beads is cumbersome. YAML generation + import might be smoother than individual `bd create` calls.

### Station 4: Queue

**Input:** Task graphs from one or more works.

**Transformation:** Tasks from multiple works are merged into a single ordered backlog. This is where prioritization happens — not at the individual work level, but across ALL active work. The queue answers: "Of everything that could be worked on right now, what should be worked on next?"

**Output:** An ordered list of dispatchable tasks. "Dispatchable" means: all dependencies satisfied, no file conflicts with in-progress tasks, and the task's parent work is not paused.

**Quality gate:** The ordering must respect the dependency graph (a task's parents must be completed or in-progress before it can be dispatched). Beyond dependencies, ordering reflects: explicit priority overrides, fan-out (tasks that unblock the most downstream work), and recency of related completions (momentum — keep working in the same area while context is warm).

**Who:** This is where it gets interesting. Today: the orchestrator (Greg or an orchestrator agent) manually decides. The vision: the system computes the ordering, the orchestrator approves or overrides. `kerf next` is the query interface to this station.

**Kerf today:** Does not exist. This is the biggest gap. There is no unified queue across works. Each work's beads live in their own bead database. The orchestrator manually picks which work and which bead to dispatch next.

### Station 5: Execute

**Input:** A dispatched task (bead) with full context: description, acceptance criteria, spec reference, area constraints.

**Transformation:** An agent writes code that satisfies the bead's acceptance criteria.

**Output:** Code changes + the agent's claim that the bead is done.

**Quality gate:** The review gate. The orchestrator (or a reviewer agent) checks: does the code match what the spec says? Not "is the code good" but "does it match the spec." Up to 3 feedback rounds, then escalate to human.

**Who:** Worker agent executes. Orchestrator agent or human reviews.

**Kerf today:** The implementation jig's Implement pass, plus harmonik/ntm for agent orchestration. This station works, but it's disconnected from the Queue station — the handoff between "what to do next" and "go do it" is manual.

### Station 6: Verify

**Input:** The complete set of code changes for a work (all beads done).

**Transformation:** Holistic review — not per-task, but the whole thing as a unit. Does it hang together? Does it match the spec as a whole, not just criterion by criterion? Do the tests pass?

**Output:** A verification report: pass, or a list of gaps.

**Quality gate:** All acceptance criteria verified. All tests pass. No scope creep (code not in spec) and no gaps (spec not in code).

**Who:** Agent performs the verification. Human may review the report.

**Kerf today:** The implementation jig's Verify pass. Works for individual works. The gap is cross-work verification — if three works all touched the adapter, does the adapter still make sense as a whole?

### Station 7: Archive

**Input:** A verified, complete work.

**Transformation:** The work is marked done. Its artifacts are preserved. It exits the active portfolio.

**Output:** An archived work that no longer appears in `kerf map` or affects dependency calculations for active work.

**Who:** Agent or human triggers archival.

**Kerf today:** `kerf archive`. Works fine.

---

## What Moves Between Stations

This is subtle. The "work item" is not a static object that moves unchanged through the line. It undergoes phase transitions — its form changes fundamentally at certain boundaries.

```
Station:     INTAKE    DESIGN    DECOMPOSE    QUEUE      EXECUTE    VERIFY    ARCHIVE
             ------    ------    ---------    -----      -------    ------    -------
Form:        stub      spec      task graph   ordered    code       report    record
                                              backlog

Identity:    codename  codename  codename     bead IDs   bead IDs   codename  codename
                                 + bead IDs

Granularity: 1 work    1 work    N tasks      N tasks    1 task     1 work    1 work
                                              (across    at a time
                                              M works)
```

Notice the granularity shift. Stations 1-2 operate on whole works. Station 3 breaks a work into tasks. Station 4 merges tasks from multiple works into a single stream. Station 5 operates on individual tasks. Station 6 re-aggregates back to whole works. Station 7 files away the whole work.

This is the "fan-out / fan-in" pattern:

```
Work A ──── spec ──── [task A1, A2, A3] ───┐
                                            ├──→ unified queue ──→ execute one at a time ──→ verify A
Work B ──── spec ──── [task B1, B2] ───────┤                                               verify B
                                            ├──→                                            verify C
Work C ──── spec ──── [task C1, C2, C3, C4]┘
```

The Queue station is the merge point. It's where isolated works become a coordinated manufacturing line. Without it, you have N independent assembly lines running in parallel with no coordination — which is exactly the problem today.

---

## The Feedback Loops

The line is not unidirectional. Things go wrong. The question is: how far back does the work travel?

### Loop 1: Execute → Execute (minor fix)

The review gate catches a small issue. The agent fixes it. The bead stays at Station 5. This is the tightest loop — it's local to one station.

### Loop 2: Execute → Decompose (task was wrong)

The agent tries to implement a bead and discovers the task was mis-scoped. It's too big, or it's missing a prerequisite, or its description doesn't match reality. The bead needs to be split or rewritten. Work flows back to Station 3. The task graph is amended. New beads are created and enter the Queue.

### Loop 3: Execute → Design (spec was wrong)

The agent discovers that the spec is wrong — it describes something that can't be built as written, or it missed a critical constraint. This is the most expensive feedback loop. The work flows all the way back to Station 2. The spec is amended. Remaining beads may need to be rewritten.

**Critical rule:** When a spec is amended, ALL in-flight beads for that work must be re-evaluated. Some may still be valid. Some may need updating. Some may be invalidated. This is the "stop the line" moment in manufacturing.

### Loop 4: Verify → Execute (integration issue)

Individual beads passed their reviews, but the whole doesn't work together. Specific beads are identified as needing rework. They re-enter the Queue at high priority.

### Loop 5: Verify → Design (design issue)

The work as a whole doesn't satisfy the spec, and the problem isn't a bug — it's a design flaw. The spec needs revision. Back to Station 2.

### Loop 6: Any Station → Intake (new work discovered)

At any point, work at any station may reveal the need for NEW work — a prerequisite nobody anticipated, a refactoring that should happen first, a bug that needs fixing before this feature can land. This new work enters at Station 1 (Intake), goes through the overlap check, and may be linked to the discovering work via a dependency.

```
                    ┌──── Loop 3 ──────────────────────────────┐
                    │         ┌──── Loop 2 ────────┐           │
                    │         │      ┌── Loop 1 ──┐│           │
                    v         v      v             ││           │
  INTAKE → DESIGN → DECOMPOSE → QUEUE → EXECUTE ──┘│→ VERIFY → ARCHIVE
    ^                                    │          │     │
    │                                    └──────────┘     │
    │         Loop 6 (any station)                        │
    └──── new work discovered ◄───── Loop 4/5 ───────────┘
```

---

## Priority as a Dynamic Property

Greg's observation about P0/P1/P2 breaking down is fundamental. Static priority labels assume a fixed value ordering. But in practice:

- Completing task X changes the priority of task Y (because Y was waiting on X)
- Discovering issue Z during execution of X makes Z urgent (it didn't exist before)
- The "most valuable next thing" changes every time something completes

Static priority is a snapshot. The line needs dynamic priority — priority that's recomputed based on the current state.

### Three Ordering Forces

Greg identified three distinct forces that affect what should be done next:

**1. Technical dependency (structural).** Task X must come before Y because Y uses X's output. This is a hard constraint — violating it produces broken code. The dependency graph encodes this. It's not negotiable.

**2. Momentum / continuity (tactical).** Task Y2 should come next because the agent just finished Y1 in the same area and the context is warm. Switching to an unrelated task Z means paying a context-switch cost. This is an efficiency heuristic, not a hard constraint.

**3. Value chain (strategic).** Task M is more valuable than task N because completing M enables a user-visible capability, or because M is on the critical path to the next milestone. This is a human judgment that the system can't compute — but it can be expressed.

These forces sometimes align and sometimes conflict. A good queue-ordering algorithm would:

1. Never violate technical dependencies (hard constraint)
2. Prefer momentum when all else is equal (soft preference)
3. Allow explicit value-chain overrides that trump momentum (human input)

### Pull-Based Priority

Manufacturing lines often use pull-based systems (kanban). Instead of pushing work through the line based on a master schedule, downstream stations pull work when they have capacity.

Applied here: an idle agent doesn't get assigned the "next task on the list." Instead, the agent asks `kerf next` and gets a computed answer based on:

- What's unblocked (dependencies satisfied)?
- What's in the same area as recently completed work (momentum)?
- What has explicit priority overrides (value chain)?
- What has the highest fan-out (unblocks the most downstream work)?

The agent pulls. The system computes. The human overrides when the computation is wrong.

This naturally handles the "priorities shift when work completes" problem. There IS no static priority list. The priority is recomputed every time someone asks.

---

## The Unified Queue Problem

The hardest architectural question in this whole system: where does the unified queue live?

Today, each work has its own beads in `bd`. The beads for Work A know nothing about the beads for Work B. The Queue station (Station 4) doesn't exist as infrastructure — it exists only in the orchestrator's head.

Options for where the queue lives:

**Option A: Compute on demand.** No persistent queue. When an agent asks "what's next?", kerf reads all active works, resolves their beads, builds the cross-work dependency graph, applies the ordering algorithm, and returns the answer. Stateless. No staleness. Potentially slow if there are many works/beads — but for realistic portfolios (5-20 works, 50-200 beads), it's trivial.

**Option B: Materialized queue in harmonik.** The queue is a data structure in the orchestration system. When work enters Station 4, it's "loaded" into the queue. The queue tracks its own state. This introduces a second source of truth — the beads in `bd` and the queue in harmonik can disagree.

**Option C: The beads ARE the queue.** All beads for all works in a project live in one shared bead database. Cross-work dependencies are just regular bead dependencies. The queue is just "all open beads, topologically sorted with priority." This is elegant but means `bd` needs to know about kerf works (coupling).

Greg's instinct toward Option A (compute on demand) aligns with kerf's filesystem-as-database philosophy. But there's a subtlety: the queue needs information from TWO systems — kerf (work status, area tags, priority) and bd (bead status, bead dependencies). The computation has to cross that boundary.

A possible resolution: kerf computes work-level ordering (`kerf next` at the work level). Within a work, `bd` computes bead-level ordering (`bd ready`). The orchestrator combines them: pick the next work from kerf, pick the next bead from bd within that work. Two queries, composed.

---

## The Information Flow Problem

Each station needs information from upstream stations — but also from the system as a whole. Here's what each station needs to see:

```
Station      Needs from upstream              Needs from the system
-------      ---------------------            ----------------------
Intake       (nothing — it's the entry)       Active works, area tags, overlap map
Design       Work stub from Intake            Specs for area peers, shared constraints
Decompose    Finalized spec from Design       Bead databases for related works
Queue        Task graphs from Decompose       All active task graphs, completion state
Execute      Dispatched task from Queue       Spec reference, area constraints
Verify       All completed beads for work     The spec, test results
Archive      Verification report              (nothing)
```

The "needs from the system" column is where coordination happens. It's also where the current system is blind. An agent at Station 5 (Execute) has the bead description, but does NOT have:

- The area tags of the work it's implementing
- The existence of other works touching the same area
- The design constraints that were decided during those other works' design phases
- The current state of the broader portfolio

Most of this information is available in spec.yaml files on the bench. It just isn't assembled and handed to the agent at the right moment.

---

## Parallel Paths and Convergence Points

The line isn't strictly serial for the portfolio. Multiple works can be at different stations simultaneously:

```
Time →

Work A:  [INTAKE] [DESIGN ·········] [DECOMPOSE] [QUEUE→EXECUTE→EXECUTE→EXECUTE] [VERIFY] [ARCHIVE]
Work B:           [INTAKE] [DESIGN ····] [DECOMPOSE] [QUEUE→EXECUTE→EXECUTE] [VERIFY] [ARCHIVE]
Work C:                    [INTAKE] [DESIGN ·····················] [DECOMPOSE] [QUEUE→EXE] ...
```

Works flow independently through Stations 1-3. They converge at Station 4 (Queue), where their tasks are interleaved. They diverge again at Station 6 (Verify), which is per-work.

The convergence at Station 4 is the critical coordination point. This is where:

- Tasks from different works that touch the same files must be sequenced (file reservation)
- Tasks from different works in the same area should be grouped (momentum)
- Cross-work dependencies are resolved (work B's tasks can't run until work A reaches a certain point)

Station 2 (Design) has a weaker convergence point: when multiple works touch the same area, their design should be aware of each other. But this is advisory ("read the area peers' specs") not structural (no merge/interleave needed). The `co-designs` relationship and area overlap warnings address this.

---

## Invariants of the Line

Properties that must hold for the line to function correctly:

**1. A work has exactly one position on the line at any given time.** It's at one station. It can move forward or backward, but it's never at two stations simultaneously. (Individual beads within a work can be at different stages, but the WORK has a station.)

**2. Forward movement requires passing the quality gate.** No skipping. A spec that hasn't been squared doesn't enter Decompose. A bead that hasn't been reviewed doesn't get closed. Gates can be lightweight, but they exist.

**3. Backward movement targets the earliest broken station.** If a spec error is found during Execute, the work goes back to Design — not to Intake. If a decomposition error is found, it goes back to Decompose — not to Design. Fix at the source, not upstream of the source.

**4. New work always enters at Intake.** Even "urgent" work. Even "small" work. Intake may be fast (seconds for a trivial fix), but it still happens. The overlap check still runs. This prevents "shadow work" that bypasses coordination.

**5. The Queue is the single point of cross-work coordination.** Before the Queue, works are independent streams. After the Queue, tasks are individual items. The Queue is where the portfolio becomes a unified manufacturing line.

**6. Information flows forward explicitly, not implicitly.** Each station's output is a concrete artifact (spec, task graph, ordered backlog, code, report). The next station reads the artifact — it doesn't infer what the previous station intended.

**7. Human override is always available but never required for forward progress.** The system computes ordering, detects overlaps, flags issues. A human can override any of it. But if the human is absent, the system should still be able to make reasonable forward progress. Advisory, not blocking.

---

## What Breaks the Line

The line breaks when these conditions arise:

**Invisible work.** Work that exists but isn't on the line. An agent starts coding something without creating a work item. There's no overlap check, no area tag, no coordination. The work is invisible to every other station.

**Stale information at a station.** The Queue was computed an hour ago. Since then, three beads completed and a new work entered. The ordering is wrong. Staleness is the enemy of dynamic priority. Compute on demand, don't cache.

**Unbounded WIP.** Too many works in Design simultaneously. Too many beads in Execute simultaneously. The coordination overhead grows quadratically with WIP. The line jams not because any station is slow, but because there are too many items in flight. WIP limits address this, even if advisory.

**Missing feedback loops.** An agent at Execute discovers a spec issue but has no mechanism to signal it. The defect propagates forward. The Verify station catches it, but by then more work has been done on top of the bad spec. The cost of the fix is now much higher. Fast feedback loops to earlier stations prevent this compounding.

**Merge conflicts at convergence.** Two works touch the same area. Their tasks interleave in the Queue. Agent A modifies file X for Work A. Agent B modifies file X for Work B. Without file reservation, they conflict. This is the concurrent-access problem, and it's a manufacturing problem too (two operations can't use the same machine at the same time).

---

## The Station Graph (Formal View)

```
Nodes: { Intake, Design, Decompose, Queue, Execute, Verify, Archive }

Forward edges (normal flow):
  Intake    → Design       [condition: work created, overlap check done]
  Design    → Decompose    [condition: spec finalized, square passes]
  Decompose → Queue        [condition: task graph complete, DAG validated]
  Queue     → Execute      [condition: task is unblocked and selected]
  Execute   → Verify       [condition: all beads for work are closed]
  Verify    → Archive      [condition: verification passes]

Backward edges (feedback):
  Execute   → Execute      [condition: review finds minor issue]
  Execute   → Decompose    [condition: task mis-scoped]
  Execute   → Design       [condition: spec error found]
  Verify    → Execute      [condition: integration issue, specific beads identified]
  Verify    → Design       [condition: design flaw found]

Lateral edges (new work):
  Design    → Intake       [condition: new work discovered during design]
  Execute   → Intake       [condition: new work discovered during implementation]
  Verify    → Intake       [condition: new work discovered during verification]

Multiplicity:
  Intake through Decompose: 1 work per traversal
  Queue: N works' tasks merged into 1 stream
  Execute: 1 task at a time per agent, M agents in parallel
  Verify through Archive: 1 work per traversal
```

The graph has one merge point (Queue) and one fan-out point (also Queue — tasks fan out to parallel agents). The rest is sequential per-work.

---

## What This Means for kerf

kerf's role in this line is Stations 1-3 plus the work-level input to Station 4. Specifically:

- **Station 1 (Intake):** `kerf new` + area tags + overlap detection
- **Station 2 (Design):** The jig passes (plan, spec, bug, spike)
- **Station 3 (Decompose):** The implementation jig's Breakdown pass
- **Station 4 (Queue):** `kerf next` / `kerf map` — the work-level ordering that feeds into bead-level ordering
- **Station 7 (Archive):** `kerf archive`

Stations 5-6 (Execute, Verify) are orchestrator territory — harmonik/ntm, with kerf providing context (area peers, dependency status, spec references).

The key insight: kerf doesn't need to own the whole line. It needs to own the **information layer** across the whole line — the metadata that tells every station what it needs to know about the work landscape. The area tags, the dependency graph, the work statuses, the overlap map. That's the persistent map (Problem 1). That's what `kerf map` computes. That's what makes the line work.

---

## Open Questions

These are the questions I don't have answers to yet:

**1. How does bead state flow back to kerf?** Kerf needs to know "all beads for Work A are done" to know Work A can move from Execute to Verify. Today, bead state lives in `bd` and kerf doesn't read it. Does kerf query `bd`? Does the orchestrator update kerf's status manually? Does something else bridge the gap?

**2. What's the right abstraction for the Queue?** Is it a kerf concept (a command that computes ordering), a harmonik concept (a queue data structure that tasks are loaded into), or a protocol between the two?

**3. How do cross-work bead dependencies work?** Within a work, beads depend on each other via `bd dep`. Across works, dependencies are in kerf's `depends_on`. But what if Task B3 specifically depends on Task A7 (not just "Work B depends on Work A")? That cross-work task-level dependency doesn't have a home today.

**4. Where does the "stop the line" decision live?** When a spec error is found during Execute, who decides to stop all beads for that work and send it back to Design? Today it's the orchestrator's judgment. Should the system support this formally — a "pause work" action that pulls all its beads from the Queue?

**5. How does momentum work in practice?** The idea of "keep working in the same area" sounds right, but the implementation is tricky. Does it mean "prefer beads from the same work"? Or "prefer beads tagged with the same area"? What if momentum conflicts with dependency ordering?
