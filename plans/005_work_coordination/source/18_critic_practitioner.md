# Practitioner-Critic Review of Deep Dives 11-16

> Perspective: someone who has managed multi-agent projects and knows the gap
> between elegant designs and what agents actually do at 2am on session 14.

---

## 11 — The Factory Line

### Would agents actually follow this?

The factory line is a mental model for humans, not a protocol for agents. And that's fine — it doesn't claim to be agent-executable. The danger is that someone tries to encode it as one. An agent cannot reason about "which station am I at?" across a 7-station pipeline. It can follow one step at a time.

The model is useful as a *design constraint* for the kerf CLI. Each kerf command maps to a station. The agent doesn't need to understand the manufacturing metaphor — it just needs to run `kerf new`, then its jig passes, then `kerf decompose`, then `kerf next`. The factory line is the why behind the command design, not something agents read.

**Failure mode:** If this gets written into agent-facing documentation ("you are at Station 3, the Decompose station"), agents will dutifully repeat this language back without it affecting their behavior. It's cargo-cult process.

### Would Greg actually use this?

Yes, as a thinking tool. The fan-out/fan-in diagram and the feedback loop taxonomy are genuinely useful for reasoning about where coordination breaks down. Greg thinks visually and architecturally. This gives him a vocabulary.

No, as something to maintain. There is no artifact to maintain here — it's a design document. That's the right form.

### What breaks first?

The model assumes works move cleanly between stations. In practice, works get stuck in liminal states. A work that's "mostly designed but the spec has a known gap" isn't at Design or Decompose. A work where 11 of 12 beads are done but the 12th is blocked by another work — is it at Execute or blocked at Queue? The clean station boundaries are messier in practice than on paper.

### Adoption path

This is already adopted — it's a thinking document. The value is in the specific station-to-command mapping at the end. That's what becomes concrete spec language.

### What to cut

Keep: The station list, the feedback loops, the fan-out/fan-in diagram. These are the load-bearing ideas.

Cut: The formal station graph in section "The Station Graph (Formal View)" and the invariants. These read well but won't be referenced during implementation. The information is already in the station descriptions.

### What's missing

**The cost of crossing station boundaries.** Every station transition requires context assembly — reading specs, loading bead state, checking areas. In a real session, the agent might cross 2-3 boundaries. Each crossing costs context window budget. The model doesn't account for the fact that agents have finite context and every "check the portfolio state" step competes with "actually do the work" for that budget.

---

## 12 — Dynamic Priority

### Would agents actually follow this?

Agents don't need to understand the priority model. They call `kerf next` and get an answer. This is well-designed — the complexity is in the computation, not in the agent protocol. The agent's job is trivial: "call this command, use the output."

The one place agents interact with priority is urgency pins. An agent discovering an urgent issue would need to run something like `kerf pin <work>`. This is simple enough to work.

### Would Greg actually use this?

The three-dimension decomposition (dependency, urgency, value) is the best idea in all six documents. It captures exactly what Greg described as broken about P0/P1/P2. The separation into "filter by actionable, override by pin, rank by structure + value" is clean and matches how he actually thinks about priority.

Value weights on areas/goals is elegant in theory. In practice: will Greg actually set and maintain area-level value weights? Probably not initially. But the system degrades gracefully to structural ranking if he doesn't, which is the right design.

**One concern:** Greg said he wants agents to work autonomously. Step 5 in the session-start protocol says "Do NOT auto-select. Present the ranked list to the user." This is the right default for now but needs an autonomous mode for harmonik-driven execution. The document acknowledges this with the automated pipeline exception, which is good.

### What breaks first?

**Graph sparsity.** If works don't have dependency edges, the structural ranking is meaningless. Fan-out is zero everywhere. The system degrades to value weights + age, which is barely better than a flat list. The document acknowledges this in the failure modes section, but the mitigation ("area tags provide implicit clustering") is hand-wavy. Area overlap is not the same as dependency. Two works in the same area aren't necessarily ordered.

Real-world prediction: in early use, Greg will create 5-8 works with minimal dependency edges. `kerf next` will return them in creation-order because nothing else differentiates them. He'll add a pin, forget to remove it, and be annoyed. Then he'll stop using `kerf next` and go back to manually picking.

**Mitigation the document doesn't discuss:** Guide the user toward adding dependency edges. When `kerf next` can't differentiate, it should say why: "3 works are equally ranked because none have dependency relationships. Consider adding `depends_on` edges." Make the gap visible instead of producing an arbitrary ranking.

### Adoption path

Start with: actionability filtering (blocked vs. unblocked) + urgency pins. No value weights, no fan-out computation, no critical path. Just "what's unblocked?" and "what did the user pin?" This is immediately useful and requires zero graph structure.

