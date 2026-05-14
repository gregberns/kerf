# Process Designer — Brainstorm for Plan 005

> Perspective: workflow design, lifecycle management, state machines, process patterns, how work flows through an organization.

---

## Idea 1: Gated Status Transitions with Portfolio Pre-checks

**What it is:** Certain status transitions in a work's jig lifecycle become "gated" — before moving forward, the system runs a portfolio-level pre-check. The gate does not block (kerf is advisory), but it surfaces conflicts, overlaps, and stale assumptions before the agent charges ahead.

**Which problems it addresses:** P2 (islands), P4 (late requirements), P5 (coherence across modes).

**Process flow:**
1. Agent completes a pass and requests a status transition (e.g., `research` -> `change-spec`).
2. kerf runs a pre-check: "Are there other works targeting the same areas/components? Have any new works been created since this work entered its current status? Have any dependent works changed status?"
3. Pre-check results are emitted as advisory output. The agent sees: "Warning: work `circuit-breaker` was created since you entered `research` and touches the same adapter layer."
4. The agent can proceed, pause to reconcile, or amend.

**Gate points (where pre-checks add the most value):**
- Before `change-spec` or `spec-draft` — you're about to commit design decisions. Last chance to notice overlap.
- Before `tasks` — you're about to decompose into implementation units. Overlap after this point is expensive.
- Before `ready` / finalization — final coherence check.

**What could go wrong:**
- If gates produce too many false positives, agents learn to ignore them (alarm fatigue).
- Area detection is hard — how do you know two works touch the "adapter layer"? Requires tagging or text analysis that may be brittle.
- Gates slow down the happy path where works are genuinely independent.

---

## Idea 2: The Intake Funnel — Triage, Tag, Cluster, Sequence

**What it is:** A formalized intake workflow that every new work passes through before entering its jig lifecycle. The funnel assigns priority, tags affected areas, detects cluster membership, and places the work in a sequence relative to other works.

**Which problems it addresses:** P2 (islands), P3 (no intake/prioritization), P5 (coherence).

**Process flow:**
1. **Capture:** `kerf new` creates the work as today, but status starts at a pre-jig value like `intake` or `triage`.
2. **Tag:** The creating agent (or a triage pass) annotates the work with affected system areas/components. These tags are structured metadata in spec.yaml, not free text.
3. **Cluster:** kerf checks existing works for overlapping area tags. If overlap is found, the new work is flagged as part of a cluster. The agent sees: "This work overlaps with [list]. Consider reviewing them before proceeding."
4. **Sequence:** The agent (or orchestrator) sets priority and dependency relationships. The work is now "triaged" and moves into its jig's first pass.

**What could go wrong:**
- Adds ceremony to work creation. For a single urgent bug fix, the funnel is overhead.
- Area tags require a taxonomy. Who defines it? How does it evolve? A fixed taxonomy is brittle; a freeform one leads to inconsistency.
- Clustering based on tags may miss semantic overlap (two works both touch "auth" but in completely different ways).
- Risk of the triage step becoming a bottleneck — every work pauses while waiting for triage.

---

## Idea 3: The Work Graph as a First-Class Process Object

**What it is:** Instead of works being individual units that happen to have `depends_on` fields, the entire work graph for a project becomes a first-class object with its own state, lifecycle, and operations. Think of it as the "project board" — not just a view, but a managed process artifact.

**Which problems it addresses:** P1 (no persistent map), P3 (no prioritization), P5 (coherence).

**Process flow:**
1. kerf maintains a `portfolio.yaml` (or similar) at the project level that is auto-generated from the work graph.
2. Every status change, work creation, or dependency change updates the portfolio view.
3. `kerf map` emits the portfolio state: works by status, dependency chains, critical path, blocked items, cluster memberships.
4. Session orientation reads the portfolio first, then drills into the specific work.
5. The portfolio tracks aggregate metrics: works by stage, velocity (works completed per session), bottlenecks (works blocked longest).

