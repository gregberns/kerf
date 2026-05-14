# Pattern Synthesis — Work Coordination Brainstorm

> Deduplication, clustering, convergence analysis, and 80/20 identification across all six brainstorming perspectives.

---

## Distinct Idea Inventory (Deduplicated)

Each idea below is a meaningfully distinct mechanism. Where multiple agents proposed the same thing with different names, they are merged. Convergence count = how many of the 6 agents independently proposed it (or something functionally equivalent).

### I1. Computed Session Orientation Document
**Convergence: 6/6** (all agents)
A command (`kerf orient`, `kerf map`, `kerf context`, or enhanced `kerf resume`) that generates a structured, computed snapshot of the entire work landscape at session start. Replaces or supplements the hand-written HANDOFF narrative with a document derived from actual work state.

- Systems Architect: Idea 10 (session orientation document)
- Developer Experience: Ideas 1, 2, 8 (kerf map, kerf orient, kerf context)
- Process Designer: Idea 8 (session orientation protocol)
- Prior Art: Derived from Kanban boards, PERT charts, knowledge graphs
- Practitioner: Ideas 1, 10 (session brief, dashboard)
- Contrarian: Ideas 1, 6, 7 (single problem = session context; better kerf list; better prompts)

### I2. Area/Component Tags on Works
**Convergence: 6/6** (all agents)
An `areas` (or `touches`) field in spec.yaml -- a list of strings naming system areas/components the work affects. Enables grouping, overlap detection, and clustering.

- Systems Architect: Idea 1 (area nodes in hypergraph)
- Developer Experience: Idea 3 (area tags + kerf cluster)
- Process Designer: Idea 2 (intake funnel tagging step)
- Prior Art: Items 4, 8 (Obsidian tags, git overlap detection)
- Practitioner: Idea 2 (area tags)
- Contrarian: Idea 10 (track spec ownership via `affects` field -- same mechanism, different granularity)

### I3. Overlap Detection and Warnings at Key Moments
**Convergence: 5/6** (all except contrarian, who argues for detection-not-prevention but still proposes `kerf check-coherence`)
When a work is created or advances status, kerf checks for area overlap with other active works and emits warnings. Automatic, advisory, integrated into existing commands.

