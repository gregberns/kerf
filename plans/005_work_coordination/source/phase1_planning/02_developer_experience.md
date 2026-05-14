# Brainstorm: Developer Experience Perspective

> Agent B — What does the agent/user actually need moment-to-moment?

Reading the five problems through a UX lens: every problem is ultimately an **information-at-the-right-time** problem. The map doesn't exist, so agents can't orient. Works are islands because nobody sees the connections. Prioritization fails because the ordering information isn't surfaced. Late requirements are painful because there's no mechanism to re-contextualize. Coherence fails because nobody has portfolio-level peripheral vision.

The question throughout: what's the **minimum viable context** at each decision point?

---

## Idea 1: `kerf map` — The Persistent Orientation Command

**Problems addressed:** 1 (No Persistent Map), 3 (No Intake/Prioritization), 5 (Coherence)

The single highest-leverage addition. A command that prints the entire work graph for a project in a dense, structured format designed for agent consumption at session start.

**What it shows:**
- All works, grouped by status phase (specifying / ready / implementing / complete)
- Dependency edges between works
- Which areas of the system each work touches (from tags or annotations)
- Age and staleness indicators
- A one-line "trajectory" summary: what's the overall direction of the project right now

**Example CLI output:**

```
$ kerf map

kerf-cli (12 works, 3 active, 2 ready, 4 implementing, 3 complete)

SPECIFYING
  swift-maple       plan    research     adapter retry logic        [adapter]    2d ago
  blue-hawk         spec    decompose    circuit breaker behavior   [adapter]    4h ago
    Warning: overlaps with swift-maple on [adapter]

READY (awaiting implementation)
  red-fox           plan    ready        connection pool redesign   [adapter]    5d ago
  gold-finch        bug     ready        timeout on large payloads  [transport]  3d ago

IMPLEMENTING
  iron-oak          impl    dispatch     auth token refresh         [auth]       1d ago  (7/23 tasks)
  pale-birch        impl    implementing logging standardization    [core]       6h ago  (19/31 tasks)

BLOCKED
  copper-elm        plan    change-spec  rate limiting              [adapter]    1d ago
    blocked by: swift-maple (research), red-fox (ready)

COMPLETE
  warm-stone        impl    complete     initial adapter scaffold   [adapter]    2w ago
  dark-pine         plan    complete     transport layer spec       [transport]  1w ago
  ...

Clusters:
  [adapter] 4 works: swift-maple, blue-hawk, red-fox, warm-stone
            Warning: 3 in-flight works touch adapter — consider reviewing together

Suggested next:
  gold-finch is ready with no blockers — candidate for implementation dispatch
  swift-maple and blue-hawk both touch [adapter] — review together before proceeding
```

**Agent interaction:** Every new agent session starts with `kerf map`. The agent reads this output and has full portfolio orientation in a single command. No HANDOFF.md needed for the structural view — HANDOFF.md becomes about decisions and open questions only, not about enumerating what exists.

**What's hard:** Where do the area tags come from? Options: (a) user assigns them at `kerf new`, (b) agent infers them from spec content, (c) they're declared in spec.yaml as a new field. Option (a) is simplest but adds friction. Option (c) is cleanest for the data model. The "overlap warning" requires comparing tags, which is trivial once tags exist.

---

## Idea 2: `kerf orient` — Session Start Briefing

**Problems addressed:** 1 (No Persistent Map), 5 (Coherence)

Distinct from `kerf map` in that it's personalized to a specific work. When an agent is about to work on `swift-maple`, it needs more than that work's SESSION.md. It needs to know what else is happening nearby.

**Example flow:**