**What could go wrong:**
- The portfolio file becomes stale if kerf commands don't consistently update it. Filesystem-as-database means no triggers.
- Aggregate metrics may be misleading — "3 works in research" means different things for 3-work vs. 30-work portfolios.
- Risk of over-engineering a "project management" layer that adds complexity without proportional value for small portfolios.

---

## Idea 4: Amendment Protocol for In-Flight Works

**What it is:** A formal process for modifying a work that has already progressed past its design phases. Instead of ad-hoc edits or creating orphaned new works, amendments follow a defined protocol that preserves traceability and forces reconciliation with completed work.

**Which problems it addresses:** P4 (late requirements), P2 (islands — by absorbing related work).

**Process flow:**
1. A new requirement arrives that overlaps with in-flight work `alpha` (currently at `implementing`).
2. The orchestrator invokes `kerf amend alpha` (or similar), which:
   a. Creates an amendment record — a timestamped entry in the work's metadata noting what changed and why.
   b. Flags all tasks/beads derived from the amended section as potentially stale.
   c. Optionally rolls the work's status back to a design phase (e.g., `change-spec`) for the amended portion only.
3. The agent re-enters the design pass scoped to the amendment, produces updated artifacts, and re-generates affected tasks.
4. Implementation resumes with awareness of what changed.

**Alternatives within the protocol:**
- **Absorb:** The new requirement is folded into the existing work (amendment).
- **Fork:** The new requirement becomes a new work with an explicit `amends` dependency on the original, inheriting its design context.
- **Defer:** The new requirement is captured but explicitly queued for after the current work completes.

**What could go wrong:**
- Amendments to in-flight work risk invalidating completed tasks. The reconciliation cost may exceed starting fresh.
- Partial status rollback is conceptually complex — a work is at `implementing` but part of it is back in `change-spec`. The status model (a single string) can't represent this without extension.
- Agents may over-use amendment (everything becomes an amendment to the first work) rather than creating properly scoped new works.

---

## Idea 5: Kanban-Style WIP Limits and Pull-Based Scheduling

**What it is:** Apply kanban principles: limit work-in-progress at each lifecycle stage, and use a pull-based model where agents pull the next highest-priority ready work rather than being assigned work.

**Which problems it addresses:** P3 (no prioritization), P1 (persistent map — the board IS the map).

**Process flow:**
1. Each lifecycle stage has an advisory WIP limit (configurable per project). E.g., max 2 works in `change-spec`, max 3 in `implementing`.
2. `kerf next` (new command) examines the work graph and returns the highest-priority work that is: (a) not blocked by dependencies, (b) not in a stage at its WIP limit, (c) not in an active session.
3. Priority is determined by: explicit priority field > dependency graph position (works that unblock the most) > creation order.
4. When a WIP limit is hit, the system signals "stop starting, start finishing" — encouraging completion of in-flight work before pulling new work.

**Theory of Constraints connection:** The WIP limits force the bottleneck to be visible. If 5 works are stuck in `integration` (the reconciliation pass), that's a signal the integration process needs attention, not that more works should enter `research`.

**What could go wrong:**
- WIP limits are meaningless without enforcement, and kerf is advisory-only. An agent can ignore the limit.
- Priority determination is hard. Explicit priority requires someone to maintain it. Graph-based priority (what unblocks the most) can lead to pathological choices (always working on infrastructure, never on features).
- Pull-based scheduling assumes a pool of ready work. In practice, you might have 1 work ready and 7 blocked, making the "pull" trivial.
- WIP limits tuned wrong create artificial bottlenecks or have no effect.

---

## Idea 6: Reconciliation Passes — Periodic Cross-Work Coherence Checks

**What it is:** A scheduled (or triggered) process that reviews all active works touching the same area and produces a reconciliation report. This is not a per-work pass but a cross-cutting activity that happens at the portfolio level.

