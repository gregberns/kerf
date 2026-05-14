# Coherence Critique — Six Deep-Dive Analyses

> Critical review of documents 11-16 for contradictions, gaps, complexity,
> and integration viability.

---

## 1. Cross-Document Contradictions

### 1.1 Session state: SESSION.md vs. session log YAML files

The protocols document (15) and the session continuity document (16) propose
incompatible session record mechanisms.

Document 15 says session records are written to **SESSION.md inside the work
directory** using an append-only markdown format:

> "Append a structured entry to SESSION.md using the format below. Do NOT
> rewrite previous entries." (15, Step 2 of Session-End Protocol)
>
> "If SESSION.md has more than 10 session entries, archive older entries.
> Move entries older than the most recent 5 to a file called
> SESSION-ARCHIVE.md." (15, Step 3)

Document 16 says session records are written to **individual YAML files in a
project-level `.kerf/sessions/` directory**:

> "Record is written to `.kerf/sessions/{timestamp}.yaml`" (16, Section 6.2)
>
> "Project-level `.kerf/sessions/` with `works_touched` field for
> filtering." (16, Section 9, Open Question 1)

These are fundamentally different designs:
- 15 uses per-work markdown files that get trimmed/archived.
- 16 uses per-session YAML files at the project level, never edited, filtered
  at read time.

Document 16's design is superior (immutable records, no archiving gymnastics,
cross-work filtering by `works_touched`). But document 15's protocols
reference SESSION.md by name in every protocol — the co-design protocol writes
reconciliation records to SESSION.md, the disruption protocol writes to
SESSION.md, the late-requirement protocol writes to SESSION.md. If the session
log moves to per-session YAML files, all of these protocols need to be
rewritten to target the new artifact. The protocols as written are incompatible
with the session continuity design.

### 1.2 `kerf orient` vs. `kerf map` + `kerf resume`

Document 16 proposes a new `kerf orient` command that combines portfolio state
with session history:

> "Run `kerf orient` — get computed state (portfolio, dependencies, areas,
> actionable)" (16, Section 6.1)

Document 15's session-start protocol uses `kerf map` and `kerf resume` as
separate steps:

> "Step 2: LOAD PORTFOLIO — Run `kerf map`."
> "Step 6: LOAD SPECIFIC WORK — Run `kerf resume {codename}`."

Document 13 proposes `kerf next` as the work-selection interface. The factory
line (11) references `kerf next` and `kerf map` as the query interfaces to
Station 4.

So we have three overlapping commands: `kerf orient` (from 16), `kerf map` +
`kerf resume` (from 15), and `kerf next` (from 11, 12, 13). These partially
overlap in function. `kerf orient` seems to subsume parts of `kerf map` and
`kerf resume`. Nobody reconciles these into a coherent command surface.

### 1.3 Work status: who owns it?

Document 13 (beads integration) says work status transitions through a
lifecycle where ownership transfers:

> "draft -> specced -> tasked -> in_progress -> done -> archived"
> "kerf owns / kerf owns / kerf+beads handoff / beads owns / kerf owns" (13,
> Section 3)

Document 11 (factory line) says a work has exactly one station and the station
determines status:

> "A work has exactly one position on the line at any given time." (11,
> Invariant 1)

Document 15 (protocols) says the agent updates status directly:

> "Step 1: UPDATE WORK STATUS — If the work's status should change based on
> what was accomplished, update spec.yaml status field." (15, Session-End
> Protocol)

These three models are not fully incompatible, but they don't agree on the
mechanics. If beads "owns" the status during `in_progress`, then the agent
shouldn't be manually updating spec.yaml status. If the factory line station
determines status, there should be a mapping from station to status values. If
the agent updates status in the session-end protocol, that conflicts with
status being "computed from beads" as document 13 proposes. The question of
who is the authoritative source for work status during implementation is
answered differently by three documents.

### 1.4 Priority: factory line vs. priority document on momentum

Document 11 (factory line) lists momentum as one of three ordering forces at
the Queue station:

> "recency of related completions (momentum — keep working in the same area
> while context is warm)" (11, Station 4)

