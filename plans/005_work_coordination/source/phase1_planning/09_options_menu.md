# Options Menu — Work Coordination & Portfolio Coherence

> Decision-ready output from the Plan 005 brainstorming exercise.
> Six agents brainstormed, two synthesized. This is the curated menu.

---

## 1. Executive Summary

Six independent perspectives converged on a single core insight: **the five problems in the plan document are symptoms of one gap — agents start sessions without computed, structured context about the work landscape.** The information to fix this largely exists already in spec.yaml files; it just isn't assembled or surfaced at the right moments. The highest-leverage interventions are small: one new field in spec.yaml, two new commands, and richer output from an existing command. The brainstorming produced 18 distinct ideas; only 3-6 of them are worth building, and only 3 are needed to cover all five problems.

---

## 2. The Convergence

These points had near-unanimous agreement (5-6 out of 6 agents):

- **Area tags are the foundation.** An `areas` list field in spec.yaml enables everything else — overlap detection, clustering, orientation output. Without it, the system is blind to which works are related. Every agent proposed this independently.

- **Computed orientation replaces HANDOFF for structure.** A command that reads all active works and emits a structured snapshot (status, dependencies, area clusters, what's actionable) is the single highest-leverage feature. HANDOFF.md becomes a supplement for judgment calls, not the primary orientation mechanism.

- **Overlap warnings should be advisory, not gates.** kerf tells you about conflicts; you decide what to do. No blocking, no mandatory reconciliation passes, no two-phase commits. This matches kerf's existing "guidance not gates" philosophy.

- **Compute on demand, don't materialize a graph file.** The spec.yaml files are the source of truth. Reading 5-20 YAML files is fast. A cached graph file introduces staleness bugs and a second source of truth.

- **Don't become Jira.** Kanban boards, velocity metrics, trajectory charts, and stage-based WIP limits are project management features. kerf helps agents write better specs — that's the test for every new feature.

---

## 3. Tiered Options

### Tier 1: Minimum Viable Coordination

The smallest package that addresses all five problems. One new spec.yaml field, two new commands, one enhanced existing command.

#### A. `areas` field in spec.yaml + overlap warnings

**What it is.** Add an `areas` list field to spec.yaml — freeform strings like `adapter`, `auth`, `database`. At `kerf new` time, check area tags of existing active works and warn on overlap. At `kerf resume` time, show area peers.

```yaml
# in spec.yaml, alongside existing fields
areas:                                  # list<string>, optional, mutable
  - adapter
  - auth
```

```
$ kerf new --type plan --title "Circuit breaker for adapter"
Created: bold-crane (plan)

⚠ Area overlap detected:
  brave-falcon (status: implementing) also touches: adapter
  green-oak (status: research) also touches: adapter
  Consider reviewing their specs before proceeding.
```

**Problems addressed:** P2 (islands), P4 (late requirements), P5 (coherence). Works declare what they touch; overlap is surfaced automatically at the earliest moment.

**Cost to build:** Small. One new optional field in the spec.yaml schema. A directory walk + YAML parse + set intersection on `kerf new` and `kerf resume`. Suggest existing tags at creation time to reduce drift.

**What it doesn't solve:** No persistent map (P1). No queue or prioritization (P3). No formal protocol for handling overlaps once detected.

#### B. `kerf map` — computed portfolio view

**What it is.** A new command that reads all active works for the current project, builds the dependency/area graph, and emits a structured text document. Works grouped by status, dependency edges shown, area clusters highlighted, actionable items flagged, blocked items explained.

```
$ kerf map

# Portfolio: acme-webapp (6 active works)

## By Status
  implementing:  brave-falcon "Adapter retry logic"
  research:      green-oak "Adapter observability"
  problem-space: bold-crane "Circuit breaker"
  problem-space: red-wave "Auth token rotation"
  ready:         calm-river "DB index migration"
  shelved:       dark-pine "Legacy API removal"

## Dependency Graph
  bold-crane → brave-falcon (must-complete-first)
  calm-river → brave-falcon (inform)

## Area Clusters
  adapter: brave-falcon, green-oak, bold-crane  ← 3 works, review together
  auth:    red-wave
  database: calm-river

## Actionable Now
  1. green-oak (research) — no blockers
  2. red-wave (problem-space) — no blockers
  3. bold-crane (problem-space) — blocked by brave-falcon (implementing)

## Blocked
  bold-crane — waiting on brave-falcon to complete
```

**Problems addressed:** P1 (no map), P3 (partially — shows what's actionable), P5 (area clusters make coherence visible).

**Cost to build:** Medium. Directory walk, YAML parse, graph construction, text rendering. No new storage. All data from existing spec.yaml files. The computation is O(n) where n = number of works; trivial for realistic portfolios.

**What it doesn't solve:** No computed priority ranking (just "actionable" vs "blocked"). No formal handling of late requirements. No shared design constraints.

#### C. Enhanced `kerf resume` with dependency and area context

**What it is.** When resuming a work, show live status of its dependencies and works sharing the same areas. No new command — richer output from the existing `kerf resume`.

```
$ kerf resume bold-crane

Resuming: bold-crane "Circuit breaker" (problem-space)
Jig: plan | Next pass: analyze

Dependencies:
  brave-falcon (must-complete-first) — status: implementing ⚠ not yet complete

Area peers (adapter):
  brave-falcon "Adapter retry logic" — implementing
  green-oak "Adapter observability" — research
  → Review these before designing. They share your adapter surface area.

SESSION.md loaded. Last session: 2026-05-06.
```

**Problems addressed:** P1 (work-level map), P2 (shows related works), P5 (coherence prompt).

**Cost to build:** Small. Pure read-path enhancement. Resolve depends_on to spec.yaml, read status. Find area peers, format output.

**What it doesn't solve:** Portfolio-level view (that's `kerf map`). Priority ranking.

**Tier 1 total cost:** One optional spec.yaml field (`areas`), one new command (`kerf map`), enriched output on two existing commands (`kerf new`, `kerf resume`). No new data model, no new storage, no new concepts.

**Tier 1 coverage:** All five problems get at least visibility. P1 and P2 are directly solved. P3 is partially addressed (actionable list in `kerf map`). P4 gets early detection via overlap warnings. P5 gets visibility but not enforcement.

---

### Tier 2: Solid Foundation

Tier 1 plus three additions that turn visibility into actionable workflow.

#### D. `kerf next` — computed work selection

**What it is.** Reads the dependency graph and statuses, emits a ranked list of actionable works. Ranking: explicit `priority` field (if set) > fan-out (unblocks the most downstream works) > creation date. Add an optional `priority` integer field to spec.yaml for human override.

```yaml
# in spec.yaml, optional
priority: 1                             # integer, optional, mutable. Lower = higher priority.
```

```
$ kerf next

Recommended next work:
  1. green-oak     (research)    — unblocks: 1 work   reason: highest fan-out
  2. red-wave      (problem-space) — unblocks: 0     reason: oldest actionable
  3. bold-crane    (problem-space) — ⚠ blocked by brave-falcon
```

**Problems addressed:** P3 (no queue) — directly and completely.

**Cost to build:** Small. Topological sort + simple scoring. The dependency resolution logic already exists in kerf. Add one optional integer field to spec.yaml.

**What it doesn't solve:** It ranks by graph structure, not business value. The `priority` field is the escape hatch for when mechanical ranking is wrong.

#### E. `co-designs` relationship type

**What it is.** A third relationship type alongside `must-complete-first` and `inform`. Meaning: "these works must be designed with mutual awareness — read each other's artifacts before making design decisions." Bidirectional. Surfaces prominently in `kerf resume` and `kerf map`.

```yaml
depends_on:
  - codename: brave-falcon
    relationship: co-designs              # new value
```

**Problems addressed:** P2 (islands), P4 (late requirements), P5 (coherence). Makes architectural entanglement explicit in the data model, not just implied by shared area tags.

**Cost to build:** Small. One new string value in the relationship enum. Display logic in `kerf resume` and `kerf map`. The overlap warning at `kerf new` can suggest it: "Consider `kerf link bold-crane brave-falcon --rel co-designs`."

**What it doesn't solve:** Coherence enforcement — it's still advisory. An agent can see the co-designs link and ignore it.

#### F. Late-requirement handling via existing commands + guidance

**What it is.** Not a new command or data structure — a documented protocol for the three resolution paths when a new requirement overlaps with in-flight work:

1. **Amend:** Add to the in-flight work's spec directly. Use when the new requirement is small and the work hasn't passed its design phase.
2. **Spawn dependent:** `kerf new` + `kerf link --rel co-designs`. Use when the new requirement is substantial enough for its own work but needs design synchronization.
3. **Pause and replan:** Shelve the in-flight work, create a new work that encompasses both, re-derive from the combined spec. Use when the overlap is so deep that independent design is impossible.

The overlap warnings from Tier 1 surface the moment this decision is needed. The `co-designs` relationship from Tier 2 records the decision. No new tooling required — just documented guidance that agents follow.

**Problems addressed:** P4 (late requirements) — directly.

**Cost to build:** Zero code. Documentation in the spec, possibly as part of a jig's pass guidance.

**What it doesn't solve:** It can't prevent an agent from ignoring the protocol. But neither can any tooling short of hard gates, which violate kerf's philosophy.

**Tier 2 total additions over Tier 1:** One new command (`kerf next`), one new relationship type (`co-designs`), one optional spec.yaml field (`priority`), documented guidance for late requirements. Still no new storage, no new concepts beyond what kerf already has.

---

### Tier 3: Ambitious

The full vision. Build these if Tiers 1-2 prove insufficient for larger portfolios.

#### G. Area specs / shared design anchors

**What it is.** When 2+ works touch the same area and design decisions need to be shared, create a lightweight constraint document at the project level: `~/.kerf/projects/{project-id}/.areas/adapter.md`. Not a full spec — a short list of constraints that individual works in that area must conform to ("adapter uses repository pattern," "all adapter errors are wrapped in AdapterError"). Surfaced automatically by `kerf resume` for any work tagged with that area.

**Problems addressed:** P5 (coherence) — the strongest mechanism. Turns implicit "these should be consistent" into explicit "here are the rules."

**Cost to build:** Medium. New directory convention, display logic in `kerf resume` and `kerf map`, creation/editing workflow.

**Speculation level:** Well-understood pattern (DDD shared kernel, design system tokens). The risk is maintenance — stale area specs mislead. Auto-archive when no active works touch the area.

#### H. `kerf audit` — graph invariant checks

**What it is.** A command that checks structural health of the work portfolio: dependency cycles, orphaned blockers (depending on a shelved/finalized work), stale works (no session activity in N days), area heat warnings (too many concurrent works in one area).

```
$ kerf audit

✓ No dependency cycles
✓ No orphaned blockers
⚠ Stale work: dark-pine — no activity for 14 days
⚠ Area heat: adapter has 3 active works (threshold: 2)
```

**Problems addressed:** P1 (map health), P2 (area heat), P5 (structural coherence).

**Cost to build:** Small-medium. Cycle detection is DFS, orphan check is edge traversal, staleness is timestamp comparison. The graph computation already exists from `kerf map`.

**Speculation level:** Low — well-understood algorithms. Risk is false positives for small portfolios. Needs threshold tuning.

#### I. WIP limits

**What it is.** A `wip_limit` setting in project config. `kerf new` warns when the number of active (non-shelved) works exceeds the limit. Advisory only.

```
$ kerf new --type plan --title "Yet another thing"
⚠ WIP limit: 5 active works (limit: 3). Consider finishing or shelving before starting new work.
Created: swift-maple (plan)
```

**Problems addressed:** P3 (queue discipline), and indirectly P2/P5 (fewer concurrent works = less coordination needed).

**Cost to build:** Tiny. One config field, one count check.

**Speculation level:** Low technically, high culturally. Only works if the human respects it. The contrarian's strongest point: if you WIP-limit to 3, most coordination problems disappear — but spec-writing has a different WIP cost model than implementation.

---

## 4. Key Decisions

### Decision 1: Command naming — `kerf map` vs. `kerf orient` vs. enhanced `kerf list`

**Options:**
- **`kerf map`** — New dedicated command for the portfolio view. Clean separation from existing commands.
- **`kerf orient [codename]`** — Dual-purpose: portfolio view (no arg) and work-specific briefing (with arg). Proposed by multiple agents.
- **Enhanced `kerf list`** — No new command. Add flags to `kerf list` for richer output (`--graph`, `--areas`, `--actionable`).

**Trade-offs:** `kerf map` is the clearest mental model — one command, one job. `kerf orient` tries to serve two audiences (portfolio and work-specific) which makes its output harder to design. Enhanced `kerf list` avoids new commands but overloads an existing one.

**Recommendation:** `kerf map` for the portfolio view. Keep `kerf resume` for work-specific context (already enhanced in Tier 1). Don't overload `kerf list` — it should stay simple. Two commands doing two things beats one command doing everything.

### Decision 2: Area tags — freeform vs. defined taxonomy

**Options:**
- **Freeform strings.** No validation, no predefined list. Suggest existing tags at creation time.
- **Defined taxonomy.** An `areas.yaml` file per project listing valid area names. Tags must match.
- **Hybrid.** Freeform, but `kerf areas` shows all tags in use so drift is visible.

**Trade-offs:** Freeform is zero-friction but drifts (`adapter` vs `adapters` vs `http-adapter`). A taxonomy is precise but requires curation and blocks agents from tagging freely. The hybrid gets most of the precision benefit without the friction.

**Recommendation:** Freeform with the hybrid mechanism. Suggest existing tags at `kerf new` time. Add `kerf areas` (or `kerf map --areas`) to list all tags in use. A formal taxonomy is premature — you don't know what areas matter until works start using them.

### Decision 3: How many relationship types?

**Options:**
- **Keep two.** `must-complete-first` and `inform` are sufficient. Area tags + overlap warnings provide the "these are related" signal.
- **Add one: `co-designs`.** A bidirectional "design together" relationship for architectural entanglement.
- **Add several.** `co-designs`, `supersedes`, `amends`, `blocks` (distinct from must-complete-first). The systems architect proposed six.

**Trade-offs:** Each new type requires schema changes, display logic, behavioral semantics, and agent guidance on when to use it. Diminishing returns with each addition. But `co-designs` fills a genuine gap — there's currently no way to say "these are bidirectionally entangled" as opposed to "A should know about B."

**Recommendation:** Add `co-designs` only. Hold everything else until a concrete failure demands a new type. Area tags + overlap warnings handle most of the signaling that additional relationship types would provide.

### Decision 4: Should `kerf next` exist, or is it part of `kerf map`?

**Options:**
- **Standalone `kerf next`.** Focused command that answers one question: "what should I work on?"
- **Section in `kerf map`.** The "Actionable Now" section of `kerf map` serves the same purpose.
- **Both.** `kerf map` shows the full picture; `kerf next` is the quick-answer shortcut.

**Trade-offs:** A standalone command is cleaner for agent consumption (parse one thing, not a whole document). But it's also another command to maintain and document.

**Recommendation:** Build it as a section in `kerf map` first. If agents frequently need just the "what's next" answer without the full portfolio context, extract it to `kerf next` later. Start with fewer commands.

### Decision 5: Build Tier 1 only, or Tier 1+2 together?

**Options:**
- **Tier 1 only.** Ship it, use it, see what's still painful.
- **Tier 1+2 together.** The `co-designs` relationship type and `priority` field are cheap; ship them with Tier 1.

**Trade-offs:** Tier 1 alone may leave P3 (queue) and P4 (late requirements) feeling underserved. But Tier 2's additions are cheap enough to add later without rework.

**Recommendation:** Tier 1 first. The `priority` field and `co-designs` relationship are cheap to add later and don't require rework of Tier 1. Use Tier 1 for a few sessions and see which problems persist. If P3 is painful, add `priority` + the actionable ranking. If P4 is painful, add `co-designs`.

---

## 5. What NOT to Build

**Event sourcing / append-only log.** Sounds architecturally elegant. Creates a second source of truth competing with spec.yaml. Consistency between them is a new bug class. The filesystem already provides what's needed — read spec.yaml files on demand.

**Materialized work graph file.** A `workgraph.yaml` that's recomputed on every mutation. It's a cache with staleness bugs. Compute on demand; for 5-20 works the cost is negligible. Only revisit if external tools need the graph without calling kerf.

**Kanban board / terminal board view.** Agents can't use a column layout. `kerf map` as structured text serves both agents and humans. The board is a rendering concern, not a coordination concern.

**Velocity / trajectory metrics.** Knowing "0.4 works/day" doesn't help an agent orient or detect overlap. This is dashboarding, not coordination.

**Typed hypergraph with cohorts.** The full graph-theory formalism (hyperedges, weighted relationships, synchronization semantics) is overengineered. The useful kernel is: works have area tags, shared areas form implicit clusters. You don't need hyperedge formalism for that.

**Two-phase commit for clustered works.** Introduces a synchronization bottleneck that violates "guidance not gates." One slow work blocks all works in its cluster.

**Automated semantic conflict detection.** Detecting "contradictory design decisions" between works is AI-complete. kerf should detect adjacency (shared areas) and surface it. Evaluating compatibility is the agent's job.

**Decision log / ADR-style records.** Another artifact to maintain. Decisions live in the spec artifacts. A parallel log means decisions in two places that drift.

**SQLite / query index.** Technically correct that the filesystem is slow for portfolio queries. Strategically wrong at current scale. Introduces index-vs-filesystem consistency bugs. Resist until 50+ concurrent works.

---

## 6. Suggested Sequence

### Phase 1: Foundation (one plan, one spec update)

1. **Add `areas` field to spec.yaml schema.** Update `specs/works.md` — add the field, define it as `list<string>`, optional, mutable, empty list default.
2. **Add overlap warnings to `kerf new`.** Update `specs/commands.md` — when creating a work, check existing active works for shared area tags, emit advisory warning.
3. **Enhance `kerf resume` with area peer display.** Update `specs/commands.md` — show dependency status and area peers when resuming.
4. **Add `kerf map` command.** Update `specs/commands.md` — new command, reads all active works, emits structured portfolio view with status groups, dependency graph, area clusters, actionable/blocked lists.

Dependencies: Step 1 must come first (everything else uses area tags). Steps 2-4 can be implemented in parallel once the schema is updated.

### Phase 2: Prioritization (if P3 proves painful)

5. **Add optional `priority` field to spec.yaml.** Update `specs/works.md`.
6. **Add ranked actionable section to `kerf map`** (or standalone `kerf next`). Update `specs/commands.md`. Ranking: priority > fan-out > age.

### Phase 3: Entanglement (if P4/P5 prove painful)

7. **Add `co-designs` relationship type.** Update `specs/dependencies.md` (or wherever relationship types are defined). Bidirectional, surfaces in `kerf resume` and `kerf map`.
8. **Document late-requirement protocol.** Could live in the jig guidance or in a dedicated spec section. Three paths: amend, spawn-dependent, pause-and-replan.

### Phase 4: Hardening (if portfolio grows)

9. **`kerf audit`** — invariant checks (cycles, orphans, staleness, area heat).
10. **Area specs** — shared design anchors for hot areas.
11. **WIP limits** — advisory warning at `kerf new` when active work count exceeds threshold.

Each phase is independently valuable. Phase 1 is the only one that should be committed to upfront. Phases 2-4 are triggered by observed pain, not planned in advance.