```
$ kerf orient swift-maple

BRIEFING: swift-maple — adapter retry logic
  Status: research (pass 4/8)
  Jig: plan
  Last session: 2d ago — "Completed problem space analysis, identified 3 retry strategies"

NEIGHBORHOOD:
  blue-hawk (spec, decompose) — circuit breaker behavior [adapter]
    Relationship: None declared. Overlaps on [adapter].
    Risk: circuit breaker and retry logic likely share state — design together.
  red-fox (plan, ready) — connection pool redesign [adapter]
    Relationship: None declared. Overlaps on [adapter].
    Risk: pool redesign may change the layer where retries attach.
  warm-stone (impl, complete) — initial adapter scaffold [adapter]
    This is the existing code you're building on.

DOWNSTREAM:
  copper-elm (plan, change-spec) — rate limiting [adapter]
    Blocked by swift-maple. Your design choices constrain copper-elm.

ACTIVE SESSIONS:
  pale-birch has an active session (logging standardization) — no overlap.

OPEN QUESTIONS (from SESSION.md):
  - Should retries be at the transport level or adapter level?
  - What's the backoff strategy for idempotent vs non-idempotent calls?

Next steps:
  kerf resume swift-maple    Start a session on this work
  kerf show blue-hawk        Review the overlapping circuit breaker work
```

**Agent interaction:** The agent runs `kerf orient <codename>` before `kerf resume`. It gets the work's own state PLUS everything nearby that might affect its decisions. This is the "peripheral vision" that agents currently lack.

**What's hard:** The "neighborhood" computation. How does kerf know that blue-hawk and swift-maple are related? Three options: (a) shared area tags, (b) explicit dependency links, (c) both. The "Risk" annotations are the real value-add but require either manual input or inference. Starting with just shared-tag detection and letting the user annotate risks is probably the right MVP.

---

## Idea 3: Area Tags on Works + `kerf cluster`

**Problems addressed:** 2 (Works Are Islands), 5 (Coherence)

A lightweight mechanism for declaring what part of the system a work touches. Not a full taxonomy — just freeform tags that enable grouping and overlap detection.

**Schema addition to spec.yaml:**

```yaml
areas:                              # list of strings, optional
  - adapter
  - transport
```

**CLI for viewing clusters:**

```
$ kerf cluster

[adapter]  4 works
  swift-maple     plan    research        adapter retry logic
  blue-hawk       spec    decompose       circuit breaker behavior
  red-fox         plan    ready           connection pool redesign
  warm-stone      impl    complete        initial adapter scaffold

[transport]  2 works
  gold-finch      bug     ready           timeout on large payloads
  dark-pine       plan    complete        transport layer spec

[auth]  1 work
  iron-oak        impl    dispatch        auth token refresh

[core]  1 work
  pale-birch      impl    implementing    logging standardization

Overlap alerts:
  swift-maple + blue-hawk: both specifying, both touch [adapter]
    Consider: review specs together before either proceeds to tasks
  red-fox: ready but 2 other [adapter] works are still specifying
    Consider: wait for swift-maple and blue-hawk before implementing red-fox
```

**The `kerf new` integration:**

```
$ kerf new --jig plan --title "circuit breaker behavior"

Codename: blue-hawk
Type: plan
Status: problem-space

What areas of the system does this work touch?
  Known areas in this project: adapter, transport, auth, core
  Areas (comma-separated, or blank to skip): adapter

Warning: 2 other works also touch [adapter]:
  swift-maple (plan, research) — adapter retry logic
  red-fox (plan, ready) — connection pool redesign
Consider reviewing these before designing in isolation.
```

**Agent interaction:** At creation time, the agent (or user) declares areas. kerf immediately surfaces overlap. This is the "hey, this new work overlaps with that existing work" moment — it happens at the earliest possible point.

**What's hard:** Tag discipline. Freeform tags drift — one agent writes "adapter", another writes "http-adapter", a third writes "network-layer". Mitigations: (a) suggest existing tags at `kerf new` time (shown above), (b) a `kerf cluster rename` command to normalize tags, (c) accept that some drift is fine and the value is in approximate grouping, not precise taxonomy.

---

## Idea 4: `kerf queue` — Prioritized Readiness View

**Problems addressed:** 3 (No Intake/Prioritization)