But document 12 (dynamic priority) places momentum below urgency and
structural ranking:

> "Layer 2: Override — pinned/urgent works go to top (contextual urgency)"
> "Layer 3: Rank — among remaining, sort by: a. Critical path membership
> (structural), b. Fan-out / unblocking power (structural), c. Inherited
> value weight from areas/goals (strategic), d. Age (tiebreaker)" (12,
> Section "How the Three Dimensions Compose")

Momentum doesn't appear in the composition at all in document 12. Document 11
says momentum is one of three forces; document 12's layered model has no
place for it. Document 12 discusses momentum only as "prefer beads from the
same work" under the pull-vs-push section, framing it as agent affinity, not
a ranking factor. These are different treatments of the same concept.

---

## 2. Gaps in the Combined Picture

### 2.1 Cross-work bead dependencies have no home

Document 11 asks: "What if Task B3 specifically depends on Task A7?" (Open
Question 3). Document 13 proposes cross-work edges in the YAML schema (the
`edges` section with cross-work references and `cross_works` block). But
document 12's priority model operates at the **work level**, not the bead
level. `kerf next` returns the next work to focus on. `bv --robot-triage`
returns the next bead within a work.

What happens when bead B3 (in work B) depends on bead A7 (in work A), and
work A is ranked lower than work B? The work-level ranking says "do work B
next." But work B can't make progress because bead B3 is blocked on something
in work A. Document 12's composition model filters by "actionable" but only
at the work level — it doesn't model partially-blocked works where some beads
are ready and some are blocked on other works.

Document 13 acknowledges this in the `task_status` output of `kerf next`
(showing `blocked: 2`, `ready: 5`), but the priority model in document 12
doesn't account for it in ranking. A work that is 80% blocked by cross-work
dependencies should rank differently than a work that is 0% blocked, but the
model has no mechanism for this.

### 2.2 How does an agent actually run `kerf next`?

All six documents reference `kerf next` as a query that returns the next
thing to work on. But the command requires synthesizing information from two
separate systems (kerf's work graph and beads' task graph). Document 13 says:

> "kerf computes work-level ordering... Within a work, `bd` computes
> bead-level ordering... Two queries, composed." (11, Section "The Unified
> Queue Problem")

But who does the composition? The orchestrator agent? The `kerf next` command
itself? If `kerf next` must query `br list` for every active work to compute
aggregate status, it needs to know about the beads CLI, parse its output, and
handle the case where beads isn't installed or configured. Document 13 says
kerf talks to beads through "exactly three channels" but the actual plumbing
of `kerf next` calling `br list` is never specified. Is it shelling out? A Go
library? A REST API?

### 2.3 Area graph vs. beads labels: redundant area tracking

Document 14 proposes `areas.yaml` as the canonical area graph. Document 13
proposes beads carry `area:<name>` labels. These overlap:

- An area exists in `areas.yaml` (document 14).
- A work is tagged with that area in `spec.yaml` (document 14).
- The work's task YAML sets `default_labels` including `area:adapter`
  (document 13).
- Each bead inherits that label.

Now there are three places where the association between a piece of work and
an area is recorded: `areas.yaml` (defines the area), `spec.yaml` (tags the
work), and beads labels (tags the tasks). If someone adds a bead manually
with `br create` and tags it `area:networking` (not a defined area), there's
no validation. The coherence enforcement in document 14 operates at the kerf
level, not the beads level. The bead label space is uncontrolled.

### 2.4 No concrete treatment of multi-agent parallelism

Document 12 mentions it briefly: "Two agents should not get the same work."
Document 11 shows multiple works flowing through the line simultaneously. But
the protocols in document 15 are written entirely from the perspective of a
single agent in a single session. The co-design protocol checks peer
SESSION.md files, but what if the peer is currently being worked by another
agent in a parallel session?

The signaling mechanism in document 15's disruption protocol says:

> "If using agent-mail or similar coordination: send a message to the agent
> working on the affected work." (15, Step 6)

