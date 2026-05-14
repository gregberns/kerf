# Implementation Phases — Work Coordination

---

## Phase 1: Areas + Map + Next (the core cycle)

**Goal:** A user can tag work items with areas, see the full portfolio, and get an ordered list of what to work on next.

**What's IN:**
- **Area taxonomy** — flat list of areas defined in `areas.yaml` per project. Name + description. No hierarchy.
- **`spec.yaml` gains `areas` field** — list of area names this work item touches.
- **Overlap detection at `kerf new` time** — warn when creating a work item in an area that has other in-flight work.
- **`kerf map`** — portfolio view: all work items with their areas, status, and bead progress (queries `br` for counts).
- **`kerf next`** — ordered list of actionable beads. Computation: filter out blocked/complete, order by dependency depth (what unblocks the most), then completion momentum (in-progress epics pull their remaining beads forward). Simple, single-pass algorithm.
- **Basic beads integration** — `kerf next` shells out to `br` to get bead status (pending/in_progress/complete, epic membership, dependency edges). kerf does not own bead state.
- **`specs/areas.md`** (new) — defines areas, storage format, overlap detection behavior.
- **`specs/queue.md`** (new) — defines `kerf next` computation. Algorithm in one place. Hardcoded for now but written so parameters are obvious.
- Updates to `specs/works.md` (areas field), `specs/commands.md` (map, next), `specs/architecture.md` (areas.yaml location).

**What's NOT YET:**
- Findings / feedback loops (beads handle findings via tags — no kerf entity)
- Rework priority (structural rework-before-new ordering)
- Urgency signals, source tracking on work items
- Cross-work bead dependencies in the queue computation
- Configurable ordering parameters
- Coordination spec, agent protocols, agent type definitions
- Session continuity changes

**Depends on:** Existing kerf (works, jigs, beads integration via `br` CLI).

---

## Phase 2: Rework priority + queue refinement

**Goal:** The queue automatically prioritizes rework and finishing started work over new work, using real signals from beads.

**What's IN:**
- **Rework-before-new ordering** — beads tagged as rework/fix (by downstream agents) get structural priority in `kerf next`. The tag is on the bead, not a kerf entity.
- **`spec.yaml` gains `source` field** — provenance: `human`, `finding`, etc. Work items born from findings rank higher.
- **`spec.yaml` gains `urgency` field** — optional, affects queue ranking.
- **Epic pull refinement** — completion momentum algorithm accounts for epic size and proximity to done.
- **`kerf map` shows rework items** — distinguishes fix work from new work in the portfolio view.
- **`kerf square` gains area overlap check** — verification warns on conflicting designs in the same area.
- **Ordering parameters become configurable** — weights for rework priority, completion momentum, dependency depth. Stored in project config. Algorithm stays in one place.
- Update `specs/queue.md` with the full priority model.
- Update `specs/verification.md` with area overlap check.

**What's NOT YET:**
- Coordination model / agent type definitions
- Cross-work bead dependencies
- Finding categories (A/B/C) as a formal concept
- Agent protocols

**Depends on:** Phase 1.

---

## Phase 3: Coordination model + cross-work dependencies

**Goal:** kerf's spec layer formally describes how multiple agents (or one agent wearing multiple hats) coordinate through shared state, and the queue handles cross-work dependencies.

**What's IN:**
- **`specs/coordination.md`** (new) — the blackboard pattern, agent read/write patterns, the four seams, polling model. Descriptive, not prescriptive. No rigid policies.
- **`specs/domain-model.md`** (new) — the entity set (work item, area, queue as computed view, batch as ephemeral), relationships, lifecycles. Documents what exists; does not invent new stored entities.
- **Cross-work bead dependencies in queue** — `kerf next` respects dependency edges that span work items.
- **`kerf resume` gains area context** — shows co-design peers (other in-flight work in the same areas) and relevant rework items.
- **`kerf setup` updated** — generated agent config includes coordination context (what to read, what to write, how to use `kerf next`).
- Update `specs/commands.md`, `specs/cli.md`, `specs/dependencies.md`.
- Update `specs/_index.md` — glossary and spec map reflect all new specs.

**What's NOT YET:**
- Finding categories as formalized routing rules
- Area hierarchy (if ever needed)
- Batch durability (if ever needed)

**Depends on:** Phase 2.

---

## Phase 4: Tuning and edge cases (ongoing)

**Goal:** Refine ordering algorithm and coordination patterns based on real usage.

**What's IN:**
- Adjust queue weights based on observed dispatch patterns.
- Add filtering flags to `kerf next` (`--area`, `--limit`, `--exclude`).
- Formalize finding categories (A/B/C) if the pattern proves useful in practice.
- Any coordination patterns that emerge from multi-agent usage.

**Depends on:** Phase 3 + real-world usage data.
