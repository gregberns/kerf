# Prior Art Research: Portfolio-Level Work Coordination

> Agent D perspective — how other tools, systems, and domains solve the problems kerf faces at the work-graph layer.

---

## 1. Epics and Initiatives (Linear, Jira, GitHub Projects)

**What it is:** Project management tools model hierarchy above individual issues. Jira has Epics > Stories > Subtasks, plus "Initiatives" above Epics. Linear has Projects > Issues > Sub-issues, plus Cycles for time-boxing. GitHub Projects provides flat boards with custom fields and views, plus "Tasklists" for nesting.

**Which problems it addresses:** P1 (persistent map), P2 (islands), P3 (intake/prioritization).

**What transfers:**
- The idea of a *grouping layer* above individual works is directly applicable. kerf has works and passes but nothing that groups related works. An "initiative" or "epic" equivalent in kerf could be a directory or manifest file that names a set of related works and describes their shared intent.
- Linear's "Projects" are time-bounded groupings with a target date and progress bar. This is a useful model for kerf: a named group of works with a completion criterion.
- Custom views over the same data (filter by area, by status, by dependency state) are powerful for orientation.

**What doesn't transfer:**
- These tools assume a human is reading a GUI and making decisions. kerf's consumer is an agent reading filesystem artifacts.
- The database-backed query model (filter, sort, group) doesn't map to filesystem-as-database without building an index.
- Priority fields (P0/P1/P2, "Urgent/High/Medium/Low") require human judgment that agents don't have.

**Concrete kerf idea:** A `portfolio.yaml` file per project that declares work groups, each with a name, intent statement, list of constituent work codenames, and an optional ordering. Works inherit group membership. `kerf map` reads this file and the current state of all listed works to produce an orientation summary. This is the "persistent map" — structured, durable, readable by agents at session start.

---

## 2. Package Manager Dependency Resolution (npm, Cargo, pip)

**What it is:** Package managers resolve version constraints across a dependency graph. npm uses a tree-based algorithm with hoisting. Cargo uses a SAT solver. pip-compile produces a locked, flat resolution. All must handle conflicts where A depends on X@1.0 and B depends on X@2.0.

**Which problems it addresses:** P2 (islands/incompatible results), P5 (cross-work coherence).

**What transfers:**
- The concept of *conflict detection* is directly transferable. When two works both modify the same spec section or touch the same system area, that's analogous to a version conflict. The system should detect it and surface it, not silently allow incompatible changes.
- The *lockfile* pattern is interesting: a resolved, flattened view of all constraints that everyone agrees on. For kerf, this could be a "resolved state" file that captures design decisions shared across related works.
- Cargo's approach of *unifying* dependencies to the same version where possible is analogous to "these works should share a single design for the adapter interface."

**What doesn't transfer:**
- Package versions are semantic and comparable. Work artifacts are documents — you can't "resolve" two contradictory spec paragraphs algorithmically.
- The resolution is fully automated in package managers. Work-level coherence requires agent judgment.

**Concrete kerf idea:** An *area manifest* pattern. For each system area (e.g., "adapter layer"), maintain a file that lists all works touching it, their current design decisions for that area, and any constraints they impose. When a new work is created that touches the adapter, kerf surfaces the manifest so the agent knows what existing decisions to respect. This is manual conflict detection — not automated resolution, but automated *surfacing*.

---

## 3. Build System Dependency Graphs (Bazel, Make, Nx)

**What it is:** Build systems model a DAG of targets with explicit dependencies. Bazel enforces hermetic builds — each target declares its inputs and outputs exactly. Nx (for monorepos) adds "affected" analysis: given a change, which targets need to be rebuilt?

**Which problems it addresses:** P3 (ordering/prioritization), P4 (late-arriving requirements), P5 (cross-work coherence).

**What transfers:**
- *Affected analysis* is highly transferable. If a spec changes, which works are affected? Nx answers this for code; kerf could answer it for works. "You just modified the adapter spec. These 3 works reference the adapter. Their designs may need updating."
- The *target graph with topological ordering* gives a natural answer to "what should be worked on next?" — whatever has all its dependencies satisfied.
- Bazel's *visibility rules* (which targets can depend on which) could translate to "which areas of the system are open for modification vs. locked by in-flight work."

**What doesn't transfer:**
- Build targets have deterministic, automated execution. Works require agent judgment.
- Build graphs are defined in code. kerf's graph would need to be inferred or manually declared.

