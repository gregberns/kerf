# Coordination

> How work moves through the system, how kerf supports multi-agent workflows, and how priority is computed.

## The Coordination Layer

kerf coordinates work across independent agents through shared state. Agents do not communicate directly. Each reads kerf's state, acts, and writes results back. Coordination emerges from the state — from what is visible on the board — not from messages between agents. kerf is the blackboard: it stores facts, computes views over those facts, and lets any agent read or write. It does not dispatch work, manage agents, or enforce protocols.

## How Work Flows

### The Five Activities

Work moves through five activities:

```
PLAN    think about what needs to change and why
SPEC    define precisely what to build
TASK    decompose the spec into atomic beads with dependencies
EXEC    implement a bead
TEST    verify that implementation matches intent
```

These are activities the system performs, not stations where work sits. Multiple activities happen concurrently on different units of work. In kerf terms: PLAN and SPEC correspond to a work item's jig passes during spec-writing. TASK corresponds to bead breakdown (e.g., the implementation jig's breakdown pass). EXEC and TEST correspond to bead execution and verification.

### Forward Flow and Feedback Loops

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
                                        directly as bead)
```

Work flows forward through the activities. When testing or execution surfaces a problem, the signal re-enters the system at the point where the root cause lives, then flows forward through the remaining activities. This is not backward movement — it is new information entering at the appropriate point.

The four feedback cycles differ by how far upstream the root cause lives:

| Root cause in | Cycle | Typical resolution |
|---|---|---|
| Code (wrong implementation) | Tight loop: EXEC <-> TEST | Minutes |
| Tasks (wrong decomposition) | Rework loop: TEST -> TASK -> EXEC -> TEST | An hour or less |
| Spec (wrong or missing) | Medium loop: TEST -> SPEC -> ... -> TEST | Hours |
| Plan (wrong approach) | Wide loop: TEST -> PLAN -> ... -> TEST | A day or more |

### Entry Points

Work enters the system at different points depending on how well-formed it is:

| Entry point | Condition | Example |
|---|---|---|
| PLAN | Vague idea, needs full cycle | "We should rethink auth" |
| SPEC | Well-formed requirement, approach known | "Add OAuth2 support per RFC 6749" |
| TASK | Known issue, clear fix needed | "Null check missing in handler.go:42" |
| EXEC | Trivial fix, bead created directly | "Fix typo in error message" |

This is one graph with multiple entry points, not separate pipelines. The entry point reflects how much upstream thinking has already been done.

### Findings Flow Through Beads

When execution or testing surfaces a problem — a bug, a gap, a contradiction — that signal is a **finding**. Findings are not a separate entity in kerf. They are tagged beads: a bead whose metadata indicates it surfaced an issue that needs attention. kerf can surface findings via `kerf next` by querying bead status and tags.

The classification of a finding determines what happens next:

- **Code-level fix.** Corrective beads are created within the existing work. They enter the queue with rework priority. This is the tight or rework loop.
- **Implementation gap.** A new work item is created (e.g., with the bug jig) covering the missing piece. Compressed planning cycle, then new beads.
- **Spec deficiency.** A new work item is created with the affected area tags. This requires full planning attention. `kerf map` surfaces it prominently so it is addressed when a planning agent is next active.

In all cases, findings flow through the existing work item and bead infrastructure. They carry traceability — which bead surfaced them, which areas they affect — but they do not require their own storage or lifecycle outside of beads and work items.

## kerf's Role: The Shared State Layer

kerf maintains a graph of work items, jig artifacts, areas, dependencies, and status. Every agent reads a projection of this graph relevant to its current task. Some agents write new nodes and edges.

### What kerf Maintains

- **Work items** — the unit of planning. Each has a codename, type, jig, status, and artifacts. See [works.md](works.md).
- **Jig artifacts** — the specification documents produced by a work's jig passes. These are the designs.
- **Areas** — named regions of the system (e.g., `cli`, `jig-system`, `bench-storage`). Areas serve two roles: planning coherence (multiple work items touching the same area are made visible during design) and execution grouping (beads in the same area can be worked together for context efficiency).
- **Dependencies** — between work items and between beads. These constrain ordering.
- **Status** — where each work item and bead stands in its lifecycle.

### What kerf Computes

The graph is the thing. Commands are projections of it:

- **`kerf map`** — the portfolio view. All work items, their statuses, areas, dependencies, in-flight beads. Used by any agent to orient: what exists, what is in progress, what is blocked, what needs attention.
- **`kerf next`** — the work queue. Given the current graph, what should be worked on? Returns an ordered list of available beads considering dependencies, area focus, and priority signals. This is the pull signal that drives execution.
- **`kerf resume`** — the work context. For a specific work item, everything an agent needs: spec artifacts, dependency status, session history, related area peers.

### What kerf Is Not

- **Not a message queue.** Items are not consumed by reading. `kerf next` is idempotent — running it ten times with no state changes produces the same result.
- **Not an orchestrator.** kerf does not dispatch work or manage agents. An agent (or a human, or a script) reads `kerf next` and acts on it.
- **Not a lock manager.** Conflicts are resolved by convention and detection, not prevention.
- **Not a notification system.** Agents poll kerf when ready. Coordination is polling-based, consistent with the filesystem-as-database architecture.

### Agent-Agnostic Design

kerf does not prescribe agent topology. One agent might do everything — plan, execute, test. Or there might be a planning agent, an allocation agent, several execution agents, and a testing agent. kerf provides the same operations either way: `kerf map` to see the state, `kerf next` to find available work, `kerf resume` to load context for a specific work item.

As an example, a team might use kerf with: a planning agent that creates work items and specs during interactive sessions, a coordinator script that polls `kerf next` and dispatches beads to worker agents, and a testing agent that verifies completed beads. But this topology is a choice made by the user, not something kerf imposes. kerf sees reads and writes to shared state — it does not care who is making them.

## Priority and Ordering

### The Pull Model

Execution is pull-based. Agents pull work when ready via `kerf next`. Everything upstream of the queue — PLAN, SPEC — is push: human thinking and planning happen regardless of downstream capacity. The queue absorbs the impedance mismatch.

```
      PUSH                              PULL
 ideas enter regardless          agents pull when ready
 of downstream capacity

 PLAN ----> SPEC                 TASK ----> EXEC ----> TEST
 (human thinking                 (kerf next is the pull signal;
  cannot be throttled)            execution pulls from queue)