**Which problems it addresses:** P2 (islands), P5 (coherence across modes), P4 (late requirements — catches them during reconciliation).

**Process flow:**
1. **Trigger:** Reconciliation runs when: (a) a new work enters a design phase and overlaps with existing works, (b) any work in a cluster changes status, (c) on a schedule (e.g., at the start of each session), or (d) explicitly invoked.
2. **Scope:** The reconciliation pass examines all active works tagged with the same area/component.
3. **Analysis:** For each cluster, the pass checks: Are the design decisions compatible? Do the specs reference each other? Are there contradictory assumptions? Are there shared interfaces that need a single source of truth?
4. **Output:** A reconciliation report listing conflicts, gaps, and recommended actions (amend, merge, re-sequence, add dependency).
5. **Action:** The orchestrator reviews the report and decides which actions to take. Actions feed back into individual works.

**What could go wrong:**
- Reconciliation is expensive — reading and comparing multiple specs requires significant context.
- The output may be noisy, especially if area tags are broad ("touches the API layer" matches half the works).
- Without someone acting on the reconciliation report, it's just more noise. Who is accountable?
- Frequency is a dial: too often = overhead, too seldom = misses the conflicts it's supposed to catch.

---

## Idea 7: Area Specs — Shared Design Anchors for Clustered Works

**What it is:** When multiple works touch the same system area, an "area spec" is created (or already exists) that serves as the shared design anchor. Individual works must conform to the area spec rather than making independent design choices for that area.

**Which problems it addresses:** P2 (islands — directly), P5 (coherence — provides the single source of truth), P4 (late requirements — new requirements update the area spec, not individual works).

**Process flow:**
1. Areas/components are identified in the project's spec structure (they may already exist as spec files like `specs/adapter.md`).
2. When a work enters a design phase and touches an area, its jig pass includes a step: "Read the area spec for [X]. Design within its constraints."
3. If the work needs the area spec to change, it proposes an amendment to the area spec (not just its own work artifacts). This amendment is visible to all other works in the cluster.
4. Late-arriving requirements that touch the same area go through the area spec first: update the shared design, then update individual works to conform.

**What could go wrong:**
- Area specs become bottlenecks — every work touching that area has to read and potentially amend the shared spec. Contention.
- Area boundaries are fuzzy. "The adapter" might mean different things to different works.
- Area specs add a governance layer that may not scale — maintaining area specs is its own workload.
- Chicken-and-egg: the first work to touch an area has to create the area spec, but it may not have enough context to make good shared decisions.

---

## Idea 8: Session Orientation Protocol — The "Standup" Pattern

**What it is:** Every agent session begins with a structured orientation step (analogous to a daily standup) that provides portfolio context, not just work-level context. This replaces the HANDOFF-only model.

**Which problems it addresses:** P1 (persistent map — the orientation IS the map consumption), P2 (islands — orientation surfaces related works), P3 (prioritization — orientation includes "what's next").

**Process flow:**
1. Agent session starts. Instead of (or before) reading HANDOFF.md, the agent runs `kerf orient` (or `kerf map`).
2. kerf emits a structured orientation block:
   - **Portfolio snapshot:** All active works, their statuses, and a summary of recent changes.
   - **Blocked/blocking:** Which works are blocked and by what. Which works, if completed, would unblock the most.
   - **Clusters:** Which works are related (same area/component). Any unresolved reconciliation issues.
   - **Recommendations:** Top 1-3 works to focus on this session, with rationale.