**Concrete kerf idea:** A `kerf next` command that reads the work graph (dependencies, statuses, area overlaps) and emits a ranked list of actionable works. "Actionable" means: all `must-complete-first` dependencies are satisfied, no conflicting works are in-flight for the same area, and the work isn't blocked. This is the topological sort applied to the work graph.

Also: an `kerf affected <spec-file>` command that, given a spec path, lists all works whose artifacts reference that spec. This is Nx's affected analysis for the work graph.

---

## 4. Obsidian / Roam / Zettelkasten (Knowledge Graphs)

**What it is:** Note-taking tools that use bidirectional links to create a knowledge graph. Obsidian uses `[[wikilinks]]` between markdown files. Roam uses block-level references. The Zettelkasten method emphasizes atomic notes with explicit links and a flat namespace. The graph emerges from the links, not from a pre-defined hierarchy.

**Which problems it addresses:** P1 (persistent map), P2 (islands), P5 (cross-work coherence).

**What transfers:**
- *Emergent structure from links rather than imposed hierarchy.* kerf already has `depends_on` in spec.yaml, but this is sparse. Richer cross-referencing (works referencing the same spec section, works mentioning the same system component) would create a denser graph.
- *Backlinks* — knowing not just "what does this work depend on" but "what depends on this work" and "what else touches the same area." Obsidian's graph view reveals clusters; kerf could reveal work clusters.
- *The flat namespace with emergent clustering* fits kerf's existing codename model. Works are flat (all in one directory per project). Clustering by area/theme emerges from metadata, not from directory structure.

**What doesn't transfer:**
- These tools are for human browsing. Agents need structured, parseable output, not graph visualizations.
- Block-level references are too granular for work-level coordination.

**Concrete kerf idea:** Add an optional `touches` field to spec.yaml — a list of system areas/components this work modifies. This is a lightweight tag system. `kerf map` can then cluster works by area, revealing overlaps. No hierarchy is imposed; the clustering emerges from the tags. When an agent creates a work touching "adapter", kerf emits: "3 other works also touch adapter: [list with statuses]."

---

## 5. Architecture Decision Records (ADRs)

**What it is:** ADRs document architectural decisions with status (proposed, accepted, deprecated, superseded). Each ADR is a numbered markdown file. The key innovation: when a decision is superseded, the old ADR links to its replacement. The full decision history is preserved with explicit supersession chains.

**Which problems it addresses:** P4 (late-arriving requirements), P5 (cross-work coherence).

**What transfers:**
- The *supersession model* is directly applicable to late-arriving requirements. When a new work overlaps with an in-flight work, instead of awkwardly merging, the new work can explicitly supersede or amend the old one's design decisions. The old decisions remain visible with a "superseded by [new-work]" marker.
- The *decision log as a first-class artifact* is valuable. Currently, design decisions are buried in pass artifacts. A separate, cross-work decision log would make it easy to check "what has been decided about the adapter" across all works.
- ADR's *lightweight, file-based format* fits kerf's filesystem-as-database philosophy perfectly.

**What doesn't transfer:**
- ADRs are typically written by humans with architectural judgment. Agents would need explicit prompting to create them.
- ADRs are per-decision, not per-work. The mapping is many-to-many.

**Concrete kerf idea:** A `decisions/` directory at the project level on the bench (`~/.kerf/projects/{project-id}/decisions/`). Each decision is a markdown file with: the decision, which works it affects, its status (active, superseded), and what supersedes it. When a new work arrives that conflicts with an existing decision, the agent either conforms to it or creates a new decision that supersedes it. `kerf show <codename>` includes relevant decisions in its output.

---

## 6. Domain-Driven Design: Bounded Contexts and Context Maps

**What it is:** DDD models a system as multiple bounded contexts, each with its own ubiquitous language and internal consistency. A *context map* documents how bounded contexts relate: shared kernel, customer-supplier, conformist, anti-corruption layer, etc. The key insight: you don't try to make everything globally consistent. You define boundaries and manage the interfaces between them.

**Which problems it addresses:** P2 (islands), P5 (cross-work coherence).