- Systems Architect: Idea 1 (cohort detection on work creation)
- Developer Experience: Idea 7 (automatic overlap at kerf new and kerf status)
- Process Designer: Idea 1 (gated status transitions with portfolio pre-checks)
- Prior Art: Items 8, 9 (git conflict detection, feature flag integration checks)
- Practitioner: Idea 4 (overlap warnings at creation time)
- Contrarian: Idea 9 (detect-don't-prevent via kerf check-coherence)

### I4. `kerf next` -- Computed Work Selection / Readiness
**Convergence: 5/6**
A command that reads the dependency graph and status of all works and returns a ranked list of what's actionable. Mechanical ranking: unblocked works first, tie-break by downstream impact (fan-out), then age. Optional manual priority override.

- Systems Architect: Idea 4 (priority score as computed property)
- Developer Experience: Idea 4 (kerf queue)
- Process Designer: Idea 5 (kanban pull-based scheduling)
- Prior Art: Items 3, 7, 11 (build system topological sort, TOC critical chain, PERT critical path)
- Practitioner: Ideas 3, 7 (kerf next + priority field)

### I5. Richer Dependency/Relationship Types
**Convergence: 4/6**
Expand beyond `must-complete-first` and `inform` to include at minimum `co-designs` (bidirectional, synchronization) and `supersedes` (lifecycle). Some agents propose more types; the contrarian and practitioner argue for keeping it minimal.

- Systems Architect: Idea 3 (6 relationship types with weights)
- Developer Experience: Idea 5 (kerf entangle with merge/coordinate/sequence/pause options)
- Process Designer: Idea 4 (amendment protocol with absorb/fork/defer paths)
- Practitioner: Idea 6 (entangle command, or just bidirectional inform)

### I6. Area Specs / Shared Design Anchors
**Convergence: 4/6**
When multiple works touch the same area, a lightweight document captures shared design constraints for that area. Individual works conform to it. The "single source of truth" for how an area should be designed.

- Systems Architect: Idea 5 (area spec documents as coherence anchors)
- Process Designer: Idea 7 (area specs as shared design anchors)
- Prior Art: Items 2, 6, 15 (package lockfiles, DDD shared kernel, design system tokens)
- Practitioner: Idea 9 (area-level spec anchors)

### I7. Materialized Work Graph
**Convergence: 4/6**
A project-level file (`workgraph.yaml`, `portfolio.yaml`, `graph.yaml`) that is the computed, queryable representation of all works and their relationships. Regenerated from spec.yaml files on every state change. Source of truth remains in spec.yaml; the graph is a cache.

- Systems Architect: Idea 2 (materialized work graph with projection views)
- Process Designer: Idea 3 (work graph as first-class process object)
- Prior Art: Items 1, 12, 13 (PM tools, MEOW, Nx project graph)
- Practitioner: Idea 5 (work graph file)

### I8. Late-Arriving Requirement Protocol
**Convergence: 4/6**
A defined protocol (not just a data structure) for handling new requirements that overlap with in-flight work. Multiple agents converge on 3-4 resolution paths: amend/absorb, fork/spawn-dependent, sequence, or pause-and-replan.

- Systems Architect: Idea 11 (entanglement protocol with Path A/B/C)
- Developer Experience: Idea 5 (kerf entangle with 4 options)
- Process Designer: Idea 4 (amendment protocol with absorb/fork/defer)
- Practitioner: Idea 6 (entangle command)

### I9. WIP Limits
**Convergence: 3/6**
Limit the number of works in-flight simultaneously (globally or per status stage). "Stop starting, start finishing."

- Process Designer: Idea 5 (kanban WIP limits per stage)
- Prior Art: Items 7, 10 (Theory of Constraints, Kanban)
- Contrarian: Idea 2 (WIP limits as the whole solution)

### I10. Work Graph Invariants / Audit
**Convergence: 2/6**
Define invariants (no orphaned blockers, cycle detection, area heat threshold, stale work detection) and check them automatically.

- Systems Architect: Idea 9 (graph invariants and consistency checks)
- Prior Art: Item 9 (periodic integration checks via kerf square --scope)

### I11. Event Log / Decision Ledger
**Convergence: 2/6**
An append-only log of all work lifecycle events or design decisions. Enables "what changed since last session" queries and historical reconstruction.

- Systems Architect: Idea 7 (event sourcing the work graph)
- Process Designer: Idea 12 (work ledger -- append-only decision log)

### I12. "What Changed" / Diff Command
**Convergence: 2/6**
A command showing what changed in the work landscape since a given timestamp or since the last session on a specific work.

- Developer Experience: Idea 10 (kerf diff)
- Practitioner: Idea 1 (orient includes "what changed since last session")

### I13. Cross-Work Area Annotations
**Convergence: 2/6**
Notes attached to area clusters (not individual works) that surface whenever any work in that area is viewed. A "broadcast to a topic" mechanism.

- Developer Experience: Idea 9 (work annotations on areas)
- Prior Art: Item 5 (ADRs as cross-work decision log)

### I14. File/Spec Path Overlap Detection
**Convergence: 2/6**
Infer overlap from concrete file paths or spec references rather than (or in addition to) manual area tags.

- Systems Architect: Idea 8 (file glob intersection)
- Contrarian: Idea 10 (track which spec files each work affects)

### I15. Lightweight Query Index
**Convergence: 1/6** (contrarian only, but reinforces the work graph idea)
A SQLite or JSON index alongside the filesystem to enable portfolio-level queries without reading every spec.yaml.

- Contrarian: Idea 8 (filesystem hits its limits for portfolio queries)

### I16. Session End Bookend / Checkpoint
**Convergence: 1/6**
Structured session end that records decisions and progress in a queryable format, not just SESSION.md narrative.

- Practitioner: Idea 8 (kerf checkpoint at session end)

### I17. Dependency-Aware Bead Generation
**Convergence: 1/6**
When generating tasks for a work, inject context beads from related/entangled works.

- Practitioner: Idea 11

### I18. Work Grouping / Initiative Layer
**Convergence: 2/6**
A level above works that groups related works into a named initiative or theme.

- Prior Art: Item 1 (Epics/Initiatives from PM tools)
- Contrarian: Idea 5 (wrong unit of work -- need a grouping level)

---

## Theme Clusters

### A. Visibility / Orientation
*How agents see the landscape at session start.*
- I1. Computed Session Orientation (6/6)
- I7. Materialized Work Graph (4/6)
- I12. What Changed / Diff (2/6)

### B. Overlap / Clustering
*How the system detects and surfaces that works are related.*
- I2. Area Tags (6/6)
- I3. Overlap Warnings (5/6)
- I14. File/Spec Path Overlap (2/6)
- I13. Cross-Work Annotations (2/6)

### C. Ordering / Prioritization
*How to decide what to work on next.*
- I4. kerf next -- Computed Selection (5/6)
- I9. WIP Limits (3/6)

### D. Coherence / Shared Design
*How multiple works touching the same area stay consistent.*
- I6. Area Specs / Shared Anchors (4/6)
- I5. Richer Relationship Types (4/6)
- I10. Graph Invariants / Audit (2/6)

### E. Change Management
*How late-arriving requirements are handled.*
- I8. Late-Requirement Protocol (4/6)

### F. Infrastructure / Data Model
*Underlying mechanisms that enable the above.*
- I7. Materialized Work Graph (4/6)
- I11. Event Log (2/6)
- I15. Query Index (1/6)
- I18. Work Grouping Layer (2/6)

---

## Problem Coverage Matrix

| Idea | P1: No Map | P2: Islands | P3: No Queue | P4: Late Reqs | P5: Coherence |
|------|:---:|:---:|:---:|:---:|:---:|
| I1. Session Orientation (6/6) | **X** | | **X** | | **X** |
| I2. Area Tags (6/6) | | **X** | | **X** | **X** |
| I3. Overlap Warnings (5/6) | | **X** | | **X** | **X** |
| I4. kerf next (5/6) | | | **X** | | |
| I5. Richer Relationships (4/6) | | **X** | | **X** | **X** |
| I6. Area Specs (4/6) | | **X** | | **X** | **X** |
| I7. Work Graph (4/6) | **X** | **X** | **X** | | |
| I8. Late-Req Protocol (4/6) | | **X** | | **X** | |
| I9. WIP Limits (3/6) | **X** | | **X** | | |
| I10. Graph Invariants (2/6) | **X** | **X** | | | **X** |
| I11. Event Log (2/6) | **X** | | | **X** | **X** |
| I12. Diff Command (2/6) | **X** | | | **X** | |
| I13. Area Annotations (2/6) | | **X** | | **X** | **X** |
| I14. File Path Overlap (2/6) | | **X** | | | **X** |

**Coverage by problem:**
- P1 (No Map): I1, I7, I9, I10, I11, I12 -- well covered, especially by I1+I7
- P2 (Islands): I2, I3, I5, I6, I7, I8, I10, I13, I14 -- most-addressed problem
- P3 (No Queue): I1, I4, I7, I9 -- addressed by I4 primarily
- P4 (Late Reqs): I2, I3, I5, I6, I8, I11, I12, I13 -- many mechanisms, I8 is the direct one
- P5 (Coherence): I1, I2, I3, I5, I6, I10, I11, I13, I14 -- the hardest, most ideas attempt it

---

## Recommended Tiers

### Tier 1: Minimal Viable Coordination (3 ideas, addresses all 5 problems)

These three ideas were independently identified as the "if I could only build N things" picks by 4+ agents. They are complementary, each is simple relative to its impact, and together they cover all five problems.

1. **I2. Area Tags on spec.yaml** (6/6 convergence)
   - Mechanism: `areas: [adapter, auth]` field in spec.yaml. Freeform strings. Suggest existing tags at creation time.
   - Why first: Foundation for everything else. Overlap detection, clustering, orient output, area specs -- all depend on knowing what areas a work touches.
   - Covers: P2, P4, P5

2. **I1. Computed Session Orientation** (6/6 convergence)
   - Mechanism: `kerf orient [codename]` generates a structured snapshot: all works by status, dependency graph, area clusters, overlap warnings, what changed since last session, actionable next steps.
   - Why second: The single highest-leverage intervention for agent effectiveness. Every agent, every session. The contrarian correctly notes this may be the whole solution for P1.
   - Covers: P1, P3, P5

3. **I3. Overlap Warnings at Key Moments** (5/6 convergence)
   - Mechanism: At `kerf new` and at key status transitions, check area tags for overlap with active works. Emit advisory warnings. No gates, no blocking.
   - Why third: Catches overlap at the earliest possible moment without requiring agents to remember to check. "Pit of success" pattern.
   - Covers: P2, P4, P5

**What Tier 1 gives you:** Agents can see the full landscape (orient), works declare what they touch (area tags), and overlap is surfaced automatically (warnings). P3 (queue) is partially addressed via orient's "suggested next" section. P4 (late reqs) is surfaced by overlap warnings. P5 (coherence) gets visibility but not enforcement.

**What Tier 1 does NOT give you:** A formal mechanism for handling late requirements (just visibility). No computed priority ranking. No shared design anchors. No formal relationship types beyond blocks/informs.

### Tier 2: Solid Coordination (add 3 more ideas)

4. **I4. `kerf next` -- Computed Work Selection** (5/6 convergence)
   - Mechanism: Read dependency graph, compute actionable works (unblocked, not in-session), rank by fan-out then age. Optional `priority` integer override in spec.yaml.
   - Fully addresses P3. Removes "what do I work on next?" from agent guesswork.

5. **I8. Late-Requirement Protocol** (4/6 convergence)
   - Mechanism: `kerf entangle <work-a> <work-b>` with 3-4 resolution paths based on current state of both works (amend, coordinate, sequence, pause-and-replan).
   - Directly addresses P4. Gives orchestrators a structured workflow instead of ad-hoc handling.

6. **I5. Richer Relationship Types** (4/6 convergence)
   - Mechanism: Add `co-designs` (bidirectional, design synchronization) and `supersedes` (lifecycle) to the existing relationship vocabulary. Minimal: just these two. The practitioner's suggestion of bidirectional `inform` via `--mutual` flag is a pragmatic alternative that may suffice.
   - Strengthens P2, P4, P5 handling by making entanglement explicit in the data model.

**What Tier 2 adds:** Formal prioritization, a protocol for the messy late-requirement case, and richer relationship modeling. The system now handles all five problems with explicit mechanisms.

### Tier 3: Ambitious / Build-When-Needed (4 ideas)

7. **I6. Area Specs / Shared Design Anchors** (4/6 convergence)
   - Only create when 2+ works touch the same area. Lightweight constraint documents, not full specs.
   - Strongest mechanism for P5 (coherence) but highest maintenance burden.

8. **I7. Materialized Work Graph** (4/6 convergence)
   - A cached `workgraph.yaml` computed from spec.yaml files. Enables external tool consumption.
   - Pragmatic alternative: compute on demand (orient/next already do this). Only materialize if external tools need it.

9. **I9. WIP Limits** (3/6 convergence)
   - Advisory limit on concurrent active works. The contrarian's strongest point: limiting WIP may render most coordination mechanisms unnecessary.
   - Low implementation cost. High cultural cost (requires discipline).

10. **I10. Graph Invariants / Audit** (2/6 convergence)
    - `kerf audit` checks: no cycles, no orphaned blockers, area heat, stale work detection.
    - A natural extension once the graph exists. Low urgency until portfolio size warrants it.

---

## Key Tensions and Trade-Offs

### T1. Explicit Tags vs. Inferred Overlap
**Agents 1,2,3,4,5** favor explicit area tags. **Agent 6 (contrarian)** and **Agent 1 (Idea 8)** propose inferring overlap from file paths or spec references. The tension: explicit tags require discipline but are reliable; inferred overlap is automatic but noisy/incomplete.

**Resolution:** Start with explicit tags (cheap, reliable). Add file-path inference later as a supplement, not a replacement. The contrarian's `affects: [specs/auth.md]` suggestion is a useful hybrid -- it's explicit but uses concrete paths rather than abstract area names.

### T2. Rich Relationship Types vs. Simplicity
**Agent 1** proposes 6 relationship types with weights. **Agent 5 (practitioner)** warns against over-structuring. **Agent 6 (contrarian)** implies the current two types may suffice.

**Resolution:** Add exactly `co-designs` (the synchronization case that 4/6 agents identified as missing). Hold on everything else. The practitioner's advice is sound: "Add structure only when you hit a concrete failure that requires it."

### T3. Gates vs. Advisory
**Agent 3 (process designer)** proposes gated transitions. **Agents 5 and 6** insist on advisory-only. kerf's existing philosophy is "guidance, not gates."

**Resolution:** Advisory warnings at transitions, not gates. The process designer acknowledges this in their cross-cutting observations. The gate should be "kerf tells you; you decide."

### T4. Materialized Graph File vs. Computed-on-Demand
**Agents 1, 3, 4** favor a persistent graph file. **Agent 5 (practitioner)** notes the alternative: skip the file, compute on demand. **Agent 6** observes the filesystem-as-database constraint makes queries hard.

**Resolution:** Compute on demand for kerf commands (orient, next). Only materialize if external tools need it. The spec.yaml files remain the source of truth. This avoids the staleness problem entirely.

### T5. Prevention vs. Detection
**Agents 1-5** lean toward preventing conflicts (overlap warnings at creation, design gates, area specs). **Agent 6 (contrarian, Idea 9)** argues for rapid detection and cheap resolution instead of prevention.

**Resolution:** Both. The Tier 1 ideas (area tags + overlap warnings) are detection mechanisms, not prevention. They surface conflicts early but don't block. Area specs (Tier 3) lean toward prevention. The right sequence is detection first, prevention mechanisms only if detection proves insufficient.

### T6. Tool Scope -- Should kerf Own This?
**Agent 6 (contrarian, Idea 4)** questions whether kerf should own work coordination at all, suggesting the kerf/beads/harmonik boundaries may be wrong.

**Resolution:** The plan document already addresses this: kerf owns work structure and the work graph; beads owns the task graph; harmonik owns execution. The coordination layer belongs in kerf because it's about work relationships, not task execution. But this tension should be monitored -- if coordination features start requiring execution-level awareness, the boundary needs revisiting.

### T7. YAGNI -- Is This Premature?
**Agent 6 (contrarian, Idea 11)** asks whether these problems are causing real pain at current scale.

**Resolution:** The existence of this brainstorming exercise suggests they are. But the contrarian's point argues for Tier 1 (minimal, low-risk, high-visibility) over Tier 3 (ambitious, speculative). Build the minimum, use it hard, expand based on real failures.

---

## The 80/20

Three ideas address the majority of problems with minimal complexity:

1. **Area tags** (a list of strings in spec.yaml)
2. **Computed orientation** (one new command that reads existing data)
3. **Overlap warnings** (advisory output added to existing commands)

These require: one new spec.yaml field, one new command, and small additions to existing command output. No new data model, no graph database, no event log, no new artifact types. They give agents visibility into the landscape, make overlap explicit, and surface conflicts at the moment they matter.

The contrarian is right that this is fundamentally a "context at session start" problem. The area tags make that context computable. The orientation command delivers it. The overlap warnings catch what orientation misses.

Everything else is an optimization on top of these three.
