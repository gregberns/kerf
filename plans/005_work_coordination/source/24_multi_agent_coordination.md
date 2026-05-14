# Multi-Agent Coordination Through Shared State

> Analysis of kerf as a blackboard architecture for independent agent coordination.

---

## 1. The Blackboard Pattern

Four agent types operate independently, coordinating through kerf as shared state. None talk to each other directly. Each reads and writes to kerf artifacts (filesystem-backed: spec.yaml, areas.yaml, beads state, work items). The coordination challenge is that locally correct decisions must produce globally coherent outcomes.

### What Each Agent Type Reads and Writes

**PLANNING agent(s)**
- Reads: existing work items, areas.yaml (system map), specs, `kerf map` output (portfolio view), other works in the same area (co-design peers)
- Writes: new work items (via `kerf new`), specs (jig passes), area definitions, dependency declarations between works, task YAML (decomposition)
- Interaction: Interactive sessions with user. Sporadic. Multiple PLANNING agents may operate on different areas concurrently.

**ALLOCATE agent**
- Reads: `kerf next` output (ranked actionable items), bead state (what's done, what's ready), dependency graph (what's unblocked), work status across all works
- Writes: dispatches beads (marks them as assigned/in-flight), possibly updates work status to "implementing"
- Interaction: Runs in a loop. Polls kerf for what's ready, pushes beads into harmonik's execution queue. Stateless between iterations — kerf is the memory.

**EXECUTE agents**
- Reads: the specific bead(s) assigned to them, the spec backing those beads, relevant code context
- Writes: code changes, bead completion status (marks beads done)
- Interaction: Spawned per-task (or small batch), ephemeral. They mostly don't interact with kerf directly — they interact with the codebase and with beads (bd/bv). Their output enters kerf's world through bead state changes.

**MERGE/TEST agent**
- Reads: completed beads, implemented code, specs (to verify against), `kerf map` (to understand what else is in-flight in the same area)
- Writes: test results, issue reports (new work items or fast-tracked intake entries), bead status updates (pass/fail), possibly status rollbacks on works
- Interaction: Runs in a loop. Watches for completed beads, tests them, feeds findings back.

### Interaction Pattern

This is **polling, not event-driven**. Each agent queries kerf's state when it's ready for more work. There is no notification mechanism — an agent runs `kerf next` or `kerf map` and acts on what it finds. This is consistent with the filesystem-as-database architecture: files don't send events.

The polling cadence differs per agent type:
- ALLOCATE: fast loop (seconds to minutes), checking for newly actionable beads
- MERGE/TEST: moderate loop, checking for newly completed beads
- PLANNING: on-demand, when user starts a session
- EXECUTE: doesn't poll — is given work and terminates

### Consistency Guarantees Needed

The critical question: can two agents update the same thing simultaneously?

**Low-conflict pairs:**
- PLANNING and EXECUTE rarely touch the same artifacts. PLANNING writes specs and work items; EXECUTE writes code and bead state.
- Two EXECUTE agents working on different beads are independent by construction (if the dependency graph is correct).

**High-conflict pairs:**
- PLANNING and MERGE/TEST both create work items. If MERGE/TEST discovers an issue while PLANNING is creating related work, you get parallel creation of potentially overlapping items in the same area.
- ALLOCATE reads bead state while EXECUTE writes it. Classic reader-writer. Stale reads are acceptable — ALLOCATE dispatches work that's already been picked up, the next iteration corrects.
- Two PLANNING sessions working on overlapping areas. This is the co-design problem already identified in the plan.

**Required guarantee:** Atomic work item creation. When MERGE/TEST creates a fast-tracked issue, it must be visible to ALLOCATE on the next poll. Filesystem atomicity (write temp file, rename) is sufficient. No distributed locks needed — eventual consistency at the iteration boundary is fine.

---

## 2. Information Flow Between Agent Types

Each arrow below represents information flowing through kerf artifacts. No direct communication.

### PLANNING --> ALLOCATE: "here are new tasks ready for execution"

**Mechanism:** PLANNING writes spec, decomposes into task YAML, loads beads into the beads system. The beads enter in a "ready" state (or blocked by dependencies). ALLOCATE runs `kerf next` which queries bead state and surfaces ready-to-dispatch items.

**The artifact chain:** spec.yaml (status: "implementing") + beads loaded in bd --> `kerf next` reads both --> ALLOCATE sees actionable items.

**Key requirement:** `kerf next` must compose information from kerf (work-level priorities, area relationships, dependencies) with information from beads (task-level readiness, completion state). This is the integration seam between the two systems.

### ALLOCATE --> EXECUTE: "work on these beads"

**Mechanism:** ALLOCATE pushes beads into harmonik's queue. This is outside kerf's domain — it's harmonik dispatching to workers. But kerf's contribution is the *selection*: which beads, in what order. ALLOCATE reads `kerf next`, which has already done the ranking.

**Note:** This is the one flow that may not go through kerf at all. ALLOCATE reads from kerf, writes to harmonik. kerf provides the intelligence; harmonik provides the execution machinery.

### EXECUTE --> MERGE/TEST: "these beads are implemented, ready for testing"

**Mechanism:** EXECUTE marks beads as completed in the beads system (bd). MERGE/TEST polls bead state looking for newly completed beads. The signal is the bead state transition: not-done --> done.

**kerf's role:** Minimal here. The beads system is the coordination channel for this flow. kerf might provide area context: "this bead belongs to work X which touches area Y, so test it in that context."

### MERGE/TEST --> PLANNING: "found issues, need re-spec"

**Mechanism:** MERGE/TEST creates a new work item in kerf with high urgency. This is the feedback injection problem (see section 3). The work item sits in kerf's intake. Next time PLANNING is active, `kerf map` shows it. If PLANNING is not active, ALLOCATE still sees it via `kerf next` and can dispatch it through the fast-track path.

**Critical gap:** PLANNING agents are interactive — they run when the user is present. If an issue requires re-speccing, it may sit until the user starts a session. The system needs to distinguish between issues that require human judgment (true re-spec) and issues that can be auto-handled (small bug fix that an EXECUTE agent can plan+implement autonomously).

### MERGE/TEST --> ALLOCATE: "this failed, needs re-prioritization"

**Mechanism:** MERGE/TEST writes a finding to kerf — either a new work item or a status annotation on an existing bead/work. ALLOCATE, on its next poll of `kerf next`, sees the new high-priority item and dispatches it.

**The priority signal:** The finding must carry enough information for `kerf next` to rank it appropriately. A failed test on in-flight work in an active area should rank higher than a new feature in an idle area.

---

## 3. The Feedback Injection Problem

This is the hardest coordination problem in the system. Greg's scenario: MERGE/TEST finds a problem and needs to "put something into the kerf system that would fast track a plan, task, implement process."

### What MERGE/TEST Writes

There are three categories of findings, and each needs different treatment:

**Category A: Simple bug fix.** The test failed, the cause is clear, the fix is small. MERGE/TEST creates a new bead (or small set of beads) attached to the existing work. No re-spec needed. This is a task-level injection into the beads system.

**Category B: Implementation gap.** The spec is correct but the implementation missed something. MERGE/TEST creates a new work item in kerf with type "bug" or "fix", linking it to the original work. The jig for this is lightweight — maybe just the bug jig. The work goes through a compressed plan/spec/task cycle.

**Category C: Spec deficiency.** The spec itself is wrong or incomplete. MERGE/TEST creates a work item that references the original spec and describes the deficiency. This genuinely needs PLANNING attention — a human or planning agent must think through the design implications.

### What Gets Written, Concretely

A new work item via `kerf new` with:
- `type: fix` or `type: bug`
- `areas:` matching the affected area(s)
- `urgency: high` (a new field, or a priority annotation)
- `source: merge-test` (provenance — who injected this)
- `related_to:` referencing the original work/bead that failed
- A description of the finding with enough context for the next agent

For Category A, MERGE/TEST might bypass kerf entirely and inject beads directly into the beads system with a dependency on the failed bead. This is the fast path.

### How ALLOCATE Discovers It

`kerf next` must surface urgent items first. The algorithm:

1. Check for items with `urgency: high` — these go to the top regardless of dependency ordering.
2. Check for items that unblock the most in-flight work (fan-out heuristic).
3. Check for items in areas with active momentum (warm context).
4. Everything else by dependency order.

ALLOCATE doesn't need special logic. It just runs `kerf next` on its normal polling cadence. The urgency annotation does the fast-tracking.

### How It Gets Fast-Tracked

The key insight: fast-tracking is not a separate queue or interrupt mechanism. It is a **priority signal** that `kerf next` respects. The MERGE/TEST agent writes a work item with high urgency. `kerf next` surfaces it. ALLOCATE dispatches it. The same pipeline handles both new work and feedback — the only difference is the urgency annotation.

For Category A (simple fixes), the fast track is even faster: MERGE/TEST writes beads directly, ALLOCATE dispatches them on the next cycle. No spec needed. The fast path must be available for small fixes.

For Category C (spec issues), fast-tracking still creates the work item, but it parks it — `kerf next` might show it but note "requires planning review." ALLOCATE skips it until a PLANNING agent processes it. The urgency annotation means the PLANNING agent sees it first when they start their session.

---

## 4. Concurrent Access Patterns

### Scenario: PLANNING creates new work while EXECUTE implements existing work

**Conflict risk: Low.** PLANNING writes to kerf (new work items, specs). EXECUTE writes to the codebase and beads. Different write targets. The only coupling: if PLANNING creates work in the same area EXECUTE is implementing, the co-design mechanism should flag it — but this is advisory, not blocking.

**Resolution:** No special handling needed. `kerf map` will show both the new work and the in-flight work. The PLANNING agent's protocol should include checking `kerf map` for in-flight work in the same area before creating new specs.

### Scenario: MERGE/TEST finds issues while ALLOCATE dispatches new beads

**Conflict risk: Medium.** MERGE/TEST writes a high-urgency work item. ALLOCATE, in the same moment, dispatches lower-priority beads. On the next cycle, ALLOCATE sees the new item and adjusts. The "wasted" cycle is one batch of lower-priority beads being dispatched — they can complete concurrently with the fix, or be paused.

**Resolution:** Acceptable latency. The worst case is one ALLOCATE cycle of suboptimal dispatching. With cycle times in seconds-to-minutes, this is fine.

### Scenario: Two PLANNING sessions creating overlapping specs

**Conflict risk: High.** This is the co-design problem from the plan. Two humans (or human+agent pairs) creating specs that touch the same area without knowledge of each other.

**Resolution:** The area-based overlap detection. When `kerf new` assigns area tags, it checks for in-flight work in those areas and emits a warning. But this only works if PLANNING sessions check before creating. It's a protocol requirement, not a system guarantee.

**What kerf can enforce:** `kerf square` can detect area overlap after the fact. `kerf map` makes it visible. But preventing it requires agent discipline — or a reservation mechanism where creating a work in an area puts a "someone is designing here" flag that other PLANNING sessions see.

### Scenario: EXECUTE marks bead done while MERGE/TEST is testing the previous bead's output

**Conflict risk: Low.** Different beads, different state transitions. But there's a logical dependency: if bead N's test fails, should bead N+1 (which depends on N) continue? The beads dependency graph handles this — N+1 shouldn't be dispatched until N is verified, not just implemented.

**Resolution:** This implies a bead lifecycle of: ready --> dispatched --> implemented --> verified --> done. Currently beads may only have: open --> closed. The gap: there's no "implemented but not verified" state. If MERGE/TEST is a real agent, beads need this intermediate state, or the MERGE/TEST agent needs to be the one that marks beads as done (not the EXECUTE agent).

---

## 5. Coordination Without Communication

kerf is the "shared state" coordination model. What can and can't be coordinated this way?

### What Shared State Handles Well

**Status visibility.** Any agent can see what any other agent has done by reading kerf state. No direct communication needed. `kerf map` is the universal status query.

**Priority and ordering.** `kerf next` computes global ordering from local state changes. Each agent just does its job and updates state; `kerf next` synthesizes the global view.

**Provenance.** Because all coordination flows through artifacts, there's a natural audit trail. You can trace: who created this work item, what triggered it, what area it touches, what it depends on.

**Asynchronous handoff.** Agents don't need to be running at the same time. PLANNING writes, terminates. Hours later, ALLOCATE reads. The artifacts are the message. This is a fundamental fit for the agent lifecycle model (agents start and stop independently).

### What Shared State Handles Poorly

**Urgency/interruption.** Shared state is polled, not pushed. If MERGE/TEST finds a critical issue, it writes to kerf, but ALLOCATE doesn't know until its next poll cycle. For fast cycle times this is fine. For truly urgent issues (breaking the build, data corruption), polling introduces latency.

**Negotiation.** Two agents need to agree on something — e.g., "should this spec change be merged into the in-flight work or kept separate?" Shared state can't facilitate back-and-forth negotiation. This requires either human intervention or a pre-defined protocol that makes the decision mechanically.

**Intent declaration.** "I'm about to work on area X, does anyone else need to know?" Shared state captures what was done, not what's about to be done. Reservations or locks could address this, but they add complexity and require cleanup when agents crash.

**Ordering guarantees across agents.** "Agent A must finish before Agent B starts" is expressible in dependency graphs but not enforceable by kerf alone. The ALLOCATE agent must respect the constraints; kerf can surface them via `kerf next` but can't prevent ALLOCATE from ignoring them.

### What Needs a Different Mechanism

**Build/merge coordination.** If two EXECUTE agents both implement beads that touch overlapping files, git merge conflicts arise. This isn't a kerf problem — it's an execution-level problem. harmonik or the MERGE/TEST agent handles it. But kerf could help prevent it by surfacing file-level overlap information in `kerf next` output, so ALLOCATE avoids dispatching conflicting beads concurrently.

**Human escalation.** When MERGE/TEST finds a Category C issue (spec deficiency) and no PLANNING agent is active, the finding sits. The system needs a way to notify the human. This is outside kerf's domain — it's a notification mechanism (email, Slack, terminal bell). kerf's job is to hold the finding so it's there when the human shows up.

---

## 6. Agent Lifecycle and Overlap

### Lifecycle Characteristics

| Agent | Lifecycle | Cadence | State |
|-------|-----------|---------|-------|
| PLANNING | Interactive sessions | Sporadic, user-driven | Stateful within session, stateless between |
| ALLOCATE | Persistent loop | Seconds to minutes | Stateless — reads kerf fresh each cycle |
| EXECUTE | Spawned per-task | Duration of one bead (minutes to hours) | Stateful during execution, terminated after |
| MERGE/TEST | Persistent loop | Minutes | Stateless — reads bead state fresh each cycle |

### Temporal Gaps

**ALLOCATE dispatches work, EXECUTE hasn't started yet.**
The bead is in "dispatched" state. If ALLOCATE dispatches the same bead again on the next cycle, you get duplicate work. The bead state must distinguish "ready" from "dispatched" from "in-progress." This is a beads-system concern, but kerf's `kerf next` should filter out already-dispatched items.

**MERGE/TEST finds an issue, PLANNING isn't active.**
The issue sits as a work item in kerf. `kerf next` surfaces it with high urgency. Two outcomes:
1. The issue is Category A/B (doesn't need re-spec): ALLOCATE dispatches it through the normal pipeline. PLANNING is not needed.
2. The issue is Category C (needs re-spec): it parks. `kerf map` shows it. When the user next starts a PLANNING session, the first thing `kerf map` shows is "there are urgent items requiring planning attention." The user addresses them.

The system must tolerate this delay. Category C issues are inherently slow — they require human design judgment. The system's job is to not lose them and to make them maximally visible.

**PLANNING creates work while ALLOCATE is mid-dispatch.**
No conflict. PLANNING creates the work item and beads. They enter the system in "ready" state. ALLOCATE picks them up on the next cycle. The pipeline is append-only from PLANNING's perspective.

**EXECUTE finishes a bead but MERGE/TEST is busy with another.**
Completed beads queue up. MERGE/TEST processes them in order on the next cycle. No coordination needed — the bead state (implemented, not yet verified) is the queue.

### The "No Agent Is Running" State

Between sessions, all agents may be offline. kerf's state is just files on disk. Nothing is lost. When any agent starts, it reads the current state and acts. This is the fundamental advantage of the blackboard pattern — the board persists regardless of who's reading it.

---

## 7. What kerf Needs to Be

kerf is not a database, not a message queue, not a coordination service. It is a **structured filesystem with computed views**.

The filesystem stores facts: work items exist, they have areas, they have dependencies, they have statuses, they have urgency annotations. These are written by various agents at various times.

The computed views synthesize those facts into actionable intelligence: `kerf map` (the full picture), `kerf next` (what to do now), `kerf resume` (context for a specific work).

### Abstract Operations kerf Must Support

**Register new work.** Create a work item with metadata (areas, type, urgency, related works). Perform overlap detection against in-flight works. This is `kerf new` enhanced.

**Query what's ready.** Given the current state of all works and beads, return an ordered list of actionable items. This is `kerf next`. It must integrate data from both kerf (work-level) and beads (task-level).

**Record a finding.** An agent discovered something — a bug, a gap, a dependency, a design concern. The finding must be durable, visible, and prioritizable. This is the feedback injection path. It might be `kerf new --urgency high --source merge-test` or a dedicated `kerf report` command.

**Update status.** Mark a work as implementing, done, blocked, needs-respec. This is a write to spec.yaml's status field. Simple, but it must be the authoritative state that all other views derive from.

**Show portfolio state.** The full picture: all works, statuses, areas, dependencies, in-flight beads, urgency flags. This is `kerf map`. It's a read-only view computed from the filesystem.

**Load work context.** For a specific work, show everything an agent needs to start working: spec, area peers, dependency status, co-design relationships, session history, related findings. This is `kerf resume`.

**Detect overlaps and conflicts.** Given a proposed change (new work, area modification), identify potential conflicts with existing work. This is part of `kerf new` and `kerf square`.

### Guarantees kerf Must Provide

1. **Durability.** A work item, once created, is never lost. Filesystem persistence provides this.

2. **Consistency at read time.** When an agent runs `kerf map` or `kerf next`, the output reflects all changes that have been flushed to disk. No caching, no stale views. Read-from-filesystem provides this.

3. **Atomic writes.** A work item creation (writing spec.yaml + area tags + initial status) should be atomic — other agents should see the complete work item or nothing. Write-to-temp-then-rename provides this at the file level; at the work level, the work directory's existence is the commit point.

4. **No ordering guarantees across agents.** kerf cannot guarantee that Agent A's write happens before Agent B's read. It doesn't try to. Every agent reads the current state and makes the best decision with what it sees. The next cycle self-corrects.

5. **Idempotent reads.** Running `kerf next` ten times with no state changes produces the same result. Views are computed, not consumed. No agent "takes" a work item by reading it (contrast with a message queue where reading dequeues).

### What kerf Is Not

- **Not a message queue.** Items are not consumed by reading. Multiple agents can read the same state.
- **Not an orchestrator.** kerf doesn't dispatch work. ALLOCATE reads kerf and dispatches via harmonik.
- **Not a lock manager.** kerf doesn't prevent concurrent access. Conflicts are resolved by convention (protocols) and detection (overlap warnings), not prevention.
- **Not a notification system.** kerf doesn't push updates. Agents poll.

### The Distributed Systems Analogy

These agents are like microservices with a shared database (the filesystem). The pattern is closer to **event sourcing without the event bus** — agents write facts (new work items, status changes, findings) and other agents derive their behavior from reading those facts.

The consistency model is **eventual consistency with human-speed convergence**. Changes propagate at the speed of the polling loop (seconds for ALLOCATE, minutes for MERGE/TEST, hours for PLANNING). This is acceptable because the unit of work (a bead) takes minutes to hours. The system doesn't need millisecond consistency — it needs cycle-level consistency.

The failure mode is **stale reads leading to suboptimal (but not incorrect) decisions.** ALLOCATE dispatches a lower-priority bead because it hasn't seen the new high-urgency item yet. Next cycle, it self-corrects. The cost is one cycle of suboptimal allocation, not data corruption or lost work.

---

## 8. Open Questions

1. **Bead lifecycle for verification.** If MERGE/TEST is a real agent, beads need an intermediate state between "implemented" and "done." Who defines this — kerf or beads? How does `kerf next` interact with it?

2. **The fast path for Category A fixes.** Should MERGE/TEST be able to inject beads directly into the beads system, bypassing kerf's work item creation? This is faster but creates beads without a backing work item — violating the spec-first principle. Is that acceptable for small fixes?

3. **Area reservation during PLANNING.** When a PLANNING agent starts speccing in area X, should kerf record "someone is designing in area X" to warn other PLANNING agents? How is this cleaned up if the session crashes?

4. **Urgency decay.** A high-urgency item that sits for days — does its urgency remain? Should `kerf next` show how long an item has been waiting? Should old unfixed issues escalate somehow?

5. **The ALLOCATE agent's identity.** Is ALLOCATE truly an agent (with its own reasoning), or is it a deterministic function (`kerf next` output fed directly into harmonik's queue)? If the latter, ALLOCATE is just a script: `while true; do kerf next | harmonik dispatch; sleep 30; done`. The coordination still works — kerf is doing the thinking.

6. **Cross-agent provenance.** When MERGE/TEST creates a work item that ALLOCATE dispatches and EXECUTE implements, the trail is: finding --> work item --> beads --> code. How much of this trail does kerf maintain vs. how much is reconstructed from filesystem timestamps and git history?