**What transfers:**
- The idea that *not everything needs to be globally coherent* is liberating. Works touching different system areas can be independent. Works touching the *same* area need coordination. The problem reduces to: identify shared boundaries and manage the interfaces.
- *Context maps as an explicit artifact* — a file that documents which works share boundaries and how they interact. This is more than dependency; it's about shared design surfaces.
- The *shared kernel* pattern: when two works must share a design element (e.g., an interface, a data schema), extract it as an explicit shared artifact that both works conform to, rather than hoping they independently converge.

**What doesn't transfer:**
- DDD's bounded contexts are long-lived architectural boundaries. Work coordination is more transient — the clusters shift as works are completed.
- The vocabulary (anti-corruption layer, conformist, etc.) is overkill for work coordination.

**Concrete kerf idea:** When multiple works touch the same system area, kerf could prompt creation of an *interface contract* — a small spec fragment that captures the shared design surface. All works touching that area must conform to or explicitly amend the contract. This is the "shared kernel" pattern applied to concurrent works. Stored at `~/.kerf/projects/{project-id}/contracts/{area-name}.md`.

---

## 7. Theory of Constraints (TOC) and Critical Chain Project Management

**What it is:** Goldratt's Theory of Constraints says every system has one bottleneck (constraint) that limits throughput. You optimize the constraint, subordinate everything else to it, and then find the new constraint. Critical Chain Project Management applies this to projects: identify the longest chain of dependent tasks, protect it with buffers, and avoid multitasking.

**Which problems it addresses:** P3 (prioritization/ordering), P1 (persistent map).

**What transfers:**
- *Identify the critical chain.* In a work graph with dependencies, the longest path determines the minimum time to complete everything. Works on the critical chain should be prioritized; works off the critical chain have slack. This is directly computable from the dependency graph.
- *WIP limits.* TOC says multitasking kills throughput. For agent work: don't have 8 works in-flight simultaneously. Limit WIP at the work level (not just the bead level). kerf could enforce or recommend WIP limits.
- *Subordinate to the constraint.* If the critical chain passes through "adapter redesign," everything else should support getting that work done first. Other works that might interfere should wait.

**What doesn't transfer:**
- TOC assumes a single system with one constraint. Multi-project, multi-agent work is more complex.
- Buffer management (adding time buffers to non-critical tasks) doesn't apply directly to spec work.

**Concrete kerf idea:** `kerf map --critical-path` computes and displays the longest dependency chain in the work graph. Works on the critical path are flagged. `kerf next` prioritizes critical-path works. A `wip_limit` setting in `project.yaml` that, when exceeded, causes `kerf new` to warn: "You have N works in-flight. Consider completing existing work before starting new work."

---

## 8. Git Merge Conflict Detection and Resolution

**What it is:** Git detects when two branches modify the same lines of the same file and forces manual resolution. The key properties: (a) conflict detection is automatic based on overlapping regions, (b) resolution is manual, (c) the system refuses to proceed until conflicts are resolved.

**Which problems it addresses:** P2 (islands), P4 (late-arriving requirements), P5 (cross-work coherence).

**What transfers:**
- *Overlap detection based on shared targets.* Two works that modify the same spec section are like two branches modifying the same file region. This overlap is detectable — it's a property of the works' declared targets, not something requiring human annotation.
- *Conflict surfacing without automated resolution.* Git doesn't resolve conflicts; it makes them visible and blocks until a human resolves them. kerf could do the same: surface overlaps, warn agents, but let them decide how to resolve.
- *The merge commit as a reconciliation artifact.* When conflicts are resolved, git creates a merge commit that captures the resolution. kerf could have a similar artifact: when overlapping works are reconciled, a "reconciliation note" captures how the overlap was handled.

**What doesn't transfer:**
- Git operates on text lines. Work overlap is semantic, not line-based.
- Git blocks merges until resolution. kerf should warn but not block (jigs are guidance, not gates).

**Concrete kerf idea:** When a work's `touches` list overlaps with an in-flight work's `touches` list, `kerf new` and `kerf resume` emit a warning: "This work overlaps with [codename] on [area]. Review [codename]'s current state before proceeding." The overlap is detected automatically from metadata; resolution is left to the agent. Additionally, a `reconciled_with` field in spec.yaml to record that two overlapping works have been explicitly reviewed for coherence.

---

## 9. Feature Flags and Trunk-Based Development

**What it is:** Feature flags decouple deployment from release. Multiple incomplete features coexist in the same codebase behind flags. Trunk-based development requires all work to merge to main frequently, using flags to hide incomplete work. The coordination challenge: multiple in-flight features touching the same code, all behind different flags, all merging to the same branch.

