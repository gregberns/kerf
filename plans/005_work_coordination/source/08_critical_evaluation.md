# Critical Evaluation — Work Coordination Brainstorm

> Stress-testing the ideas, ranking by leverage, finding gaps, identifying what to actually build.

---

## Top 10 Ideas Ranked by Leverage

Scoring: Impact (1-5), Feasibility (1-5), Adoption (1-5, where 5 = zero friction). Leverage = I x F x A.

### 1. Area Tags on spec.yaml (Arch #1, DX #3, Pract #2)

**Impact: 5** | **Feasibility: 5** | **Adoption: 4** | **Leverage: 100**

An `areas` list field in spec.yaml. Freeform strings. This is the single most important idea across all six documents because nearly every other idea depends on it. Without area tags, overlap detection is impossible, clustering is impossible, and orientation is crippled.

**Stress test:** Will agents apply tags consistently? Mostly yes, because: (a) the field is trivial to populate, (b) `kerf new` can suggest existing tags, and (c) the value is immediately visible in overlap warnings. Tag drift (`adapter` vs `http-adapter`) is real but manageable -- suggest existing tags at creation, accept approximate grouping. The Contrarian's point #10 (track `affects` for spec files) is a sharper version of this same idea and could coexist: areas for conceptual grouping, `affects` for concrete spec-file targeting. But areas alone get you 80% of the value.

**Failure mode:** Tags applied retroactively after overlap damage is done. Mitigate by prompting at `kerf new` time.

---

### 2. `kerf orient` / `kerf map` -- Computed Portfolio View (Arch #10, DX #1/#2, Pract #1/#10)

**Impact: 5** | **Feasibility: 4** | **Adoption: 5** | **Leverage: 100**

A command that reads all active works, builds the graph from spec.yaml files, and emits a structured orientation document. The Contrarian nails why this is high-leverage: all five problems are symptoms of "agents start sessions without enough context." This is the fix.

**Stress test:** The data to compute this already exists in spec.yaml files (status, depends_on, and with area tags added above). The computation is a directory walk + YAML parse + graph assembly. For 5-20 works, this takes milliseconds. No index needed. No new storage. The output format matters -- it needs to be dense enough for agent context windows (200-400 lines max) but complete enough for orientation.

**Failure mode:** Output too long for large portfolios. Mitigate by filtering to active works only, with a `--scope` flag for historical.

**Note:** `kerf orient <codename>` (work-specific briefing) and `kerf map` (portfolio-level view) are the same computation at different zoom levels. Build one, derive the other. I'd start with the portfolio view (`kerf map`) since it subsumes the work-specific view.

---

### 3. Overlap Warnings at `kerf new` (DX #7, Pract #4, Arch #8)

**Impact: 4** | **Feasibility: 5** | **Adoption: 5** | **Leverage: 100**

When creating a work, check area tags of existing active works and warn on overlap. Zero new commands, zero behavior change. The information appears exactly when you need it, without being asked.