This is a hand-wave. The entire coordination system — detecting concurrent
work, preventing conflicting modifications, serializing access to shared
resources like `areas.yaml` — is unaddressed. File reservation is mentioned
in document 11 but never specified.

### 2.5 How does the area graph get bootstrapped for existing projects?

Document 14 says: "The agent reads the codebase and specs, proposes an initial
area graph, the user reviews and approves" (Open Question 5). But there's no
protocol for this. Given that protocols are supposed to drive agent behavior,
the absence of a bootstrap protocol is notable. What does the agent produce?
Just node names? Edges too? How does the user review a graph (ASCII art? YAML
diff?)?

### 2.6 No treatment of "done" criteria at the work level

Document 11 defines Station 6 (Verify) as "all beads for work are closed."
But document 13 says `kerf next` can return `suggested_action: "needs_review"`
when "all tasks done, work needs verification." Who triggers verification?
How does the system know that "all beads closed" means the work should
transition to Verify? Document 15's session-end protocol updates status
manually. Document 13 says beads "owns" status during `in_progress`. None of
them define the trigger for the in_progress -> done transition with enough
specificity to implement.

---

## 3. Complexity Audit

### 3.1 Total surface area

Across all six documents, the proposed system includes:

**New commands:**
- `kerf map` — portfolio view
- `kerf next` — computed work selection
- `kerf orient` — session orientation (or is this `kerf map` + `kerf resume`?)
- `kerf decompose` — task YAML generation + loading
- `kerf validate-tasks` — YAML validation
- `kerf areas init` — bootstrap area graph
- `kerf areas add` — add area node
- `kerf areas link` — add area edge
- `kerf areas rename` — rename area node
- `kerf areas impact` — blast radius query
- `kerf link` — create work-to-work relationships
- `kerf handoff` / `kerf close` — session end
- `kerf audit` (mentioned in plan but deferred)

**New data structures:**
- `areas.yaml` — area graph (nodes + typed directed edges)
- Task YAML schema (beads + edges + cross-work references)
- Mnemonic maps (CSV, per-work)
- Session log records (YAML, per-session)
- Session resolution records
- Urgency pins (on works)
- Value weights (on areas or goals)
- `co-designs` relationship type in `depends_on`

**New protocols:**
- Session-start (9 steps)
- Session-end (5 steps)
- Co-design (5 steps with sub-branches)
- Late-requirement (6 steps with a 3x3 decision matrix)
- Disruption (6 steps with 4 disruption types)

**New computation:**
- Cross-work dependency resolution via mnem-maps
- Priority ranking (critical path, fan-out, value weights, urgency pins)
- Area heat mapping
- Blast radius computation via graph traversal
- Work status aggregation from bead counts
- Session log relevance filtering + tiered detail

### 3.2 Where the budget is spent

The complexity is concentrated in three areas:

1. **The priority/queue computation** (documents 11, 12). Critical path
   analysis, fan-out scoring, value weights, urgency pins, momentum. This is
   sophisticated scheduling theory applied to what is currently a manual
   decision. The question: does the current workflow actually have enough
   concurrent works to justify this? Greg's harmonik example had 8 specs. For
   8 items, a human can pick the next one in seconds. The machinery pays off
   at 30+ concurrent works — a scale the system hasn't reached.

2. **The beads integration pipeline** (document 13). YAML schema, loader,
   mnem-maps, cross-work edge resolution, forward-deferred edges, status
   queries. This is proven from harmonik, but it's a significant integration
   layer. Every piece needs to work correctly for `kerf next` to return
   accurate status.

3. **The area graph** (document 14). Typed directed edges, hierarchical
   ownership, blast radius computation, adjacency warnings. This is an
   architecture modeling system embedded inside a work coordination tool. The
   "medium-plus" recommendation is sound, but even "medium-plus" requires
   defining, maintaining, and querying a graph data structure.

### 3.3 Is it tractable?

Individually, each document proposes something reasonable. Together, the
implementation surface is very large for a CLI tool that currently has ~5
commands. The dependency chain is long:

- `kerf next` depends on the priority model, which depends on the area graph
  (for value weights), which depends on `areas.yaml`, which depends on
  `kerf areas init`.
