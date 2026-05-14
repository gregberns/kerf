# Final Synthesis — Work Coordination & Portfolio Coherence

> Definitive synthesis from 16 agents. Resolves contradictions, incorporates
> critic feedback, presents a buildable vision with honest complexity accounting.

---

## 1. The Big Picture

kerf today structures individual works well. An agent can pick up a jig, walk through passes, produce a spec, and hand it off. But the moment you have 5+ works touching the same codebase, nobody knows the shape of the whole. The agent sees one work. The human remembers some of it. The HANDOFF document lies about the rest.

The gap is a **coordination layer** — not execution (that's harmonik), not task management (that's beads), but the connective tissue between works. Which works exist, what state they're in, how they relate to each other, what area of the system they touch, and what should happen next.

Here is how the three tools fit together with this new layer:

```
                     THE PICTURE

   kerf                    beads (br/bv)              harmonik
   ────                    ────────────               ────────
   Structures work.        Tracks tasks.              Executes tasks.
   Owns the work graph.    Owns the task graph.       Owns agent sessions.
   Knows about areas,      Knows about beads,         Knows about workers,
   dependencies,           dependencies between        queues, dispatch.
   priorities.             beads, completion state.

         │                       │                         │
         │    work-level         │    task-level            │
         │    ordering           │    ordering              │
         ▼                       ▼                         ▼
   ┌──────────────────────────────────────────────────────────┐
   │              THE COORDINATION LAYER                       │
   │                                                           │
   │  kerf map    → "here is everything, here is the shape"    │
   │  kerf next   → "work on this, because of these reasons"   │
   │  kerf orient → "here is what you need to start a session" │
   │  session log → "here is what previous sessions learned"   │
   │  areas.yaml  → "here is what the system is made of"       │
   │  protocols   → "here is how to handle coordination events"│
   └──────────────────────────────────────────────────────────┘
```

kerf goes from "a spec-writing CLI" to "the brain of the factory line." It doesn't execute anything. It doesn't manage agents. It holds the map, computes what's next, and gives every agent that touches the system the context it needs to make locally correct decisions that are globally coherent.

---

## 2. Resolved Contradictions

The coherence critic (doc 17) identified three contradictions that must be resolved before speccing. Here are the rulings.

### 2.1 Session Records: per-work SESSION.md vs. per-session YAML files

**The contradiction:** Doc 15 (protocols) writes session records to `SESSION.md` inside each work directory, with trimming/archiving. Doc 16 (session continuity) writes immutable per-session YAML files to a project-level `.kerf/sessions/` directory, filtered at read time.

**Winner: Per-work SESSION.md with structured markdown format.**

Reasoning: Doc 16's per-session YAML files are architecturally cleaner, but they create a practical problem the protocols need to solve: cross-work signaling. The co-design protocol needs to write "I checked this peer and found X" somewhere the next session on that work can find it. With per-session YAML files at the project level, this information is scattered across timestamped files and requires the filtering machinery (works_touched, area tags) to reassemble.

Per-work SESSION.md keeps all coordination context for a work in one place. The next agent resuming that work reads one file, not a filtered view across dozens. The append-only discipline and structured sections (COMPLETED / NEXT / DISCOVERED / BLOCKED) from doc 15 prevent the telephone game just as effectively as immutable YAML files — as long as the rule "never rewrite previous entries" is enforced by convention and verified by `kerf square`.

**Key compromise from doc 16:** Adopt the computed-vs-stored principle rigorously. SESSION.md stores ONLY non-computable knowledge (decisions, discoveries, warnings, unfinished reasoning). Status, dependency state, area peers — all computed fresh by `kerf orient` / `kerf map`. This keeps SESSION.md small and prevents the HANDOFF bloat pattern.

**Key compromise from doc 15:** Adopt doc 16's archiving logic instead of the trim-to-5 rule. When SESSION.md exceeds 10 entries, move older entries to SESSION-ARCHIVE.md. But do NOT summarize — just move. The archive is available if needed but doesn't consume context by default.

### 2.2 Command Overlap: orient vs. map vs. resume vs. next

**The contradiction:** Doc 16 proposes `kerf orient` (combines portfolio + session history). Doc 15 uses `kerf map` + `kerf resume` as separate steps. Doc 13 proposes `kerf next` for work selection. These partially overlap.

**Winner: Three distinct commands, no `kerf orient`.**

