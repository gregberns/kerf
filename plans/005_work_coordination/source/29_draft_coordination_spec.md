# Coordination

> Domain model, flow graph, agent interaction, and kerf as shared state layer.

## Overview

kerf coordinates work across independent agents through shared state. Agents do not communicate directly. Each reads kerf's state, acts, and writes results back. Coordination emerges from the state, not from messages.

This spec defines the domain concepts that make coordination possible, the flow graph that governs how work moves through the system, the agent roles that operate on the graph, and the blackboard pattern that ties them together.

## Relationship to Existing Concepts

The coordination layer builds on existing kerf concepts rather than replacing them:

| Coordination concept | Existing kerf concept | Relationship |
|---|---|---|
| Intent | [work](works.md) | An intent **is** a work. Every work is a reason for change with a codename, type, and lifecycle. |
| Design | jig pass artifacts | A design **is** the artifact set produced by a work's jig passes (e.g., problem space, components, spec draft). The design lifecycle maps to the work's status progression. |
| Task | bead (external, via bd) | A task is a bead. kerf generates task definitions during process jig passes; the beads system tracks execution state. |
| Area | (new concept) | A named region of the system. Not yet modeled in kerf. See [Areas](#areas) below. |
| Finding | (new concept) | A signal from downstream that re-enters the system as a new work. See [Findings](#findings) below. |
| Batch | (transient grouping) | A group of tasks dispatched together. Ephemeral — not stored in kerf. Assembled by the allocator from available tasks. |

The coordination layer adds two new first-class concepts (Area, Finding), one transient grouping (Batch), and a flow model over the existing work/jig/bead stack.

## Domain Concepts

### Intents Are Works

An intent is a reason for change — an idea, a bug, a gap, a discovered requirement. In kerf, an intent is a [work](works.md). It has a codename, a type, a jig, a status, and artifacts.

The intent lifecycle maps to work status:

```
captured  -->  designed  -->  tasked  -->  absorbed
   |              |              |            |
   v              v              v            v
work created   jig passes    tasks/beads   all derived
(spec.yaml)    complete      generated     tasks complete
```

An intent that enters through the feedback path (a finding) is still a work — created via `kerf new` with metadata indicating its origin.

Intents are the unit of planning. Multiple intents may produce tasks that interleave during execution. A single intent may produce tasks spanning multiple areas.

### Designs Are Jig Artifacts

A design is the set of specification artifacts that describe how an intent will be realized. In kerf, a design is the output of a work's jig passes — the markdown files produced during the spec-writing workflow.

The design lifecycle maps to jig progression:

```
drafting  -->  coherent  -->  sufficient  -->  frozen
   |              |               |               |
   v              v               v               v
early jig      checked         all passes      status at or
passes         against         complete,        past terminal
active         area peers      ready for        value; changes
               (see Areas)     task generation   require new work
```

Designs are immutable once a work's status reaches the terminal value in its `status_values` list. If reality diverges from a frozen design, that divergence is a new intent (a new work), not a modification to the existing design. This preserves the historical record and prevents retroactive coherence problems.

### Tasks Are Beads

A task is an atomic unit of implementation work. In kerf, tasks are beads managed by the external beads system (bd). kerf generates task definitions during process jig passes (e.g., the implementation jig's breakdown pass). The beads system tracks execution state.

Task lifecycle:

```
pending  -->  available  -->  claimed  -->  complete
                 |               |
              (unblocked)     (in-progress)
                                 |
                              failed --> [new corrective tasks]
```

Tasks carry traceability back to a design and transitively to an intent. Tasks have dependencies on other tasks, possibly from different intents. Tasks belong to one primary area.

The key separation: tasks are the unit of execution, but not the unit of planning. Planning produces intents and designs. The system derives tasks from designs. Execution operates on tasks grouped by availability and area, independent of which intent spawned them.

### Areas

An area is a named region of the system — a subsystem, module, layer, or interface boundary. Areas are the clustering mechanism that bridges planning and execution.

Properties:
- **Name** — stable identifier (e.g., `cli`, `jig-system`, `bench-storage`)
- **Connections** — which other areas this area interfaces with
- **Description** — what this area is responsible for

Areas are long-lived. They are created when the system map is defined and evolve slowly. An area may be split, merged, or retired, but these are rare structural changes.

Areas serve two roles:

1. **Planning coherence.** When multiple intents touch the same area, the system makes that visible during design. This is how design conflicts are caught early — the agent designing in an area sees what other work is in flight there.

2. **Execution grouping.** Tasks from different intents that share an area can be batched together for context efficiency. The area is the natural unit of agent context.

Intents touch areas. Tasks belong to areas. Designs reference areas. The area graph is the shared structure between planning and execution.

### Findings

A finding is something discovered during execution or testing that needs to flow back into the system — a bug, a gap, a contradiction, an unforeseen interaction. Findings are the feedback mechanism.

Properties:
- **Description** — what was found
- **Origin** — which task or verification activity surfaced this
- **Areas affected** — may span multiple areas and multiple original intents
- **Category** — determines how far upstream the finding must travel (see [Feedback Loops](#the-four-feedback-cycles))

Finding lifecycle:

```
surfaced  -->  triaged  -->  becomes new work
```

A finding, once triaged, becomes a new work (intent) that enters the system through `kerf new`. Findings are not backward movement — they are new inputs entering through a different door than human ideas, following the same flow once inside.

Three categories of findings determine routing:

| Category | Root cause | Resolution path | Planning required |
|---|---|---|---|
| A — simple fix | Code (wrong implementation) | Direct to tasks/beads | No |
| B — implementation gap | Tasks (wrong decomposition) | Lightweight work (bug jig) | Minimal |
| C — spec deficiency | Spec (wrong or missing) | Full planning cycle | Yes |

Category A findings may bypass the full work creation path and inject corrective beads directly. Categories B and C create new works with appropriate urgency signals.

### Batches

A batch is a group of tasks selected for execution together. Batches are assembled by the allocator from available tasks based on area coherence, dependencies, and priority.

Batches are **ephemeral**. They are not stored in kerf. A batch exists for the duration of a dispatch cycle — assembled, dispatched, forgotten. The tasks within a batch carry all necessary traceability; the batch itself is just a transient grouping.

A batch may contain tasks from multiple intents if those tasks share an area and are all available. The batch is an execution concept, not a planning concept. This is the separation between planning structure (intents group by problem) and execution structure (batches group by availability).

Batch completion is tracked through the individual tasks within it. When all tasks from an intent are complete (across however many batches they were distributed to), the intent is absorbed.

## The Flow Graph

### Activity Nodes

The system has five activities:

```
PLAN   --  think about what needs to change and why
SPEC   --  define precisely what to build
TASK   --  decompose the spec into atomic beads with dependencies
EXEC   --  an agent implements a bead
TEST   --  verify that implementation matches intent
```

These are activities the system performs, not stations where work sits. Multiple activities happen concurrently on different units of work.

In kerf terms: PLAN and SPEC correspond to a work's jig passes during spec-writing. TASK corresponds to the implementation jig's breakdown pass. EXEC and TEST correspond to bead execution and verification.

### Forward Path and Feedback Cycles

```
                      +-------------------------------------+
                      |            WIDE LOOP                |
                      |                                     |
                 +----+-------------------+                 |
                 |    |    MEDIUM LOOP     |                 |
                 |    |                    |                 |
            +----+----+--------+          |                 |
            |    |    | TIGHT  |          |                 |
            |    |    | LOOP   |          |                 |
            v    v    v        |          |                 |
 --IN-> PLAN --> SPEC --> TASK --> EXEC --> TEST --> OUT-->
                                   ^  ^      |
                                   |  |      |
                                   |  +------+
                                   |  REWORK LOOP
                                   |
                                   +-- FAST TRACK
                                       (known fix enters
                                        directly as task)
```

Work flows forward through the activity nodes. Feedback flows back through four cycle types of increasing radius.

### Intake Paths

Work enters the system at different points depending on how well-formed it is:

| Entry point | Condition | Example |
|---|---|---|
| PLAN | Vague idea, needs full cycle | "We should rethink auth" |
| SPEC | Well-formed requirement, approach known | "Add OAuth2 support per RFC 6749" |
| TASK | Known bug, clear fix | "Null check missing in handler.go:42" |
| EXEC | Trivial fix, bead created directly | "Fix typo in error message" |

This is one graph with four entry points, not four pipelines. The entry point reflects how much upstream thinking has already been done outside the system.

### The Four Feedback Cycles

All feedback loops follow the same principle: a signal from downstream re-enters the system at the point where the root cause lives, then flows forward through the remaining activities.

**Tight loop (EXEC <-> TEST).** The cause is a localized implementation error. The spec is correct; the code does not match it yet. Resolution: minutes. The same or next agent fixes the bead and re-tests.

**Rework loop (TEST -> TASK -> EXEC -> TEST).** The spec covers the behavior, but the task decomposition missed something. New beads are created within the existing spec. Resolution: an hour or less.

**Medium loop (TEST -> SPEC -> TASK -> EXEC -> TEST).** Testing reveals a gap in the spec itself — behavior never specified, or the spec assumed something false. Spec amendment required, then new tasks generated and executed. Resolution: hours.

**Wide loop (TEST -> PLAN -> SPEC -> TASK -> EXEC -> TEST).** The approach is wrong, not just the details. Full re-plan required. New or replaced spec sections, many new beads. Resolution: a day or more.

The determining factor is how far upstream the root cause lives:

```
Root cause in...        Loop type      Resolution time
code (wrong impl)       tight          minutes
tasks (wrong decomp)    rework         hour
spec (wrong/missing)    medium         hours
plan (wrong approach)   wide           day+
```

The agent at TEST (or EXEC) classifies the failure by type. The type determines the loop radius. This classification maps to finding categories: tight loops are Category A findings, rework loops are Category A or B, medium loops are Category B or C, wide loops are Category C.

## Agent Roles

### The Four Types

Four agent types operate on the system. Each reads and writes through kerf as shared state.

| Agent type | Activities | Lifecycle | Cadence |
|---|---|---|---|
| PLANNING | PLAN, SPEC, TASK | Interactive, user-driven | Sporadic |
| ALLOCATE | reads queue, dispatches | Persistent loop, stateless | Seconds to minutes |
| EXECUTE | EXEC | Spawned per-task, ephemeral | Duration of one bead |
| MERGE/TEST | TEST | Persistent loop, stateless | Minutes |

PLANNING covers three activities because they require coherent thinking and often happen in a single session. ALLOCATE is not an activity node — it is the mechanism that moves work from the queue to EXEC.

### Simplified Topology

In practice, ALLOCATE and MERGE/TEST run as a single coordinating agent. EXECUTE and TEST sub-activities run as subagents spawned by that coordinator.

```
+--------------------------------------------------+
|           COORDINATOR AGENT                      |
|  (performs ALLOCATE + MERGE roles)               |
|                                                  |
|   reads: kerf next, kerf map                     |
|   writes: findings (as new works), status        |
|                                                  |
|   spawns:                                        |
|     +------------------+  +------------------+   |
|     | EXEC subagent    |  | TEST subagent    |   |
|     | (per-task)       |  | (per-batch or    |   |
|     |                  |  |  periodic)       |   |
|     | reads: bead,     |  | reads: completed |   |
|     |   backing spec   |  |   code, specs    |   |
|     | writes: code,    |  | writes: test     |   |
|     |   bead status    |  |   results        |   |
|     +------------------+  +------------------+   |
+--------------------------------------------------+
```

The coordinator dispatches beads to EXEC subagents by passing instructions and a bead ID — not a full briefing. The bead itself contains everything the subagent needs. The coordinator kicks off TEST subagents periodically or after a batch completes to verify implemented work.

PLANNING operates independently, driven by user sessions. It reads the current state of the system via `kerf map` and `kerf resume`, produces works with specs and tasks, and exits.

### What Each Agent Reads and Writes

```
                       kerf (shared state)
                      +--------------------+
                      |                    |
  PLANNING ---------->| works (intents)    |<--------- COORDINATOR
    writes:           | jig artifacts      |            reads:
    works             |   (designs)        |            kerf next
    jig artifacts     | task definitions   |            kerf map
    task definitions  | areas              |
    areas             | findings           |
                      |   (as new works)   |
  COORDINATOR ------->|                    |<--------- PLANNING
    writes:           | bead status        |            reads:
    findings          |   (via bd)         |            kerf map
      (as new works)  |                    |            kerf resume
    bead status       +--------------------+
```

EXEC subagents mostly interact with the codebase and the beads system (bd), not with kerf directly. Their output enters kerf's world through bead state changes.

### The Seams

**PLANNING -> COORDINATOR seam.** PLANNING produces works with tasks. The coordinator reads `kerf next` to find available tasks. The seam is the queue computation — it composes kerf's work-level information (areas, dependencies) with the beads system's task-level information (readiness, completion state).

**COORDINATOR -> EXEC subagent seam.** The coordinator selects and dispatches. The subagent receives a bead ID and instructions. The bead references the backing spec so the agent can verify its work against intent. The coordinator sets the bead to in-progress before or at dispatch so it does not appear in subsequent `kerf next` results.

**EXEC -> COORDINATOR (TEST) seam.** EXEC subagents mark beads as implemented. The coordinator watches for completed beads and runs verification. The beads system is the coordination channel.

**COORDINATOR -> PLANNING seam.** The coordinator writes findings as new works in kerf. PLANNING reads them on next session via `kerf map`. For Category A and B findings, the coordinator can handle resolution without PLANNING involvement. Only Category C findings (spec deficiencies) truly require PLANNING attention and incur the "PLANNING is offline" latency.

Two paths for findings:
1. **Small findings (Category A/B):** The coordinator creates corrective tasks and dispatches them directly. The next items in the queue are the fixes.
2. **Large findings (Category C):** The coordinator saves a new work in kerf with high urgency. When the user next starts a PLANNING session, `kerf map` surfaces it prominently.

## The Blackboard

### kerf as Shared State

kerf is the shared state layer through which independent agents coordinate. It stores facts (works, designs, tasks, areas, findings, statuses). It computes views over those facts. It does not execute anything, dispatch anything, or communicate between agents.

More precisely: kerf maintains **the graph**. The nodes are works (intents), jig artifacts (designs), task definitions, areas, and findings. The edges are dependencies, area memberships, traceability links, and priority signals. Every agent reads a projection of this graph relevant to its role. Some agents write new nodes and edges.

### Projections

The graph is the thing. Commands are projections of it:

**`kerf map`** — the portfolio view. All works, their statuses, areas, dependencies, in-flight beads, urgency flags. Used by PLANNING to orient at session start. Used by the coordinator to understand what is in flight.

**`kerf next`** — the work queue. Given the current graph, what should an agent work on? Returns an ordered list of available tasks considering dependencies, area focus, and priority signals. This is the pull signal that drives execution.

**`kerf resume`** — the work context. For a specific work, everything an agent needs: spec, area peers, dependency status, session history, related findings.

### The Polling Model

Coordination is polling-based, not event-driven. Each agent queries kerf's state when ready for more work. There is no notification mechanism. This is consistent with the filesystem-as-database architecture.

Polling cadences differ by agent type:
- **Coordinator (ALLOCATE):** fast loop, seconds to minutes
- **Coordinator (MERGE/TEST):** moderate loop, minutes
- **PLANNING:** on-demand, when user starts a session
- **EXEC subagents:** do not poll — receive work and terminate

The consistency model is eventual consistency with human-speed convergence. Changes propagate at the speed of the polling loop. The system does not need millisecond consistency — it needs cycle-level consistency. Stale reads lead to suboptimal but not incorrect decisions. The next cycle self-corrects.

### What kerf Is Not

- **Not a message queue.** Items are not consumed by reading. Multiple agents can read the same state. `kerf next` is idempotent — running it ten times with no state changes produces the same result.
- **Not an orchestrator.** kerf does not dispatch work. The coordinator reads kerf and dispatches.
- **Not a lock manager.** kerf does not prevent concurrent access. Conflicts are resolved by convention and detection, not prevention.
- **Not a notification system.** kerf does not push updates. Agents poll.

### Guarantees

1. **Durability.** A work, once created, is never lost. Filesystem persistence provides this.
2. **Consistency at read time.** When an agent reads kerf state, the output reflects all changes flushed to disk. No caching, no stale views.
3. **Atomic writes.** Work creation (spec.yaml + artifacts) is atomic at the work level — the work directory's existence is the commit point.
4. **No ordering guarantees across agents.** kerf cannot guarantee that one agent's write happens before another's read. Every agent reads current state and makes the best decision with what it sees.
5. **Idempotent reads.** Views are computed, not consumed.

## Priority and Ordering

### The Pull Model

Execution is pull-based. Agents pull work when ready via `kerf next`. Everything upstream of the queue (PLAN, SPEC) is push — human thinking cannot be throttled. The queue absorbs the impedance mismatch between planning (push, bursty) and execution (pull, steady).

```
      PUSH                              PULL
 ideas enter regardless          agents pull when ready
 of downstream capacity

 PLAN ----> SPEC                 TASK ----> EXEC ----> TEST
 (human thinking                 (kerf next is the pull signal;
  cannot be throttled)            execution pulls from queue)
```

### Computed Priority

Priority is computed from graph structure, not assigned as static labels. Static labels (P0/P1/P2) rot. Priority derives from:

1. **Rework before new work.** Tasks born from findings (rework) have structural priority over tasks born from intents (new work). The queue computation distinguishes them by origin, not by a human-assigned label. Downstream issues take precedence over accepting new tasks from upstream.

2. **Completion momentum.** When most tasks from an intent are complete, the remaining tasks get priority. This prevents orphaned work — when four of five tasks are done, the fifth should not be stranded while tasks from another area are prioritized higher.

3. **Dependency fan-out.** Tasks that unblock the most downstream work rank higher. This is computed from the dependency graph.

4. **Area focus.** Prefer to finish work in an area before starting work in a new area. This reduces context switching for agents and avoids leaving areas in a partially-modified state.

These factors compose into a ranking that `kerf next` computes fresh on each invocation. No stored priority field. The ranking reflects the current state of the graph.

### The Queue

The queue is a computed view, not a stored entity. It is the ordered set of tasks that are:
- Not blocked by dependencies
- Not already in-progress
- Not complete

The queue is what `kerf next` returns. It incorporates dependency structure, area coherence, completion momentum, and rework priority. It composes information from kerf (work-level) with information from the beads system (task-level readiness).

## Feedback Loops

### How Findings Flow Back

When EXEC or TEST surfaces a problem, the coordinator classifies it by how far upstream the root cause lives. The classification determines the path:

**Category A (code-level fix):**
The coordinator creates corrective beads attached to the existing work. No new work item needed. The beads enter the queue with rework priority and are dispatched on the next cycle. This is the tight loop or rework loop.

**Category B (task/implementation gap):**
The coordinator creates a new work via `kerf new` with type `bug` and the affected area tags. The work goes through a compressed jig cycle (bug jig). The urgency signal causes `kerf next` to surface resulting tasks promptly.

**Category C (spec deficiency):**
The coordinator creates a new work via `kerf new` with high urgency and the affected area tags. This work requires PLANNING attention — a human or planning agent must think through the design implications. `kerf map` surfaces it prominently. Until addressed, the coordinator continues other work and avoids dispatching tasks in the affected area if possible.

### Design-Level Issues Are First-Class

Findings that reveal spec deficiencies (Category C) are not second-class citizens routed through an error-handling path. They enter the system as works with full traceability: which task surfaced them, which areas they affect, which existing designs they relate to. They appear in `kerf map` alongside planned work. They flow through the same jig process as any other work.

The system ensures that design-level issues do not get lost during the gap between MERGE/TEST discovering them and PLANNING addressing them. The work exists in kerf, visible to all agents, with urgency metadata that causes PLANNING to address it first when active.

### Invariants

1. **Every task traces to a design, every design traces to an intent.** There is no orphaned work. The system can always answer "why are we doing this?"

2. **The area graph is the coherence mechanism.** When multiple intents touch the same area, the system makes that visible during design. Design conflicts are caught before tasks are created.

3. **Tasks are the unit of execution, intents are the unit of planning.** These are different granularities with different grouping criteria. Execution grouping (batches by area) is independent of planning grouping (intents by problem).

4. **Findings are first-class inputs, not exceptions.** The feedback loop is a designed part of the system. Findings flow through the same pipeline as human ideas — they just carry urgency signals that affect queue position.

5. **Designs are frozen once tasks are derived.** If reality changes, that is a new intent, not a design modification.

6. **The queue is a live view, not a stored list.** It is computed from current task states, dependencies, and priority signals on each invocation.

## Concurrent Access

### Conflict Pairs

| Conflict pair | Risk | Resolution |
|---|---|---|
| PLANNING + EXECUTE | Low | Different write targets (specs vs code). `kerf map` shows in-flight work for awareness. |
| COORDINATOR + itself (ALLOCATE vs TEST) | Low | Single agent, sequential within its loop. |
| Two PLANNING sessions | High | Area overlap detection required. `kerf new` checks for in-flight work in the same areas and warns. |
| Two EXEC subagents | Medium | File reservation (outside kerf). kerf can help by surfacing area overlap in `kerf next` to avoid dispatching conflicting beads concurrently. |

The "two PLANNING sessions" case is the only one requiring active prevention. All others are handled by detection and self-correction on the next polling cycle.

### The "No Agent Is Running" State

Between sessions, all agents may be offline. kerf's state is files on disk. Nothing is lost. When any agent starts, it reads current state and acts. The blackboard persists regardless of who is reading it.
