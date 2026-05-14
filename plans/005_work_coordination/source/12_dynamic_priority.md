# Dynamic Priority in a Work Graph

> Thinking document. Not a proposal. Exploring the shape of priority when it isn't a label.

---

## The Failure of Static Labels

Greg's harmonik experience is the canonical case. P0/P1/P2/P3 fail because they encode a judgment made at one moment and pretend it holds forever. The moment the landscape shifts — a work completes, a dependency resolves, a new urgent thing arrives — the labels are wrong. Nobody updates them because updating labels is invisible maintenance work that produces no forward progress. The system quietly becomes a liar.

This is not a discipline problem. It is a structural problem. A priority label is a *cache* of a priority *judgment*. Like all caches, it goes stale. The question is whether you can eliminate the cache and compute priority from live inputs, or whether the cache is unavoidable and you need a refresh mechanism.

---

## Three Dimensions, Not One

Greg identified three distinct things people flatten into "priority":

**1. Technical dependency (structural, computed).** X must be done before Y. This is not a preference — it is a constraint. The graph encodes it. It is fully computable from `depends_on` edges. Violating it produces broken work.

**2. Contextual urgency (ephemeral, user-driven).** The user just finished Y1, found issue Y2, and wants Y2 handled now so they can get that area working. This is real and legitimate, but it is *situational*. It was not true yesterday and may not be true tomorrow. It is a human standing in front of a workbench saying "hand me that tool."

**3. System value (strategic, subjective).** M and N are both actionable. M is "more valuable" by some judgment about what the system needs. This is the hardest dimension because it requires understanding the system's goals, which live in a human's head and change slowly.

These three dimensions compose, but they are fundamentally different things:

- Dependency is a **hard constraint** — it eliminates options. It answers "what CAN be done" not "what SHOULD be done."
- Urgency is a **temporary override** — it says "right now, do this regardless of what the computed answer would be."
- Value is a **tiebreaker among equals** — when multiple things are actionable and no urgency applies, value determines order.

Any priority model that flattens all three into a single number (P0/P1/P2) loses this structure and forces constant manual reconciliation.

---

## What Can Be Computed, What Cannot

### Fully computable from the graph

- **Actionability.** A work is actionable when all its `must-complete-first` dependencies are satisfied and it is not shelved/finalized. This is a binary predicate, trivially computed.

- **Fan-out / unblocking power.** How many downstream works does completing X unblock? This is computable: count the works whose only unsatisfied dependency is X, or more subtly, count the total works transitively reachable from X that are currently blocked. Works that unblock more downstream work are structurally more important.

- **Critical path membership.** If you model the work graph as a project network, the critical path is the longest chain of dependent works. Works on the critical path have zero slack — any delay to them delays the overall portfolio. Works off the critical path can slip without affecting the total timeline. This is directly from CPM/PERT and is computable given the graph.

- **Depth / distance to leaves.** How far is this work from the "frontier" of final deliverables? Works close to root (foundations) need to happen early. Works at the leaves (polish, final integration) happen last. This is a topological property.

### Partially computable (needs input, but computation helps)

- **Area heat.** How many active works touch the same area? High-heat areas are coordination risks. This doesn't tell you which work to prioritize, but it tells you which *area* needs attention — and by extension, which works in that area are most entangled.

- **Staleness.** A work that has been "in progress" for a long time with no session activity is either stuck or abandoned. Staleness is computable from timestamps. What to do about it is not.

- **Chain position.** Greg described priority as a "chain" — X is priority, then Y, then Z. When the works form a linear chain (A blocks B blocks C), the chain ordering is trivially computed: work on the earliest unfinished member. When chains branch or merge, it gets more complex but is still graph-computable.

### Not computable — requires human judgment

- **System value.** The system cannot know that "parallelism support" is strategically more important than "better error messages" without being told. This is irreducibly subjective. Any priority model must have an input mechanism for this.

- **Contextual urgency.** The system cannot know that the user just hit a frustrating bug and wants that area fixed now. This is ephemeral human state. The model needs a way to accept it and a way to let it expire.

---

## Properties a Priority Model Needs

### 1. Dependency is treated as a filter, not a score

Dependency should remove options, not contribute to a priority number. A blocked work has no priority — it is simply not in the candidate set. Mixing "blocked" into a priority score (e.g., "this is P0 but blocked, so effectively P2") creates confusion. The correct structure:

```
all works → filter by actionable → rank the remaining
```

Filtering is structural. Ranking is where priority lives.

### 2. Computed rank shifts automatically as work completes