3. The agent reads the orientation, selects its focus, and proceeds.
4. At session end, the agent runs `kerf debrief` (or updates the work's session notes), recording what was accomplished and any new information.

**What could go wrong:**
- Orientation output could be too long for agent context windows, defeating the purpose.
- "Recommendations" require a prioritization model — garbage in, garbage out.
- If the orientation is too detailed, agents spend context budget on portfolio info they don't need. If too terse, it misses critical context.
- The debrief step is easy to skip, degrading the orientation's quality over time.

---

## Idea 9: Event-Driven Coordination — Lifecycle Hooks

**What it is:** Key lifecycle events in the work graph emit signals that trigger coordination actions. This is the "reactive" approach — instead of periodic checks or manual reconciliation, the system responds to events as they happen.

**Which problems it addresses:** P2 (islands — overlap detected at creation), P4 (late requirements — amendment triggers re-evaluation), P5 (coherence — events propagate across modes).

**Events and their coordination actions:**

| Event | Coordination Action |
|-------|-------------------|
| Work created | Check for area overlap with existing works. If overlap, flag cluster. |
| Status changed to design phase | Load context from related works in same cluster. |
| Status changed to `tasks` | Verify no conflicting design decisions in cluster. |
| Status changed to `ready` | Final coherence check against cluster and dependencies. |
| Dependency completed | Notify dependent works. Update portfolio readiness. |
| Work amended | Flag potentially stale tasks in dependent works. |
| New work overlaps in-flight work | Emit warning with options: absorb, defer, fork. |

**Process flow:**
1. kerf commands already change state (create work, change status, add dependency).
2. After each state change, kerf runs the relevant hooks.
3. Hooks produce advisory output — warnings, recommendations, context loads.
4. The agent or orchestrator acts on the output.

**What could go wrong:**
- Hook logic becomes complex and hard to maintain. Each new event type needs new coordination logic.
- Advisory hooks that agents ignore provide no value but add noise.
- Event-driven systems can have cascading effects — one status change triggers warnings on 5 related works, each of which triggers more warnings.
- Without a way to "acknowledge" or "dismiss" warnings, they accumulate and become stale.

---

## Idea 10: The Readiness Model — Explicit "Ready to Implement" Criteria

**What it is:** Replace the implicit assumption that `status: ready` means "ready to implement" with an explicit readiness model that checks multiple dimensions. A work is truly ready only when all dimensions are green.

**Which problems it addresses:** P2 (islands — readiness checks for cluster coherence), P3 (prioritization — only ready works enter the implementation queue), P4 (late requirements — readiness degrades when new overlapping work arrives).

**Readiness dimensions:**
1. **Jig completeness:** All required passes have artifacts. (This is what `kerf square` checks today.)
2. **Dependency satisfaction:** All `must-complete-first` dependencies are complete.
3. **Cluster coherence:** No unresolved conflicts with other works in the same area cluster.
4. **Freshness:** The work's design decisions haven't been invalidated by changes to the codebase or related specs since the work was designed.
5. **Task decomposition:** Tasks/beads are generated and their dependency graph is valid.

**Process flow:**
1. `kerf square` (or a new `kerf ready` command) evaluates all readiness dimensions.
2. Output is a structured checklist: green/yellow/red for each dimension with details.
3. Only works that are fully green enter the implementation queue.
4. When a work loses readiness (e.g., a new overlapping work arrives), its readiness status degrades from green to yellow, signaling the need for review.

**What could go wrong:**
- "Cluster coherence" and "freshness" are judgment calls, not binary checks. Automating them may produce false positives/negatives.
- Readiness degradation (going from green to yellow) could create churn — a work is ready, then not ready, then ready again, as related works arrive and are resolved.
- The readiness model adds complexity to the status model. Now you have both a jig status AND a readiness status, and they can conflict.
- Overly strict readiness criteria could mean nothing is ever "ready" in a fast-moving project.

---

## Idea 11: Two-Phase Commit for Clustered Works

**What it is:** Borrowing from distributed systems: when multiple works in a cluster are approaching their design completion, they enter a "two-phase commit" — each work tentatively finalizes its design, then all works in the cluster are reviewed together before any of them move to implementation.

**Which problems it addresses:** P2 (islands — forces coordinated design review), P5 (coherence — the review IS the coherence mechanism).

**Process flow:**
1. Work A reaches `change-spec` (or equivalent design completion pass). It is "tentatively done" — design is proposed but not committed.
2. kerf checks: is Work A part of a cluster? If yes, it enters a `pending-review` state.
3. When all works in the cluster reach `pending-review` (or a timeout/threshold is reached), a cluster review is triggered.
4. The review compares the design decisions across all clustered works, identifies conflicts, and either:
   a. **Commits** — all designs are compatible, all works advance to `tasks`.
   b. **Aborts** — conflicts found, specific works are sent back to their design phase with feedback.

**What could go wrong:**
- Waiting for all works in a cluster to reach the review point creates a synchronization bottleneck. If one work is slow, all are blocked.
- The timeout/threshold mechanism is hard to tune — too aggressive and you review incomplete clusters, too lenient and you wait forever.
- Clusters may be large and poorly defined. A "review all works touching the API" could be 10 works.
- This is a significant departure from kerf's "jigs are guidance, not gates" philosophy. It introduces hard coordination points.

---

## Idea 12: The Work Ledger — Append-Only Decision Log

**What it is:** Each project maintains an append-only ledger of all significant decisions, state changes, and coordination events across all works. The ledger provides the "persistent map" (P1) and serves as the institutional memory that survives across sessions.

**Which problems it addresses:** P1 (persistent map — the ledger IS the memory), P4 (late requirements — the ledger captures when and why requirements changed), P5 (coherence — the ledger makes cross-cutting decisions traceable).

**Process flow:**
1. Every significant event appends an entry to the project ledger: work created, status changed, dependency added, design decision made, amendment applied, reconciliation completed.
2. Each entry is timestamped, tagged with the affected work(s) and area(s), and includes a brief rationale.
3. At session start, the agent reads the recent ledger entries (since last session) to understand what changed in the portfolio.
4. The ledger is queryable: "Show me all decisions affecting the adapter layer in the last week."

**What could go wrong:**
- The ledger grows indefinitely. Without summarization or compaction, it becomes as overwhelming as accumulated HANDOFF docs.
- Entry quality depends on agents writing good rationale. If entries are just "status changed to research," the ledger is a changelog, not a decision log.
- Querying the ledger requires tooling that doesn't exist yet.
- Duplication with existing mechanisms — session notes, spec.yaml timestamps, and snapshots already capture some of this information.

---

## Cross-Cutting Observations

**On gating vs. advisory:** kerf's philosophy is "jigs are guidance, not gates." But the coordination problems exist precisely because there are no gates. The tension is real: gates add friction but prevent drift; advisory signals are low-friction but easy to ignore. The right answer is probably: gates at high-leverage points (before design commitment, before finalization), advisory everywhere else.

**On the area/component taxonomy:** Ideas 1, 2, 6, 7, and 10 all depend on knowing which system areas a work touches. This is the hardest prerequisite — getting area tagging right is foundational. Options range from explicit tags (manually maintained) to spec-file references (which specs does this work modify?) to code-path analysis (which packages does this work's implementation touch?). Spec-file references are probably the most natural fit for kerf since works already produce spec artifacts.

**On Theory of Constraints:** The constraint in this system is not processing capacity (agents are cheap) but coherence. The bottleneck is "making sure multiple works don't make contradictory decisions." WIP limits (Idea 5) address this indirectly by reducing the number of works that can be simultaneously in design phases. But the direct intervention is reconciliation (Idea 6) or area specs (Idea 7).

**On the two process modes (plan-first vs. spec-first):** Both modes pass through a design phase where architectural decisions are made. The coordination layer should hook into that phase regardless of which mode produced it. This suggests the coordination triggers should be on status values that map to "design decisions being made" rather than on jig-specific pass names.

**On minimal viable coordination:** If forced to pick only two ideas, I'd pick Idea 8 (session orientation) and Idea 4 (amendment protocol). Orientation solves the immediate pain of agents losing context. Amendment solves the immediate pain of late requirements. Everything else builds on top.