```

### Computed Priority

Priority is computed from graph structure, not assigned as static labels. The factors that compose into the ranking:

1. **Rework before new work.** Beads born from findings (rework) have structural priority over beads born from new work items. Fixing what is broken takes precedence over starting something new.

2. **Completion momentum.** When most beads from an epic or work item are complete, the remaining beads get priority. This prevents orphaned work — when four of five beads are done, the fifth should not be stranded while beads from another area are dispatched.

3. **Dependency fan-out.** Beads that unblock the most downstream work rank higher. Computed from the dependency graph.

4. **Area focus.** Prefer to finish work in an area before starting work in a new area. This reduces context switching and avoids leaving areas in a partially-modified state.

These factors compose into a ranking that `kerf next` computes fresh on each invocation. No stored priority field. The ranking reflects the current state of the graph.

### The Ordering Algorithm

The ordering algorithm lives in one place in the codebase — the `kerf next` computation. The weights and parameters are expected to be configurable over time as real-world usage reveals optimal patterns. Some factors may initially be hardcoded; when they are, they should be obvious and localized so they can be extracted into configuration later.

#### Configurable Weights

Three scoring weights are read from `project.yaml` under a `queue:` section:

- `fan_out` — multiplier applied per transitive downstream dependent a work unblocks. Default: `10.0`.
- `momentum` — multiplier applied to the completed/total bead ratio (a work at 100% completion gets the full value added). Default: `5.0`.
- `creation` — small tiebreaker added per position from newest, favoring older works. Default: `0.1`.

```yaml
queue:
  fan_out: 10.0
  momentum: 5.0
  creation: 0.1
```

When the `queue:` section is absent, or any individual field is unset, the defaults above are used. Each field is independent — specifying `fan_out` alone leaves `momentum` and `creation` at their defaults.

### Batches Are Ephemeral

When an agent pulls multiple beads to work on together (e.g., beads in the same area), that grouping is a batch. Batches are ephemeral — assembled, worked, done. kerf does not store dispatch history or batch records. The beads themselves carry all necessary traceability; the batch is just a transient convenience.

## Integration Points

### kerf and Beads

kerf generates bead definitions during task breakdown (the TASK activity). The beads system (bd) tracks bead execution state — who claimed it, whether it is complete, whether it failed. kerf queries bead status to compute its views:

- `kerf next` needs to know which beads are available (not blocked, not in-progress, not complete).
- `kerf map` needs to know how many beads are done vs. remaining for each work item.
- Completion momentum requires knowing how close an epic is to done.

kerf reads bead status but does not own it. The beads system is the source of truth for bead lifecycle. kerf is the source of truth for work items, specs, areas, and the relationships between them.

### Information Flow

```
kerf                              beads (bd)
 |                                   |
 |-- work items, specs, areas ------>|  (bead definitions reference
 |                                   |   backing specs and areas)
 |                                   |
 |<-- bead status, completion -------|  (kerf queries to compute
 |                                   |   next, map, momentum)
```

The boundary is clean: kerf owns planning artifacts, beads owns execution state. The `kerf next` computation bridges both — it composes kerf's work-level information (areas, dependencies, priority signals) with the beads system's task-level information (readiness, completion state) to produce the ordered queue.