- **`kerf map`** — Portfolio-level view. All works, statuses, dependencies, area clusters, actionable/blocked. Machine and human readable. This is Station 4's query interface.
- **`kerf next`** — Answers one question: "what should I work on?" Returns a ranked list of actionable works with reasons. Machine-parseable output. Feeds into harmonik's orchestrator.
- **`kerf resume`** — Work-level context load. Enhanced to show dependency status, area peers, co-design relationships, and relevant SESSION.md entries. This is what the agent reads when starting work on a specific item.

`kerf orient` is dropped. It tried to be all three commands in one, which makes its output too long and its purpose unclear. The session-start protocol becomes: run `kerf map` (see the landscape), run `kerf next` (pick a work), run `kerf resume <codename>` (load context for that work). Three steps, three commands, each doing one thing.

### 2.3 Work Status Ownership: computed from beads vs. station-based vs. manual

**The contradiction:** Doc 13 says beads "owns" status during implementation (computed from bead counts). Doc 11 says station position determines status. Doc 15 says the agent manually updates spec.yaml status in the session-end protocol.

**Winner: Dual-source with kerf as authority, beads as input signal.**

The rule: **spec.yaml `status` field is always the authoritative source for work status.** kerf owns it throughout the lifecycle. However, when beads integration is enabled, `kerf map` and `kerf next` query bead state to ENRICH their output — showing task-level progress (3/12 beads done) alongside the work-level status.

The agent updates spec.yaml status as part of the session-end protocol. This is a deliberate human-in-the-loop decision (or orchestrator-in-the-loop for automated pipelines). Automatic status transitions from bead completion are deferred — they sound clean but create a second source of truth where spec.yaml and bead state can disagree.

When beads integration is enabled, `kerf map` can flag inconsistencies: "Work X has status 'implementing' but all beads are closed — consider advancing status to 'done'." Advisory, not automatic.

---

## 3. The System Shape

### 3.1 The Factory Line — Real Stations

The 7-station model (doc 11) is valuable as a thinking tool but the practitioner critic is right: clean station boundaries don't match how Greg actually works. Greg's real workflow is:

1. **Batch intake:** Have a bunch of ideas, create 5-8 works in a sitting.
2. **Batch design:** Walk each work through jig passes over a few sessions. Some get fully specced, some stall at research.
3. **Batch decompose:** Once specs are solid, decompose them all into task YAML in one or two sessions.
4. **Long execute:** Load all the beads, point harmonik at them, and let agents grind through 40+ beads over multiple sessions.
5. **Verify and close:** Review the results, verify, archive.

The factory line stations exist, but a single work doesn't move smoothly through them. Multiple works bunch up at the same station, then move in batches. The Queue (Station 4) isn't a continuous flow — it's loaded all at once and drained over time.

**What this means for kerf:** Don't model "which station is this work at" as a first-class concept. The jig passes already track design progress. Bead state tracks execution progress. kerf's job is to make the portfolio state visible, not to enforce a station progression.

The factory line's real value is in the **feedback loops** (doc 11, Section "The Feedback Loops"). When execution reveals a spec problem, the work goes back to design. When verification reveals integration issues, specific beads re-enter the queue. kerf needs to support these backward movements — but as status changes on the work, not as station transitions.

**Recommendation:** Use the factory line as the conceptual model in documentation and design discussions. Do NOT encode stations as a data model in kerf. The existing jig pass progression + spec.yaml status values already capture the same information in a form that's more granular and more accurate.

### 3.2 The Area Graph — Hierarchy is the Sweet Spot

Both critics converge: typed directed edges between areas are premature. They require architectural judgment to create, nobody will maintain them, and stale edges are worse than no edges (they produce wrong adjacency warnings).

**Build this:**

```yaml
# ~/.kerf/projects/{project-id}/areas.yaml
areas:
  adapter:
    description: "External service integration layer"
  adapter.retry:
    description: "Retry logic for adapter calls"
    parent: adapter
  adapter.pool:
    description: "Connection pooling for adapters"
    parent: adapter
  auth:
    description: "Authentication, identity, token management"
  database:
    description: "Persistence layer, migrations"
  api:
    description: "HTTP/REST interface, routing"
  core:
    description: "Domain logic, business rules"
```

No edges. No `calls`, `reads`, `owns` (beyond the parent/child hierarchy). Just a defined taxonomy of what the system is made of, with hierarchy for grouping.

