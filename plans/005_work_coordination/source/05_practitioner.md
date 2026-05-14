# Practitioner Perspective — Work Coordination Brainstorm

> Someone who has lived through multi-session, multi-agent implementation runs and developed workarounds for the exact failures described.

---

## 1. The Session Brief: A Computed Orientation Document

**What it is:** A `kerf orient` command that generates a structured snapshot — not a narrative HANDOFF, but a computed document synthesized from actual work state. It reads all active works for the project, their statuses, their dependency edges, what changed since last session, and what's blocked/actionable. Output is a single Markdown document an agent reads at session start.

**Which problems it helps:** Problem 1 (no persistent map), Problem 3 (no intake queue), Problem 5 (coherence).

**How you'd use it day-to-day:** First thing every session: `kerf orient`. The agent reads the output and has the full landscape in 200-400 lines. No stale HANDOFF narrative. No "continue where we left off" guesswork. The orient output includes: (a) all works grouped by status, (b) the dependency graph as a simple ASCII/text representation, (c) which works are actionable (no incomplete must-complete-first deps), (d) which works share areas/components, (e) what changed since the last session timestamp.

**The catch:** Requires that works and their metadata are actually kept up to date. If agents don't update status fields, the orient output is stale. But this is self-reinforcing — once the orient output is useful, agents have incentive to update status.

**Why this is high-leverage:** It replaces the manually-maintained HANDOFF with a computed artifact that can't drift from reality. The HANDOFF becomes a short "intent and judgment calls" supplement rather than the entire orientation mechanism.

---

## 2. Area Tags on Works (Component/Area Tagging)

**What it is:** A simple `areas` list field in spec.yaml. Free-form strings like `adapter`, `auth`, `database`, `api-layer`. No taxonomy, no hierarchy, just tags. `kerf orient` and `kerf show` use them to group works and surface overlaps.

**Which problems it helps:** Problem 2 (works are islands), Problem 4 (late requirements), Problem 5 (coherence).

**How you'd use it day-to-day:** When creating a work, you tag it with the areas it touches. `kerf orient` groups works by area, so you immediately see "3 works touch `adapter`" and can read them together. When a late requirement arrives, the agent checks `kerf list --area adapter` and sees what else is in flight there before designing in isolation.

**The catch:** Tags are only useful if applied consistently. But they're low-cost to add (a list of strings) and the orient command makes their value immediately visible. The real risk is tag proliferation — `adapter` vs `adapters` vs `adapter-layer`. A simple normalization (lowercase, strip trailing s) handles 80% of that.

**Why this matters:** This is the cheapest possible intervention for Problem 2. No graph theory, no ontology, just "what does this touch?" as a list of strings. The value comes from the *grouping* in the orient output, not from the tags themselves.

---

## 3. The "Next" Command: Computed Work Selection

**What it is:** `kerf next` reads the dependency graph, area tags, and status of all works. It returns a ranked list of what's actionable, with reasons. Not AI prioritization — mechanical: "these works have no unresolved must-complete-first deps, are not shelved, and are in the earliest lifecycle stage." Tie-breaking by: most downstream dependents (i.e., unblocks the most), then oldest.

**Which problems it helps:** Problem 3 (no intake queue).

**How you'd use it day-to-day:** Agent finishes a work, runs `kerf next`. Gets back a list like:

```
1. brave-falcon  (status: problem-space, unblocks: 3 works)
2. green-oak     (status: research, unblocks: 1 work)
3. red-wave      (status: analyze, unblocks: 0 works)
```

The agent picks #1 unless the user has expressed a preference. The user can override by just saying "work on green-oak instead."

**The catch:** "Most downstream dependents" is a rough proxy for priority. It doesn't capture business value, urgency, or "Greg really cares about this one." But it's a sane default that beats random selection. A `priority` field (integer, optional) in spec.yaml could override the mechanical ranking for cases where the user has an opinion.

---

## 4. Overlap Warnings at Work Creation Time

**What it is:** When `kerf new` creates a work, it checks area tags of existing active works. If there's overlap, it emits a warning: "Note: 2 other active works also touch `adapter`: brave-falcon (status: implementing), green-oak (status: research). Consider reviewing their specs before proceeding." That's it. A warning, not a gate.

**Which problems it helps:** Problem 2 (works are islands), Problem 4 (late requirements), Problem 5 (coherence).

**How you'd use it day-to-day:** You create a circuit-breaker work. kerf says "hey, brave-falcon (retry logic) also touches `adapter` and is 40% implemented." The agent now knows to read brave-falcon's spec before designing the circuit-breaker. It might decide to add a dependency, or just design with awareness.

**The catch:** Only works if area tags are applied. Also, the warning is only at creation time — if you tag areas later, you miss the overlap signal. Could add overlap checking to `kerf orient` too (and should).

---

## 5. The Work Graph File: A Single Durable Artifact

