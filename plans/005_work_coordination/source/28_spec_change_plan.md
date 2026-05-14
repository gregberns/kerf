# Spec Change Plan — Work Coordination

> What specs need to change, what new specs are needed, and in what order.

---

## Terminology Map: Old → New

Before listing files, here is how existing spec terminology maps to the system shape (doc 25).

| Existing term | New/evolved concept | Notes |
|---|---|---|
| work | Intent + Design (combined) | A "work" today conflates the reason-for-change (intent) with the spec artifacts (design). The new model separates them — but the work *directory* likely remains, now understood as the container for an intent and its design. |
| codename | Intent identifier | Unchanged mechanically. Still the directory name. |
| jig | (unchanged) | Jigs still govern the design process within a work. |
| pass | (unchanged) | Passes still structure jig workflows. |
| session | (unchanged) | Session tracking is unaffected. |
| status (on work) | Intent lifecycle + Design lifecycle | Today one status field. May need to represent two lifecycles or remain a single combined status. |
| — (not modeled) | Task | Maps to bead. kerf generates task definitions; bd tracks execution state. |
| — (not modeled) | Area | Named region of the system. New first-class concept. |
| — (not modeled) | Finding | Structured feedback from downstream. New concept. |
| — (not modeled) | Batch | Ephemeral execution grouping. Per Greg: not necessarily durable. |
| — (not modeled) | Queue | Computed view via `kerf next`. Not stored. |

---

## Spec Files — Ordered by Dependency

### 1. `specs/_index.md` — MODIFIED

**Purpose:** System overview, glossary, and spec map.

**Changes:**
- Update glossary: add entries for Intent, Design (as artifact), Task, Area, Finding, Batch, Queue
- Update or retire entries that shift meaning (work → intent+design container)
- Add new spec files to the spec map (domain-model, areas, findings, queue, coordination)
- Update system overview paragraph to reflect kerf as "the shared state layer through which independent agents coordinate" (doc 25 §5)
- Add key invariant: "Priority is computed, not labeled"
- Add key invariant: "The queue is a live view, not a stored list"
- Add key invariant: "Findings are first-class inputs, not exceptions"

**Dependencies:** All other spec files (this is the map)

**Complexity:** Medium — mostly additive, but the glossary rewrite requires care to not break existing terms that are still valid.

---

### 2. `specs/architecture.md` — MODIFIED

**Purpose:** Bench layout, project identity, global configuration.

**Changes:**
- Add `areas.yaml` (or `areas/` directory) to bench directory structure under each project
- Add finding storage location to bench directory structure
- Update `project.yaml` schema: add `areas` configuration (whether areas are enabled, default area definitions)
- Add `tools.tasks` to project.yaml to formally declare bd/beads integration
- Consider whether `config.yaml` needs new fields for queue computation settings or finding urgency defaults
- Document the area file format and location

**Dependencies:** None (foundational spec)

**Complexity:** Medium — structural additions to existing schemas, no conceptual difficulty.

---

### 3. `specs/domain-model.md` — NEW

**Purpose:** Defines the six entities (Intent, Design, Task, Area, Finding, Batch), the Queue computed view, their properties, lifecycles, and relationships.

**Changes (all new content):**
- Entity definitions with properties (from doc 25 §1)
- Relationship graph (Intent → Design → Task, Intent → Area, Task → Area, Finding → Intent, Task → Batch)
- Lifecycle state machines for each entity:
  - Intent: captured → designed → tasked → absorbed
  - Design: drafting → coherent → sufficient → frozen
  - Task: pending → available → claimed → complete (+ blocked, failed)
  - Finding: surfaced → triaged → becomes intent
  - Batch: assembled → dispatched → complete (ephemeral)
- The Intent/Batch distinction (planning concept vs execution concept)
- How current "work" maps to Intent + Design
- The planning/execution separation (doc 21 §6): intents group by problem, batches group by availability
- Invariants: every task traces to a design; area graph is the coherence mechanism; designs frozen after tasking; priority is computed

**Dependencies:** `specs/architecture.md` (for storage locations), `specs/works.md` (for backward compatibility with work concept)

**Complexity:** High — this is the conceptual core. Must be precise and normative. Large amount of new content derived from docs 21 and 25.

---

### 4. `specs/areas.md` — NEW

**Purpose:** The area graph — system map, area definitions, connections, and coherence checking.

