# Plan 005 — Work Coordination & Portfolio Coherence

> STATUS: PROBLEM DEFINITION — not yet proposing solutions

## The Gap

kerf structures individual works well. Harmonik (+ beads) executes tasks well. But nobody owns the layer between them: the **work graph** — the portfolio-level view of what needs to happen, in what order, with what relationships, across what areas of the system.

This gap manifests as five distinct problems.

---

## Problem 1: No Persistent Map

Each agent session starts with a HANDOFF document — a flat narrative of what happened last and what to do next. Over 5-10 sessions (each completing 10-40 tasks via subagents), the HANDOFF accumulates minutia but loses the big picture.

**What's missing:** A structured, durable view of "here is all the work, here is what's done, here is what's in flight, here is what's ahead." The agent can't orient itself in the landscape. It has breadcrumbs from the last session, not a map of the territory.

The `bv` tool may help with visibility into bead state, but beads are task-level. The gap is at the work level — which works exist, what state they're in, how they relate, and what the overall trajectory looks like.

**Concrete failure mode:** Session 7 of an implementation run. The agent reads HANDOFF.md, sees "continue implementing auth adapter beads." It has no awareness that there are 4 other works queued that also touch the adapter, that 2 of them have specs that overlap, or that the current work is blocking 3 downstream works. It just grinds forward on the next bead.

---

## Problem 2: Work Items Are Islands

kerf creates one work at a time. Each work has its own jig passes, its own artifacts, its own lifecycle. But real systems have *clusters* of related changes to the same area.

**Example:** Three work items all touch the adapter layer:
- Work A: Add retry logic to the adapter
- Work B: Add metrics/observability to the adapter  
- Work C: Change the adapter's connection pooling

Implemented independently, each might make reasonable but incompatible design choices. Work A adds a retry wrapper. Work B adds instrumentation at a different layer. Work C restructures the internals in a way that breaks both A and B's assumptions.

**What's missing:** A mechanism to say "these three things are related and should be thought about together." Not necessarily implemented in the same PR, but designed with awareness of each other.

---

## Problem 3: No Intake Queue or Prioritization

When a new idea arrives, it becomes a work item via `kerf new`. But then what?

- Where does it sit relative to other work?
- What should be worked on next?
- If it depends on something in-flight, when does it become actionable?
- If three things could be done next, which one matters most?

Beads handle task-level dependencies *within* a work. Work-level `depends_on` exists in spec.yaml but is manually set and only models "must-complete-first" relationships. There is no prioritization, no queue, no triage mechanism.

**Concrete failure mode:** You have 8 open works for a project. An agent finishes one and needs to pick the next. It has no signal for what's most important, what's blocked, or what would unblock the most downstream work. It picks arbitrarily or asks the user.

---

## Problem 4: Late-Arriving Requirements

You write a spec for adapter retry logic. Tasks get generated. Implementation starts. Three hours later, while thinking about something else, you realize the adapter also needs circuit-breaker behavior — and it should share state with the retry logic.

**The dilemma:**
- The retry work is in-flight. Some tasks are done, some aren't.
- The circuit-breaker requirement overlaps architecturally — they share state, interact at the same layer.
- Creating a new independent work item means the circuit-breaker will be designed without knowledge of the retry implementation choices already made.
- Merging it into the in-flight work is messy — the spec is "done," tasks are generated, some are complete.

**What's missing:** A way to surface "this new requirement is architecturally entangled with that in-flight work" and handle it gracefully. Options might include: amending the in-flight work's spec, creating a dependent work that explicitly inherits the design context, or pausing the in-flight work to re-plan both together.

---

## Problem 5: No Coherence Across the Two Process Modes

kerf supports plan-first and spec-first workflows. The coordination problem manifests differently in each:

**Plan-first:** Coordination fails at *intake*. You write plan A for the adapter, start implementing. Later you write plan B, also touching the adapter. Plan A's spec changes didn't account for B's needs. The plans are islands (Problem 2) and there was no mechanism to catch the overlap at creation time.

**Spec-first:** Coordination fails at the *spec* stage. The adapter spec and the core-system spec both have opinions about the interface between them. They were written at different times, possibly by different agents. They may contradict or make incompatible assumptions. There's no reconciliation point.

**Both modes:** Coordination fails during *implementation*. The executing agent has task-level context (the current bead) but not portfolio-level context (how this task fits into the larger picture of all the changes happening to this area of the system).

**What's needed:** Something that maintains coherence regardless of which direction you're approaching from — whether you started with a plan and derived specs, or started with specs and decomposed into tasks.

---

## What This Is NOT

This plan is not proposing that kerf become an orchestrator or task executor. The boundaries:

- **kerf** — structures work, maintains the work graph, provides orientation and coherence
- **beads (bd/bv)** — models the task graph within a work, tracks task state and dependencies
- **harmonik** — executes tasks, manages agent sessions, coordinates implementation

The missing layer is between kerf and beads: the *work graph* that connects individual works into a coherent portfolio with ordering, relationships, and area-level design coherence.

---

## Dimensions to Explore in Decomposition

These are not solutions — they're the dimensions along which solutions might exist:

1. **The map** — A durable, structured view of all work for a project. Not a narrative (HANDOFF), not a task list (beads), but a work-level graph showing state, dependencies, and trajectory.

2. **Area/component clustering** — Grouping works by what part of the system they touch. Making overlap visible. Enabling "think about these together" before they're independently designed.

3. **Work ordering and readiness** — Given the dependency graph and current state, what's actionable? What's blocked? What would unblock the most? Not necessarily automated prioritization, but making the information available.

4. **Late-arriving requirement handling** — A process for "new thing overlaps with in-flight thing." Could be spec amendment, dependent work with inherited context, or deliberate re-planning.

5. **Session orientation** — What an agent needs at the start of a session to understand where it is in the landscape. More than a handoff narrative, less than re-reading every spec.

6. **Cross-work design coherence** — When multiple works touch the same area, ensuring they're designed with awareness of each other. This might be a review/reconciliation pass, or it might be structural (area-level specs that individual works must conform to).

---

## Next Steps

1. Review this problem definition — is anything missing or mischaracterized?
2. Brainstorm solution approaches for each dimension
3. Identify which dimensions are most critical / highest leverage
4. Decompose into spec changes