When work A completes and unblocks B, C, and D, those three should immediately appear in the actionable set with appropriate rankings. No human needs to "promote" them. The fan-out score of whatever was blocking them should propagate naturally.

This is the core property that static labels lack. In a graph-computed model, completing A changes the inputs to the ranking function for B, C, and D automatically. Their position in the ranked list shifts without anyone touching a label.

### 3. Human override is an input, not a label

When the user says "I want Y2 done next," that is not a relabeling. It is a signal. The model needs a place to accept this signal that is:

- **Scoped** — it applies to a specific work or a small set of works, not a global reordering.
- **Decaying** — it should have a natural expiration. If Y2 sits for three sessions without being worked, the urgency signal is probably stale. This could be explicit (TTL) or implicit (urgency is considered alongside recency).
- **Overriding but not destructive** — it changes the ranking temporarily without erasing the structural information. When the urgency passes, the structural ranking reasserts itself.

Concretely: a `pinned` or `urgent` marker on a work that pushes it to the top of the actionable list but does not change the underlying graph. With an optional timestamp so the system can note "this was pinned 5 sessions ago — is it still urgent?"

### 4. Value is a slow-changing weight, not a per-work label

P0/P1/P2 fails because it is applied per-work and needs constant updating. An alternative: value is applied at the *area* or *goal* level, and works inherit it.

If the user says "parallelism is the current strategic priority," that is a statement about a goal or area, not about individual works. All works contributing to parallelism inherit higher value weight. When the strategic priority shifts to "reliability," the weight shifts with it — and all parallelism works drop, all reliability works rise, without touching any individual work.

This is analogous to how Weighted Shortest Job First (WSJF) in SAFe uses "cost of delay" — a property of the value stream, not of the individual feature. The individual feature's priority is derived from the value stream it belongs to, combined with its size and risk.

The mechanism could be as simple as: areas (or goals, or themes) have a value weight. Works tagged with those areas inherit the weight. The ranking function considers inherited value alongside fan-out.

### 5. The ranking function must be transparent

Whatever produces the "work on this next" answer, the agent (and the user) must be able to see *why*. "Work on X because it unblocks 3 things and is in the highest-value area" is actionable. "Work on X because its priority score is 847" is not.

This argues against continuous numeric scores and toward a small number of discrete ranking factors that can be displayed as reasons:

- "Unblocks 3 downstream works" (fan-out)
- "On the critical path" (structural)
- "Pinned by user" (urgency override)
- "In high-value area: parallelism" (strategic value)
- "Oldest actionable work" (age as final tiebreaker)

The ranking is the *composition* of these factors, and the output shows which factors dominated.

---

## How the Three Dimensions Compose

The composition is not addition. It is a layered filter/rank:

```
Layer 0: All works
Layer 1: Filter — remove non-actionable (blocked, shelved, finalized)
Layer 2: Override — pinned/urgent works go to top (contextual urgency)
Layer 3: Rank — among remaining, sort by:
           a. Critical path membership (structural)
           b. Fan-out / unblocking power (structural)
           c. Inherited value weight from areas/goals (strategic)
           d. Age (tiebreaker)
```

This gives contextual urgency the ability to override everything (the user pinned it), but only when explicitly invoked. Without a pin, the ranking is fully computed from graph structure and value weights. When a pin expires or is cleared, the work falls back to its computed position.

This structure also means the three dimensions never conflict in an unresolvable way:
- Dependency is pre-filter, not in the ranking at all.
- Urgency is an explicit override with a clear "this was the human's call" signal.
- Value and structure compete within the ranking, but both are legible.

---

## The Chain Problem

Greg described priority as a chain: X is priority, then Y, then Z. This maps directly to dependency chains in the graph. When the chain is linear, the model handles it trivially — the earliest unfinished, unblocked member is the one to work.

The interesting cases:

### Parallel chains with no dependencies between them

Two independent chains: A1 → A2 → A3 and B1 → B2 → B3. Which chain do you start? This is where value weight matters. If chain A contributes to a higher-value area, start there. If both are equal value, fan-out doesn't help (both are equally deep). Age is the final tiebreaker, which is arbitrary but deterministic.

For multi-agent scenarios, parallel chains are an opportunity: assign one agent per chain. The model should make this visible: "These two chains are independent and can be worked in parallel."

### New urgent item preempting the current chain

The user finds bug Y2 while working chain Y. Y2 is not in the graph yet. The user creates it with an urgency pin. It jumps to the top. The current chain pauses.

