# The System Shape — Synthesis

> Consolidation of the domain model (doc 21), flow dynamics (doc 22), manufacturing lens (doc 23), and multi-agent coordination (doc 24) into a single coherent picture.

---

## 1. The Domain

### The Entities

Four analyses, four slightly different entity sets. Here is where they agree and where they diverge.

**Agreed by all four analyses:**

| Entity | What it is | All docs agree? |
|--------|-----------|-----------------|
| Intent | A reason for change (idea, bug, gap) | Yes -- doc 21 defines it, docs 22-24 all assume it |
| Task | An atomic unit of execution (= bead) | Yes -- the unit every analysis operates on |
| Area | A named region of the system | Yes -- the clustering and coherence mechanism |

**Agreed by most, with tension:**

| Entity | Docs that use it | Tension |
|--------|-----------------|---------|
| Design | 21 (explicit), 22 (implicit as SPEC activity), 23 (implicit as Design station) | Doc 22 treats it as an activity, not a noun. Doc 21 treats it as an artifact. Both are correct -- the activity produces the artifact. Not a real conflict. |
| Finding | 21 (explicit), 22 (as feedback signal), 23 (as defect/rework trigger), 24 (as Category A/B/C injection) | All agree findings exist. Doc 24 adds the useful distinction: findings differ by how far upstream they need to travel. |
| Batch | 21 (explicit), 23 (as execution unit), 24 (implicit in ALLOCATE dispatch) | Doc 22 doesn't use the word "batch" -- it talks about concurrent flows. The tension: is a batch a first-class entity or just a transient grouping? See below. |

**Used by one analysis only:**

| Entity | Which doc | Assessment |
|--------|----------|------------|
| Queue | 21 | Doc 21 calls it an entity. Docs 22-24 treat it as a computed view. The computed-view interpretation wins -- Greg said "I don't like anything that manages sessions or state kerf doesn't own." A queue-as-entity implies stored state. A queue-as-view is just `kerf next`. |

### Resolved: The Entity Set

The system has **six fundamental things:**

```
INTENT ---- a reason for change (the "why")
DESIGN ---- the specification artifacts (the "what, precisely")
TASK ------- an atomic execution unit (the "do this")
AREA ------- a named region of the system (the "where")
FINDING ---- a signal from downstream (the "wait, this is wrong")
BATCH ------ a transient group of tasks dispatched together (the "work packet")
```

And one **computed view**, not an entity:

```
QUEUE ------ the ordered set of available tasks (what kerf next returns)
```

### Relationships

```
                    touches
          INTENT ─────────── AREA
            │                  │
        produces           belongs to
            │                  │
            v                  v
          DESIGN            TASK ←──── derived from ──── DESIGN
            │                │  │
         frozen           grouped      depends on
         after             into           │
         tasks               │            v
                             v          TASK (cross-intent ok)
                          BATCH

          FINDING
            │     \
         becomes   affects
            │       \
            v        v
         INTENT    AREA (possibly multiple)
```

### The Intent/Batch Distinction

Doc 21 says Intent and Batch are fundamentally different. This holds up across all four analyses.

- **Intent** is a planning concept. It groups by *problem*: "these are all about the adapter."
- **Batch** is an execution concept. It groups by *availability*: "these tasks are all unblocked, in the same area, and ready now."

A batch may contain tasks from five different intents. An intent's tasks may be spread across ten batches executed over days. Greg's observation drives this: "Why does execution have or need to have anything to do with how those ideas were generated?"

The area is the bridge. Planning uses areas for coherence ("two intents touch the same area -- check for conflicts"). Execution uses areas for grouping ("these tasks are in the same area -- batch them for context efficiency").

### Lifecycles

