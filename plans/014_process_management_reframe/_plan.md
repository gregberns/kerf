# Plan 014 — Process-Management Reframe

> **Status: REFRAME + v1 MVP slice.** No code or spec changes have landed yet. The reframe (weights are an internal control surface; users describe their work, not their weights) stays. Following the 2026-05-19 independent review, the v1 slice cut from 11 beads to 3. See "2026-05-19 revision" immediately below for what changed; sections further down preserve the original reasoning.

## 2026-05-19 revision (post independent-review)

The independent review (`critiques/independent_review.md`) returned **proceed-with-changes, cut roughly in half**. Accepted in full. Concretely:

**v1 ships as 3 beads, one wave (see `beads.md`):**
1. **B1** — `maturity` field at project level (`experimental` / `stable` / `frozen`), enum confirmed by user.
2. **B2** — T=0 analyzer skeleton; graph-shape signals append to `kerf next`'s existing `reason` field (no new command). Reusing the existing surface is the explicit antidote to the "two tools that do similar things" concern.
3. **B3** — Collision-tolerance refactor in `internal/queue/`, driven by `maturity`.

**Spun out into follow-on plans (not in 014):**
- **Plan 014b** (`plans/014b_adaptive_scheduler/`) — T>0 adaptive scheduler (was B9) + derived-weights joiner (was B4) + kerfsim symmetry (was B6/B11). Gated on Plan 013 closing.
- **Plan 014c** (`plans/014c_deferred_queue/`) — Deferred-queue data model + UX (was B5/B10). Orthogonal to weight derivation; its own user-flow design.
- **Plan 014d** (`plans/014d_advise_surface/`) — `kerf advise --explain` surface (was B7). "Open until justified" — only if B2's reuse of `kerf next.reason` proves insufficient after dogfooding.

**Dropped from v1 entirely (see "deferred fields" below for the preserved enums):**
- `work_shape` bead field — overlaps existing `rework:` / `finding:` label vocabulary in `specs/coordination.md` §79. Derive from labels if needed later; do not declare.
- `correctness` bead field — aspirational; will drift to `untested` or get set to `verified` once and never revisited. If real, derive from CI / test presence.
- Per-work override of `maturity` — project-level alone covers the 80% case (review §2).
- Named profile surface (`throughput` / `balanced` / `safety`) as a user-visible noun — was B8. v1 commits to a single user-facing noun: `maturity`. Profile framing is deferred until B2 proves the graph signal is actually useful (review §3, §6).
- A new `kerf advise` command — deferred to 014d behind a justification gate (review §1).

**Why one user-facing noun?** Both critique A and the independent review flagged that the original plan shipped *two* declarative surfaces simultaneously (B1 declarative inputs + B8 profile surface). That is the worst outcome — users see two parallel naming axes. v1 picks `maturity` because it is the only field in the original three that "pulls its weight" (review §2) and because it composes cleanly with existing config patterns.

### Deferred fields (preserved from user-confirmed enums)

The user confirmed the following enum values on 2026-05-19. They are preserved here so 014b–d can pick them up without re-litigating:

- `maturity` — `experimental` / `stable` / `frozen` *(SHIPS in B1)*.
- `correctness` — `untested` / `tested` / `verified` *(deferred; derive from CI/test presence if needed; do not ship as a static enum)*.
- `work_shape` — `feature` / `bug` / `refactor` / `spike` / `infra` *(deferred; derive from existing `rework:` / `finding:` labels in `specs/coordination.md` §79 if needed)*.
- Profile names — `throughput` / `balanced` / `safety` *(deferred to a possible 014d follow-on; v1 has no profile surface)*.

### What the revision preserves unchanged

- The reframe itself (weights are internal; users describe work, not weights).
- The T=0 / T>0 two-layer architecture as a guide for follow-ons.
- The "kerf is the process-management layer" framing.
- All decisions in the "Critique reconciliation (2026-05-19)" section further below remain valid for v1's smaller scope.

---

## 2026-05-19 freshness check

### 2026-05-19 freshness check (pre-revision context, preserved)