The key question: when Y2 is done, does the agent return to the chain, or re-evaluate? It should re-evaluate. The act of doing Y2 may have changed the graph (new dependency, new area overlap, a downstream work is now unblocked). The ranking function runs again on the updated graph. The agent doesn't "resume the chain" — it asks "what's next?" and the answer might be the same chain, or might not.

This is the pull model. The agent finishes a unit of work, asks `kerf next`, gets the current best answer. No pre-planned schedule survives contact with reality.

### Multiple agents on different chains

Each agent finishes a work, asks `kerf next`, and gets the top-ranked actionable work. Two agents should not get the same work. This requires knowing what is "in-flight" — currently being worked by another agent.

The status field already models this (a work in "implementing" is in-flight). `kerf next` filters out in-flight works. Each agent gets the highest-ranked work that is not already claimed.

This is exactly a pull-based work queue with concurrent consumers. It is a well-understood pattern. The main risk is stale claims — an agent starts a work, crashes, the work is permanently "in-flight." This needs a heartbeat or timeout mechanism, but that is an execution concern (harmonik's domain), not a priority concern (kerf's domain).

---

## Pull vs. Push

The model described above is pull-based. Agents finish work, ask what's next, get an answer. This is superior to push for several reasons:

- **No stale schedules.** A push system pre-assigns work to agents. If priorities shift, the schedule is wrong. A pull system always gives the current best answer.
- **Natural load balancing.** Fast agents pull more work. Slow agents pull less. No manual assignment needed.
- **Resilience to disruption.** If an agent dies, its unclaimed work stays in the pool. Another agent picks it up.
- **Simplicity.** No scheduler, no assignment logic, no rebalancing. Just a ranking function and a "give me the next thing" interface.

The cost: no guaranteed parallelism. If two agents both finish at the same time, they might both want the same top-ranked work. This is a concurrency problem, not a priority problem, and is solvable with simple claiming (optimistic locking, status transition).

Push has one advantage: it can ensure that specific work is assigned to agents with relevant context. "This agent has been working on the adapter — give it the next adapter work." This is a form of locality/affinity optimization. It can be layered on top of pull: `kerf next --prefer-area adapter` or equivalent.

---

## Priority Decay and Refresh

If priority is computed from the graph, it refreshes automatically every time the graph changes (a work completes, a new work is added, a dependency is resolved). There is no decay problem for the computed components.

The urgency pin is the one element that can go stale. Options:

1. **Explicit removal.** The user pins a work as urgent, and later unpins it. Requires discipline (same problem as updating P0/P1/P2).
2. **TTL.** The pin expires after N sessions or N days. If it's still important, the user re-pins. This converts "updating labels" from "edit every work's priority" to "re-pin the one work that's still urgent," which is lower friction because it is exceptional, not routine.
3. **Session-scoped urgency.** The pin applies only to the current session. If the user starts a new session and doesn't re-pin, the urgency is gone. This is the most aggressive decay but matches the ephemeral nature of contextual urgency.

Value weights on areas/goals also need refresh, but they change slowly. A quarterly or milestone-based review of "what areas are strategically important" is sufficient. This is not the same burden as updating P0/P1 on individual works.

---

## The "What Changed" Problem

An agent enters a new session. The work graph may have changed since its last session — other agents completed work, the user added new works, priorities shifted. How does the agent orient?

If priority is computed, the agent simply asks `kerf next` and gets the current answer. It does not need to understand what changed — only what the current state is. This is a significant advantage over static labels, where the agent would need to understand the delta ("these P2s are now P1s because...").

However, for the user's benefit and for the orchestrator's judgment, a "what changed" view is still valuable:

- Works that became actionable since last session (dependencies resolved)
- Works that were pinned/unpinned
- New works added
- Area value weights that shifted

This is a diff against a snapshot, not a priority concern per se. But it supports the priority model by showing *why* the current ranking differs from what the agent might remember.

---

## Prior Art Assessment

### Critical Path Method (CPM)
Directly applicable. The critical path through the work graph identifies works where delay is most costly. Works on the critical path should rank higher. Works with float can slip. The limitation: CPM requires duration estimates, which are hard for creative work. But even without durations, the structure of the graph (which works are on the longest chain) provides useful signal.

### Theory of Constraints (TOC)
The "identify the bottleneck, subordinate everything else" principle maps to: find the work that is blocking the most downstream progress, and prioritize it. This is the fan-out metric. TOC's insight that the constraint moves (once you fix one bottleneck, another appears) is exactly the dynamic Greg described — once the core loop is done, parallelism becomes the constraint.