**What it is:** A `work-graph.yaml` file at the project level (`~/.kerf/projects/{project-id}/work-graph.yaml`) that is computed (not manually maintained) from the set of active works. It's regenerated on every `kerf` command that mutates state. Contains: all works with status, all dependency edges, all area tags, and derived clusters (works sharing areas).

**Which problems it helps:** Problem 1 (no persistent map), Problem 3 (intake queue), Problem 5 (coherence).

**How you'd use it day-to-day:** External tools (harmonik, custom scripts, the user's own dashboard) can read this file to get the full picture without walking the filesystem. It's also what `kerf orient` reads internally. The key insight: the work graph already exists implicitly across scattered spec.yaml files. This just materializes it as a single queryable artifact.

**The catch:** Keeping it in sync. If someone edits spec.yaml directly (which is allowed — filesystem is the database), the graph file is stale until the next kerf command runs. Could mitigate with a "recompute" command, but the real answer is: it's a cache, not a source of truth. If it's stale, regenerate it.

**Alternative:** Skip the file entirely. Just have `kerf orient` and `kerf next` compute from spec.yaml files on demand. The filesystem IS the graph. The file is only worth it if external tools need to read it without calling kerf.

---

## 6. The "Entangle" Command: Explicit Work Linking for Late Requirements

**What it is:** `kerf entangle brave-falcon circuit-breaker` creates a bidirectional `entangled-with` relationship type (new, alongside `must-complete-first` and `inform`). Entangled works share a design context: when you resume one, kerf loads the other's current artifacts as context. When you run `kerf square` on one, it checks for contradictions with the other.

**Which problems it helps:** Problem 4 (late requirements), Problem 2 (works are islands).

**How you'd use it day-to-day:** Circuit-breaker requirement arrives late. You create the work, then `kerf entangle retry-logic circuit-breaker`. From now on, any agent working on either work sees the other's artifacts in its context. The specs are designed with mutual awareness. Implementation can still happen separately, but the design phase is coupled.

**The catch:** "Check for contradictions" is vague. What does that actually mean computationally? Realistically, kerf can surface the entangled work's artifacts for the agent to review — the agent checks for contradictions, not kerf. The value is in the *surfacing*, not in automated analysis.

**Simpler alternative:** Just use `inform` dependencies bidirectionally. The new relationship type might not be necessary if `inform` already causes context loading. The key insight is bidirectionality — if A informs B, B should also inform A. Maybe just make `kerf link A B --mutual` add inform deps in both directions.

---

## 7. Priority Field + Priority Override

**What it is:** An optional `priority` integer field in spec.yaml (lower = higher priority, like Unix nice values or P0/P1/P2). Defaults to unset (meaning "use the mechanical ranking from `kerf next`"). When set, overrides the dependency-based ranking.

**Which problems it helps:** Problem 3 (intake queue).

**How you'd use it day-to-day:** Most works don't get a priority — the dependency graph determines order. But when Greg says "drop everything, this one matters," you `kerf set brave-falcon priority 0` and it jumps to the top of `kerf next` output. Simple, explicit, no AI magic.

**The catch:** Priority systems tend to degrade. Everything becomes P0. Mitigate by making it optional and defaulting to mechanical ranking. Priority is the escape hatch, not the primary mechanism.

---

## 8. Session Bookends: Structured Start and End

**What it is:** Two lightweight rituals:
- **Session start:** `kerf orient` (already described above) — the agent reads the landscape.
- **Session end:** `kerf checkpoint` — the agent writes a structured entry: what was accomplished (list of works touched, status changes), what decisions were made (design choices with rationale), what's blocked or needs attention. This goes into a project-level `sessions.log` or similar, not into individual work SESSION.md files.

**Which problems it helps:** Problem 1 (persistent map), Problem 5 (coherence).

**How you'd use it day-to-day:** At session end, instead of writing a free-form HANDOFF.md, the agent runs `kerf checkpoint` which prompts for structured data. The next session's `kerf orient` includes a "last session summary" section pulled from this. Over 10 sessions, you have a structured log of decisions and progress, not a growing narrative blob.

**The catch:** Agents might not run `kerf checkpoint` at session end (sessions end in various ways — timeout, crash, user closes the window). Mitigate: `kerf orient` at session start can detect "last session didn't checkpoint" and warn. The checkpoint doesn't have to be perfect — even "these works changed status since last orient" is useful.

---

## 9. Area-Level Spec Anchors

**What it is:** For areas that multiple works touch, create a lightweight "area spec" — not a full kerf work, but a reference document that captures the current design of that area. Lives at `~/.kerf/projects/{project-id}/.areas/adapter.md` or similar. When works tagged with `adapter` are being designed, agents read the area spec for constraints. When a work changes the adapter's design, it updates the area spec.

**Which problems it helps:** Problem 2 (works are islands), Problem 5 (coherence).

**How you'd use it day-to-day:** First work touching the adapter writes the area spec as part of its design pass. Second work reads it and designs within those constraints (or proposes amendments). Third work does the same. The area spec is the coordination surface — it accumulates the design decisions that all adapter works must respect.

