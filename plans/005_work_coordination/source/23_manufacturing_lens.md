# Manufacturing Lens — TPS Applied to kerf's Work Coordination

> Analytical document. Mapping Toyota Production System concepts to the agent-driven SDLC system kerf is building. Concrete, specific, honest about where the analogy holds and where it breaks.

---

## 1. The Pull System (Kanban)

### How It Works in TPS

A downstream station consumes a part. That consumption generates a signal (the kanban card) that travels upstream, authorizing the upstream station to produce exactly one replacement. Nothing is produced without a downstream signal. The card IS the authorization.

### Mapping to kerf

The stations are: Intake -> Design -> Decompose -> Queue -> Execute -> Verify -> Archive.

**Downstream = Execute.** The executing agent is the consumption point. When it finishes a bead, it has consumed one unit of work. That consumption is the pull signal.

**The pull signal = `kerf next`.** When an agent calls `kerf next`, it is pulling. It is saying "I have capacity. Give me work." This is textbook kanban. The system does not push beads at agents — agents pull when ready.

**The kanban card = the bead itself.** In TPS, the kanban card travels with the physical part AND travels back upstream as a production signal. In kerf, the bead serves both roles: it describes the work (travels with the "part") and its completion triggers the next pull (signals capacity). The bead's status transition (open -> in-progress -> done) IS the kanban signal.

**How pull prevents overproduction at each boundary:**

| Boundary | Overproduction risk | Pull prevents it by... |
|---|---|---|
| Design -> Decompose | Speccing 12 works, decomposing all of them, then executing none | Only decomposing a work when execution capacity exists downstream |
| Decompose -> Queue | Generating 200 beads across 8 works when only 20 can be executed this week | Only loading beads into the queue when agents pull them |
| Queue -> Execute | Dispatching 10 beads to agents who can only handle 3 | Agent pulls one at a time; no dispatch without pull signal |
| Execute -> Verify | Completing beads faster than they can be verified | Verification pulls completed work when ready |

**Adapter/retry/circuit-breaker example:** Under pull, the retry work (Work A) enters Design, gets decomposed into beads, and execution agents start pulling beads. When the circuit-breaker requirement arrives (Work C), it enters Intake. Under a push system, you'd immediately decompose Work C and dump its beads into the queue alongside Work A's beads, creating chaos. Under pull, Work C sits at Design or Decompose until downstream capacity exists. The Queue station only feeds beads to agents when they pull. Work C's beads don't compete with Work A's beads until an agent asks for more work.

### Where Pull Breaks Down

**The design stations don't have a natural pull signal.** In manufacturing, every station has a downstream consumer that pulls. But who pulls a spec into existence? The human has an idea. That's a push — the idea pushes into Intake regardless of downstream capacity. You can't tell the human "don't have ideas until we have capacity to spec them." Intake and Design are inherently push-driven. Pull starts at the Queue.

**Feedback loops violate the single-direction flow that pull assumes.** When Verify finds a spec error and sends work back to Design, that's not a pull — it's a defect signal (see Jidoka below). The work re-enters the line at an upstream station, which has nothing to do with downstream capacity. TPS handles this with separate defect-handling flows, not the main kanban loop.

**Batch planning is a push.** Greg described planning sessions where many works are planned at once. That's batch push into Stations 1-3. This is fine — TPS doesn't require pull at every station, only at the constraint point. The constraint in this system is agent execution capacity, so pull at the Queue is what matters.

---

## 2. Jidoka (Autonomation / Stop-and-Fix)

### How It Works in TPS

Any worker can pull the andon cord to stop the production line when they detect a defect. The line stops. A team swarms the problem. They don't just fix the defect — they fix the PROCESS that allowed the defect. Production resumes only when the root cause is addressed. The key insight: stopping the line is CHEAP compared to letting defects flow downstream and compound.

### Mapping to kerf