**Enforcement:** Works can only use areas that exist in `areas.yaml`. `kerf new` validates against the list. Unknown areas produce an error with the valid set displayed. Agents can add new areas via `kerf areas add` (additive only — removing/renaming requires human action).

**What this enables now:**
- Overlap detection at `kerf new` time (same area or parent-child)
- Grouped display in `kerf map` (all adapter works together)
- Parent-area rollup for heat mapping (3 works across adapter sub-areas = adapter is hot)

**What this enables later (without schema migration):**
- Add an `edges:` section to `areas.yaml` when real need emerges
- Build adjacency queries, blast radius, impact analysis against those edges
- The data model is forward-compatible

### 3.3 Priority Model — Honest About Sparse Graphs

The three-dimension decomposition (doc 12) is the best idea in the entire exercise:
1. **Dependency** — hard constraint, filters the candidate set
2. **Urgency** — temporary human override, decays over time
3. **Value** — strategic tiebreaker, inherited from areas/goals

But the practitioner critic flags a real problem: **in early use, the graph will be sparse.** Five works with no dependency edges means fan-out is zero everywhere, critical path is meaningless, and `kerf next` returns creation-order with no useful differentiation. The agent learns to ignore it.

**Build this for Phase 1:**

```
kerf next algorithm:
  1. Filter: remove blocked, shelved, finalized, in-flight works
  2. Override: pinned works go to top
  3. Rank remaining by:
     a. Has incomplete dependencies that are close to finishing (soft signal)
     b. Creation date (oldest first — deterministic tiebreaker)
  4. Display: show reason for ranking. If all works are equally ranked,
     say so: "4 works are equally ranked — no dependency relationships
     differentiate them. Consider adding depends_on edges."
```

No value weights. No critical path analysis. No fan-out scoring. Those are Phase 3 features that pay off at 20+ works with a dense graph.

**The pin mechanism:**
- `kerf pin <codename>` — marks a work as urgent, goes to top of `kerf next`
- Pins have a TTL: they expire after 3 sessions (or a configurable number)
- Maximum 2 concurrent pins (forces prioritization)
- `kerf unpin <codename>` for explicit removal
- `kerf next` shows pinned works with "PINNED by user" as the reason

This directly solves Greg's "user finishes testing, finds an issue, wants it prioritized for next session" workflow. Create the work, pin it, next agent picks it up.

### 3.4 Session Continuity — What Replaces HANDOFF

HANDOFF.md is replaced by two mechanisms working together:

**Mechanism 1: Computed orientation (`kerf map` + `kerf resume`).** Fresh every time. Never stale. Replaces the 60-70% of HANDOFF that was status information.

**Mechanism 2: Per-work SESSION.md with structured append-only entries.** Captures the 30-40% of HANDOFF that is non-computable: decisions, discoveries, warnings, unfinished reasoning. Each session appends one entry. Previous entries are never rewritten.

```markdown
## Session: 2026-05-08

### Completed
- Wrote adapter retry spec (specs/adapter-retry.md)
- Updated spec.yaml status to change-spec

### Next
- Write component decomposition for retry policy
- Check co-design relationship with brave-falcon on shared error types

### Discovered
- The adapter's Close() must be called before Reconnect() or you get fd leaks
- brave-falcon assumes synchronous retry; our design assumes async — conflict

### Blocked
- None
```

**Rules enforced by convention (checked by `kerf square`):**
- COMPLETED: only things actually done this session
- NEXT: specific enough for a fresh agent to execute
- DISCOVERED: genuine surprises, not routine observations
- BLOCKED: genuine blockers, not "might be tricky"
- Append only — never edit previous entries
- Archive entries older than the most recent 5 to SESSION-ARCHIVE.md

**What about the session YAML records from doc 16?** Deferred. The structured markdown is simpler to write, simpler to read, and agents are more reliable at producing markdown than YAML. If scaling to 50+ sessions reveals that the markdown format doesn't support sufficient filtering, migrate to YAML then. Build for now, not for hypothetical scale.

### 3.5 Agent Protocols — Essential vs. Nice-to-Have

The practitioner critic raises a real concern: the full protocol stack (9-step session start, 5-step co-design, 6-step late requirement, 6-step disruption) consumes 650-1400 lines of context before work begins. That's too much.

**Essential protocols (build in Phase 1):**

1. **Session-end: structured SESSION.md entry.** 4 sections, append-only. This is standalone, immediately useful, prevents the telephone game. Embedded in a `kerf handoff` command.