**The catch:** This is overhead. Maintaining area specs alongside work specs is duplicative. Works well for hot areas (the adapter that 5 works touch), wasteful for areas only one work touches. Mitigate: only create area specs when the area overlap warning fires. If only one work touches an area, there's no spec to maintain.

**Trap warning:** This can easily become a bureaucratic bottleneck. Keep area specs minimal — design constraints and interface contracts, not full specifications. If the area spec becomes a second source of truth competing with the actual code specs, you've made things worse.

---

## 10. The Weekend Hack: A "kerf status" Dashboard

**What it is:** A single command that dumps everything an orchestrator (human or agent) needs in one screen. Not a TUI, not a web UI — just a well-formatted text dump. Works grouped by status (active / blocked / completed). Dependency edges shown inline. Area clusters highlighted. Actionable items starred.

Example output:

```
PROJECT: acme-webapp (7 active, 3 blocked, 12 completed)

ACTIONABLE (no blocking deps):
  * brave-falcon   [implementing]  adapter retry     (unblocks: 3)
  * green-oak      [research]      API redesign      (unblocks: 1)
  * red-wave       [analyze]       logging overhaul  (unblocks: 0)

BLOCKED:
  ~ circuit-break  [problem-space] circuit breaker   (blocked by: brave-falcon)
  ~ fast-panda     [tasks]         perf testing      (blocked by: green-oak, red-wave)
  ~ slim-tiger     [ready]         DB migration      (blocked by: fast-panda)

AREA CLUSTERS:
  adapter: brave-falcon, circuit-break, fast-panda
  api:     green-oak, red-wave

LAST SESSION (2026-05-07):
  Completed 12 beads on brave-falcon (implementing → implementing)
  Design decision: retry uses exponential backoff with jitter, shared state via context object
```

**Which problems it helps:** All five, to some degree.

**How you'd use it day-to-day:** This IS your session start. Read this, orient, pick work, go. At session end, it's your "what did we accomplish" check.

**The catch:** It's read-only. It tells you the state but doesn't help you change it. That's fine — the value is in the visibility, not in automation.

---

## 11. Dependency-Aware Bead Generation

**What it is:** When generating beads (tasks) for a work, check if the work has `inform` or `entangled-with` dependencies on other works. If so, include a "context bead" at the start of the bead list that says "read these artifacts from these related works before proceeding." Not a code task — a context-loading task.

**Which problems it helps:** Problem 2 (works are islands), Problem 4 (late requirements).

**How you'd use it day-to-day:** You generate beads for the circuit-breaker work. It depends on (inform) the retry-logic work. The first bead is: "Read brave-falcon's spec and implementation status. Note design decisions about shared state, backoff strategy, and the adapter interface changes. Record constraints that circuit-breaker must respect." Every subsequent bead executes with that context loaded.

**The catch:** This is a beads/harmonik concern, not a kerf concern. kerf's job is to surface the relationships; the task executor decides how to use them. But kerf could emit "recommended context" as part of `kerf show` output that harmonik consumes.

---

## Ranking by Leverage

If I could only build one thing: **Idea 1 (kerf orient)** — the computed session brief. It's the single highest-leverage intervention because it replaces the lossy HANDOFF narrative with a computed snapshot that can't drift. Every other problem becomes more manageable once agents can see the full landscape at session start.

If I could build two: Add **Idea 2 (area tags)**. Dirt cheap to implement (a list of strings in spec.yaml), and it enables overlap detection (Idea 4) and area clustering in the orient output.

If I could build three: Add **Idea 3 (kerf next)**. Removes the "what should I work on?" question from agents and humans. It's mechanical, not magical, and it gives a sane default ordering.

Everything else is useful but secondary. The pattern is: **visibility first, then structure, then automation**. Most coordination failures come from agents not seeing the landscape, not from lacking sophisticated workflows.

## Traps to Avoid

1. **Over-structuring the graph.** Typed edges, weighted relationships, transitive closure — all sound good in design and create maintenance burden in practice. Start with two relationship types (blocks, informs) and free-form area tags. Add structure only when you hit a concrete failure that requires it.

2. **Mandatory processes.** Any coordination mechanism that's required rather than helpful will be skipped or resented. Everything should be advisory with visible value. Warnings, not gates.

3. **Automated conflict detection.** "kerf will detect when two specs contradict" sounds amazing and is nearly impossible to implement well. Surface the overlap, let the agent (or human) evaluate. Don't try to be smart about content — be smart about adjacency.

4. **Building a project management tool.** kerf is a spec-writing tool. The coordination layer should make spec-writing more coherent, not turn kerf into Jira. If you find yourself adding story points, sprints, or burndown charts, you've gone too far.

5. **Perfect session handoff.** The goal isn't perfect context transfer between sessions — it's "enough context to not make incompatible decisions." A 90% accurate computed snapshot beats a 60% accurate manually-written narrative. Don't let perfect be the enemy of good.