- **Still relevant — the core reframe and the T=0 / T>0 layering.** Nothing shipped since 2026-05-15 has implemented declarative inputs, T=0 static analysis, derived weights, deferred queue, profile surface, or collision-tolerance refactor. The scheduler in `internal/queue` still uses one global weight vector (`specs/coordination.md` §165–180, `momentum` default 5.0).
- **Outdated — the "likely follow-on plans" numbering.** Plans 015–020 shipped, but for the harmonik-beta-feedback track (triage / init / storage / filter / jigs), NOT for the items 014 sketched. The reframe's own follow-ons need fresh numbers (≥ 025) and to absorb the `kerf next` ranking + entry items routed in from Plan 015 (the harmonik beta-feedback triage) Theme 3.
- **Plan 013 (self-diagnostics) is concurrently being re-planned** in a sibling agent; it remains 014's named feed for the T>0 adaptive layer. Disjoint scope: 013 owns transcript-extraction and detector surfaces; 014 owns how those signals enter the queue's weight derivation.
- **New input absorbed from Plan 015:** `kerf next` ranking-and-entry items (Theme 3 of `plans/015_harmonik_beta_feedback/triage.md`) explicitly route to 014. Plan 019 ships the payload-ordering and rank-label vocabulary (`empty` / `unwired` / `broken`); 014 still owns the algorithm itself and the new "later" bucket.
- **No conflicts** with Plans 022 / 023 / 024 (test harness / property contracts / CI) — they are testing infrastructure, orthogonal to scheduler architecture.

## Intent

- Record what the weight-tuning track (Plan 011 — simulator validation, pillar D) actually taught us, which is **not** "here is the winning weight set."
- Reframe kerf's scheduler from "tune one global weight vector" to **"derive weights from declarative inputs the agent already knows."**
- Establish that kerf's job is the **process-management layer** the agent reasons about (what to work on next, queue health, when to merge, what to defer) — not "help do the work."
- Surface open questions so follow-on plans can pick them up; do not answer them here.

## Background — what triggered the reframe

- **Plan 007 — simulator** shipped the engine.
- **Plan 008 — exploratory testing** smoke-tested seven synthetic scenarios; flagged rework-metric brittleness and under-saturation.
- **Plan 011 — simulator validation** closed the learning loop: concurrency sweep, adversarial runs, saturated re-runs, then weight tuning.
- **Plan 012 — real-workload corpus** delivered eleven real-data scenarios (eight harmonik pilots, three kerf plans) with fitted phase durations.
- **Plan 013 — self-diagnostics from Claude transcripts** is in flight; it surfaces procedural issues live from session logs.

**Plan 011 / D produced null results twice.**

- v1 sweep (synthetic corpus): no weight combo dominated the default. Rework path under-exercised — only one scenario ever moved the metric.
- v2 sweep (saturated all_pilots scenario, `momentum × rework × fan_out` grid; see `plans/011_sim_validation/weight_tuning/weight_tuning_v2_report.md`): again no winner under the 60%-dominance / 5%-loss-cap decision rule. The interesting findings were *systemic*, not score-based.

### Systemic findings from v2

1. **`momentum` is structurally too small to compete.** Math is right (`internal/queue/queue.go:110-117`) — the term is `(beads_complete / total_beads) * weights.Momentum`. At momentum ≤ 10 it almost never flips the top-of-queue rank against a rework term of 15+. This is a tuning-ceiling / structural-balance issue, not a bug. Raising momentum into a competitive range is possible but not the right answer in isolation — see reframe below.
2. **Rework was empirically dead** in v1's corpus; only the v2 saturated scenario exercised it enough to register. v2 also surfaced a +27% throughput combo that trips the area-collisions guardrail.
3. **Real-world signal corroborates (1).** In harmonik, the coding agent has been observed taking on too many parallel works rather than finishing existing ones — exactly what weak momentum looks like in the wild. After a big push, cleanup / bug-fix / polish tasks pile up unfinished while the agent fans out into new feature work. Not theoretical; user pain.

## The reframe

### "Find one winning weight set" was the wrong goal

- Optimal weights depend on what the *user* (often a coding agent like Claude) is trying to accomplish:
  - shipping new code vs. polishing existing
  - correctness-critical vs. experimental
  - tight dependency chains vs. independent parallel works
- No single weight vector serves all three axes simultaneously. The two null sweeps are evidence of that, not failures of the sweep methodology.

### Don't expose weights to users