- `kerf next` also depends on beads integration (for task status), which
  depends on the YAML schema, the loader, and mnem-map infrastructure.
- The protocols depend on `kerf orient` (or `kerf map`), which depends on
  the status aggregation from beads, which depends on the `work:<id>` label
  convention.

The minimum viable path through all this is:
1. `areas.yaml` with flat list (no edges yet)
2. `kerf map` reading spec.yaml status fields (no beads yet)
3. `kerf next` doing topological sort on `depends_on` (no fan-out, no value
   weights, no beads integration)
4. Session-end protocol writing structured markdown (not YAML)
5. Session-start protocol reading `kerf map` output

This is achievable. The risk is that the documents set expectations for the
full system and nobody builds the bridge from "minimum viable" to "full
vision."

---

## 4. Greg's Concerns vs. Proposed Solutions

### 4.1 Beads loading and integration

**Greg's concern:** "With very large specs, there's a big challenge getting
tasks loaded into beads. I had the system generate a yaml file and then that
could be loaded into beads." He also said: "I don't want to force someone into
using beads."

**Coverage:** Document 13 addresses this well. The YAML intermediate
representation, the loader, and the `tools.task_tracker` project setting are
direct responses. The optional integration gate is exactly what Greg asked for.
This concern is adequately addressed.

### 4.2 Areas as system map, not just tags

**Greg's concern:** "The areas are kind of the system map — in a way a
simplified architectural diagram. I wonder if we could have a more
sophisticated graph that could represent what parts talk to each other."

**Coverage:** Document 14 directly addresses this with the typed directed
graph. The "medium-plus" recommendation is a reasonable scope. Well covered.

### 4.3 Tying work items to beads

**Greg's concern:** "The question is how do we tie the idea of work items to
groups of beads to execute." And: "Let's say the agent finishes 5 things. What
does `kerf next` display? How does it know what the status of tasks are?"

**Coverage:** Document 13's `work:<id>` label convention and status query
model address this directly. The `kerf next` output format in document 13
shows exactly what Greg asked for (task_status with total/closed/open/blocked/
ready counts). Well covered in design; untested in practice.

### 4.4 Dynamic priority and the failure of P0/P1/P2

**Greg's concern:** Extensive feedback about how static priority labels break
down. Described three types of ordering (technical dependency, contextual
urgency, system value). Said priorities are a "chain" that changes as work
completes.

**Coverage:** Document 12 is a thorough treatment. The three-dimensional
model, the urgency pin mechanism, the value weight inheritance from areas —
all directly respond to Greg's observations. Possibly the most thoroughly
addressed concern.

### 4.5 HANDOFF document degradation

**Greg's concern:** "That document drifts over time... the bloat actually
caused the agent to have issues knowing what was critical. It also drifted —
it was playing the 'telephone game.'"

**Coverage:** Document 16 addresses this directly and well. The computed
orientation + immutable session log design is a clean solution to all five
failure modes Greg described. Well covered.

### 4.6 Manufacturing line / kanban process

**Greg's concern:** "With the system we're thinking about, it's a
'manufacturing line' of work to be designed, tasked, and allocated. We need to
think about how that system works."

**Coverage:** Document 11 is a direct response. Thorough and detailed.
However, Greg's follow-up questions are partially unresolved:

- "If a dependency is identified, is that worked on first?" — Addressed by
  document 12's dependency-as-filter model.
- "If user completes testing and finds an issue, how is that tasked and
  prioritized ahead of other items?" — Addressed by document 12's urgency
  pin mechanism.
- "Especially between sessions" — Partially addressed by document 16's
  session log, but there's no specific protocol for "user found a bug during
  manual testing and wants it prioritized for the next agent session." The
  urgency pin is the mechanism, but the workflow of how the user creates the
  pin and ensures the agent picks it up is not spelled out.

### 4.7 Co-design synchronization

**Greg's concern:** "Once a relationship is generated, we'd want to figure
out how to signal to an agent that the relationship has been investigated/
addressed... I want REALLY good guidance for the agents on how to address."