### Weighted Shortest Job First (WSJF / SAFe)
WSJF ranks by cost-of-delay divided by job size. Cost of delay captures urgency, value, and risk. The transferable insight: priority is a *ratio* of value to cost, not an absolute number. A high-value work that takes a long time might rank below a medium-value work that takes an hour. The limitation: estimating cost of delay and job size requires judgment that agents don't reliably have.

### Cost of Delay
The economic concept underneath WSJF. "What does it cost us per unit time to NOT have this done?" Works on the critical path have the highest cost of delay (they delay everything downstream). Works with no dependents have low cost of delay (nothing is waiting for them). This is computable from the graph without estimates — the number of transitively blocked works is a proxy for cost of delay.

### Topological Sort with Weights
The standard algorithmic answer. Topological sort gives a valid ordering. Weights (fan-out, value, age) break ties within topological equivalence classes. This is clean, well-understood, and directly implementable.

### Kanban Pull System
The agent-asks-for-next-work model is pure kanban pull. The ranking function is the priority lane. WIP limits prevent overcommitment. This is the execution model that best fits the computed priority approach.

---

## Failure Modes

### 1. Over-automation of value judgment
If the system computes everything, the user stops thinking about priorities. The system's structural ranking may be locally optimal (unblock the most things) but strategically wrong (the most-blocked chain is the least important one). The value weight input is the safety valve — but only if the user actually uses it.

**Mitigation:** Make the ranking transparent. Show the factors. Let the user see "this is top-ranked because it unblocks 5 things" and override with "yes but those 5 things are low value." The system learns from the override (the user is telling you about value).

### 2. Pin accumulation
Users pin things as urgent and forget to unpin. Over time, everything is pinned, and the pin mechanism is as useless as P0/P1/P2.

**Mitigation:** TTL on pins. Or limit the number of concurrent pins (WIP limit for urgency). If you can only have 1-2 pinned works, you have to choose.

### 3. Graph disconnection
If many works have no dependency relationships (isolated nodes), the graph provides no structural signal. Fan-out is zero everywhere. Critical path is meaningless. The ranking degrades to value weight + age, which is barely better than static labels.

**Mitigation:** Area tags provide implicit clustering even without explicit dependencies. Works in the same area are related even without edges. The system can suggest dependencies when area overlap is detected. But fundamentally, if the work graph is sparse, computed priority has less to work with.

### 4. Stale value weights
The user sets "parallelism" as the high-value area, then six months later it's done and the weight is still there. New works tagged "parallelism" get artificially boosted.

**Mitigation:** Value weights should be tied to active goals, not areas. When the goal is achieved (or its works are all complete), the weight naturally stops applying. Alternatively, periodic review prompts: "Your area value weights haven't been updated in N sessions."

### 5. Gaming the graph for priority
An agent (or user) adds artificial dependencies to boost a work's structural priority. "If I make everything depend on my work, it ranks highest."

**Mitigation:** This is probably not a realistic concern for a single-user, agent-assisted workflow. If it becomes one, it is a social problem, not a technical one.

---

## Shape of the Model

Pulling it together, the priority model has this shape:

**Inputs:**
- The work graph (works, statuses, dependency edges) — computed from spec.yaml
- Area tags on works — declared in spec.yaml
- Value weights on areas or goals — declared by the user, changing slowly
- Urgency pins — set by the user, decaying over time
- Completion events — when a work finishes, the graph updates

**Computation:**
- Filter to actionable (unblocked, not in-flight, not shelved/finalized)
- Urgency pins override ranking (but not filtering — a pinned blocked work is still blocked)
- Structural ranking: critical path membership, fan-out, chain depth
- Value ranking: inherited from area/goal weights
- Tiebreaker: age

**Output:**
- An ordered list of actionable works with reasons for their ranking
- Visibility into what is blocked and what would unblock it
- Visibility into what changed since the last query

**Properties:**
- No labels to maintain — ranking is recomputed on every query
- Completing a work automatically reshuffles the ranking
- Human input is via value weights (slow, strategic) and urgency pins (fast, tactical)
- The model degrades gracefully — with no value weights and no pins, you still get a structural ranking; with no dependencies, you still get value-based ranking; with neither, you get age-based ordering, which is at least deterministic

**What it replaces:**
- P0/P1/P2/P3 labels — replaced by computed rank with transparent factors
- Manual priority updating after completions — replaced by graph recomputation
- "What should I work on next?" as a question requiring human judgment every time — replaced by `kerf next` with a legible answer

**What it does NOT replace:**
- The human judgment that "parallelism matters more than polish" — this must be input as a value weight
- The ephemeral "I want this NOW" — this must be input as an urgency pin
- The understanding of why a work matters — the system ranks, the human decides whether the ranking is right