A command that answers: "What should be worked on next?" Shows works ordered by readiness and impact, with the dependency graph resolved.

**Example output:**

```
$ kerf queue

Ready to implement (no blockers):
  1. gold-finch    bug     ready    timeout on large payloads     [transport]
     No dependencies. Standalone fix.
  2. red-fox       plan    ready    connection pool redesign      [adapter]
     Warning: 2 works specifying in [adapter] — implementing now may cause rework.

Ready to spec (no blockers):
  3. swift-maple   plan    research   adapter retry logic         [adapter]
  4. blue-hawk     spec    decompose  circuit breaker behavior    [adapter]
     Suggestion: review 3 and 4 together — both touch [adapter].

Blocked:
  5. copper-elm    plan    change-spec  rate limiting             [adapter]
     Blocked by: swift-maple (research)

Unstarted:
  (none)

Recommended next action:
  gold-finch is ready and unblocked — dispatch to implementation.
  swift-maple and blue-hawk should be reviewed together before proceeding.
```

**The priority signal:** kerf doesn't auto-prioritize (that's the user's job). But it can surface the information needed for prioritization: what's unblocked, what would unblock the most downstream work, what has overlap risks. The numbered list is a suggestion, not a mandate.

**User interaction:** The user (Greg, acting as orchestrator) runs `kerf queue`, scans the output, and tells the agent "implement gold-finch next" or "hold off on red-fox until the adapter cluster is resolved." The command gives the user the information density needed for fast triage.

**What's hard:** The "recommended next action" is the tricky part. Simple heuristics work: unblocked + ready = highest priority, works that block the most others = higher urgency. But truly good recommendations require understanding the user's goals, which kerf doesn't have. Keeping the recommendations as suggestions rather than directives is important.

---

## Idea 5: `kerf entangle` — Linking Late-Arriving Requirements

**Problems addressed:** 4 (Late-Arriving Requirements), 2 (Works Are Islands)

A command for the "I just realized something" moment. When a new requirement arrives that's architecturally entangled with an in-flight work, the user needs a way to declare the entanglement and choose a resolution path.

**Example flow:**

```
$ kerf entangle blue-hawk swift-maple

Entangling: blue-hawk (circuit breaker) with swift-maple (adapter retry)

swift-maple is currently: research (pass 4/8, no tasks generated yet)
blue-hawk is currently: decompose (pass 3/8, no tasks generated yet)

Both works touch: [adapter]

Options:
  1. MERGE — Fold blue-hawk into swift-maple. Blue-hawk's problem space
     becomes additional context in swift-maple. One combined spec.
     Best when: the requirements are tightly coupled and should be one design.

  2. COORDINATE — Keep separate works but add a mutual `inform` dependency.
     Both specs reference each other. Review together before tasks.
     Best when: requirements are related but separable.

  3. SEQUENCE — Make blue-hawk depend on swift-maple (`must-complete-first`).
     Swift-maple's design decisions become constraints for blue-hawk.
     Best when: one naturally precedes the other.

  4. PAUSE & REPLAN — Shelve both. Create a new umbrella work that designs
     the adapter changes holistically, then re-derive individual works.
     Best when: the overlap is deep enough that independent design won't work.

Choose (1-4):
```

**When one work is already implementing:**

```
$ kerf entangle blue-hawk iron-oak

Entangling: blue-hawk (circuit breaker) with iron-oak (auth token refresh)

iron-oak is currently: implementing (7/23 tasks complete)
blue-hawk is currently: decompose

These works do NOT share areas. Are you sure they're entangled? [y/N]: y

Options (limited — iron-oak is in implementation):
  2. COORDINATE — Add inform dependency. Iron-oak's agent reads blue-hawk's
     spec for context on remaining tasks.

  3. SEQUENCE — blue-hawk waits for iron-oak to complete.

  Note: MERGE and PAUSE are not available — iron-oak has completed tasks.

Choose (2-3):
```