**Changes (all new content):**
- What an area is: a named region of the system (subsystem, module, layer, interface boundary)
- Area properties: name, connections, description
- Area storage format (`areas.yaml` or directory structure under the project)
- The area graph: areas connect to other areas; the graph is the system's structural map
- How intents reference areas ("touches")
- How tasks belong to areas
- Coherence checking: when two intents touch the same area, surface the overlap during design
- Area overlap detection in `kerf new` and `kerf square`
- Co-design awareness: when a PLANNING agent works in an area, other in-flight work in that area is surfaced
- Area lifecycle: long-lived, rarely split/merged/retired
- Per Greg (doc 26): area reservation during concurrent planning is not a priority — detection is sufficient

**Dependencies:** `specs/domain-model.md` (entity definitions), `specs/architecture.md` (storage)

**Complexity:** Medium — well-defined concept, but the coherence-checking mechanics need careful specification.

---

### 5. `specs/works.md` — MODIFIED

**Purpose:** Work lifecycle, spec.yaml schema, codenames, status, types.

**Changes:**
- Reframe "work" as the container for an Intent and its Design artifacts
- Add `areas` field to `spec.yaml` schema (list of area names this work touches)
- Add `urgency` field to `spec.yaml` schema (for findings-turned-intents; optional, affects queue ranking)
- Add `source` field to `spec.yaml` schema (provenance: human, merge-test, etc.; optional)
- Add `related_to` field to `spec.yaml` schema (links to originating work/bead for findings)
- Update status discussion: the recommended status values may need to reflect the Intent lifecycle (captured → designed → tasked → absorbed) alongside or instead of jig-specific statuses. Resolve whether jig statuses and intent lifecycle are the same progression or parallel tracks.
- Add work types: `fix` (for finding-derived works)
- Update SDLC Work Patterns section: add the feedback loop pattern (finding → fix work → implementation)
- Reference `specs/domain-model.md` for entity semantics
- Reference `specs/areas.md` for area tagging

**Dependencies:** `specs/domain-model.md`, `specs/areas.md`, `specs/architecture.md`

**Complexity:** High — the spec.yaml schema changes ripple through the entire system. Must reconcile the existing jig-driven status model with the new intent lifecycle model without breaking existing workflows.

---

### 6. `specs/findings.md` — NEW

**Purpose:** Finding lifecycle, feedback injection, finding categories, and triage.

**Changes (all new content):**
- What a finding is: a signal from downstream (execution or testing) that needs to flow back
- Finding properties: description, severity, origin (which task/bead/test), affected areas, type (bug, spec gap, design conflict, missing requirement)
- Finding lifecycle: surfaced → triaged → becomes intent
- The three categories (from doc 24 §3):
  - Category A: simple bug fix — bypasses planning, goes directly to task/exec
  - Category B: implementation gap — lightweight planning (bug jig)
  - Category C: spec deficiency — requires full planning attention
- How findings enter kerf (likely via `kerf new` with urgency/source metadata, or a dedicated command)
- How findings affect queue priority (urgency signal)
- Per Greg (doc 26): not concerned about fast-path vs spec-first tension; agent instructions handle it
- Per Greg (doc 26): downstream issues/rework should be prioritized over new upstream tasks
- The feedback loop as a flow pattern, not an error-handling path

**Dependencies:** `specs/domain-model.md` (entity definitions), `specs/works.md` (work creation for findings), `specs/queue.md` (priority impact)

**Complexity:** Medium — the concept is well-defined in the source docs; the spec work is translating it into normative format and deciding the exact CLI surface.

---

### 7. `specs/queue.md` — NEW

**Purpose:** The queue computed view — what `kerf next` returns, how priority is computed, the pull model.

