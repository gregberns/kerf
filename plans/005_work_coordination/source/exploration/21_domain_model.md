# Domain Model — Work Coordination System

> First-principles modeling. Terminology chosen for clarity, not continuity with existing tools.

---

## 1. The Fundamental Things

### 1.1 Intent

**What it is:** A captured idea about something the system should do or become. Intents are born from human thinking, from test failures, from discovered gaps — anywhere a "this needs to change" signal originates.

**Properties:**
- Origin (human idea, test failure, execution discovery, dependency surfaced during planning)
- Description (what and why, at whatever fidelity exists)
- Scope estimate (is this a sentence or a chapter?)
- Area tags (which parts of the system does this touch?)

**Lifecycle:** `captured → designed → tasked → absorbed`

An intent starts as a raw signal. It gets designed (specs written). Design produces tasks. Once all tasks derived from it are complete, the intent is absorbed — the system has incorporated the change.

**Key insight:** Intents are not execution units. They are *reasons* for work. Multiple intents may produce tasks that interleave during execution. A single intent may produce tasks that span multiple system areas.

**Relationships:**
- An intent touches one or more **areas**
- An intent produces zero or more **tasks** (zero if still being designed)
- Intents can conflict with, extend, or depend on other intents
- An intent has a **design** (the spec artifacts that describe the change)

---

### 1.2 Design

**What it is:** The specification artifacts that describe how an intent will be realized. This is the structured thinking — the "measure twice" part. Plans, specs, analysis, component breakdowns.

**Properties:**
- The artifacts themselves (markdown files, structured documents)
- Completeness (is the design sufficient to produce tasks?)
- Coherence status (has this been checked against other designs touching the same areas?)

**Lifecycle:** `drafting → coherent → sufficient → frozen`

A design is drafted, then checked for coherence with other designs in overlapping areas, then deemed sufficient to generate tasks from, then frozen (changes after this point require a new intent, not modification of this design).

**Key insight:** Designs are immutable outputs. The kerf jig process produces them. Once a design reaches `frozen`, it becomes a historical record. If reality diverges, that's a new intent, not a design edit.

**Relationships:**
- A design belongs to exactly one intent
- A design references one or more **areas**
- A design produces **tasks**
- Designs touching the same area should be checked for coherence

---

### 1.3 Task

**What it is:** An atomic unit of implementation work. Something one agent can pick up and complete in one focused effort. This is what existing systems call a "bead."

**Properties:**
- Description (what to do)
- Area (which part of the system)
- Dependencies (which other tasks must complete first)
- Status (pending, in-progress, complete, failed, blocked)
- Origin (which design/intent produced this task)

**Lifecycle:** `pending → available → claimed → complete` (or `→ failed → [new corrective tasks]`)

**Key insight:** Tasks are the unit of execution, but they are NOT the unit of planning. Planning produces intents and designs. The system then derives tasks from designs. Execution operates on tasks, potentially from many different intents, grouped by criteria that have nothing to do with which intent spawned them.

**Relationships:**
- A task traces back to a **design** and transitively to an **intent**
- A task has dependencies on other **tasks** (possibly from different intents)
- A task belongs to one primary **area**
- Tasks are grouped into **batches** for execution

---

### 1.4 Area

**What it is:** A named region of the system. A structural concept — a subsystem, module, layer, or interface boundary. The system's map.

**Properties:**
- Name (stable identifier)
- Connections (which other areas does this area interface with)
- Description (what this area is responsible for)

**Lifecycle:** Areas are long-lived. They're created when the system map is defined and evolve slowly. An area may be split, merged, or retired, but these are rare structural changes.

**Key insight:** Areas are the clustering mechanism. They're how the system answers "what else is happening here?" When multiple intents touch the same area, that's a coherence signal. When tasks from different intents share an area, that's an execution grouping signal.

**Relationships:**
- Areas connect to other areas (the system graph)
- Intents touch areas
- Tasks belong to areas
- Designs reference areas

---

### 1.5 Batch