Add fan-out later, when the graph is dense enough to make it meaningful.

### What to cut

Cut: WSJF/SAFe/CPM prior art discussion. It's intellectually honest but adds nothing actionable. Greg isn't going to read a paragraph about Weighted Shortest Job First and decide to use it.

Cut: Value weights on areas. Not because it's a bad idea, but because it's premature. When the system has 30+ works across multiple areas and the simpler ranking isn't enough, revisit.

Keep: The three-dimension decomposition. The layered filter/override/rank model. Pin decay via TTL. The pull-based model. These are the load-bearing ideas.

### What's missing

**What does the output of `kerf next` actually look like for the agent?** The beads integration doc (13) has a JSON format. This doc doesn't. The agent needs a concrete, parseable answer, not a philosophical ranking. Nail the output format early.

---

## 13 — Beads Integration

### Would agents actually follow this?

This is the most grounded of the six documents. The YAML intermediate representation is proven (harmonik used it at scale). The pipeline stages are clear. The label convention (`work:<id>`) is mechanical and agents can follow it.

**The risk is in the decomposition step.** The agent producing the task YAML needs to hold the full spec in context, understand the YAML schema, know the mnemonic conventions, and produce correct cross-work edges. This worked in harmonik with human review rounds. Will it work with less supervision?

The YAML schema is reasonable but verbose. An agent producing 50 beads with cross-work edges is doing significant creative work. If the output has errors (missing edges, wrong mnemonics, cycles), who catches them? The validation step is critical and should be non-negotiable.

### Would Greg actually use this?

Yes. Greg specifically asked for this. The harmonik experience demonstrated the pattern works. The question is friction: how many commands does Greg (or his orchestrator agent) have to run to go from spec to loaded beads?

The proposed pipeline: spec -> agent produces YAML -> kerf validates -> loader ingests. That's 3-4 steps, some manual. If `kerf decompose` wraps all of this into one command, friction is low. If Greg has to manually run the loader, manually check mnem-maps, manually reconcile forward-deferred edges — he won't do it consistently.

### What breaks first?

**Cross-work edge resolution.** This is always where it gets messy. Forward-deferred edges sound clean ("log it now, materialize later") but in practice:
- Who runs the reconciliation pass? When?
- What if work B's actual bead IDs don't match what work A guessed?
- What if work B never gets decomposed and the forward edge just sits there forever?

In harmonik, a human ran the reconciliation. In an autonomous system, this needs to be automatic — and the document doesn't fully specify how.

**The other likely failure:** `br list -l work:X --json` assumes beads has a working JSON output mode with label filtering. If it doesn't, or if the output format changes, every kerf command that queries beads breaks. This integration point needs a contract test, not just a convention.

### Adoption path

1. Define the YAML schema. Ship it as a spec document, not code.
2. Implement `work:<id>` label convention manually for one project. Verify the queries work.
3. Build `kerf map` with bead status queries.
4. Build `kerf decompose` last — it's the most complex and depends on everything else working.

### What to cut

Keep everything. This document is the most implementation-ready of the six and has the least fat. If anything, it could be expanded with concrete error handling for the loader and a specification of the `br` output format kerf depends on.

### What's missing

**Error recovery during loading.** What happens when the loader fails halfway? Are 30 of 50 beads created? Can you re-run safely? The document mentions idempotent re-runs via mnem-map, but the failure-mid-load scenario needs more thought. Partial loads create a database state that doesn't match the YAML, which is exactly the kind of inconsistency this system is designed to prevent.

---

## 14 — Areas as Graph

### Would agents actually follow this?