**Which problems it addresses:** P2 (islands), P4 (late-arriving requirements).

**What transfers:**
- *Parallel work on the same area is expected and managed, not prevented.* Feature flags accept that multiple changes to the same area will be in-flight simultaneously. The key is making each change self-contained and aware of the others.
- *The concept of "integration points"* — moments where all in-flight changes are checked against each other. Feature flag systems do this via CI/CD and integration tests. kerf could do this via periodic coherence checks.
- *Incremental integration rather than big-bang merges.* Instead of completing all related works independently and hoping they integrate, check integration at each step.

**What doesn't transfer:**
- Feature flags are a code-level mechanism. kerf operates at the spec level.
- CI/CD provides automated integration testing. Spec-level coherence can't be automatically tested.

**Concrete kerf idea:** A `kerf square --scope=area:adapter` command that runs coherence checks across all works touching the adapter area. Instead of squaring one work at a time, square a group of related works together. The check would verify: do these works' specs make compatible assumptions about the shared area? This is the periodic integration check adapted to specs.

---

## 10. Kanban and WIP Limits

**What it is:** Kanban visualizes work flowing through stages with explicit WIP limits per stage. When a stage hits its WIP limit, upstream work stops — you "stop starting, start finishing." The pull-based model means work is pulled into a stage only when capacity is available.

**Which problems it addresses:** P3 (intake/prioritization), P1 (persistent map).

**What transfers:**
- *WIP limits at the work level* directly address the "8 works in-flight, no idea what to do next" problem. If the WIP limit is 3, the answer is clear: finish one of the 3 before starting a 4th.
- *The board as a persistent visual map.* Kanban boards show all work, grouped by stage. `kerf map` could produce a Kanban-style view: works grouped by status, with WIP counts per status.
- *Pull-based flow.* Instead of pushing new works into the system, works are "pulled" when an agent finishes something and has capacity. `kerf next` is the pull mechanism.
- *Explicit policies per stage.* "Work in 'research' status must have its area overlaps reviewed before moving to 'change-spec'." These are stage transition policies.

**What doesn't transfer:**
- Kanban assumes humans managing a board. Agents need structured data, not visual boards.
- Kanban's stages are fixed. kerf's statuses are jig-specific and vary per work type.

**Concrete kerf idea:** `kerf map` output formatted as a stage-based view, grouping works by status with counts. A `wip_limit` per status category (e.g., max 2 works in "implementing" simultaneously). `kerf new` respects the limit with a warning. This gives the "stop starting, start finishing" discipline to agent-driven work.

---

## 11. PERT/CPM Critical Path Analysis

**What it is:** PERT (Program Evaluation and Review Technique) and CPM (Critical Path Method) model projects as networks of activities with dependencies and duration estimates. The critical path is the longest path through the network — any delay on it delays the whole project. Slack/float on non-critical activities tells you how much they can slip.

**Which problems it addresses:** P3 (prioritization/ordering).

**What transfers:**
- *Topological sort of the dependency graph produces a natural ordering.* This is computationally simple given kerf's existing `depends_on` data.
- *Float calculation* — which works have slack and which don't — helps an agent decide what to work on. A work with zero float (critical path) should be prioritized over one with weeks of float.
- *The network diagram as a persistent map.* PERT charts are literally the "map of the territory" that P1 describes as missing.

**What doesn't transfer:**
- Duration estimates require human judgment. Agent work duration is unpredictable.
- PERT's probabilistic estimates (optimistic/pessimistic/most likely) add complexity without clear value for spec work.

**Concrete kerf idea:** `kerf map --graph` produces a dependency graph in a text-based format (e.g., Mermaid or DOT) showing works as nodes, dependencies as edges, and statuses as colors/labels. This is the PERT chart for the work portfolio. The topological sort is computed and emitted as the recommended work order.

---

## 12. The MEOW/Gas Town Model (Steve Yegge)

**What it is:** Yegge's "molecules of work" concept from the Gas Town series. Work is decomposed into small, durable units (molecules/beads) with explicit state. The key ideas: (a) work state is durable and survives session boundaries, (b) molecules have explicit lifecycle states, (c) an orchestrator manages the flow of molecules through the system.

**Which problems it addresses:** P1 (persistent map), P3 (ordering).