- Weights are an internal control surface. Asking the user (or the agent) to pick `momentum: 8` is asking them to reason about a quantity they have no calibrated intuition for.
- Expose instead **declarative inputs the agent already reasons about** — code maturity, correctness priority, work shape — and **derive weights from those**.
- Profiles (e.g. `throughput`, `balanced`, `safety`) are a candidate user-facing surface, but they are downstream of the declarative inputs, not the top-level knob.

### Kerf is the process-management layer

- Kerf answers, on the agent's behalf: *what to work on next and why, is the queue healthy, when to merge or wait, what to defer.*
- Not: "help do the work." That stays with the agent.
- This framing matters because it justifies new queue mechanisms (a "later" / deferred bucket, explicit prioritization of cleanup over fan-out, finish-before-start pressure) that don't make sense if kerf is just a ranking function.

### The 5% area-collisions guardrail is not a law

- Currently `tools/sweep_decision.go` (or wherever the v2 decision rule lives) bakes the ≤5% area-collisions floor into every weight comparison.
- That should be a **knob inside a safety profile**, not a universal rule. A user shipping experimental work may rationally accept higher collision rates for throughput; a user on a correctness-critical codebase may want it tighter than 5%.
- Restating per the standing user preference: capturing this as a knob, not laying down a rule that "all weight sets must satisfy."

## Two layers of mathematical determination

The reframe replaces "search for weights" with two stacked layers that produce them.

### T=0 — static, graph-only

- Run before any execution. Inputs: the task graph as declared (works, beads, deps, areas).
- Strong signals available with no runtime data:
  - **Critical-path length** → momentum weight. Long chains → favor finishing what's started; short independent works → favor fan-out.
  - **Fan-out width** → fan_out weight and a parallelism ceiling.
  - **Area-overlap density** → predicts the collision floor; informs the collision tolerance.
  - **Dependency-graph shape** → constrains optimal scheduling (which works can run in parallel, which serialize).
- This is well-studied territory. Borrow, don't invent:
  - List scheduling (classical OR).
  - Critical Path Method (CPM).
  - HEFT (Heterogeneous Earliest Finish Time) for task DAGs on heterogeneous workers.
- T=0 produces a *starting* weight estimate plus structural advice (e.g., "this graph has a critical path of length 14 — momentum-heavy profile recommended").

### T>0 — adaptive, live observations

- Once execution starts, observed signals refine the T=0 estimate:
  - observed rework rate
  - observed merge-conflict rate
  - observed phase durations (spin-up, task work, reviewer round-trip, merge)
  - abandoned-dispatch rate
- **This is exactly the data Plan 013 — self-diagnostics from Claude transcripts collects.** Plan 013 is the feed for the T>0 layer, not a separate effort.
- The T>0 layer adjusts weights live as evidence accumulates. The agent doesn't see the weights; it sees the resulting `kerf next` ordering and queue-health signals.

## Open questions

These are surfaced, not answered. Follow-on plans should pick them up.

1. **Declarative inputs.** What exactly does the agent declare?
   - Candidates: code maturity (greenfield / mature / legacy), correctness priority (experimental / standard / safety-critical), work shape (independent / chained / mixed), deploy cadence, others?
   - Per-project, per-work, or both?
2. **Input-to-weights mapping.** Table lookup, formula, or learned?
   - Table is most legible and debuggable.
   - Formula composes better with the T=0 graph signals.
   - Learned needs a much larger corpus than Plan 012 produced.
3. **Deferred / "later" queue.** Where does it live in the data model?
   - New bead status? New queue-membership flag? New plan-level field?
   - How is it surfaced in `kerf next` and `kerf triage`?
4. **Profiles — preset or derived?** Do we ship `throughput` / `balanced` / `safety` as named presets the user picks, or always derive them from declarative inputs?
   - Probably both: presets as legible shortcuts, with the derivation visible as `kerf advise --explain`.
5. **Static-analysis output surface.** Where does the T=0 layer expose itself to the agent?
   - New `kerf advise` command (recommends a profile / weight set with rationale).
   - New field on `kerf next` output (the "why" of this ranking).
   - Separate `kerf plan-report` for the whole graph.
   - Probably more than one of these; pick what fits the agent's read pattern.