```
INTENT:   captured --> designed --> tasked --> absorbed
                                                 ^
                                                 |
              (all derived tasks complete) ------+

DESIGN:   drafting --> coherent --> sufficient --> frozen
                                                    |
              (changes after this = new intent) ----+

TASK:     pending --> available --> claimed --> complete
              |                       |
              +-- blocked             +-- failed --> [new corrective tasks]

FINDING:  surfaced --> triaged --> becomes intent (re-enters at top)

BATCH:    assembled --> dispatched --> complete (ephemeral)
```

---

## 2. The Flow Graph

### The Activity Nodes

Doc 22 names five activities. Doc 23 names seven stations. Doc 21 names seven verbs. Here is the reconciliation:

Doc 22's five activities are the right level. Doc 23's "Intake" and "Archive" are boundary events (entry/exit), not sustained activities. Doc 21's seven verbs map cleanly onto the five activities plus intake and triage.

```
Activities:    PLAN    SPEC    TASK    EXEC    TEST
               think   define  decomp  build   verify
```

Intake is an entry point (work enters). Archive is an exit point (work leaves). Triage is a sub-activity within PLAN when processing findings. None of these need to be top-level nodes in the flow graph.

### The Graph with Cycles

```
                          +-------------------------------------+
                          |            WIDE LOOP                |
                          |                                     |
                     +----+-------------------+                 |
                     |    |    MEDIUM LOOP     |                 |
                     |    |                    |                 |
                +----+----+--------+           |                 |
                |    |    | TIGHT  |           |                 |
                |    |    | LOOP   |           |                 |
                v    v    v        |           |                 |
 --IN-->  PLAN --> SPEC --> TASK --> EXEC --> TEST --> OUT-->
                                     ^  ^       |
                                     |  |       |
                                     |  +-------+
                                     |  REWORK LOOP
                                     |
                                     +-- FAST TRACK
                                         (known bug enters
                                          directly as task)
```

### The Four Cycle Types

All four analyses agree on the existence of feedback loops. Doc 22 names four types. Doc 23 (manufacturing lens) agrees but frames them differently -- as "defect routing by type." The two framings compose:

| Cycle | Doc 22 name | Doc 23 framing | Root cause location | Resolution time |
|-------|-------------|---------------|-------------------|-----------------|
| EXEC <-> TEST | Tight loop | Rework at station | Code (wrong impl) | Minutes |
| TEST -> TASK -> EXEC | Rework loop | Missing operation | Tasks (wrong decomp) | Hour |
| TEST -> SPEC -> TASK -> EXEC | Medium loop | Design defect | Spec (wrong/missing) | Hours |
| TEST -> PLAN -> SPEC -> TASK -> EXEC | Wide loop | Process defect | Plan (wrong approach) | Day+ |

Doc 23 adds the jidoka principle: the agent at TEST (or EXEC) must classify the failure by type. The type determines the loop radius. This is not "backward movement" -- it is new information entering the system at the appropriate point, flowing forward through the remaining activities.

**How rework and new work compose (the key tension):**

Doc 23 says: "rework before new work." Doc 22 says: "feedback loops have different latencies." These compose as follows:

- Rework tasks get structural priority in the queue (doc 23's rule)
- But rework tasks still flow through the same activities as new work (doc 22's observation)
- The priority is not a label -- it is a property of origin. Tasks born from findings are rework. Tasks born from intents are new work. The queue computation distinguishes them by type, not by a human-assigned label.

### Intake Paths

Work enters the system at different points depending on how well-formed it is:

```
Vague idea          --> PLAN (full cycle)
Well-formed req     --> SPEC (skip planning)
Known bug, clear fix --> TASK (skip planning + spec)
Trivial fix         --> EXEC (direct fast track)
```

This is not four different pipelines. It is one graph with four entry points. The entry point is determined by how much upstream thinking has already been done outside the system.

### Where Pull Applies and Where It Does Not

Doc 23 is precise about this:

```
          PUSH                              PULL
  <-- ideas enter regardless -->   <-- agents pull when ready -->
       of downstream capacity

  PLAN ----> SPEC                  TASK ----> EXEC ----> TEST
  (human thinking                  (kerf next is the pull signal;
   cannot be throttled)             execution pulls from queue)
```

Pull starts at the queue. Everything upstream of the queue is push (or at best, loosely paced). This is fine -- TPS does not require pull at every station, only at the constraint. The constraint is agent execution capacity.

The sawtooth pattern (doc 22) is a consequence:

```
Work in system
  ^
  |    /\        /\        /\
  |   /  \      /  \      /  \
  |  /    \    /    \    /    \
  | /      \  /      \  /      \
  |/        \/        \/        \
  +--------------------------------> time
   plan  exec  plan  exec  plan
   push  pull  push  pull  push
```

Planning pushes work into the system in batches. Execution pulls it out steadily. The amplitude of the sawtooth is the planning batch size. The frequency is the planning cadence.

---

## 3. The Agents

### The Four Types

Doc 24 defines four agent types. Doc 22 defines five activities. Here is the mapping:

```
Agent type       Activities it performs    Lifecycle
-----------      ---------------------    ---------
PLANNING         PLAN, SPEC, TASK         Interactive, sporadic, user-driven
ALLOCATE         (reads queue, dispatches) Persistent loop, stateless
EXECUTE          EXEC                     Spawned per-task, ephemeral
MERGE/TEST       TEST                     Persistent loop, stateless
```

PLANNING covers three activities (PLAN, SPEC, TASK) because they require coherent thinking and often happen in a single session. ALLOCATE is not an activity node -- it is the mechanism that moves work from the queue to EXEC. It reads `kerf next` and dispatches.

### What Each Agent Reads and Writes Through kerf

```
                    kerf (shared state)
                   +-------------------+
                   |                   |
   PLANNING ------>| intents           |<------ reads: ALLOCATE
     writes:       | designs           |         (via kerf next)
     intents       | tasks             |
     designs       | areas             |<------ reads: MERGE/TEST
     tasks         | findings          |         (via kerf map)
     areas         |                   |
                   |                   |
   MERGE/TEST --->| findings           |<------ reads: PLANNING
     writes:       | status updates    |         (via kerf map,
     findings      |                   |          kerf resume)
     status        +-------------------+
                          ^
                          |
                    EXECUTE writes:
                    bead completion status
                    (mostly through beads system,
                     not kerf directly)
```

### The Seams

The critical boundaries between agent types:

**PLANNING -> ALLOCATE seam:** PLANNING produces tasks with dependencies and area tags. ALLOCATE reads the queue (`kerf next`). The seam is the queue computation -- it must compose kerf's work-level information (priority, area, urgency) with the beads system's task-level information (readiness, completion state).

**ALLOCATE -> EXECUTE seam:** ALLOCATE selects and dispatches. EXECUTE receives a bead with everything it needs. kerf's role is minimal here -- the bead itself is the interface. But the bead must reference the backing spec so the agent can verify its work against intent.

**EXECUTE -> MERGE/TEST seam:** EXECUTE marks beads complete. MERGE/TEST polls for completed beads. The beads system is the coordination channel. kerf provides area context for testing ("this bead belongs to a work that touches area X").

**MERGE/TEST -> PLANNING seam:** MERGE/TEST writes findings to kerf. PLANNING reads them on next session. This is the feedback injection path -- the hardest seam, because PLANNING is sporadic (user-driven) while findings may be urgent.

Doc 24's three finding categories matter here:

```
Category A: simple bug fix     --> bypasses PLANNING, goes to TASK/EXEC
Category B: implementation gap --> lightweight PLANNING (bug jig)
Category C: spec deficiency    --> requires full PLANNING attention
```

Only Category C truly crosses the MERGE/TEST -> PLANNING seam and incurs the "PLANNING is offline" latency. Categories A and B can be handled by ALLOCATE dispatching through compressed paths.

### The Blackboard Pattern

Doc 24 describes kerf as a blackboard. Doc 22 describes information flowing in all directions through a broadcast layer. These are the same idea:

- Agents do not communicate directly
- Agents read shared state, act, write results back
- Coordination is emergent from the state, not from messages
- Polling, not events -- consistent with filesystem backing

The blackboard pattern has a specific weakness: urgency. If MERGE/TEST finds a critical issue, it writes to the blackboard. But no one knows until they next look at the blackboard. Doc 24 acknowledges this and calls it acceptable: "the worst case is one ALLOCATE cycle of suboptimal dispatching." With cycle times in seconds-to-minutes, this is fine for all but truly catastrophic failures.

---

## 4. The Dynamics

### Steady-State Patterns

**Pipeline filling (project start):**
All work is at PLAN/SPEC. No feedback loops exist yet. Risk: over-planning in a vacuum. Mitigation: execute a thin slice early to establish feedback before planning everything. Doc 23's "walking skeleton" pattern.

```
PLAN: ||||||||    SPEC: ||||    TASK:     EXEC:     TEST:
        busy         busy       empty     empty     empty
```

**Balanced flow (mid-project):**
Work at every stage. Feedback loops active. New planning incorporates lessons from testing. This is the healthy state.

```
PLAN: |||    SPEC: |||    TASK: |||    EXEC: ||||||||    TEST: |||
       some        some        some        busy               some
```

**Crisis (major finding):**
Wide feedback loop activated. Feature flow pauses. All attention goes to the fix cycle. The system enters recovery mode.

```
PLAN: ||||||||    SPEC: ||||||||    TASK: ||||    EXEC: ||    TEST: blocked
        surge          surge          surge        draining     waiting
```

**Wind-down (project end):**
No new planning. Queue draining. Most activity is at TEST with tight/rework loops for remaining issues.

```
PLAN:     SPEC:     TASK: |    EXEC: |||    TEST: ||||||||
  empty     empty     few       finishing       busy
```

### The Sawtooth and Crisis Interaction

The sawtooth (planning pushes, execution pulls) is the normal heartbeat. A crisis interrupts it:

```
Work in system
  ^
  |    /\     /\   CRISIS
  |   /  \   /  \   |
  |  /    \ /    \  v  /----\
  | /      X      \/  / fix  \
  |/               +-+       +--/\     /\
  +-----------------------------------> time
                    ^         ^
                    |         |
                crisis     return to
                starts     normal flow
```

During crisis, the sawtooth flattens because planning switches from new work to fix planning. The fix cycle is short (tight/rework loops) so the "wave" is smaller and faster. Return to normal flow re-establishes the original sawtooth.

### Pull vs. Push Over Time

Early in a project, the system is push-dominated (lots of planning, nothing to pull). As execution ramps up, pull dominates the downstream half. In steady state, both coexist: push upstream, pull downstream, with the queue as the boundary.

The manufacturing lens (doc 23) says this boundary is exactly right. You cannot throttle human ideas (push is natural upstream). You can and should throttle execution (pull prevents overproduction downstream). The queue absorbs the impedance mismatch.

### Concurrent Access Under Load

Doc 24 analyzes the conflict pairs. The key insight: most conflicts are benign because the system is eventually consistent at the polling-cycle boundary.

```
Conflict pair              Risk    Why it's ok
PLANNING + EXECUTE         Low     Different write targets (specs vs code)
MERGE/TEST + ALLOCATE      Medium  One stale cycle, self-corrects
Two PLANNING sessions      High    Area overlap detection needed
EXECUTE + EXECUTE          Medium  File reservation needed (outside kerf)
```

The "two PLANNING sessions" case is the only one that requires active prevention. All others are handled by detection + self-correction on the next cycle.

---

## 5. What kerf IS

Three metaphors from three analyses:

- Doc 21: "a structured filesystem with computed views"
- Doc 23: "the production control board" (kanban board)
- Doc 24: "the blackboard" (blackboard architecture)

**These are the same thing described at different levels of abstraction.**

The filesystem is the implementation: facts stored as files.
The blackboard is the coordination pattern: agents read/write shared state.
The production control board is the operational metaphor: making the invisible visible.

kerf is **the shared state layer through which independent agents coordinate**. It stores facts (intents, designs, tasks, areas, findings, statuses). It computes views over those facts (what's next, what's the map, what's the context for this work). It does not execute anything. It does not dispatch anything. It does not communicate between agents.

More precisely: kerf is the system that maintains **the graph** -- the nodes are intents, designs, tasks, areas, and findings; the edges are dependencies, area memberships, traceability links, and priority signals. Every agent reads a projection of this graph relevant to its role. Some agents write new nodes and edges. The graph is the single source of truth about what the system is doing, has done, and needs to do.

`kerf next` is a view: "given the current graph, what should an agent work on?"
`kerf map` is a view: "given the current graph, what does the portfolio look like?"
`kerf resume` is a view: "given the current graph, what does this specific work look like?"

The commands are projections. The graph is the thing.

---

## 6. Open Questions

### Unresolved Tensions

**1. Batch: entity or ephemeral grouping?**
Doc 21 lists Batch as a first-class entity with a lifecycle. Doc 22 doesn't mention batches -- it talks about concurrent flows. Doc 24 treats batch as something ALLOCATE creates transiently. If batches are ephemeral (assembled, dispatched, forgotten), they don't need to be stored in kerf. If they're durable (for tracking what was dispatched together, for post-mortems), they do. Greg hasn't weighed in on this.

**2. The ALLOCATE agent: agent or script?**
Doc 24 raises this directly. If ALLOCATE is just `while true; do kerf next | dispatch; sleep 30; done`, it's a script, not an agent. The "intelligence" is entirely in `kerf next`'s ranking algorithm. If ALLOCATE needs to reason about batch composition (group by area, respect changeover costs, apply heijunka), it's an agent. This affects where complexity lives -- in kerf's queue computation or in ALLOCATE's reasoning.

**3. Bead lifecycle for verification.**
Today beads go: open -> closed. If MERGE/TEST is a real agent, beads need: open -> implemented -> verified -> closed. Who owns this lifecycle extension -- kerf or the beads system (bd)? Doc 24 flags this as the critical integration question.

**4. The fast path vs. spec-first principle.**
Category A findings (simple bug fixes) want to bypass the full plan/spec/task cycle and inject beads directly. This violates the spec-first principle ("never write code not backed by a spec"). Is a finding with a clear fix effectively its own micro-spec? Or does the spec-first principle need an exception for trivial fixes? Greg's Prime Directive says fix the spec first. But for a one-line bug fix, creating a full work item feels like over-processing (muda #4 from doc 23).

**5. Area reservation during concurrent planning.**
Doc 24 suggests a reservation mechanism for areas during PLANNING. But reservations require cleanup when sessions crash, and kerf doesn't manage sessions. Is area overlap detection (after-the-fact warning) sufficient? Or does concurrent planning need prevention, not just detection? The manufacturing lens doesn't help here -- factories don't have two engineers redesigning the same station simultaneously.

**6. How does "rework before new work" interact with heijunka?**
Doc 23 says rework gets structural priority (finish what's started). Doc 23 also says spread work across areas (heijunka, prevent tunnel vision). These conflict when the rework is concentrated in one area while new work spans others. Which principle wins? Probably rework (fix defects first is deeper than spread work evenly), but Greg should confirm.

### Needs Greg's Input

- Is the six-entity model right? Specifically: does Batch need to be durable, or is it just "whatever ALLOCATE dispatches this cycle"?
- Is the ALLOCATE agent an agent with reasoning, or a thin script over `kerf next`?
- Does the spec-first principle have an exception for trivial fixes (Category A findings)?
- When rework-priority and area-diversity conflict, which wins?