**What transfers:**
- kerf already embodies this at the work level (works have durable state in spec.yaml) and beads embody it at the task level. The gap is the *layer between*: the work graph as a durable artifact.
- The idea of *explicit lifecycle state at every level* reinforces that the work graph itself needs durable state — not just individual works, but their relationships and collective state.
- The orchestrator concept maps to kerf providing orientation data that an external orchestrator (harmonik) consumes.

**What doesn't transfer:**
- MEOW is conceptual/aspirational, not a shipped system with proven patterns.
- The tight coupling between molecules and execution doesn't fit kerf's "structure but don't execute" philosophy.

**Concrete kerf idea:** Make the work graph itself a durable, versioned artifact. Not just individual spec.yaml files, but a project-level `graph.yaml` or similar that captures: all works, their states, their relationships, area overlaps, and the current recommended ordering. This is snapshotted alongside individual works. When an agent starts a session, it reads the graph artifact for orientation, not individual spec.yaml files.

---

## 13. Monorepo Workspace Protocols (Nx, Turborepo, pnpm workspaces)

**What it is:** Monorepo tools manage multiple packages in a single repository. They solve coordination problems: which packages are affected by a change, what order to build/test them, how to avoid redundant work. Nx's "project graph" is a persistent, computed DAG of all packages and their dependencies, cached and incrementally updated.

**Which problems it addresses:** P2 (islands), P3 (ordering), P5 (coherence).

**What transfers:**
- *The project graph as a computed artifact.* Nx doesn't ask developers to manually declare inter-package dependencies — it infers them from imports. kerf could infer work relationships from shared area tags, shared spec references, or overlapping artifact content.
- *Affected detection.* "What is affected by this change?" is the core question for both monorepos and work graphs. When a spec changes, which works are affected?
- *Task pipelines.* Nx defines that for each package, "build depends on build of dependencies." kerf could define that for each work, "implementation depends on implementation of dependencies."

**What doesn't transfer:**
- Monorepo tools operate on code with deterministic dependency detection (import analysis). Work relationships are semantic.
- Caching and incremental computation assume reproducible builds. Spec work is creative, not reproducible.

**Concrete kerf idea:** `kerf map` computes the work graph by reading all spec.yaml files in the project, building a dependency graph, annotating with area tags, and caching the result. Incremental updates when a single work changes. The graph is the computed artifact that all other coordination features build on — ordering, overlap detection, affected analysis.

---

## 14. Wiki Cross-Referencing and "What Links Here" (MediaWiki/Confluence)

**What it is:** MediaWiki tracks every page that links to a given page ("What links here"). Confluence has "Incoming links" on every page. When you edit a page, you can see everything that references it and might be affected. Categories group pages into navigable collections.

**Which problems it addresses:** P2 (islands), P4 (late-arriving requirements), P5 (coherence).

**What transfers:**
- *"What links here" for works.* Given a spec file, which works reference it? Given a work, which other works depend on it or touch the same areas? This reverse-lookup capability is missing from kerf's current dependency model (which only tracks forward references in `depends_on`).
- *Categories as lightweight grouping.* Wiki categories are just tags — a page can belong to multiple categories. Works could similarly belong to multiple area groups without hierarchical structure.
- *Orphan detection.* Wikis flag pages with no incoming links. kerf could flag works with no connections to other works — they might be missing relationships.

**What doesn't transfer:**
- Wikis are centralized databases with query capabilities. kerf is filesystem-based.
- Real-time link tracking requires indexing. kerf would need to build an index on demand.

**Concrete kerf idea:** `kerf show <codename>` includes a "Related works" section computed by: (a) reverse dependency lookup (what depends on this), (b) shared area tags, (c) shared spec references. This is the "What links here" feature. Also: `kerf map --orphans` flags works that have no dependencies, no dependents, and no area overlaps — potential islands that should be connected.

---

## 15. Design System Tokens and Shared Constraints (Figma, Style Dictionary)

**What it is:** Design systems define shared tokens (colors, spacing, typography) that all components must use. When a token changes, all components using it are affected. Style Dictionary is a build system that transforms tokens into platform-specific outputs. The key idea: shared constraints are defined once and consumed by many.

**Which problems it addresses:** P2 (islands), P5 (cross-work coherence).