6. **Collision tolerance as a knob.** What's the right shape — single percentage, per-area override, dynamic based on observed merge-conflict cost?

## Likely follow-on plans

One line each. Not drafted; not sequenced.

- **015 — declarative inputs schema.** Define what the agent declares, where it lives (`project.yaml`? per-work?), and how it surfaces in `kerf init` / `kerf show`.
- **016 — T=0 static analyzer.** Critical-path / fan-out / area-overlap signals on the graph; `kerf advise` surface.
- **017 — derived weights.** Mapping from declarative inputs (+ T=0 signals) to internal weights. Replaces the current single global weight vector.
- **018 — deferred queue.** Data-model and UX for the "later" bucket; integration with `kerf next` and `kerf triage`.
- **019 — adaptive layer wiring.** Consume Plan 013's diagnostic findings to adjust weights live; loop closure with the T=0 estimate.
- **020 — profile surface.** User-visible profiles (`throughput` / `balanced` / `safety`); presets and the derivation path.
- **021 — collision tolerance refactor.** Move the 5%-area-collisions guardrail out of universal decision logic and into the safety profile.

## Notes

- Plan 011 / D's null results are not a failure; they are the evidence that "one winning weight set" was the wrong question. Future plans should cite this when explaining why the scheduler architecture changed.
- The two-layer (T=0 graph, T>0 adaptive) structure is a guide for follow-on plans, not a contract. If a follow-on finds a better decomposition it should say so and update this plan.

## Critique reconciliation (2026-05-19)

Three critiques live in `critiques/`: (A) architecture/spec coherence, (B) parallelization, (C) spec coverage.

**Accepted:**

- **A1 — Re-number follow-ons to ≥ 025.** The 015–021 numbers Plan 014 originally sketched have been consumed by the harmonik-beta track. Beads in this plan refer to follow-ons as "P025…" placeholders.
- **A2 — `coordination.md` is the primary spec home.** Carve out `process-management.md` only if the new content exceeds ~200 lines.
- **A3 — `kerf advise --explain` is the canonical T=0 surface.** Do not re-load the `kerf next` payload Plan 019 just trimmed.
- **A4 — kerfsim compatibility paragraph.** Direct `--weights` YAML input stays; declarative-inputs path is additive.
- **A5 — Profile is the user noun.** Declarative inputs + T=0 graph signals are the mechanism that picks/blends profiles. Open question 4 resolves to: ship both, with derivation visible via `kerf advise --explain`.
- **B1 — Three independent first beads** (declarative-inputs schema, T=0 analyzer skeleton, collision-tolerance refactor) fan out in parallel; downstream beads serialize after.
- **B2 — Hotspot integration-test requirement.** Any bead touching `internal/queue` must run `go test ./internal/queue/...` against the merged state per CLAUDE.md.
- **C1 — Spec-sentence-per-bead table is the contract.** Each bead names the sentence it satisfies before dispatch.
- **C2 — kerfsim BeadSource coupling.** Weight derivation must either run on the simulator's side too or be fed in as a separate input; the bead surfaces this as a required design choice.
- **C3 — Deferred-queue is the biggest single bead by surface area** (crosses `coordination.md` + `dependencies.md` + three internal packages). Decompose into schema bead + dep-gating bead + UX bead.

**Rejected / deferred:**

- **(Implicit in A)** Auto-numbering follow-ons inside this plan. Reason: the orchestrator owns plan numbering. Beads.md lists work-items, not future plan names.
- **(C)** Pre-drafting every spec sentence in this plan. Reason: bead authors draft the sentence when they own the bead; pre-drafting risks freezing the wrong wording.

## Specs touched (forward look)

- `specs/coordination.md` — derivation paragraph, T=0 signals section, T>0 adaptive section, collision-tolerance demotion.
- `specs/works.md` and `specs/architecture.md` — declarative-input fields on `spec.yaml` and `project.yaml`.
- `specs/commands.md` — new `kerf advise` section; additive note on `kerf next` / `kerf triage` interaction with the new rationale data (no payload changes).
- `specs/simulator.md` — additive `--declarative` input path; preserve `--weights` path.
- `specs/dependencies.md` — deferred bead status interaction with `IsComplete`.
- `specs/testing.md` — one new property contract (weight-derivation symmetry between `kerf` and `kerfsim`).