**What it is:** A group of tasks selected for execution together. Batches are the bridge between "what needs doing" and "what's being done right now." They are assembled by considering dependencies, area coherence, and priority — NOT by intent boundaries.

**Properties:**
- The tasks in the batch
- Ordering constraints (derived from task dependencies)
- Area focus (batches tend to cluster by area for coherence, but don't have to)
- Status (assembled, in-progress, complete)

**Lifecycle:** `assembled → dispatched → complete`

**Key insight:** Batches are ephemeral execution containers. They may contain tasks from 5 different intents if those tasks all touch the same area and are all available. The batch is NOT a design concept — it's an execution concept. This is the separation Greg identified: "why does execution have or need to have anything to do with how those ideas were generated?"

**Relationships:**
- A batch contains **tasks**
- A batch is dispatched to an execution agent
- A batch may produce **findings**

---

### 1.6 Finding

**What it is:** Something discovered during execution or testing that needs to flow back into the system. A bug, a gap, a contradiction, an unforeseen interaction. Findings are the feedback signal.

**Properties:**
- Description (what was found)
- Severity (how urgently does this need attention)
- Origin (which task/test/merge activity surfaced this)
- Areas affected (may span multiple areas — and may span multiple original intents)
- Type (bug, spec gap, design conflict, missing requirement)

**Lifecycle:** `surfaced → triaged → [becomes a new intent]`

A finding is surfaced by an execution or testing agent. It gets triaged (is this real? how urgent?). If it needs action, it becomes a new intent that enters the system at the top of the flow. High-severity findings get priority treatment.

**Key insight:** Findings are the feedback loop mechanism. They are NOT "backward movement" — they are new inputs entering the system through a different door than human ideas, but following the same flow once inside. A finding becomes an intent, which gets designed (possibly very quickly for a simple bug), which produces tasks, which get executed.

**Relationships:**
- A finding originates from execution of **tasks** or from testing/merge activities
- A finding may relate to multiple **areas** and multiple **intents**
- A finding becomes a new **intent** once triaged
- Findings carry priority signals that influence **queue** ordering

---

### 1.7 Queue

**What it is:** The ordered backlog of available tasks. Not a simple FIFO — it's a priority-ordered, dependency-respecting view of "what can be done next, and in what order."

**Properties:**
- Available tasks (those not blocked by dependencies)
- Ordering (derived from structural dependencies, area focus, priority signals, and finding severity)
- Filters (by area, by intent, by priority)

**Lifecycle:** The queue is a live, computed view. It doesn't have its own lifecycle — it reflects the current state of all tasks, dependencies, and priority signals.

**Key insight:** The queue is where the ALLOCATE agent looks. It answers `kerf next`. It incorporates dependency structure, area coherence (prefer to finish an area before starting another), and priority signals (findings-turned-intents that are high-severity get boosted). The queue does NOT use static priority labels (P0/P1/P2) — those rot. Priority is computed from structural position and explicit signals.

---

## 2. The Activities (Verbs)

### 2.1 Capture

**Actor:** Human or testing/merge agent
**Input:** An idea, a bug report, a discovered gap
**Output:** An intent
**What happens:** A raw signal is recorded in the system with enough information to be triaged and eventually designed.

### 2.2 Design

**Actor:** Planning agent (or human)
**Input:** An intent
**Output:** A design (spec artifacts)
**What happens:** The intent is analyzed, researched, and specified. This is the kerf jig process. Coherence checks happen here — if the intent touches areas where other designs exist, those are surfaced for consideration.

### 2.3 Task

**Actor:** Planning agent
**Input:** A completed design
**Output:** Tasks with dependencies and area assignments
**What happens:** The design is decomposed into atomic implementation units. Dependencies between tasks (including cross-intent dependencies) are identified. Tasks are tagged with areas.

### 2.4 Allocate

**Actor:** Allocate agent (or human via kerf commands)
**Input:** The queue (available tasks)
**Output:** A batch of tasks dispatched to an execution agent
**What happens:** Tasks are selected from the queue based on availability, area coherence, and priority. They're grouped into a batch and dispatched. The allocate agent doesn't need to understand the designs — it reads the queue and makes execution-ordering decisions.

### 2.5 Execute

**Actor:** Execution agent(s)
**Input:** A batch of tasks
**Output:** Completed tasks + findings (if any)
**What happens:** Each task in the batch is implemented. The agent works through them respecting dependency order. If something unexpected is discovered, a finding is surfaced.

### 2.6 Verify

**Actor:** Merge/test agent (or human)
**Input:** Completed tasks / implemented code
**Output:** Findings (or confirmation of correctness)
**What happens:** The implemented work is tested, reviewed, merged. Problems are surfaced as findings. This is where the feedback loop closes.

### 2.7 Triage

**Actor:** Human or allocate agent
**Input:** A finding
**Output:** A new intent (with priority signal) — or a dismissal
**What happens:** A finding is evaluated. If it needs action, it becomes a new intent with appropriate urgency. High-severity findings produce intents that jump toward the front of the queue once designed and tasked.

---

## 3. The Flows

### 3.1 The Main Flow (Happy Path)

```
Human idea
  → [capture] → Intent
    → [design] → Design (with coherence checks against areas)
      → [task] → Tasks (with cross-intent dependencies)
        → [allocate] → Batch
          → [execute] → Completed tasks
            → [verify] → Confirmed correct
```

### 3.2 The Feedback Loop

```
[execute] or [verify]
  → Finding surfaced
    → [triage] → New Intent (with priority signal)
      → [design] → Design (may be minimal for known bugs)
        → [task] → Tasks (high priority, enter queue near top)
          → [allocate] → Batch (picked up quickly)
            → [execute] → Fix implemented
              → [verify] → Confirmed
```

This is not "backward movement." It's a cycle. Findings enter at the same point ideas do — they just carry urgency signals that affect queue position.

### 3.3 The Coherence Check (Area Overlap)

```
Intent A touches Area X
Intent B (new) also touches Area X
  → [design] for B surfaces: "A also touches X"
    → Designer reviews A's design alongside B
      → B's design is coherent with A's
        → Tasks from both can safely interleave
```

This happens during the design phase. The area graph makes overlap visible before tasks are created.

### 3.4 The Cross-Cutting Bug

```
[verify] finds bug spanning Areas X, Y, and Z
  → Finding: "interaction between X and Y breaks Z"
    → [triage] → Intent (high severity, touches X, Y, Z)
      → [design] → may reference designs from original intents
        → [task] → tasks tagged to X, Y, Z with appropriate deps
          → [allocate] → batch prioritized due to severity
```

The bug doesn't need to "belong to" any single original intent. It becomes its own intent that touches whatever areas it touches.

---

## 4. The Invariants

1. **Every task traces to a design, every design traces to an intent.** There is no orphaned work. You can always answer "why are we doing this?"

2. **The area graph is the coherence mechanism.** If two intents touch the same area, the system makes that visible during design. This is how design conflicts are caught early.

3. **Tasks are the unit of execution, intents are the unit of planning.** These are different granularities with different grouping criteria. Execution grouping (batches) is independent of planning grouping (intents).

4. **Findings are first-class inputs, not exceptions.** The feedback loop is a designed part of the system, not an error-handling path. Findings flow through the same pipeline as human ideas.

5. **Priority is computed, not labeled.** Static labels (P0/P1) rot. Priority derives from: structural position in the dependency graph (what unblocks the most?), area focus (finish what's started), and explicit urgency signals (from findings or human input).

6. **Designs are frozen once tasks are derived.** If reality changes, that's a new intent, not a design modification. This preserves the historical record and prevents retroactive coherence problems.

7. **The queue is a live view, not a stored list.** It's computed from current task states, dependencies, priority signals, and area focus. It's always current.

---

## 5. System Boundaries

### Inside kerf (this system manages):
- Intents: capturing, storing, linking to areas
- Designs: the jig process, coherence checks
- Tasks: generating from designs, storing with dependencies and area tags
- Areas: the system map, area graph
- Queue: computing what's next, priority ordering
- Findings: receiving, storing, triage support

### Outside kerf (other tools manage):
- **Execution:** Agents actually implementing tasks (harmonik, direct agent sessions)
- **Task state tracking:** Bead-level status (the `br`/`bd` tool — kerf generates the task definitions, the bead system tracks execution state)
- **Agent session management:** Starting, stopping, managing agent processes
- **Code:** The actual codebase being built
- **Testing infrastructure:** Running tests, CI, merge operations

### The Interface:
- kerf exports tasks in a format the bead system can import (YAML with dependencies and edges)
- The bead system's status feeds back into kerf's queue computation (kerf needs to know what's done to compute what's available)
- Findings can be submitted to kerf by any agent through a capture mechanism
- `kerf next` is the primary interface for the allocate agent

---

## 6. The Planning/Execution Separation

Greg's observation: "Why does execution have or need to have anything to do with how those ideas were generated?"

The domain model captures this cleanly:

**Planning world** (intents, designs): Organized by *problem structure*. "These three things are all about the adapter" — they're grouped because they share a problem space. Planning agents think in terms of intents and designs.

**Execution world** (tasks, batches): Organized by *execution structure*. "These twelve tasks can all run in this order because their dependencies are satisfied and they're in the same area" — they're grouped because they can be done together effectively. Execution agents think in terms of tasks and batches.

**The bridge**: Tasks carry traceability back to designs and intents, but batches are assembled without regard to intent boundaries. The area graph is the shared structure between both worlds — planning uses it for coherence, execution uses it for grouping.

**The connection points:**
- Design → Task generation (planning produces execution inputs)
- Task completion → Intent absorption (execution signals planning that an intent is realized)
- Finding → Intent (execution feeds back into planning)
- Queue computation reads from both worlds (task availability from execution, priority signals from planning)

---

## 7. Cross-Cutting Concerns

### Multi-area intents
An intent can touch multiple areas. Its design must check coherence with existing designs in ALL touched areas. Its tasks are tagged to individual areas but the intent itself spans them. This is straightforward — the area tag is on the task, the multi-area nature is on the intent.

### Cross-intent task dependencies
Task A from Intent 1 may depend on Task B from Intent 2. This is a normal part of the task graph. The dependency exists at the task level, not the intent level. The bead system handles this via cross-work edges (already proven in the harmonik work).

### Findings that span intents
A finding might say "the interaction between what Intent 1 built and what Intent 3 built is broken." The finding becomes its own intent, touching the relevant areas. It doesn't need to "belong to" Intent 1 or Intent 3 — it's its own thing. Its design will reference the original designs for context.

### Late-arriving requirements
A new intent arrives that overlaps with an in-flight intent. The area graph makes this visible. Options:
1. If the in-flight intent's design is not yet frozen: update the design to incorporate the new requirement (they merge into one intent).
2. If the design is frozen but tasks aren't all complete: the new intent becomes a separate intent with explicit awareness of the first. Its design references the first's design. Its tasks may depend on the first's tasks.
3. If the first intent is fully absorbed: the new intent simply designs against the current state of the system.

The key: the area graph surfaces the overlap early. The decision of how to handle it is made during design, not during execution.

---

## 8. Mapping to Existing Concepts

For grounding — how these domain concepts relate to things that already exist:

| Domain concept | Current kerf term | Current external tool |
|---|---|---|
| Intent | "work" (partially) | — |
| Design | jig artifacts | — |
| Task | — | bead (br/bd) |
| Area | — (not yet modeled) | — |
| Batch | — | harmonik dispatch unit |
| Finding | — (handled ad hoc) | — |
| Queue | — | harmonik queue (partially) |

The biggest gaps in the current system: **Area** (no system map exists), **Finding** (no structured feedback path), and **Queue** (no computed priority ordering). The biggest conceptual shift: separating **Intent** (the planning unit) from **Task** (the execution unit) and not assuming they share boundaries.