**The andon cord = a status signal from any station back to the responsible upstream station.** When an executing agent discovers a spec error, that's an andon pull. The question is: what stops?

**"Stop the line" has different scopes in this system:**

| Scope | What stops | When |
|---|---|---|
| Bead-level | Just this one bead | Minor implementation issue, review feedback |
| Work-level | All beads for this work | Spec error found — the spec that generated these beads is wrong |
| Area-level | All beads touching this area | Design flaw in the area — multiple works are building on bad assumptions |
| System-level | Everything | Architectural error discovered — but this is probably never warranted |

**The critical case is work-level stop.** Adapter/retry/circuit-breaker example: Agent is executing bead A4 (add retry wrapper to HTTP calls) and discovers the spec assumed the adapter uses synchronous calls, but it's actually async. This is a spec error. The jidoka response:

1. **Stop:** All beads for Work A are paused. Beads A5, A6, A7 do NOT proceed to execution. They may be building on the same wrong assumption.
2. **Signal:** The work's status changes to something that makes it visible — back to Design or marked with a flag.
3. **Swarm:** The spec is corrected. The async constraint is documented.
4. **Evaluate:** Completed beads A1-A3 are reviewed — do they still hold under the corrected spec? Maybe A1 and A2 are fine but A3 needs rework.
5. **Resume:** The work re-enters Decompose (A3 is re-tasked, A5-A7 may need revision), then flows back to Queue.

**Fix the process, not just the defect.** This is the deeper jidoka principle. If a spec error was found during execution, the question isn't just "fix this spec" — it's "why did the spec pass the quality gate at Design with this error?" Possible process fixes:

- The Design station's quality gate didn't verify assumptions against the actual codebase
- The spec-writing jig doesn't include a pass that checks implementation reality
- The area's shared constraints don't document the async requirement

In kerf terms: if `kerf square` passed a spec with a factual error about the codebase, maybe `kerf square` needs a pass that validates technical assumptions — not just structural completeness.

**The andon signal mechanism.** Greg's system has multiple agent types (planners, allocators, executors, testers/mergers) that may not communicate directly. The andon signal needs to work through kerf:

- Executing agent marks a bead as "blocked: spec-error" (not just "failed")
- kerf detects the blocked-with-reason status
- `kerf next` stops serving beads from that work
- `kerf map` shows the work has a jidoka stop
- The next planning agent (or allocator) sees it and routes the work back to Design

This is different from a vague "I had trouble" — the signal must carry the TYPE of failure. Spec error, decomposition error, missing prerequisite, area conflict. Each type routes differently.

---

## 3. Heijunka (Production Leveling)

### How It Works in TPS

Toyota doesn't build 500 Corollas on Monday and 500 Camrys on Tuesday. They build Corolla-Camry-Corolla-Camry throughout the week. This levels demand on upstream suppliers (no spike for Corolla parts on Monday) and keeps all stations busy (the Camry station isn't idle Monday).

### Mapping to kerf

**The risk of unlevel production:** If all 15 beads for the adapter work are executed sequentially before touching anything else, several things go wrong:

1. Other areas of the system starve. The database layer has beads ready but no agent touches them for days.
2. If the adapter design is wrong, you've sunk 15 beads of effort before finding out.
3. Testing can't start on the adapter until all 15 beads are done (batching delay — see Section 4).

**Leveled production looks like:** Execute 3-4 adapter beads, then 2 database beads, then 2 more adapter beads, then 1 API bead. Mix the areas. This:

- Gives earlier feedback on each area (you learn about the adapter's async problem after 3 beads, not 15)
- Keeps verification possible incrementally (test a small adapter slice while database beads execute)
- Prevents "tunnel vision" where all attention goes to one area

**How to level in practice.** The Queue station's ordering algorithm should factor in area diversity. Not just "what's unblocked with the most fan-out" but also "has this area received disproportionate attention recently?" This is a soft factor — it nudges, not blocks.

**Tension with momentum.** Momentum (keep working in the same area while context is warm) directly opposes heijunka (spread work across areas). TPS handles this by making the changeover cost low — Toyota invested heavily in reducing setup times so switching between Corolla and Camry took minutes, not hours. In the agent system, the "changeover cost" is context loading: reading a different spec, understanding a different area. If that cost is low (good bead descriptions, good spec references), heijunka wins. If it's high (agents need 20 minutes of context to start each bead), momentum wins.

Greg should watch for the "all adapter, all the time" pattern as a heijunka warning sign. If `kerf map` shows 15/20 completed beads are in one area while 3 other areas are untouched, the production is unlevel.

---

## 4. Single-Piece Flow vs. Batch

### How It Works in TPS

Single-piece flow: one unit moves through all stations before the next unit enters. Batch: 100 units pile up at Station 1, then all move to Station 2, then all to Station 3. Single-piece flow is almost always faster in total throughput, even though each individual station is "less efficient" (more changeovers). The reason: batching creates inventory (see Muda) and hides defects.

### The Tension in kerf

Greg described two workflows:

1. **Big planning sessions** — batch design 6 groups of tasks, generate hundreds of beads, then execute. This is batch processing at Stations 1-3.
2. **Iterative fixes** — find problem, plan small, task small, execute small. This is closer to single-piece flow.

**Batch planning is not inherently wasteful here.** The TPS argument against batching is: if Station 1 produces 100 units and Station 3 finds a defect, you've already produced 99 more units with the same defect. But in kerf, the "defect" risk at Design is different. A planning session that designs 6 works isn't producing 6 identical things — it's producing 6 different things. A defect in Work A's spec doesn't imply a defect in Work B's spec (unless they share an area, which is the co-design problem).

**Where batching IS harmful:**

- **Decompose-to-Execute batching.** Generating 200 beads and dumping them all into the queue before executing any is a batch. If the first 5 beads reveal a spec problem, the other 195 may be wasted. Single-piece flow here would mean: decompose a small slice, execute it, verify it, then decompose the next slice. This is the "walking skeleton" pattern — build a thin vertical slice through the whole system before filling out the horizontal layers.
- **Execute-to-Verify batching.** Executing all beads for a work before verifying any is a batch. If bead A3 is wrong, beads A4-A15 may build on top of the mistake. Better: verify in small batches (every 3-5 beads) so feedback arrives sooner.

**The practical recommendation:** Batch at Intake and Design (it's natural and low-risk). Flow at Decompose, Queue, Execute, and Verify (where defect cost compounds). Think of Stations 1-2 as a "planning cell" that operates in its own rhythm, and Stations 3-6 as the "production line" that operates in flow.

---

## 5. Kanban Boards and WIP Limits

### How It Works in TPS

A kanban board visualizes work at each station. Each station has a maximum number of kanban cards — the WIP limit. When a station is at its limit, upstream stations must stop producing, even if they have capacity. This creates deliberate idle time upstream, which is counterintuitive but prevents the system from drowning in inventory.

### What the Board Looks Like

```
| Intake | Design | Decompose | Queued | Executing | Verifying | Done |
|--------|--------|-----------|--------|-----------|-----------|------|
| spark  | retry  |           | A3,B1  | A2        | (none)    | A1   |
|        | c-brkr |           | A4,B2  |           |           |      |
|        |        |           | C1     |           |           |      |
```

Rows under Intake and Design are works. Rows under Queued and Executing are beads. This reflects the granularity shift at the Queue station.

### WIP Limits (If Greg Wanted Them)

Greg said he doesn't want WIP limits yet. But if he did:

| Station | Possible WIP Limit | Rationale |
|---|---|---|
| Design | 3 works | More than 3 works in design means planning faster than executing — overproduction |
| Decompose | 2 works | Decomposition should be fast; if works pile up here, something's wrong |
| Queued | 20 beads | More than 20 queued beads means execution can't keep up — stop feeding the queue |
| Executing | Depends on agent count | 1 per agent, naturally limited |
| Verifying | 2 works | Verification backlog means quality debt |

**The WIP limit Greg doesn't realize he already has:** Agent count. If you have 3 executing agents, your WIP at Execute is naturally limited to 3. That's a physical constraint, like the number of machines on a factory floor. Everything upstream must ultimately flow through that bottleneck. Anything produced faster than 3 agents can consume is inventory.

### Making WIP Visible Without Enforcing Limits

This is what Greg actually wants. `kerf map` should show counts at each station:

```
Design:    2 works (retry, circuit-breaker)
Decompose: 0
Queued:    7 beads across 2 works
Executing: 1 bead (A2)
Verifying: 0
```

This lets a human spot imbalance. "Why are 7 beads queued but only 1 executing? Am I bottlenecked on agents?" That's the value of the kanban board even without limits — making the invisible visible.

---

## 6. Kaizen (Continuous Improvement)

### How It Works in TPS

Every worker is expected to identify improvements. Small changes are made continuously. There are no "improvement projects" — improvement is embedded in daily work. The A3 report (a structured one-page problem/solution format) is how improvements are proposed and tracked.

### Mapping to kerf

**Where improvement data lives:**

| Signal | Source | What it reveals |
|---|---|---|
| Beads that get blocked by spec errors | Execute -> Design feedback loop | Spec-writing jig isn't catching assumption errors |
| Beads that need re-decomposition | Execute -> Decompose feedback loop | Decomposition pass isn't scoping beads correctly |
| Works that pass Verify on first try | Verify station | Design and decomposition are working well for this area |
| Areas with repeated rework | `kerf map` area view over time | Systemic design weakness in that area |

**How kerf can support kaizen:**

1. **Track rework.** When a work loops back from Execute to Design, record it. After 10 works, you can ask: "What percentage of works required spec rework? What was the most common reason?" This is the manufacturing equivalent of tracking defect rates per station.

2. **Track cycle time.** How long does a work take from Intake to Archive? Where does it spend the most time? If works spend 3 days in Design and 1 day in Execute, the constraint is Design — invest in better spec-writing processes.

3. **Improve the jigs.** The jig system (plan, spec, bug, implementation) IS the "standard work" of TPS — the documented best-known method for each operation. When a jig consistently produces specs that fail during execution, the jig needs improvement. Add a pass. Change a prompt. Require a specific check. This is kaizen applied to the tooling.

**The critical feedback loop Greg identified:** "If something isn't fully implemented, and an issue is discussed but not prioritized or processed, then the issue remains." This is exactly the TPS concern with defects flowing downstream. The kaizen response: make the "issue found during execution" pathway as frictionless as possible. One command to create a high-priority bead. No ceremony. The harder it is to report a problem, the more problems get ignored.

---

## 7. Muda (Waste) Identification

### The Seven Wastes Applied

**1. Overproduction:** Speccing works that never get built. Decomposing works into beads that sit in the queue indefinitely. Greg's batch planning sessions risk this: 6 groups of tasks designed, but only 2 groups are ever executed because priorities shift. TPS fix: produce only what's been pulled. Design the next work only when execution capacity is approaching.

But note the nuance: in manufacturing, overproduction wastes physical materials. In software, overproduction wastes agent time and human review time. A spec that's written but never used isn't wasting steel — it's wasting the hour the agent spent writing it and the 20 minutes the human spent reviewing it. The cost is real but different.

**2. Waiting:** Beads blocked on dependencies that haven't been completed. An executing agent with no available beads because all unblocked ones are taken. An allocator agent waiting for `kerf next` to have results because all works are stuck in Design. TPS fix: level the flow so every station always has work. Surface blocked beads with their specific blocker so the blocker can be prioritized.

**3. Transport:** Moving work artifacts between tools unnecessarily. Exporting beads from `bd` to YAML, manually copying into harmonik, manually updating kerf status. Every "copy the output of Tool A into Tool B" is transport waste. TPS fix: minimize handoffs. Have `kerf next` query `bd` directly rather than requiring a human to relay the information.

**4. Over-processing:** Writing a 200-line spec for a 3-line bug fix. Running all jig passes (plan, analyze, decompose, research, draft, review) for a trivial change. The bug jig exists precisely to avoid this — it's a shorter path for simpler work. TPS fix: right-size the process to the work. Not everything needs the full assembly line. Triage at Intake should determine WHICH line the work enters.

**5. Inventory:** Queued beads that aren't being executed. Specs waiting for decomposition. Completed beads waiting for verification. In manufacturing, inventory costs money (storage, capital tied up, obsolescence risk). In this system, inventory risks staleness — a bead written 2 weeks ago may reference code that's changed since then. The "inventory cost" in this system is context decay.

**6. Motion:** Agents re-reading specs, re-loading context, re-scanning the work graph when they shouldn't need to. An agent that needs to read 5 spec files to understand one bead is doing unnecessary motion. TPS fix: put everything the agent needs at its workstation. The bead should contain or reference everything needed — no scavenger hunts.

**7. Defects:** Specs that don't match the codebase. Beads that implement the wrong thing. Tasks that are mis-scoped. Integration failures at Verify. Every defect triggers rework — the most expensive waste, because rework goes through the line AGAIN. TPS fix: build quality in at each station (jidoka). Don't pass defects forward. Catch the spec error at Design, not at Execute.

### The Eighth Waste (Modern Addition): Unused Talent

In this system: agent capabilities that aren't leveraged. A powerful planning agent used only for trivial bug triage. An executing agent that could identify spec issues but isn't given a mechanism to report them. Give agents the andon cord.

---

## 8. Priority and Scheduling Through the TPS Lens

### What TPS Offers Instead of P0/P1/P2

Greg rejected static priority labels because they decay. P2 becomes P1 when P0 is done. The labels require constant maintenance. TPS doesn't use priority labels either. It uses three mechanisms:

**Takt time — matching production rate to demand rate.**

Takt time = available production time / customer demand. In a factory, if customers buy 480 cars per day and the factory runs 480 minutes, takt time is 1 minute per car. Every station must complete its work in under 1 minute.

Applied to kerf: What's the "demand rate" for completed beads? If the testing/merging agent can verify and merge 8 beads per day, and the planning system generates 15 beads per day, there's a mismatch. The system will build inventory (queued beads). Either slow down planning or speed up verification.

Takt time replaces priority with PACE. Instead of asking "which bead is most important?" ask "at what rate do beads need to flow through each station to keep the line balanced?" This reframes the scheduling problem entirely.

**Sequence scheduling based on downstream need.**

In TPS, the sequence is determined by what the downstream station needs next — not by what the upstream station thinks is important. The final assembly line determines the sequence for the paint shop, which determines the sequence for the body shop.

Applied to kerf: What does Verify need? Completed, coherent slices of functionality. So Execute should prioritize completing a testable slice over starting new work. What does the user need? A working feature. So the queue should prioritize beads that complete a user-visible capability over beads that start a new one.

This is Greg's "in-flight" intuition formalized: don't pull new work forward unless in-flight work is done. In TPS terms: finish the car on the line before starting a new one. A half-built car on the line is the most expensive form of inventory.

**Just-in-time task generation.**

In TPS, parts arrive at the station exactly when needed — not before (inventory) and not after (waiting). Applied to kerf: don't decompose Work C into beads until Work A's beads are nearly exhausted. The decomposition is done "just in time" for execution to pull from.

This directly addresses the "stale beads" problem. If beads are generated just before execution, they reflect the current state of the codebase. If they're generated weeks in advance, they may reference code that's changed.

### What Replaces Pins

Greg rejected "pins" (explicit priority overrides with TTLs). TPS offers an alternative: **the expedite lane.**

In a kanban system, there's sometimes a dedicated "expedite" lane that bypasses the normal WIP limits. It's limited to one item at a time. It's for genuine emergencies. It doesn't require labeling everything with priorities — it's a structural mechanism for the exceptional case.

Applied to kerf: Instead of `kerf pin <work>`, have `kerf expedite <work>`. Semantics:

- Only one work can be expedited at a time
- Expedited work's beads jump to the front of the queue
- When the expedited work is complete (or back to stable), the expedite lane is empty again
- No TTL needed — the structural limit (one at a time) prevents abuse

This is simpler than priority levels. There's normal flow and there's the one emergency. If everything is an emergency, nothing is — and the one-at-a-time limit enforces that discipline.

### The Feedback-Loop Priority Problem

Greg's scenario: "Testing finds an issue, that issue needs to be fast-tracked through plan/task/implement." In TPS, this is a defect that triggers a rework order. Rework orders have inherent priority — they go to the FRONT of the queue because they represent work that was ALREADY counted as done. The line's output numbers assumed that unit was complete. Every minute it stays broken, the line's effective throughput is lower than reported.

Applied to kerf: When Verify finds a problem:

1. New beads are created for the fix
2. These beads are tagged as rework (not new work)
3. Rework beads automatically get higher queue position than new-work beads
4. No priority labels needed — the distinction is structural (rework vs. new work)

This is the "high priority queue" Greg described, but implemented as a structural property rather than a label. Rework-beads-go-first is a rule of the system, not a human judgment call each time.

---

## 9. Where the Analogy Breaks Down

### Fundamental Differences Between Software and Manufacturing

**1. The product is not standardized.**

Toyota builds thousands of the same car. Each car goes through identical stations with identical operations. In kerf, every bead is unique. Every spec is different. The "stations" are the same, but the work at each station is novel every time. This means:

- You can't optimize station operations the way Toyota does (time studies, ergonomic optimization) because the operations differ each time
- Defect rates are less predictable — a jig that works for one spec may fail for another
- "Standard work" can only be defined at the process level (how to write a spec), not at the content level (what to put in the spec)

**2. The feedback loops are much longer and more expensive.**

In manufacturing, a defect is usually detected at the next station (visual inspection, measurement). In software, a spec error might not be detected until integration testing, many beads later. The "distance" between defect creation and detection is much larger. This makes jidoka (early detection) more valuable here than in manufacturing — and harder to achieve.

**3. The work changes the factory.**

When Toyota builds a car, the factory is unchanged afterward. When an agent implements a bead, the codebase changes. Future beads operate on a different "factory" than earlier beads did. This is why beads can become stale — they were designed for the factory as it was, not as it is. Manufacturing doesn't have this problem (the factory is a fixed asset, not a mutable one).

This is the deepest disanalogy. In TPS, the environment is controlled. In software, the environment is the product, and it changes with every operation. This means:

- Just-in-time task generation is MORE important here than in manufacturing (environment changes fast)
- Inventory (queued beads) has a higher spoilage rate than physical inventory
- The queue ordering must account for "what has changed since this bead was created"

**4. Parallelism works differently.**

In manufacturing, parallel production means duplicate stations (two stamping presses). Each produces identical output. In software, parallel execution means two agents working on different beads. They can conflict (touching the same files), interact (one's output affects the other's work), and interfere in ways that duplicate machines can't. File reservation is the kerf equivalent of machine scheduling — but it's more complex because the "machines" (files) are shared and mutable.

**5. There is no physical material flow.**

The kanban card works in TPS because it physically travels with the part. You can see it. You can count the cards. The "inventory" is physically visible on the factory floor. In kerf, work is abstract. You can't look at the factory floor and see 47 beads piling up. This makes the kanban board (the visualization) MORE important in software than in manufacturing — it's the only way to make the invisible visible. `kerf map` IS the factory floor.

**6. The customer doesn't have a fixed demand rate.**

Takt time assumes a known, relatively stable demand rate. In software, "demand" (ideas, bugs, feature requests) is irregular and unpredictable. You can't calculate a meaningful takt time when demand comes in bursts. The takt time concept is still useful as a diagnostic (are we producing faster than we can verify?) but not as a scheduling parameter.

**7. "Quality" is subjective and contextual.**

In manufacturing, quality is measurable: the part is within tolerance or it isn't. In software, "does this match the spec?" is not always binary. Specs can be ambiguous. Implementation choices that satisfy the spec's letter but not its intent are common. The quality gate at each station is fuzzier here, which means the jidoka principle (detect defects early) requires judgment, not measurement.

---

## 10. Synthesis: What to Take from TPS

### High-Value Concepts for kerf

1. **Pull at the Queue.** `kerf next` is the pull signal. Agents pull work. The system doesn't push. This is the single most important TPS concept for kerf and it's already in Greg's thinking.

2. **Rework-before-new-work.** Beads created from feedback loops (Verify -> Execute, Execute -> Design -> Decompose -> Queue) get structural priority over new-work beads. No labels needed — the distinction is type-based.

3. **Make WIP visible.** `kerf map` as a kanban board showing work counts at each station. No enforcement, just visibility. Let the human spot imbalance.

4. **Jidoka at Execute.** Give executing agents a typed failure signal. "Spec error" stops the work. "Decomposition error" re-tasks. "Minor issue" loops locally. The type determines the response — don't ask the agent to decide how far back the work should travel.

5. **Expedite lane instead of priority labels.** One slot. One emergency at a time. Structural limit prevents priority inflation.

6. **Just-in-time decomposition.** Don't generate all beads upfront. Decompose a work into beads close to when execution will pull them. This keeps beads fresh and reduces inventory waste.

### Concepts to Defer

- **WIP limits** — Greg's right that it's too early. Make WIP visible first. Limits come when you have data on where the line jams.
- **Takt time** — Useful as a diagnostic after the system is running. Not actionable during construction.
- **Formal heijunka** — Worth monitoring (is work spread across areas?) but don't build scheduling logic for it yet. Let `kerf map` show area concentration and let the human decide.

### Concepts to Reject

- **Strict single-piece flow through all stations.** Batch planning is natural and low-cost. Flow matters at Decompose-through-Verify, not at Intake-through-Design.
- **Standardized work at the content level.** Each bead is unique. You can standardize the process (how to implement a bead) but not the operation (what code to write). Don't try.

---

## 11. The Priority Question Answered

Greg asked: what replaces P0/P1/P2?

TPS says: priority is an emergent property of system state, not a label on a work item. The ordering algorithm for `kerf next` should compute priority from:

1. **Hard constraints:** dependency graph (must come first)
2. **Structural priority:** rework beads before new-work beads (defects before features)
3. **Completion proximity:** beads that finish a testable slice before beads that start new work (in-flight before new)
4. **Expedite:** the one emergency slot, if occupied
5. **Fan-out:** beads that unblock the most downstream work
6. **Momentum:** same-area preference as a tiebreaker

No P0/P1/P2. No pins. No TTLs. The ordering is recomputed every time an agent pulls. Priorities can never be stale because they're never stored — they're computed from current state.

This is the deepest TPS lesson: **the schedule is not a plan, it's a computation.** It's derived from the current state of the system, not from a prediction about the future. Every time `kerf next` is called, it's a fresh computation. The "priority" of any given bead can change between one call and the next, because the state of the system changed.

Greg's instinct — "don't pull any work items forward unless there is no [in-flight] work" — is exactly right. That's the pull system. That's kanban. That's the assembly line. The system doesn't need priority labels. It needs a computation that answers "given the current state of the line, what should the next agent work on?" And the answer changes every time the state changes.