Agents would use the defined taxonomy (can't use areas not in `areas.yaml` — this is enforced by kerf). That's the easy part and it works. Agents adding new areas via `kerf areas add` is feasible. Agents maintaining edge relationships between areas is not — they don't understand architecture well enough to declare "api calls auth."

**The practical reality:** Greg defines the initial area graph. Agents add leaf nodes when they encounter new sub-areas. Nobody maintains the edges because edges require architectural judgment that agents don't have. After 20 sessions, the nodes are reasonably accurate and the edges are stale or incomplete.

### Would Greg actually use this?

The defined taxonomy with validation — yes, absolutely. This prevents the drift Greg hates (different agents using different names for the same thing). It's a one-time setup cost with ongoing enforcement. Good leverage.

The directed graph with typed edges — probably not in practice. Greg would need to define `calls`, `reads`, `owns` relationships for his whole system architecture. This is the kind of modeling exercise that's satisfying to do once and then never updated. The adjacency queries and blast radius computations are cool demos but require a maintained graph to be useful.

**The hierarchy (parent/child) is the sweet spot.** `adapter` owns `adapter.retry` and `adapter.pool`. This is easy to set up, easy to extend, and enables the grouping in `kerf map` that Greg would actually look at. The edge types can wait.

### What breaks first?

The edges. Specifically, the edges go stale because:
1. Nobody remembers to update them when the architecture changes.
2. Agents can't reliably determine edge correctness.
3. The queries that use edges (blast radius, adjacency warnings) produce wrong results from stale edges, eroding trust in the system.

The document's "medium-plus" strategy hedges this correctly: capture edges in the schema but don't build commands that depend on them yet. This is wise. But there's a temptation to build the cool graph queries early because they demo well. Resist.

### Adoption path

1. Flat list of area names with validation. No hierarchy, no edges. Just `areas.yaml` with names and descriptions, enforced by `kerf new`.
2. Add parent/child hierarchy. This is a small schema change with immediate value for `kerf map` grouping.
3. Stop. Don't add edges until there are 3+ real instances where edge-based queries would have prevented a problem. Build from demonstrated need, not anticipated coolness.

### What to cut

Cut: Edge types (`calls`, `reads`, `owns`). The schema can include them for forward-compatibility, but don't build any queries or commands against them.

Cut: `kerf areas impact`. Blast radius analysis is a v3 feature at best. Nobody will use it until the area graph is stable and well-maintained, which won't happen until after extensive real-world use.

Cut: The C4/DDD prior art section. Informative but not actionable.

Keep: Defined taxonomy with validation. Hierarchy with parent. The "error when unknown area" enforcement. The `kerf map` grouping by area.

### What's missing

**How agents discover which areas a work touches.** The document assumes agents will correctly tag works with areas from the defined set. In practice, an agent creating a work needs to scan the area list, understand each area's meaning, and pick the right ones. With 20+ areas, this is a non-trivial classification task. Consider having `kerf new` show the area list and ask the agent to pick, rather than expecting the agent to remember the taxonomy.

---

## 15 — Agent Protocols

### Would agents actually follow this?

This is the most carefully designed document of the six for agent executability. The numbered steps, preconditions, branches, and escalation points are exactly what agents need. The author clearly understands that agents need decision trees, not heuristics.

**However:** The protocols are long. The session-start protocol is 9 steps. The co-design protocol is 5 steps with sub-branches. The late-requirement protocol is 6 steps with a 3x3 decision matrix. An agent that reads all of these at session start is spending 20-30% of its context window on process before doing any work.

The critical insight in this document — embedding protocols in command output rather than separate files — partially addresses this. If `kerf resume` outputs the relevant protocol steps inline, the agent doesn't need to remember to read them. But the command output will be long.

**The bigger concern:** The co-design protocol (Section 3) asks agents to do something genuinely hard — read two works' artifacts, identify shared interfaces, check for contradictions, and reconcile. Steps 4 and 5 require architectural reasoning that current LLMs do inconsistently. An agent might declare "no conflicts found" because it didn't look hard enough, or flag a "deep incompatibility" because it misread a type signature. The protocol is well-designed for an agent that can reliably compare interface definitions. Current agents are unreliable at this.

**Graceful degradation (Section 8) is the key insight.** "Every protocol should produce value even if only half the steps are followed." This is the right design principle. The session-end protocol is useful even if agents only write COMPLETED and NEXT. The co-design protocol is useful even if agents only read peer specs without formal reconciliation. Design for 60% compliance and be pleasantly surprised by 80%.

### Would Greg actually use this?

The session-end protocol — yes. It directly replaces the HANDOFF that Greg knows is broken. The structured format (COMPLETED/NEXT/DISCOVERED/BLOCKED) is faster to write than a narrative and more useful to read. Greg would set this up and benefit immediately.

The co-design protocol — only if it's automated. Greg won't manually check whether agents ran the co-design check. If `kerf square` detects skipped co-design checks (as proposed), that's useful. If it requires Greg to review SESSION.md entries, he won't.

The late-requirement protocol — the stage x depth matrix is brilliant. This is the kind of decision tree that turns a 30-minute "what should I do?" conversation into a 30-second lookup. Greg would reference this himself, not just agents.

The disruption protocol — too complex for initial deployment. 6 steps with multiple branches and cross-work notification. In practice, agents will either handle disruptions locally (which they already do) or escalate to Greg (which they already do). The protocol formalizes what happens naturally. Formalize it later when you have evidence that the natural behavior is insufficient.

### What breaks first?

**SESSION.md as coordination mechanism.** The protocols write to SESSION.md. The co-design protocol writes to the peer work's SESSION.md. The disruption protocol adds "External Update" entries to affected works' SESSION.md files. This means SESSION.md is both a session log and an inter-agent communication channel.

This is a collision of concerns. Agent-mail already exists for inter-agent communication. Using SESSION.md for this purpose means the next agent has to scan SESSION.md for entries it didn't write, from agents working on other works, about events it wasn't involved in. This gets noisy fast.

**Prediction:** After 10 sessions touching 5 works, SESSION.md files will have a mix of session records, co-design check records, and external updates from other works' disruptions. The structured format helps, but the volume will be a problem. The 5-entry trim rule (Session-End Step 3) will discard external updates that were written between sessions, losing cross-work coordination data.

### Adoption path

1. Session-end protocol with COMPLETED/NEXT/DISCOVERED/BLOCKED. This is standalone, immediately useful, and requires no other infrastructure.
2. Session-start protocol with `kerf map` + `kerf resume`. Depends on those commands existing.
3. Late-requirement protocol with the stage x depth matrix. Can be embedded in `kerf new` overlap warnings.
4. Everything else later.

### What to cut

Cut: The disruption protocol (Section 6). It's comprehensive but premature. Agents handle disruptions ad hoc and it's mostly fine. Formalize when there's evidence of systematic failure.

Cut: Writing to peer works' SESSION.md (cross-work signaling). Use agent-mail or a dedicated coordination artifact instead.

Cut: Protocol evolution meta-protocol (Section 7). Important conceptually but it's describing what you'd do naturally: observe agents, fix the protocols. Documenting this process doesn't make it happen better.

Keep: Session-start and session-end protocols. Co-design protocol structure (but simplify the reconciliation steps). Late-requirement stage x depth matrix. The principle of embedding protocols in command output. Graceful degradation design.

### What's missing

**Context budget accounting.** How much context does following all applicable protocols consume? If an agent starts a session on a work with co-design relationships:
- `kerf map` output: ~100 lines
- `kerf resume` output + embedded protocol: ~50 lines
- Co-design protocol + peer artifact reading: ~200-500 lines per peer
- SESSION.md history: ~100-250 lines
- The actual spec for the work: ~200-500 lines

That's potentially 650-1400 lines of context before the agent writes a single line of code. On a 200k-token model, that's manageable. On a 100k-token model with a complex implementation task, it's a significant chunk. The protocols should have a "light mode" for sessions where context is at a premium.

---

## 16 — Session Continuity

### Would agents actually follow this?

The distinction between computed state and stored context is the single most important idea across all six documents. "Information that can be recomputed from authoritative sources should never be stored in a handoff document." This is correct, actionable, and would prevent the majority of HANDOFF degradation Greg described.

Agents would follow `kerf orient` trivially — it's a command they run. The output is computed; the agent just reads it.

Agents writing structured session records is where compliance drops. The YAML format is clear, but agents will:
- Write vague decisions ("decided to use the standard approach")
- Omit warnings because they don't recognize what's warning-worthy
- Leave the `unfinished` section empty because "nothing is unfinished, I just ran out of context"
- Write correct `discoveries` entries — this is the section agents are naturally good at, because surprising things are salient

**Partial compliance is still valuable.** Even a session record with only `works_touched` and `decisions` (and the decisions are vague) is better than a HANDOFF that's been through 5 rounds of summarization.

### Would Greg actually use this?

Absolutely yes. This directly solves a problem Greg described in detail and is angry about. The telephone game analysis (Section 1) is a precise diagnosis of what went wrong. The computed-orientation-plus-immutable-log solution is the right architecture.

**Greg would need to resist the urge to "just quickly edit" a past session record.** The immutability property is critical but fragile — one agent that rewrites old records breaks the guarantee. This should be enforced by tooling (write-once file permissions, or kerf refusing to modify existing session files).

### What breaks first?

**The relevance filtering.** The `works_touched` field is the join key for filtering session records by work. If an agent works on `brave-falcon` but also makes a discovery about `adapter.pool` that affects `green-oak`, the record gets tagged `works_touched: [brave-falcon]`. When someone orients on `green-oak`, they miss the discovery.

The area-based fallback ("show records tagged with the same areas as the current work") partially addresses this, but it depends on works having accurate area tags. And it can over-include: if `adapter` is a popular area, every session record that touched any adapter work shows up for every other adapter work.

**Prediction:** Within 20 sessions, the relevance filtering will either be too narrow (missing important cross-work context) or too broad (showing too many records). The tiered detail mechanism helps with volume, but it can't help if the wrong records are selected.

**Mitigation:** Let agents tag individual discoveries/warnings with areas, not just the session as a whole. A discovery about `adapter.pool` gets `area: adapter.pool` even if the session's `works_touched` doesn't include that work. This is more tagging work for the agent but dramatically improves filtering.

### Adoption path

1. `kerf orient` that computes portfolio state from spec.yaml + bead status. This is immediately useful even without session records. It replaces the 60-70% of HANDOFF content that's computable.
2. Session records with just `works_touched` + `decisions` + `warnings`. Skip `discoveries` and `unfinished` initially. Two fields is easier for agents to comply with than four.
3. Add relevance filtering once there are enough records to need it (10+ sessions).
4. Add resolution marking once there are stale warnings (20+ sessions).

### What to cut

Cut: Resolution marking (Section 5.3). It's elegant but adds complexity to a system that doesn't exist yet. Stale warnings are a problem you can solve later.

Cut: The tiered detail mechanism (Section 5.2). Premature optimization. When you have 50 session records and filtering isn't enough, design the tiers based on what you've observed agents actually write — not what you predict they'll write.

Keep: The core insight (compute vs. store). `kerf orient`. Immutable append-only session records. The structured format. The separation of concerns (status is computed, context is stored).

### What's missing

**What happens when `kerf orient` is wrong?** The computed state depends on spec.yaml status fields and bead state being accurate. If an agent finished a work but didn't update spec.yaml status, `kerf orient` will show it as in-progress. If bead state is stale (agent completed work outside the beads system), the bead count will be wrong. Computed state is only as good as its inputs.

The document implicitly assumes all state transitions go through kerf/beads commands that update the authoritative sources. In practice, agents sometimes do things manually (edit a file, fix a bug, move on without updating status). A "computed, never stale" view that's based on stale inputs is confidently wrong — arguably worse than a HANDOFF that the agent knows to distrust.

**Mitigation:** `kerf orient` should surface uncertainty. "Last spec.yaml update: 3 days ago" or "bead status query failed" makes staleness visible instead of hiding it behind a computed facade.

---

## Cross-Cutting Assessment

### If you could only build three things

1. **`kerf orient` (computed session orientation).** Replaces HANDOFF for status. Immediately useful. Forces the right architecture (compute, don't cache). From doc 16.

2. **Structured session records (append-only, immutable).** Replaces HANDOFF for context. Prevents telephone game. The simplest version is just COMPLETED/NEXT/DISCOVERED/BLOCKED appended to a file. From docs 15 + 16.

3. **Area taxonomy with validation.** Flat list of area names in `areas.yaml`, enforced by `kerf new`. Prevents naming drift. Enables overlap detection. No hierarchy, no edges, no graph queries. From doc 14.

These three things are independent, each solves a real problem Greg described, and each can be built and validated in isolation.

### What's the biggest risk across all six documents?

**Complexity budget exhaustion.** Each document is individually reasonable. Together, they describe a system with: 7 stations, 3 priority dimensions, a YAML schema for task decomposition, a graph database for system areas, 5 structured protocols, and a session log with tiered filtering and resolution marking. An agent trying to operate in this system would need to understand all of it.

The documents acknowledge this risk in places (graceful degradation, protocols embedded in command output). But the combined system has a surface area that exceeds what a single developer — let alone a single agent session — can hold in working memory.

**The discipline required:** Build the simplest version of each thing. Ship it. Watch agents use it (or fail to use it). Build the next increment based on observed failure, not anticipated need. Most of the sophistication in these documents (edge-typed area graphs, critical-path priority computation, cross-work co-design reconciliation, resolution marking on session entries) should be deferred until the simpler version proves insufficient.

### What none of the documents address

**Testing and validation of the coordination system itself.** How do you know `kerf next` is producing useful rankings? How do you know the co-design protocol is catching real conflicts? How do you know the session records are being written with useful content? There's no observability story for the coordination layer.

You'll build all of this, agents will use it, and you'll have no way to tell whether it's actually improving outcomes versus adding overhead — unless you deliberately instrument the coordination system and look at the data. Which sessions used `kerf orient`? Did agents that used it make better decisions than agents that didn't? Did the co-design check catch any real conflicts, or was it always "no conflicts found"?

Without this feedback loop, you'll either over-invest in coordination (building features nobody uses) or under-invest (not knowing that agents are silently ignoring critical steps). The protocols are designed for graceful degradation, which is good — but graceful degradation also means silent failure, which means you don't learn.

**Suggestion:** Add a simple coordination log — every time kerf runs a coordination command (orient, next, areas check, co-design check), log it with the outcome. Periodically review: are these commands being used? Are they producing useful results? Are agents acting on the output?