**What transfers:**
- *Shared constraints as first-class artifacts.* Instead of hoping that independently-designed works make compatible choices, define the shared constraints explicitly and require works to reference them. For kerf: "the adapter interface is defined in this contract; all works touching the adapter conform to it."
- *Propagation of changes.* When a token changes, every consumer is affected. When a shared constraint changes, every work referencing it needs review. This is automatable.
- *Single source of truth for shared decisions.* Not duplicated across works; referenced from a central location.

**What doesn't transfer:**
- Design tokens are simple key-value pairs. Architectural constraints are complex documents.
- Automated propagation works for tokens; spec-level changes require judgment.

**Concrete kerf idea:** A `constraints/` directory at the project level. Each constraint is a markdown file defining a shared design decision (e.g., "adapter-interface.md" defines the adapter's public API contract). Works declare which constraints they depend on via a `conforms_to` field in spec.yaml. When a constraint is modified, `kerf affected constraints/adapter-interface.md` lists all conforming works. This is the "design token" pattern applied to architectural decisions.

---

## Summary Table

| # | Prior Art Domain | Primary Problem(s) | Key Transferable Pattern | Proposed kerf Mechanism |
|---|-----------------|-------------------|------------------------|------------------------|
| 1 | PM tools (Linear/Jira) | P1, P2, P3 | Grouping layer above works | `portfolio.yaml` with named work groups |
| 2 | Package managers | P2, P5 | Conflict detection via shared targets | Area manifests listing design decisions per area |
| 3 | Build systems (Bazel/Nx) | P3, P4, P5 | Affected analysis, topological ordering | `kerf next`, `kerf affected` commands |
| 4 | Knowledge graphs (Obsidian) | P1, P2, P5 | Emergent clustering from links/tags | `touches` field in spec.yaml, area-based clustering |
| 5 | ADRs | P4, P5 | Decision log with supersession chains | `decisions/` directory at project level |
| 6 | DDD context maps | P2, P5 | Shared kernel, explicit interface contracts | `contracts/` for shared design surfaces |
| 7 | Theory of Constraints | P1, P3 | Critical chain, WIP limits | `kerf map --critical-path`, `wip_limit` config |
| 8 | Git merge conflicts | P2, P4, P5 | Automatic overlap detection, manual resolution | Overlap warnings on `kerf new`/`kerf resume` |
| 9 | Feature flags / trunk-based | P2, P4 | Periodic integration checks | `kerf square --scope=area:X` for cross-work coherence |
| 10 | Kanban | P1, P3 | WIP limits, stage-based views, pull model | Stage-grouped `kerf map`, WIP limits per status |
| 11 | PERT/CPM | P3 | Dependency graph, critical path, float | `kerf map --graph` with topological ordering |
| 12 | MEOW/Gas Town | P1, P3 | Durable work graph as versioned artifact | Project-level `graph.yaml`, snapshotted |
| 13 | Monorepo tools (Nx) | P2, P3, P5 | Computed project graph, affected detection | Computed + cached work graph in `kerf map` |
| 14 | Wiki cross-references | P2, P4, P5 | Reverse lookups, "what links here" | Related works section in `kerf show`, orphan detection |
| 15 | Design system tokens | P2, P5 | Shared constraints as first-class artifacts | `constraints/` dir, `conforms_to` in spec.yaml |

---

## Cross-Cutting Observations

**Three patterns appear repeatedly across domains:**

1. **The computed graph.** Nearly every domain builds an explicit graph of relationships and uses it for ordering, overlap detection, and orientation. kerf has the raw materials (spec.yaml with `depends_on`) but doesn't compute or persist the aggregate graph. This is the highest-leverage gap.

2. **Shared constraints as first-class artifacts.** Package managers have lockfiles. DDD has shared kernels. Design systems have tokens. ADRs have decisions. The pattern: when multiple work items share a design surface, extract the shared part into an explicit, referenceable artifact rather than hoping for convergence. This directly addresses Problems 2 and 5.

3. **Overlap detection with manual resolution.** Git doesn't resolve conflicts; it surfaces them. Build systems detect affected targets; they don't decide what to do about them. The pattern: automatically detect overlaps (via area tags, shared spec references, or dependency analysis), surface them clearly, and leave resolution to the agent. This fits kerf's "guidance not gates" philosophy.

**One anti-pattern to avoid:**

Every domain that tried *automated resolution of semantic conflicts* failed or produced brittle results. Package managers can resolve version constraints because versions are ordered. But semantic conflicts (incompatible design choices) require judgment. kerf should detect and surface, not attempt to resolve.
