# Critical Decisions — Confirm Before Spec Work

These are structural decisions that, if wrong, require significant spec rework. Each needs a confirm or revise.

---

### 1. Intent replaces "work" as the planning unit

**Question:** Do we rename/replace the current "work" concept with "Intent" — a reason-for-change that is explicitly NOT an execution unit?

**Current position:** Intent captures the "why." It produces designs and tasks but does not dictate execution grouping. A work item today conflates planning and execution; Intent separates them. Tasks from multiple intents can be batched together for execution.

**Why it matters:** Every kerf command, file structure, and jig artifact currently uses "work" as the organizing concept. This is the single biggest structural change.

**Confirm / Revise?**

---

### 2. Area is a first-class entity kerf manages

**Question:** Does kerf own and maintain a system map of Areas — named regions of the codebase — and use them for coherence checks and execution grouping?

**Current position:** Areas are how kerf answers "what else is happening here?" Planning uses areas to detect overlap between intents. Execution uses areas to group tasks into coherent batches. Areas are long-lived and evolve slowly.

**Why it matters:** If areas are first-class, kerf needs commands to define them, specs need to reference them, and coherence checks during design become a core feature. If areas are just tags, the coherence story gets much simpler but weaker.

**Confirm / Revise?**

---

### 3. Finding is a first-class entity with a structured ingestion path

**Question:** Does kerf provide a structured way for downstream agents (EXEC, MERGE/TEST) to record findings that flow back into the planning cycle?

**Current position:** Findings are the feedback loop mechanism. They enter kerf with severity, affected areas, and origin info. They become new intents after triage. High-severity findings get priority in the queue.

**Why it matters:** This is what doc 22 calls "the single most important thing kerf needs that it doesn't have." Without it, feedback stays in HANDOFF docs and gets lost. With it, kerf needs a `kerf finding` command (or similar), a triage workflow, and the queue must incorporate finding-derived intents at higher priority.

**Confirm / Revise?**

---

### 4. Queue is a computed view, not stored state

**Question:** Is `kerf next` a live computation over current task states, dependencies, and priority signals — rather than a stored, manually-ordered list?

**Current position:** The queue is computed every time it's asked for. It factors in: dependency satisfaction (what's unblocked), area focus (finish what's started), and priority signals (rework before new work). No static P0/P1 labels.

**Why it matters:** A computed queue means kerf must have read access to bead status (from bd) to know what's complete/blocked. This defines the kerf-beads integration boundary. A stored queue would be simpler but stale.

**Confirm / Revise?**

---

### 5. Design freeze is the commitment boundary

**Question:** Once tasks are generated from a design, is the design frozen — with further changes requiring a new intent rather than editing the existing design?

**Current position:** Designs go `drafting -> coherent -> sufficient -> frozen`. After frozen, the design is a historical record. If reality changes, that's a new intent with a new design that references the old one.

**Why it matters:** This is the immutability guarantee that makes traceability work. But it also means late-arriving requirements that overlap with in-flight work MUST become separate intents — they can't be folded into the existing design. If this is too rigid, the late-requirement handling (Problem 4 from the plan) gets harder.

**Confirm / Revise?**

---

### 6. kerf reads bead status but does not own it

**Question:** Does kerf query the beads system (bd) for task completion status, while bd remains the owner of execution state?

**Current position:** kerf generates task definitions that become beads. bd tracks execution state (pending, in-progress, complete, failed). kerf reads bd's state to compute the queue and determine intent absorption. Neither system owns the other's data.

**Why it matters:** This is the integration seam. If kerf reads from bd, they need a stable interface. If kerf duplicates bead state, they'll drift. Greg said in doc 26 that during execution there will be review/test phases — those need to write status somewhere. The question is whether that's bd, kerf, or both.

**Confirm / Revise?**

---

### 7. ALLOCATE and MERGE/TEST collapse into one agent (for now)

**Question:** For the near term (without harmonik), do ALLOCATE and MERGE/TEST run as one persistent agent that reads `kerf next`, dispatches sub-agents for EXEC, and handles merging/testing?

**Current position:** Greg said in doc 26: "most likely the ALLOCATE and MERGE are done in the same agent. EXEC, TEST would be done by subagents." This combined agent is the primary consumer of kerf's computed queue.

**Why it matters:** This affects what kerf needs to surface. If one agent does both allocation and merge/test, kerf doesn't need to coordinate between them — it just needs `kerf next` to be correct and `kerf finding` (or equivalent) to accept feedback. The specs should model the four logical roles but design the interface for this collapsed reality.

**Confirm / Revise?**

---

### 8. Rework priority is structural, not labeled

**Question:** When rework (from findings) and new work compete for execution, does rework win by default — and is this encoded in the queue computation rather than as a manual priority label?

**Current position:** Tasks born from findings are structurally prioritized over tasks born from new intents. "Finish what's started" and "fix what's broken" both outrank "start something new." Greg confirmed in doc 26: "downstream issues/rework should be prioritized over accepting new tasks coming from upstream."

**Why it matters:** This determines the `kerf next` algorithm's core ranking logic. If rework priority is structural (baked into queue computation), kerf must track task origin (finding vs. new intent). If it's manual, the agent/user sets it and kerf just sorts.

**Confirm / Revise?**

---

### 9. Batch is ephemeral — kerf does not track dispatch history

**Question:** Are batches (groups of tasks dispatched together) transient — assembled, dispatched, forgotten — or does kerf need to remember what was dispatched together?

**Current position:** Doc 25 left this open. Greg's doc 26 response didn't resolve it directly but said: "when there are 5 tasks that need to get done, I want to make sure nothing gets lost." This suggests the concern is task completeness tracking (no stranded tasks), not batch history.

**Why it matters:** If batches are ephemeral, kerf stays simpler — it just computes what's next. If batches are durable, kerf needs a batch entity with lifecycle tracking. The "nothing gets lost" concern might be solvable through intent absorption tracking (an intent isn't absorbed until ALL its tasks complete) rather than batch tracking.

**Confirm / Revise?**