2. **Session-start: 3-step orientation.** Run `kerf map`. Run `kerf next` (or receive assigned work). Run `kerf resume <codename>`. That's it. No 9-step protocol — just three commands that produce the right context.

3. **Late-requirement decision matrix.** The stage-by-depth matrix from doc 15 is brilliant — 9 cells, 6 are mechanical, 3 escalate to user. Embed in `kerf new` output when overlap is detected. An agent reads it at exactly the moment it needs it.

**Nice-to-have protocols (build later when needed):**

4. **Co-design protocol.** The full 5-step reconciliation (read peer artifacts, classify stages, check for contradictions) requires architectural reasoning agents do inconsistently. Simplify to: "Read peer's spec.yaml and latest SESSION.md entry. Note any obvious conflicts in your own SESSION.md. If uncertain, escalate." Build the full protocol when agents get better at interface comparison.

5. **Disruption protocol.** Simplify to one rule: "If the fix requires changing a spec or affects another work, STOP and escalate to the user. Otherwise, fix locally and note in SESSION.md." The 4-type classification is overkill for now.

**The key design principle (from doc 15):** Embed protocols in command output, not in separate files. The agent doesn't need to remember to read a protocol document — it gets the relevant instructions when it runs `kerf resume` or `kerf new`. Protocols are delivered at the moment of need, not pre-loaded into context.

### 3.6 Beads Integration — The Interface

The beads integration design (doc 13) is the most implementation-ready analysis. The YAML intermediate representation, the `work:<id>` label convention, and the status query model are proven from harmonik.

**The interface between kerf and beads:**

```
kerf writes to beads:
  - Task YAML + loader creates beads with work:<id> labels

kerf reads from beads:
  - br list -l work:<id> --json → bead counts (total, open, closed, blocked)
  - Mnem-map CSVs → cross-work reference resolution

kerf does NOT:
  - Store bead state
  - Manage bead dependencies
  - Dispatch beads to agents (that's harmonik)
```

**The cross-work edge problem:** Doc 11 asks "What if Task B3 depends on Task A7?" Doc 13 proposes cross-work edges in the YAML. Doc 17 notes these have no representation in the priority model.