**Coverage:** Document 15's co-design protocol is a direct, detailed response.
The stage-based precedence model (AHEAD/SAME/BEHIND) and the escalation
rules are concrete. This is one of the strongest protocol designs in the set.
**However**, the signaling mechanism (writing to SESSION.md) contradicts
document 16's session log design, as noted in Section 1.1 above.

### 4.8 `kerf resume` usage patterns

**Greg's concern:** "'resume' I don't think is used a ton... a lot of the
time once I've gone through and planned it out, I just have the agent go and
build out all the tasks."

**Coverage:** This feedback is acknowledged but not fully internalized. The
session-start protocol (15) still uses `kerf resume` as a key step (Step 6).
Document 16 proposes `kerf orient` which partially replaces resume. But the
deeper point — that the workflow is often "plan -> decompose -> batch
execute" rather than "resume -> do a little -> hand off" — isn't reflected in
the factory line model, which assumes works progress through stations over
multiple sessions. The batch execution pattern (one session plans everything,
then harmonik runs 40 beads) doesn't fit neatly into the per-station model.

### 4.9 Feeding into harmonik

**Greg's concern:** "I'd love to have this information (`next` output) feed
into harmonik's system (via an agent probably)."

**Coverage:** Document 13 describes this flow (kerf next -> orchestrator
agent -> harmonik queue -> worker agents). The `kerf next` JSON output format
is designed for machine consumption. This is well-addressed in concept. The
missing piece: how does the orchestrator agent actually invoke harmonik?
That's harmonik's concern, but the boundary is fuzzy — nobody specifies the
handoff format between `kerf next` output and harmonik's queue input.

---

## 5. The Integration Question

### 5.1 Factory line (11) + beads integration (13)

These mesh reasonably well. The factory line's Station 3 (Decompose) maps to
document 13's YAML production stage. Station 4 (Queue) maps to the
`kerf next` computation. Station 5 (Execute) maps to beads execution.

**Tension:** The factory line says "the Queue is the single point of
cross-work coordination" (Invariant 5). But beads integration says the queue
is computed on demand, not materialized. The factory line's language implies a
persistent queue ("tasks from multiple works are merged into a single ordered
backlog"). Document 13's approach is query-time computation. These are
compatible in outcome but different in mental model. Someone implementing
Station 4 from document 11 would build a persistent queue. Someone
implementing from document 13 would build a stateless query.

### 5.2 Priority model (12) + area graph heat mapping (14)

These compose well in theory. Document 12 says value weights can be applied to
areas. Document 14 defines what areas are and how they're structured. Heat
mapping (14) is "how many works touch this area." Value weights (12) are "how
important is this area strategically."

**Gap:** Heat mapping measures activity; value weights measure importance.
High heat on a low-value area is a coordination concern but not a priority
signal. Low heat on a high-value area means nobody's working on the important
stuff. Neither document discusses how heat and value interact in the priority
computation. They're two independent signals with no specified composition.

### 5.3 Protocols (15) + kerf commands and data structures

The protocols reference:
- `kerf map` — exists in concept across multiple documents
- `kerf resume {codename}` — exists today
- `kerf square {codename}` — exists today
- `kerf link` — proposed but not specified in any document
- `kerf new` — exists today
- `spec.yaml` — exists today
- `SESSION.md` — contradicted by document 16's session log design

The command references are mostly sound, but `kerf link` is used in protocols
without being defined anywhere. The late-requirement protocol says:

> "Add the relationship: `kerf link {new} {existing} --rel {type}`." (15,
> Step 5)

This command doesn't exist and its behavior isn't specified. What relationship
types does it accept? Where does it write? How does it interact with the
existing `depends_on` field in spec.yaml?

### 5.4 Session continuity (16) + protocol session start/end (15)

These are the most directly conflicting pair, as covered in Section 1.1.
They agree on the problem (HANDOFF degradation) and agree on the principles
(computed state + structured records). They disagree on the mechanism
(SESSION.md vs. per-session YAML, `kerf orient` vs. `kerf map` + `kerf
resume`, trim/archive vs. relevance filter).

Reconciliation is possible but requires choosing one mechanism and rewriting
the other document's references. The session continuity design (16) is more
developed and more robust. The protocols (15) should be updated to target
document 16's artifacts.

---

## 6. Reality Check

### 6.1 Over-engineered for current state

**Critical path / WSJF / CPM analysis (document 12).** This is scheduling
theory for portfolios with dozens of concurrent works. kerf currently manages
single-digit works per project. A topological sort with human override would
cover 95% of real usage. The theory is interesting but the engineering effort
to implement critical path analysis is not justified by current scale.

**Typed directed edges in the area graph (document 14).** The "medium-plus"
recommendation already acknowledges this: "Edge-based queries (adjacency,
blast radius) are computed from the same data but surfaced later." But even
storing the edges adds schema complexity, validation requirements, and
maintenance burden. A flat area list with `parent` hierarchy is sufficient for
the near term and could be extended later.