**Changes (all new content):**
- Queue is a computed view, not a stored entity
- Queue computation inputs: task availability (dependencies satisfied), area focus, urgency signals, finding severity, structural position in dependency graph
- Priority model: computed, not labeled. No static P0/P1/P2. Priority derives from:
  - Structural position (what unblocks the most downstream work)
  - Area focus (finish what's started — prefer tasks in areas with active momentum)
  - Urgency signals (findings-turned-intents with high severity)
  - Rework priority (tasks born from findings rank above new work — per Greg doc 26)
- The pull model: agents pull from the queue when ready. Push upstream (ideas enter regardless of capacity), pull downstream (execution pulls from queue).
- Queue filters: by area, by intent, by urgency
- How `kerf next` composes kerf-level information (work priorities, areas) with beads-level information (task readiness, completion state) — the integration seam
- Per Greg (doc 26): rework/downstream issues should be prioritized over new upstream tasks
- The sawtooth pattern: planning pushes work in batches, execution pulls it out steadily

**Dependencies:** `specs/domain-model.md`, `specs/areas.md`, `specs/findings.md`, `specs/works.md`

**Complexity:** High — the priority computation algorithm is the most complex new logic. Must be specified precisely enough to implement but flexibly enough to tune.

---

### 8. `specs/coordination.md` — NEW

**Purpose:** Multi-agent coordination through shared state — the blackboard pattern, agent types, seams, and concurrent access.

**Changes (all new content):**
- kerf as the blackboard: agents read/write shared state, no direct communication, coordination is emergent
- The four agent types and their read/write patterns:
  - PLANNING: reads areas/map/specs, writes intents/designs/tasks/areas
  - ALLOCATE: reads queue (`kerf next`), dispatches beads
  - EXECUTE: reads beads/specs, writes code/bead completion
  - MERGE/TEST: reads completed beads/specs, writes findings/status updates
- Per Greg (doc 26): ALLOCATE and MERGE/TEST are likely the same agent for now; EXEC and TEST are sub-agents within it
- Polling, not events — consistent with filesystem backing
- The four seams (PLANNING→ALLOCATE, ALLOCATE→EXECUTE, EXECUTE→MERGE/TEST, MERGE/TEST→PLANNING)
- Concurrent access patterns and conflict risks (from doc 24 §4)
- Consistency model: eventual consistency at polling-cycle boundary
- What kerf handles well (status visibility, priority, provenance, async handoff) vs poorly (urgency/interruption, negotiation)
- The flow graph with five activity nodes (PLAN, SPEC, TASK, EXEC, TEST)
- The four feedback loop types (tight, rework, medium, wide) and their characteristics
- Intake paths: work enters at different points based on how well-formed it is
- This spec is descriptive of the coordination model — it does NOT specify commands or data formats (those live in other specs)

**Dependencies:** `specs/domain-model.md`, `specs/queue.md`, `specs/findings.md`, `specs/areas.md`

**Complexity:** High — large amount of new conceptual content. Must be clear about what kerf does vs what is outside kerf's boundary (orchestration tools, beads system, harmonik).

---

### 9. `specs/commands.md` — MODIFIED

**Purpose:** All command specifications.

**Changes:**
- **`kerf` (no args):** Update output to include area map summary, finding count, active jig chain already specified
- **`kerf new`:** Add `--area` flag (tag work with areas at creation), add `--urgency` flag, add `--source` flag, add `--related-to` flag. Add overlap detection behavior (warn when creating work in an area with in-flight work). Update output to include area context.
- **`kerf next` (new or heavily reworked):** Specify as the primary queue interface. Must document: inputs to the ranking algorithm, output format (ordered list of actionable items), how it composes kerf data with beads data, filter flags (`--area`, `--limit`). This is the biggest command change.
- **`kerf map` (new or heavily reworked):** Specify as the portfolio view. Shows: all works by status, area overlap visualization, dependency graph, in-flight beads summary, urgency flags.
- **`kerf resume`:** Update output to include area context, related findings, and co-design peers.
- **`kerf show`:** Update output to include area tags, urgency, source, related findings.
- **`kerf list`:** May be superseded by or merged with `kerf map`. Decide relationship.
- **`kerf square`:** Add area overlap detection to verification checks.
- **`kerf status`:** Support new status values for intent/design lifecycles.
- **New command or flag for recording findings:** Either `kerf new --source merge-test --urgency high` or a dedicated `kerf finding` / `kerf report` command. Decide which.
- **`kerf init`:** Update to include area graph initialization (define initial areas for the project).
- **`kerf setup`:** Update generated agent config to include coordination model instructions, agent type roles.

**Dependencies:** All other specs (commands are the CLI surface for everything)

**Complexity:** High — many commands affected, `kerf next` and `kerf map` are substantial new specifications. This is the largest single spec change.

---

### 10. `specs/cli.md` — MODIFIED

**Purpose:** CLI design principles, output philosophy, agent-first design.

**Changes:**
- Update "What kerf Is Not" section: kerf is not an orchestrator, but it IS the shared state layer for coordination. Sharpen the boundary.
- Add principle: "kerf provides the intelligence (via computed views); orchestration tools provide the execution machinery"
- Update Agent Discovery section: agents discover their role (PLANNING, ALLOCATE, etc.) through `kerf setup` output
- Update Human-Agent Handoff Protocol: incorporate the finding feedback path (MERGE/TEST → kerf → PLANNING)
- Add a note about the polling model: kerf is polled, not push-based

**Dependencies:** `specs/coordination.md` (for the coordination model), `specs/commands.md` (for command behavior)

**Complexity:** Low — mostly additive clarifications to existing principles.

---

### 11. `specs/verification.md` — MODIFIED

**Purpose:** Square verification checks.

**Changes:**
- Add area overlap check: warn when works in the same area have potentially conflicting designs
- Add finding coverage check: warn when findings exist that haven't been triaged
- Add traceability check: every task should trace to a design and intent
- Reference `specs/areas.md` for area coherence semantics

**Dependencies:** `specs/areas.md`, `specs/domain-model.md`

**Complexity:** Low — adding new check types to existing framework.

---

### 12. `specs/dependencies.md` — MODIFIED

**Purpose:** Work dependencies, cross-project references.

**Changes:**
- Add cross-intent task dependencies: tasks from different intents can depend on each other
- Document how the dependency graph feeds into queue computation
- Add relationship type for findings: `originated-from` (finding traces back to a work)
- Reference `specs/queue.md` for how dependencies affect priority

**Dependencies:** `specs/domain-model.md`, `specs/queue.md`

**Complexity:** Low — extending existing dependency model.

---

## Specs NOT Changed

The following existing specs are not affected by this plan:

- `specs/sessions.md` — Session tracking is unchanged
- `specs/snapshots.md` — Snapshot mechanics are unchanged
- `specs/finalization.md` — Finalization process is unchanged (may need minor updates if areas affect finalization, but not a primary concern)
- `specs/testing.md` — Testing strategy is unchanged
- `specs/future.md` — May need updating to move items out of "future" if they're now being built, but this is a bookkeeping change
- `specs/jig-system.md` — Jig mechanics are unchanged
- `specs/jig-*.md` (individual jigs) — Individual jig definitions are unchanged, though new jig types may be needed later for finding-specific workflows

---

## Implementation Order

The dependency graph suggests this authoring order:

```
Phase 1 (foundations — no dependencies on new specs):
  1. specs/architecture.md          (storage changes)
  2. specs/domain-model.md          (conceptual core)

Phase 2 (new concepts — depend on domain model):
  3. specs/areas.md                 (area graph)
  4. specs/findings.md              (feedback system)

Phase 3 (computed views — depend on entities):
  5. specs/queue.md                 (priority computation)

Phase 4 (coordination model — depends on everything above):
  6. specs/coordination.md          (blackboard, agents, flows)

Phase 5 (surface changes — depend on the model specs):
  7. specs/works.md                 (schema updates)
  8. specs/dependencies.md          (dependency extensions)
  9. specs/verification.md          (new checks)
 10. specs/commands.md              (CLI surface)
 11. specs/cli.md                   (principles update)
 12. specs/_index.md                (glossary and map — last, once we know what exists)
```

---

## Key Decisions Needed Before Writing

1. **Does "work" remain the user-facing term?** The domain model introduces "intent" and "design" but the existing CLI uses "work" everywhere. Options: (a) keep "work" in CLI, map to intent+design internally; (b) rename to "intent" in CLI; (c) keep "work" but add "intent" and "design" as sub-concepts visible in output.

2. **Is `kerf next` a new command or a rework of `kerf list`?** Today `kerf list` shows works. The new `kerf next` shows actionable *tasks*. These are different granularities. Likely: `kerf list` stays (shows works/intents), `kerf next` is new (shows tasks from the queue), `kerf map` is new (shows the portfolio view).

3. **How does kerf integrate with bd/beads?** The queue computation needs bead state. Options: (a) kerf shells out to bd; (b) kerf reads bead files directly; (c) kerf maintains its own task state independent of beads. This affects `specs/queue.md` significantly. Per Greg (doc 26): "we need the basics, then don't worry about it."

4. **Finding ingestion: `kerf new` with flags or a dedicated command?** Using `kerf new --urgency high --source merge-test` is simpler (no new command). A dedicated `kerf finding` command is more discoverable. Decision affects `specs/commands.md`.

5. **Single status field or dual lifecycle tracking?** Today a work has one `status`. The domain model has separate Intent and Design lifecycles. Options: (a) one status field that maps to the combined lifecycle; (b) two fields (`intent_status` and `design_status`). Option (a) is simpler and more backward-compatible.