**Resolution:** Cross-work bead-level dependencies live in the beads system (they're just regular `br dep add` edges in a shared database). kerf doesn't model them — kerf models work-level dependencies (`depends_on` in spec.yaml). The orchestrator (harmonik or an orchestrator agent) is responsible for the composition: kerf says "work on Work B next"; beads says "bead B3 is blocked on bead A7 in Work A"; the orchestrator handles this by working on other beads in Work B or switching to Work A.

This is the "two queries, composed" approach from doc 11. It keeps the systems loosely coupled. kerf doesn't need to understand bead-level dependency graphs. Beads doesn't need to understand work-level priority.

**The optional gate:** All beads integration is behind `tools.task_tracker: beads` in project.yaml. Without it, `kerf map` shows only kerf-level status, `kerf next` uses only work-level dependencies, and `kerf decompose` isn't available. kerf works fine without beads — it just has less information.

---

## 4. What to Build — Phased

### Phase 1: Orientation and Areas

**Goal:** An agent starting a session can see the landscape and pick up work without reading HANDOFF.md.

**Build:**
- `areas.yaml` schema — flat list with parent hierarchy, descriptions, enforced by kerf
- `kerf areas add <name>` — add a new area
- `areas` field in spec.yaml — list of area names, validated against areas.yaml
- `kerf map` — reads all active works, groups by status and area, shows dependency graph, flags actionable/blocked
- `kerf resume` enhancement — show dependency status, area peers, co-design warnings, last SESSION.md entry
- `kerf handoff` — scaffold structured SESSION.md entry (COMPLETED / NEXT / DISCOVERED / BLOCKED), agent fills in content
- Overlap warnings on `kerf new` — check area tags of existing works, warn on overlap

**NOT in Phase 1:**
- `kerf next` (just use the actionable list in `kerf map`)
- Beads integration (status comes from spec.yaml only)
- Priority computation (manual work selection)
- `co-designs` relationship type (use `inform` for now)
- Pins, value weights, urgency mechanisms
- `kerf decompose`

**Data structures:**

```yaml
# areas.yaml (new)
areas:
  adapter:
    description: "External service integration layer"
  adapter.retry:
    parent: adapter
    description: "Retry logic for adapter calls"
```

```yaml
# spec.yaml (additions to existing)
areas:                     # NEW — list<string>, validated against areas.yaml
  - adapter
  - adapter.retry
```

```markdown
# SESSION.md (new per-work file)
## Session: 2026-05-08

### Completed
- ...

### Next
- ...

### Discovered
- ...

### Blocked
- ...
```

**Independently useful?** Yes. `kerf map` alone replaces HANDOFF for portfolio orientation. Areas prevent naming drift. SESSION.md prevents the telephone game. An agent can start a session, run `kerf map`, pick a work, run `kerf resume`, and have better context than any HANDOFF ever provided.

**Testable against real use?** Yes. Use it for the next kerf development cycle. Create 3-5 works, define areas, run through jig passes. Does `kerf map` give you what you need? Is SESSION.md better than HANDOFF?

### Phase 2: Computed Priority and Beads Status

**Goal:** `kerf next` answers "what should I work on?" using live data from both kerf and beads.

**Build:**
- `kerf next` — filter actionable, apply pins, rank by dependency + age, display reasons
- `kerf pin <codename>` / `kerf unpin <codename>` — urgency override with TTL
- `co-designs` relationship type in depends_on
- Beads status query in `kerf map` — when `tools.task_tracker: beads` is set, query `br list -l work:<id> --json` for task-level progress
- `kerf next` JSON output mode — machine-parseable for harmonik orchestrator consumption
- Task YAML schema definition — document the schema agents should produce for decomposition
- `work:<id>` label convention — documented, enforced by the loader

**NOT in Phase 2:**
- Fan-out scoring, critical path analysis, value weights (simple ranking is enough)
- `kerf decompose` command (agents produce YAML manually, loader loads it)
- Area graph edges
- Cross-work bead dependency resolution tooling
- Session log YAML files / tiered filtering

**Data structures:**

```yaml
# spec.yaml (additions)
depends_on:
  - codename: brave-falcon
    relationship: co-designs         # NEW value alongside must-complete-first, inform
pinned: true                         # NEW — optional, set by kerf pin
pinned_at: "2026-05-10"             # NEW — for TTL computation
```

```yaml
# project.yaml (additions)
tools:
  task_tracker: beads               # gates all beads-specific behavior
beads:
  label_prefix: "work"
```

```json
// kerf next --json output
{
  "ranked": [
    {
      "codename": "brave-falcon",
      "title": "Adapter retry logic",
      "status": "implementing",
      "reason": "PINNED by user",
      "areas": ["adapter", "adapter.retry"],
      "task_status": {"total": 12, "closed": 7, "open": 3, "blocked": 2}
    },
    {
      "codename": "green-oak",
      "title": "Adapter observability",
      "status": "research",
      "reason": "oldest actionable, no dependencies differentiate",
      "areas": ["adapter"],
      "task_status": null
    }
  ]
}
```

**Independently useful?** Yes. `kerf next` with even simple ranking is better than manual selection. Beads status in `kerf map` closes the visibility gap between work-level and task-level state. The JSON output lets harmonik's orchestrator consume `kerf next` programmatically.

### Phase 3: The Full Pipeline

**Goal:** End-to-end flow from spec to loaded beads, with cross-work awareness and richer priority.

**Build:**
- `kerf decompose <codename>` — assembles context (spec, areas, mnem-maps), triggers agent decomposition, validates YAML, invokes loader
- Cross-work edge resolution — mnem-map management, forward-deferred edge reconciliation
- Fan-out scoring in `kerf next` — prioritize works that unblock the most downstream
- `kerf link <A> <B> --rel <type>` — create work-to-work relationships from CLI
- Late-requirement protocol embedded in `kerf new` overlap output — the stage-by-depth decision matrix
- Co-design check in `kerf square` — "this work has co-design relationships but no reconciliation recorded in SESSION.md"

**NOT in Phase 3:**
- Value weights on areas/goals (still not enough works to justify)
- Area graph edges (still not enough maintenance discipline)
- `kerf audit` (graph invariant checks)
- `kerf areas impact` (blast radius)
- Disruption protocol formalization
- Session log YAML migration

### Phase 4: Scale and Sophistication (when the graph is dense)

**Goal:** Priority computation and coordination at 20+ concurrent works.

**Build (only when observed need demands it):**
- Value weights on areas — inherited by works for strategic priority
- Area graph edges — typed directed relationships, adjacency warnings
- `kerf audit` — cycle detection, orphan checks, staleness warnings, area heat
- `kerf areas impact` — blast radius queries
- Session record migration to YAML — if markdown SESSION.md proves insufficient at scale
- Tiered session filtering — if 50+ sessions make the log unwieldy

---

## 5. Open Questions

These need real-world testing or Greg's explicit decision. The analyses can't resolve them.

**1. How does `kerf next` actually talk to beads?**
The interface is `br list -l work:<id> --json`. But: does kerf shell out to `br`? Use a Go library? Parse JSON output? What happens when `br` isn't installed? This is an implementation question that needs a concrete answer before Phase 2. Recommendation: shell out. It's the simplest coupling and matches kerf's filesystem-centric philosophy.

**2. Who runs the session-end protocol?**
If the agent is running through harmonik, does harmonik trigger `kerf handoff` at session end? Or does the agent need to remember? If the agent forgets, the SESSION.md entry is missing, and the next session has a gap. Recommendation: harmonik should call `kerf handoff` as part of its session teardown. Make it mechanical, not optional.

**3. How granular should areas be?**
"adapter" is coarse — two works in the same area might never conflict. "adapter.retry.exponential-backoff" is too fine — areas change with every implementation decision. Practical rule: if two works in the same area would never interact, the area is too coarse. If two areas always change together, they should be one area. This needs calibration from real use.

**4. What happens to works that are never finished?**
The archive station exists but there's no treatment of abandoned works. After 30 sessions with no activity, a work is probably dead. Should `kerf map` warn about stale works? Should there be a `kerf shelve` (already exists) vs. `kerf abandon` distinction? Defer until there are enough works to see the pattern.

**5. Multi-agent concurrency.**
Both critics noted this is unaddressed. Two agents working on area-overlapping works simultaneously is a real scenario when harmonik is dispatching. File reservation (harmonik's domain) handles code conflicts. But spec-level conflicts (two agents editing spec.yaml for related works) are unhandled. Recommendation: defer. Use harmonik's file reservation for files. Accept that spec-level conflicts will be caught by `kerf square` after the fact, not prevented in real-time.

**6. Does the computed orientation actually help?**
The hypothesis is that `kerf map` + `kerf resume` gives better context than HANDOFF. This is plausible but unproven. After Phase 1, do a deliberate comparison: have agents start sessions both ways and evaluate the quality of their work. If computed orientation doesn't measurably help, the whole coordination layer needs rethinking.

---

## 6. The kerf Evolution

kerf was a **spec-writing CLI**. A tool for individual works: create a work, walk through jig passes, produce artifacts, finalize to the repo.

kerf becomes a **work coordination system**. It still writes specs — that's the core. But it also holds the map of all work, computes what should happen next, enforces area coherence, and gives every agent session the context it needs to be locally correct and globally coherent.

This is a significant scope increase. Let's be honest about it:

**What's being added:**
- A project-level data model (areas.yaml, work status aggregation)
- Cross-work visibility (kerf map, overlap warnings)
- Priority computation (kerf next, pins, ranking)
- Session continuity (structured SESSION.md, computed orientation)
- Beads integration (status queries, YAML pipeline, cross-work edges)
- Agent protocols (session lifecycle, co-design, late requirements)
- 5-8 new commands across all phases

**What's NOT being added:**
- Execution (still harmonik)
- Task management (still beads)
- Project management (no boards, no velocity, no sprints)
- Automated conflict resolution (advisory, not gates)

**Is the scope increase warranted?**

Yes, but only if kerf is going to be used for projects with 5+ concurrent works managed by autonomous agents across multiple sessions. That's the use case where the coordination gap causes real failures — the concrete failure modes in the plan document are all real and all observed.

For a single developer manually running one work at a time through jig passes, the coordination layer is unnecessary overhead. kerf's existing commands are sufficient.

The bet is: Greg is building toward autonomous multi-agent SDLC. Harmonik dispatches agents. Beads tracks tasks. But nobody holds the map. kerf is the natural home for the map because it already owns the work concept and the spec artifacts. The coordination layer is what makes kerf the brain of the factory line instead of just a jig-pass runner.

The phased approach manages the risk. Phase 1 (areas + map + session records) is small, immediately useful, and validates the core hypothesis: does computed orientation beat HANDOFF? If it does, Phases 2-4 build on proven ground. If it doesn't, stop and reassess before investing further.