**Forward-deferred edge resolution (document 13).** This solves a real
problem from harmonik at ~400 beads. Current kerf projects are unlikely to
have cross-work bead dependencies at that scale for some time. The mechanism
is sound but the implementation priority should reflect actual need.

**The disruption protocol's 4-type classification (document 15).** LOCAL,
INTERFACE, DESIGN, SCOPE — with different response paths for each. In
practice, agents will struggle to classify disruptions correctly. A simpler
rule — "if you need to change a spec, stop and escalate" — covers most
cases and is easier for agents to follow.

### 6.2 Solving problems not yet encountered

**Session log scaling to 50+ sessions (document 16).** The relevance
filtering, tiered detail, and resolution marking mechanisms are designed for
a problem that doesn't exist yet. Most kerf projects have had <10 sessions.
Building the full session log infrastructure before validating the basic
"computed orientation + simple session record" pattern is premature.

**Area graph blast radius computation (document 14).** This assumes projects
where changes to one area ripple through connected areas via graph edges. The
current usage pattern doesn't produce this kind of architectural analysis
need. Simple area overlap ("these works share an area tag") solves the
immediate problem.

**Pin accumulation and TTL decay (document 12).** The failure mode of too
many urgency pins assumes sustained, multi-month usage with many works. The
pin mechanism itself isn't implemented yet. Building TTL and limits on top of
it is speculative.

### 6.3 Where the analysis is most grounded

**The HANDOFF critique (document 16, Section 1).** The five failure modes are
observed, not hypothetical. This is the most grounded analysis because it
diagnoses a problem Greg has actually experienced and describes why it
happens at a structural level.

**The beads integration pipeline (document 13).** This is grounded in the
harmonik experience. The YAML schema, loader, mnem-maps, and cross-work
edges were all proven at scale. The analysis is translating a working system
to kerf's context, not designing from scratch.

**The session-start and session-end protocols (document 15, Sections 2 and
5).** These are concrete, step-by-step, and directly executable. They could
be implemented tomorrow with the existing kerf commands.

---

## 7. Summary of Critical Findings

**Must resolve before speccing:**
1. SESSION.md vs. session log YAML — pick one artifact model and rewrite all
   protocol references to match.
2. `kerf orient` vs. `kerf map` + `kerf resume` — decide whether orient is
   a new command or a composition of existing commands.
3. Work status ownership during implementation — is it computed from beads
   or manually updated by agents?

**Most important gaps:**
1. Cross-work bead dependencies have no representation in the priority model.
2. Multi-agent concurrency is acknowledged but completely unspecified.
3. `kerf link` is used in protocols but never defined.
4. The batch-execution workflow (plan everything then run 40 beads) doesn't
   map to the per-station model.

**Highest-leverage simplifications:**
1. Start with flat area list + parent hierarchy. No typed edges.
2. Start with topological sort + human override. No critical path, no
   fan-out scoring, no value weights.
3. Start with simple structured session record (markdown, per-work). Evolve
   to YAML + filtering when scale demands it.
4. Start with `kerf next` returning just the work-level recommendation
   without querying beads. Add beads status integration as a second step.