**Agent interaction:** The user notices the entanglement (agents rarely will — they don't have the cross-work peripheral vision). The user runs `kerf entangle`, kerf presents options based on the current state of both works. The user chooses, kerf records the relationship and surfaces it in `kerf orient` and `kerf map`.

**What's hard:** The options are genuinely different workflows, not just metadata. "MERGE" means combining spec.yaml files and artifacts. "PAUSE & REPLAN" means shelving works and creating a new one. These are multi-step operations that kerf would need to orchestrate. An MVP might just record the entanglement as a dependency with a note, and leave the actual workflow to the user/agent.

---

## Idea 6: Dashboard View with `kerf board`

**Problems addressed:** 1 (No Persistent Map), 3 (Prioritization)

A kanban-style board view optimized for the user (Greg) as visual thinker. Not for agents — for the human orchestrator who needs to see the whole picture at a glance.

**Example output (terminal, column layout):**

```
$ kerf board

                         kerf-cli — 12 works

 SPECIFYING (3)      READY (2)          IMPLEMENTING (2)    COMPLETE (3)
 ----------------    ----------------   ----------------    ----------------
 swift-maple         red-fox            iron-oak            warm-stone
   plan/research       plan/ready         impl/dispatch       impl/complete
   [adapter] 2d        [adapter] 5d       [auth] 1d           [adapter] 2w
                                          7/23 tasks
 blue-hawk           gold-finch         pale-birch          dark-pine
   spec/decompose      bug/ready          impl/implementing   plan/complete
   [adapter] 4h        [transport] 3d     [core] 6h           [transport] 1w
                                          19/31 tasks
 copper-elm                                                  ...
   plan/change-spec
   [adapter] 1d
   BLOCKED

 Clusters: [adapter]=4  [transport]=2  [auth]=1  [core]=1
 Alerts: 3 works specifying in [adapter] — review together
```

**What it does that `kerf map` doesn't:** Visual grouping by phase, designed for human scanning. `kerf map` is a structured data dump for agents; `kerf board` is a spatial layout for humans. Same data, different rendering.

**What's hard:** Terminal column layout is finicky — works with long titles overflow, terminal widths vary, and the output is harder for agents to parse than a flat list. This might be the one command where a `--format` flag (board vs. list) makes sense. Or it might be that `kerf map` serves both audiences well enough and `kerf board` is unnecessary complexity.

---

## Idea 7: Automatic Overlap Detection at `kerf new` and `kerf status`

**Problems addressed:** 2 (Works Are Islands), 4 (Late-Arriving Requirements), 5 (Coherence)

Rather than a separate command, build overlap detection into the existing workflow. Every time a work is created or advances status, kerf checks for area overlap and emits warnings.

**At creation:**

```
$ kerf new --jig spec --title "adapter circuit breaker" --areas adapter

Created: blue-hawk (spec, problem-space)

Overlap detected:
  swift-maple (plan, research) also touches [adapter]
  red-fox (plan, ready) also touches [adapter]
  Recommendation: Run `kerf orient blue-hawk` to see the full neighborhood.
```

**At status change to "ready":**

```
$ kerf status swift-maple ready

swift-maple: research -> ready

Pre-implementation check:
  2 other works in [adapter] are still specifying:
    blue-hawk (spec, decompose)
    copper-elm (plan, change-spec)
  Implementing swift-maple independently may cause rework if these
  works make incompatible design choices.
  
  Options:
    Proceed anyway: kerf status swift-maple ready --confirm
    Review together first: kerf cluster adapter
```

**At task generation (entering implementation):**

```
$ kerf status iron-oak implementing

iron-oak: dispatch -> implementing

Context for implementation agent:
  Related works in [auth]:
    (none — iron-oak is the only [auth] work)
  Related works by dependency:
    swift-maple (plan, ready) — iron-oak depends on swift-maple [inform]
    Read swift-maple's spec for adapter context.
```

**Agent interaction:** The agent doesn't need to remember to check for overlaps — kerf tells it. This is the "pit of success" approach: doing the right thing (checking for conflicts) is the default path, not an extra step.

**What's hard:** Warning fatigue. If every status change triggers overlap checks and most are benign, agents and users will learn to ignore them. Mitigation: only warn when overlapping works are in specific "dangerous" phase combinations (both specifying, or one specifying while another is implementing in the same area). Quiet overlap (one complete, one specifying) is noted but not warned.

---

## Idea 8: `kerf context` — Minimum Viable Context Packet

**Problems addressed:** 1 (No Persistent Map), 5 (Coherence)

The session-start problem distilled to its essence: what is the absolute minimum an agent needs to read to orient? Not SESSION.md (too narrow). Not every spec (too broad). A computed context packet.

**Example:**

```
$ kerf context swift-maple

# Context for swift-maple — adapter retry logic

## This Work
Status: research (pass 4/8 in plan jig)
Created: 2d ago. Last session: "Completed problem space analysis."
Open questions:
  - Should retries be at the transport level or adapter level?
  - Backoff strategy for idempotent vs non-idempotent calls?

## Key Decisions Already Made (from artifacts)
  - Adapter uses the repository pattern (from warm-stone, complete)
  - Transport layer is finalized (from dark-pine, complete)

## Active Constraints
  - blue-hawk (circuit breaker) is also specifying in [adapter] — your retry
    design should be compatible with circuit breaker integration.
  - red-fox (connection pool redesign) is ready — your retry attachment point
    may change when pool redesign is implemented.
  - copper-elm (rate limiting) is blocked on you — your design choices about
    where retries live will constrain where rate limiting can go.

## Files to Read
  ~/.kerf/projects/kerf-cli/swift-maple/SESSION.md
  ~/.kerf/projects/kerf-cli/swift-maple/03-components.md
  ~/.kerf/projects/kerf-cli/blue-hawk/01-problem-space.md  (related work)
```

**What makes this different from `kerf orient`:** `kerf orient` shows the neighborhood objectively. `kerf context` is opinionated — it tells the agent what matters and what to read. It's a briefing, not a map.

**Agent interaction:** The agent runs `kerf context <codename>`, reads the output, reads the 3-4 files listed, and is oriented. Total context consumed: maybe 2-3K tokens for the context packet plus whatever the files contain. Compare to reading a 5-page HANDOFF.md that's 80% irrelevant to the current work.

**What's hard:** Computing "Key Decisions Already Made" requires reading artifact content from completed works and summarizing. That's either an expensive operation (reading and parsing multiple files) or it requires pre-computed summaries. A pragmatic approach: at `kerf shelve` or `kerf finalize`, require a one-line "design decision summary" that gets stored in spec.yaml. Then `kerf context` just reads metadata, not full artifacts.

---

## Idea 9: Work Annotations — Structured Metadata for Cross-Cutting Concerns

**Problems addressed:** 2 (Works Are Islands), 4 (Late-Arriving Requirements), 5 (Coherence)

Sometimes the problem isn't that works don't know about each other — it's that there's no place to record cross-cutting information that doesn't belong to any single work.

**New concept:** Annotations are notes attached to area clusters, not individual works. They live at the project level and surface whenever any work in that area is shown.

**Schema:**

```yaml
# ~/.kerf/projects/kerf-cli/annotations.yaml
annotations:
  - area: adapter
    created: 2026-05-07T10:00:00Z
    note: "All adapter changes must use the repository pattern established in warm-stone. Retry, circuit breaker, and pool changes should share a common middleware chain."
    author: greg
  - area: adapter
    created: 2026-05-08T14:00:00Z
    note: "New requirement: circuit breaker and retry must share state. Design these together."
    author: greg
```

**How they surface:**

```
$ kerf show swift-maple

swift-maple — adapter retry logic
  ...

Area annotations for [adapter]:
  [2026-05-07] All adapter changes must use the repository pattern
    established in warm-stone. Retry, circuit breaker, and pool changes
    should share a common middleware chain.
  [2026-05-08] New requirement: circuit breaker and retry must share
    state. Design these together.
```

**CLI for adding them:**

```
$ kerf annotate adapter "circuit breaker and retry must share state — design together"

Added annotation to [adapter].
This annotation will appear when viewing any work that touches [adapter]:
  swift-maple, blue-hawk, red-fox, warm-stone
```

**Agent interaction:** The orchestrator (Greg) realizes something cross-cutting. Instead of updating 3 different works' SESSION.md files, he adds one annotation to the area. Every agent that subsequently works on any adapter work sees the annotation. This is the "broadcast to a topic" pattern.

**What's hard:** Annotation hygiene. Annotations accumulate. Old ones become irrelevant. Need a mechanism to archive or resolve them. A simple approach: annotations can be marked `resolved: true` and stop surfacing. Or they auto-archive when all works in the area are complete.

---

## Idea 10: `kerf diff` — What Changed Since Last Session

**Problems addressed:** 1 (No Persistent Map), 4 (Late-Arriving Requirements)

Between sessions, other agents may have advanced other works, new works may have been created, and the landscape may have shifted. The agent needs to know what changed.

**Example:**

```
$ kerf diff --since "2026-05-07T10:00:00Z"

Changes since 2026-05-07 10:00:

NEW WORKS:
  blue-hawk (spec, decompose) — circuit breaker behavior [adapter]
    Created 2026-05-08 by session abc123

STATUS CHANGES:
  swift-maple: problem-space -> research
  iron-oak: breakdown -> dispatch (7 tasks generated)
  pale-birch: dispatch -> implementing (started 19 tasks)

COMPLETED:
  dark-pine (plan) — transport layer spec [transport]

NEW DEPENDENCIES:
  copper-elm now depends on swift-maple (must-complete-first)

NEW ANNOTATIONS:
  [adapter] "circuit breaker and retry must share state"

AREA IMPACT:
  [adapter] gained 1 new work (blue-hawk) and 1 new annotation
  [transport] dark-pine completed — area may be stable now
```

**Tied to sessions:** `kerf diff` could default to "since the last session ended on this work" when given a codename:

```
$ kerf diff swift-maple
Changes since last session on swift-maple (2026-05-06 16:30):
  ...
```

**Agent interaction:** The agent runs `kerf diff <codename>` right after `kerf resume`. It gets a changelog of everything relevant that happened while it was away. This is the "what did I miss?" command.

**What's hard:** Requires tracking timestamps on all state changes, which kerf mostly already does (the `updated` field in spec.yaml, session timestamps). The main gap is tracking when dependencies and annotations change. The snapshot system (`.history/`) already captures spec.yaml changes, so computing diffs from snapshots is feasible.

---

## Idea 11: Inline Dependency Status in `kerf resume`

**Problems addressed:** 1 (No Persistent Map), 3 (Prioritization)

The simplest possible improvement: when resuming a work, show the live status of all its dependencies and all works that depend on it. No new commands, just richer output from an existing command.

**Current `kerf resume` output (from the spec):** Shows the work's own state, SESSION.md, current pass, jig instructions.

**Enhanced `kerf resume` output:**

```
$ kerf resume copper-elm

RESUMING: copper-elm — rate limiting
  Status: change-spec (pass 5/8)
  Jig: plan
  Last session: 1d ago

DEPENDENCIES (what copper-elm needs):
  swift-maple (plan, research) — adapter retry logic
    Status: research — NOT READY
    Last activity: 2d ago
    Impact: copper-elm's rate limiting design depends on where retries live.
    
DEPENDENTS (what needs copper-elm):
  (none)

AREA PEERS (same [adapter] area, not direct dependencies):
  blue-hawk (spec, decompose) — circuit breaker behavior
  red-fox (plan, ready) — connection pool redesign

SESSION.md:
  ...

Next steps:
  Warning: dependency swift-maple is not ready. Proceeding may require
  rework if swift-maple's design changes the retry attachment point.
  kerf show swift-maple    Review dependency state
  Continue writing the change-spec pass for copper-elm
```

**What's hard:** Almost nothing. This is a data assembly problem — read the dependency graph, resolve each dep's current status, format it. The hardest part is the "Impact" line, which requires either manual annotation or inference. An MVP can omit the impact line and just show status.

---

## Idea 12: Session-to-Session Continuity via `kerf trajectory`

**Problems addressed:** 1 (No Persistent Map), 5 (Coherence)

A command that shows the project's trajectory over time — not just current state but velocity and direction. Useful for the orchestrator to see whether things are converging or diverging.

**Example:**

```
$ kerf trajectory

kerf-cli — last 7 days

          Specifying  Ready  Implementing  Complete
  May 01:    5         0         0           0
  May 03:    3         2         0           0
  May 05:    3         2         1           1
  May 07:    3         2         2           2
  Today:     3         2         2           3

Velocity: ~0.4 works completed/day
Funnel: 3 specifying -> 2 ready -> 2 implementing (pipeline is balanced)

Attention needed:
  [adapter] cluster has been specifying for 5 days with no work reaching ready.
  swift-maple has been in "research" for 2 days — may be stuck.
```

**What's hard:** Requires historical data. The snapshot system already captures spec.yaml over time, so computing historical status is possible but requires scanning snapshots across all works. For a project with 15 works and 100 snapshots each, that's 1,500 directories to scan. Might need a lightweight index file that caches status history. Alternatively, `kerf trajectory` could be computed from spec.yaml timestamps (created, updated) and current status, which is approximate but cheap.

---

## Cross-Cutting Observations

### The Information Hierarchy

These ideas form a natural hierarchy of information density:

| Level | Command | Audience | Detail |
|-------|---------|----------|--------|
| Portfolio | `kerf board` | Human orchestrator | Visual, spatial, at-a-glance |
| Portfolio | `kerf map` | Agent | Structured, complete, parseable |
| Portfolio | `kerf queue` | Both | Actionable — what's next? |
| Portfolio | `kerf trajectory` | Human orchestrator | Temporal — are we converging? |
| Cluster | `kerf cluster` | Both | Area-focused grouping |
| Work | `kerf orient` | Agent | Neighborhood-aware briefing |
| Work | `kerf context` | Agent | Minimum viable context packet |
| Delta | `kerf diff` | Agent | What changed since last session? |
| Action | `kerf entangle` | Human orchestrator | Handle late-arriving overlaps |
| Passive | Inline warnings | Both | Overlap detection in existing commands |

### The MVP Stack

If I had to pick three ideas that together address the most problems with the least complexity:

1. **Area tags on spec.yaml + overlap warnings at `kerf new`** (Idea 3/7) — minimal schema change, immediate value at creation time. Addresses Problems 2, 4, 5.

2. **`kerf map`** (Idea 1) — one command, portfolio-level orientation. Addresses Problems 1, 3, 5.

3. **Enhanced `kerf resume` with dependency status** (Idea 11) — zero new commands, just richer output. Addresses Problems 1, 3.

These three together give: area awareness at creation, portfolio view at session start, and dependency context at resume. The other ideas (entangle, context, diff, trajectory, board, annotations) are all valuable but build on these foundations.

### The Key UX Principle

**Show the right information at the right time without being asked.**

Agents don't know what they don't know. They won't run `kerf cluster` unprompted because they don't know clusters exist or matter. The highest-leverage interventions are the ones that inject information into the existing workflow — warnings at `kerf new`, dependency status at `kerf resume`, overlap alerts at `kerf status`. The explicit query commands (`kerf map`, `kerf queue`, `kerf orient`) are for deliberate exploration; the inline warnings are for the cases where the agent or user would have missed a connection entirely.