**Stress test:** Requires area tags (idea #1). Trivial to implement once tags exist -- iterate active works, intersect tag lists. Warning fatigue risk is low because overlap at creation time is genuinely useful signal. The DX perspective's "pit of success" framing is right: the agent doesn't need to know to check, kerf tells it.

**Failure mode:** Areas not tagged yet at creation time (agent doesn't know what the work touches until partway through). Acceptable -- the warning fires when it can and is silent when it can't. Not worse than today.

---

### 4. Enhanced `kerf resume` with Dependency/Area Context (DX #11)

**Impact: 4** | **Feasibility: 5** | **Adoption: 5** | **Leverage: 100**

When resuming a work, show live status of dependencies and area peers. No new commands. No behavior change. Just richer output from an existing command.

**Stress test:** This is a pure read-path enhancement. Read the work's depends_on, resolve each to its spec.yaml, read status. Read area tags, find peers. Format and emit. The hardest part is the "Impact" annotation (what the dependency means for this work), which should be omitted from MVP -- just show status.

**Failure mode:** Negligible. Worst case, extra output that agents skim past. Best case, agents catch blocking dependencies and overlap they'd otherwise miss.

---

### 5. `kerf next` -- Computed Work Selection (Arch #4, DX #4, Pract #3)

**Impact: 4** | **Feasibility: 4** | **Adoption: 4** | **Leverage: 64**

Given the dependency graph and current statuses, emit a ranked list of actionable works. Mechanical: unblocked + earliest-stage first, tie-break by fan-out (unblocks the most).

**Stress test:** Requires dependency graph traversal. Topological sort is O(V+E), trivial at portfolio scale. The ranking heuristic (fan-out > age) is transparent and sensible. The Practitioner is right that a `priority` integer field in spec.yaml is the correct escape hatch for human override -- it should be optional and rare.

**Failure mode:** Recommendations are wrong for the user's actual priorities. But the output is a suggestion, not a gate. The user can always say "no, do this instead." Fan-out ranking is better than random, which is the current state.

---

### 6. `co-designs` Relationship Type (Arch #3, Pract #6)

**Impact: 4** | **Feasibility: 4** | **Adoption: 3** | **Leverage: 48**

Add a third relationship type beyond `must-complete-first` and `inform`. `co-designs` means "these works must be designed with mutual awareness." Bidirectional. Surfaces in `kerf resume` and `kerf orient`.

**Stress test:** The schema change is trivial -- add a new string to the relationship enum. The behavioral difference from `inform` is subtle: `co-designs` implies both sides should read each other's artifacts, not just one reading the other. The real question: will agents use it? Only if it provides visible value at resume time. If `kerf resume` on work A shows "co-designs with work B -- read B's spec before proceeding," it pulls its weight.

**Failure mode:** Agents don't create co-designs links because they don't know they should. Overlap warnings (idea #3) can suggest it: "Both touch adapter -- consider `kerf link A B --rel co-designs`." But this requires agents to act on suggestions, which is unreliable. Area tags + overlap warnings may provide 80% of the value without explicit linking.

---

### 7. Entanglement / Amendment Protocol (Arch #11, DX #5, Process #4)

**Impact: 4** | **Feasibility: 3** | **Adoption: 3** | **Leverage: 36**

A defined protocol for late-arriving requirements: amend, spawn-dependent, or pause-and-replan. The Systems Architect's three-path model is clean.

**Stress test:** The paths are genuinely different workflows. "Amend" means adding to spec.yaml. "Spawn dependent" means `kerf new` + `kerf link`. "Pause and replan" means shelving and creating a new work. kerf could guide the choice but not automate it. The DX version (`kerf entangle`) presents options interactively -- that's useful for humans but agents need a non-interactive path. 

**Failure mode:** Overengineered. In practice, the user will say "fold this into the existing work" or "make a new work that depends on it." They don't need a formal protocol -- they need the tooling to support both paths, which it mostly already does. The real gap is visibility (knowing the overlap exists), not process (knowing what to do about it).

**Verdict:** The protocol is documentation, not features. The only new thing needed is the `amends` or `co-designs` relationship type. The rest is judgment + existing commands.

---

### 8. Area Annotations / Broadcast Notes (DX #9)

**Impact: 3** | **Feasibility: 4** | **Adoption: 3** | **Leverage: 36**

Cross-cutting notes attached to areas, not individual works. "All adapter changes must use the repository pattern." Surface whenever any work in that area is viewed.

**Stress test:** This is the "shared constraint" pattern from Prior Art #6 (DDD) and #15 (design tokens). It's a simple YAML file or markdown per area. Surfacing in `kerf show` and `kerf resume` is trivial. The value is real when the orchestrator has cross-cutting insight that no individual work captures.

**Failure mode:** Annotation hygiene. Stale annotations mislead. But annotations are tiny and infrequent -- the maintenance burden is minimal. Auto-archive when all works in the area complete.

---

### 9. Graph Invariant Checks / `kerf audit` (Arch #9)

**Impact: 3** | **Feasibility: 3** | **Adoption: 3** | **Leverage: 27**

Cycle detection, orphaned blockers, stale work detection, area heat warnings. Run at session start or on demand.

**Stress test:** Most invariants are trivially computable from the graph: cycle detection is DFS, orphan detection is edge traversal, staleness is timestamp comparison. Area heat requires area tags. The output is useful but not critical -- it catches edge cases, not the common case.

**Failure mode:** False positives in a small portfolio. If you have 5 works, "2 works touching the same area" is not a crisis. Thresholds need to be tuned to portfolio size.

**Verdict:** Valuable but second-wave. Build it after `kerf map` exists, since `kerf map` provides most of the same visibility in a more natural format.

---

### 10. WIP Limits (Contrarian #2, Process #5, Prior Art #7/#10)

**Impact: 3** | **Feasibility: 5** | **Adoption: 2** | **Leverage: 30**

Advisory limit on concurrent active works. `kerf new` warns when exceeded.

**Stress test:** The Contrarian makes a strong case: if you WIP-limit to 3, most coordination problems disappear. True in theory. In practice, spec-writing has a different WIP cost model than implementation -- having 8 works in various spec stages is less harmful than 8 concurrent implementations. The real value is the *signal*: "you have N active works, consider finishing some."

**Failure mode:** Agents ignore the advisory limit because they're following user instructions to create new works. The limit only works if the human orchestrator respects it.

**Verdict:** A single `wip_limit` config setting with a warning at `kerf new` is cheap and worth adding. Not transformative.

---

## Gap Analysis

### Gaps the brainstorming did not adequately address:

1. **How agents actually consume orientation data.** Everyone proposes computed orientation documents, but nobody specifies how this integrates with harmonik's session-start protocol. Does harmonik call `kerf map` before launching a worker? Does the orchestrator include it in the worker's prompt? The integration surface between kerf and the execution layer is underspecified.

2. **Multi-project coordination.** All proposals assume a single project. Cross-project dependencies exist in the spec but none of the brainstorming addresses cross-project area overlap or coherence. This may not matter yet, but it's a blind spot.

3. **What happens when the graph is wrong.** Dependencies and area tags will be wrong sometimes. Nobody proposes a mechanism for discovering that the graph is stale or incorrect (beyond `kerf audit`'s invariant checks, which catch structural problems but not semantic ones like wrong area tags).

4. **Scaling down.** Every proposal assumes 5-20 concurrent works. For a project with 1-2 works, all this coordination machinery is noise. Nobody proposes graceful degradation -- the commands should be useful but terse for small portfolios.

5. **The "affects" field.** The Contrarian's #10 (tracking which spec files a work modifies) is arguably higher-signal than area tags for detecting overlap, since it's concrete rather than conceptual. Nobody else picked this up. It could coexist with area tags: `areas` for conceptual grouping, `affects` for concrete spec-file targeting.

---

## False Solutions to Avoid

### 1. Event Sourcing / Append-Only Log (Arch #7, Process #12)
Sounds architecturally elegant. In practice, it's a significant departure from kerf's filesystem-as-database model. The spec.yaml files ARE the source of truth. An event log is a second source of truth competing with the first. Consistency between them is a new bug class. The filesystem already provides what's needed -- just read the spec.yaml files on demand.

### 2. Petri Net Model (Arch #6)
Conceptually interesting, practically useless. Token-based capacity modeling doesn't fit a world where agents are spawned on demand. The "synchronization transition" for co-design is just a complicated way to say "these two works need to be designed together," which a `co-designs` edge says more simply.

### 3. Two-Phase Commit for Clustered Works (Process #11)
Introduces a synchronization bottleneck that violates kerf's "guidance not gates" philosophy. One slow work blocks all works in its cluster. In practice, works in the same area will rarely reach their design milestone at the same time, so the synchronization point is never reached naturally.

### 4. Trajectory / Velocity Metrics (DX #12)
Sounds useful for project management. Irrelevant for the actual problems. Knowing "0.4 works completed/day" doesn't help an agent orient or detect overlap. This is dashboarding, not coordination. Don't build it.

### 5. Kanban Board / Board View (DX #6)
Terminal column layout for visual scanning by humans. Agents can't use it. `kerf map` as a structured list serves both audiences. The board is a rendering concern, not a coordination concern.

### 6. Typed Hypergraph with Cohorts (Arch #1)
The full proposal is overengineered. Cohorts, hyperedges, weighted relationships, synchronization semantics -- this is a graph theory paper, not a CLI feature. The useful kernel is: works have area tags, and works sharing areas form implicit clusters. You don't need hyperedge formalism for that.

### 7. Decision Log / ADR-style Records (Prior Art #5, Process #12)
Another artifact to maintain. The decisions ARE in the spec artifacts. Creating a parallel decision log means decisions live in two places and drift. Area annotations (idea #8 above) capture the cross-cutting intent without duplicating work-level decisions.

---

## Hard Trade-offs

### 1. Area Tags: Freeform vs. Defined Taxonomy

Freeform tags (`areas: [adapter]`) are low-friction but drift. A defined taxonomy (`areas.yaml`) is precise but requires curation.

**Resolution:** Start freeform. Suggest existing tags at `kerf new` time. Add a `kerf areas` command that lists all tags in use (so drift is visible). A defined taxonomy is premature -- you don't know what areas matter until works start using them.

### 2. Computed vs. Stored Graph

Compute the graph on every command (filesystem walk) vs. store it in a cache file (`work-graph.yaml`).

**Resolution:** Compute on demand. For 5-20 works, the cost is negligible. A cache file is a second source of truth with staleness bugs. The Practitioner's alternative ("skip the file, compute from spec.yaml on demand") is correct. Only introduce caching if external tools need the graph without calling kerf.

### 3. Overlap Detection: Tags vs. Spec-File References vs. File Globs

Three ways to detect overlap: conceptual area tags, concrete spec-file references (`affects: [specs/auth.md]`), or implementation file globs (`touched_files: [internal/adapter/*.go]`). Each has different coverage and precision.

**Resolution:** Start with area tags. They're available earliest in the work lifecycle (at creation), require no knowledge of implementation details, and are good enough for "these works might conflict." Spec-file references are a good future addition for higher precision. File globs are too late (known only at task generation) and too noisy (common files create false overlaps).

### 4. One Command vs. Many Commands

The DX perspective proposes 10+ new commands (`map`, `orient`, `context`, `diff`, `queue`, `board`, `cluster`, `entangle`, `trajectory`, `annotate`). The Contrarian says "just make `kerf list` better."

**Resolution:** The Contrarian is almost right. The actual need is 2-3 new commands, not 10. `kerf map` for portfolio view, `kerf next` for work selection, and enhanced `kerf resume` for work-specific context. Everything else is either a flag on these commands or a feature that should be built into existing commands rather than being its own command.

### 5. Prevention vs. Detection

Build systems that prevent overlap (gates, required reviews, synchronization points) vs. systems that detect and surface it (warnings, advisories, computed views).

**Resolution:** Detection. kerf's philosophy is "guidance not gates." Prevention adds friction to the happy path (where most works don't overlap). Detection is cheap, always-on, and respects agent autonomy. The only exception: overlap warnings at `kerf new` are a form of prevention that's worth the trivial cost.

---

## Minimum Viable Proposal: The 3-Thing Package

### Thing 1: `areas` Field in spec.yaml + Overlap Warnings

**What:** Add an `areas` list field to spec.yaml. At `kerf new` time, check existing active works for shared areas and warn. At `kerf resume` time, show area peers.

**Why this first:** It's the foundation everything else builds on. Without it, overlap detection is impossible. With it, the most critical coordination information (what else touches this area) flows naturally through existing commands.

**Spec changes:** works.md (add `areas` field to spec.yaml schema), commands.md (add overlap check to `kerf new`, add area peer display to `kerf resume`).

**Build cost:** Tiny. A list field, a directory walk, a set intersection. No new commands.

### Thing 2: `kerf map` -- Computed Portfolio View

**What:** A new command that reads all active works for the current project, builds the graph, and emits a structured text view: works grouped by status, dependency edges, area clusters, actionable items, blocked items, overlap warnings.

**Why this second:** It replaces HANDOFF.md for structural orientation. An agent reads this at session start and has the full landscape. The Contrarian's point #1 is correct: most coordination failures come from agents not seeing the landscape. This gives them the landscape.

**Spec changes:** commands.md (new `kerf map` command). No data model changes beyond `areas` from Thing 1.

**Build cost:** Medium. Directory walk, YAML parse, graph construction, text rendering. No new storage. All data comes from existing spec.yaml files.

### Thing 3: `kerf next` -- Computed Work Selection

**What:** A command that reads the graph and emits ranked actionable works. Actionable = not blocked by incomplete `must-complete-first` dependencies. Ranking = explicit priority (if set) > fan-out (works that unblock the most) > creation date. Optional `priority` integer field in spec.yaml for human override.

**Why this third:** It answers "what should I work on?" without requiring manual triage. Combined with `kerf map` (which shows the landscape) and area overlap warnings (which show conflicts), this gives agents and orchestrators the three things they need: where am I, what's related, and what's next.

**Spec changes:** commands.md (new `kerf next` command), works.md (optional `priority` field in spec.yaml).

**Build cost:** Small. Topological sort + simple scoring. The dependency resolution logic already exists.

### Why these three and not others:

- They require no new concepts beyond what kerf already has (works, dependencies, spec.yaml, the bench).
- They add one new data field (`areas`), one optional field (`priority`), and two new commands.
- They compose: `kerf map` uses area tags for clustering, `kerf next` uses the dependency graph for ranking, and overlap warnings use area tags for detection.
- They degrade gracefully: if `areas` is empty, `kerf map` still shows status and dependencies. If `priority` is unset, `kerf next` uses mechanical ranking.
- They don't require behavior change: agents already run kerf commands at session start. Adding `kerf map` to that ritual is trivial. Overlap warnings fire automatically.

### What's deliberately excluded:

- **Area specs / contracts / annotations** -- valuable but second-wave. Build after you see which areas are actually hot.
- **`co-designs` relationship type** -- the overlap warnings from area tags provide most of this value without requiring explicit linking.
- **Amendment protocol** -- the existing commands (`kerf new` + `kerf link`) already support the key paths. A formal protocol is documentation, not tooling.
- **Event log / work graph file** -- unnecessary. Compute on demand from spec.yaml.
- **`kerf orient` / `kerf context` / `kerf diff`** -- refinements of `kerf map` that can be added later as flags or subcommands. Don't build three commands when one will do.

---

## Danger Zones

### 1. Becoming a Project Management Tool

The most frequently mentioned risk, and the most real. Kanban boards, velocity metrics, WIP limits, trajectory charts, stage-based views -- this is Jira territory. kerf is a spec-writing tool. The coordination layer should help specs be coherent, not help managers track progress. The test: does this feature help an agent write a better spec, or does it help a human feel more in control? Only build the former.

**Specific offenders:** `kerf board`, `kerf trajectory`, WIP limits per status stage, priority scoring formulas with configurable weights.

### 2. Scope Creep Through "Just One More Relationship Type"

The Systems Architect proposes 6 relationship types. Today there are 2. The path from 2 to 6 is paved with good intentions and ends in a taxonomy that nobody uses correctly. Each new type requires: schema changes, display logic, behavioral differences, agent guidance on when to use it. The marginal value of each new type is lower than the last, while the maintenance cost is constant.

**Safe path:** Add `co-designs` if area tags prove insufficient. Nothing else until there's a concrete failure that demands a new type.

### 3. Automated Semantic Analysis

Multiple proposals suggest kerf should detect "contradictory design decisions" or "incompatible assumptions" between works. This is an AI-complete problem. kerf should detect *adjacency* (works touching the same area) and surface it. Evaluating *compatibility* is the agent's job. Building "smart" conflict detection will produce false positives that erode trust in all warnings.

### 4. The Index / Cache / Database Temptation

The Contrarian's point #8 (filesystem hits limits, add SQLite) is technically correct and strategically wrong. For 5-20 works, computing the graph from spec.yaml files is fast enough. Introducing an index means maintaining consistency between the index and the filesystem. The first time the index is stale and gives wrong results, trust is destroyed. Resist until portfolio size makes on-demand computation genuinely slow (50+ works).

### 5. Mandatory Process Steps

Two-phase commit for clusters, gated status transitions, required reconciliation passes -- these all violate "jigs are guidance, not gates." The moment kerf blocks an agent from advancing because some cross-cutting check hasn't been performed, it stops being a helpful tool and becomes an obstacle. Advisory warnings, always. Gates, never.
