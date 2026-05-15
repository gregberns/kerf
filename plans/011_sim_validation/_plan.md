# Plan 011 — Simulator Validation Quick Wins

> **Status: DRAFT.** Captures simulator follow-ups from Plan 007 + Plan 008 that do not require new data ingestion. The real-workload-corpus pillar (harmonik bead YAMLs, Claude-transcript duration distributions) lives in Plan 012 and runs in parallel.

## Intent

Plan 007 shipped a working simulator (`cmd/kerfsim/`). Plan 008 smoke-tested it with seven synthetic scenarios at three to five agents each. We learned kerf wins on size-asymmetric mixes and loses to `random` on area collisions, but **the learning loop was never closed**: adversarial scenarios were authored and never run, five of seven exploratory scenarios were under-saturated (agents idle ≥79%), proposed weight tunes were never tested, and we never varied agent count on the same workload.

This plan closes that loop using only what we already have on disk.

## Why

- Single-orchestrator with N subagents is the real deployment shape, and N varies per user. Kerf's scoring needs to behave sanely across that range, not just at N=4 where we happened to test.
- Three rework-sensitive metrics were stuck at zero across all Plan-008 runs. Diagnosis (bead `kerf-3b2`) showed under-saturation was one cause and a structural definition gap was another. Until both are fixed and re-run, no conclusion involving rework behavior is trustworthy — and rework is kerf's largest weight (15.0).
- The adversarial scenarios in `plans/008_exploratory_testing/sim_scenarios/adversarial.md` were specifically designed to attack kerf's weights. Skipping them means we have no empirical worst-case data.

## Pillars

### A — Concurrency sweep

**Goal:** measure how throughput, area_collisions, top_of_queue_churn, agent_idle_pct, and goal_completion_3d move as a function of agent count, holding workload constant.

**Approach:**
- Extend `cmd/kerfsim/run.go` with `--agents-sweep "1,2,3,5,7,10"` that, given one scenario YAML, runs it once per agent count per policy per seed.
- Output layout: `results/<scenario>/seed_<n>/agents_<k>/<policy>/summary.json`.
- Aggregation script: per-scenario sweep table (agent-count → metric).
- Scenario YAMLs keep their `agents:` field as a default; the sweep flag overrides.

**Hypothesis to test:** momentum weight's collision cost grows linearly with N because more agents pile into the same hot area.

### B — Adversarial scenarios (already authored)

Run the five designed-but-never-executed scenarios in `plans/008_exploratory_testing/sim_scenarios/adversarial.md` (`adv-rework-swamp`, etc.) with the same 3-seed × 4-policy matrix Plan 008 used. Confirm or refute the predicted kerf losses recorded in `scoring_critique.md`.

### C — Saturated re-runs

Re-run the seven exploratory scenarios after re-sizing `agents`, `ticks`, and `bead_arrivals` so `agent_idle_pct` drops below ~0.3. Findings explicitly recommended this. Until done, every Plan-008 conclusion involving wait-time or churn is suspect.

### D — Weight tuning

Once A–C produce a credible benchmark suite, sweep weights:
- `momentum`: {0, 1, 2, 5} (current 5.0; flagged as the area-collision liability).
- `rework`: {5, 8, 15} (current 15.0; oversize given common-case zero waits, pending re-validation under saturation).
- Optional negative `area_diversity` term penalizing dispatches that pile agents on the same area.

**Decision rule (placeholder; user to confirm):** a weight change is adopted only if it dominates the current default on at least 60% of scenarios and no scenario loses by more than 5% on any metric.

### E — `priority_inversions` semantic fix

Bead `kerf-3b2` diagnosed the metric as structurally unreachable as currently defined: initial new-work beads all have `ArrivalTick = 0`, so no rework can be "older." Diagnosis in `plans/008_exploratory_testing/sim_scenarios/diagnosis.md`. Fix: redefine "older" to compare `arrival_tick` strictly less with a deterministic tie-break by bead ID, or change the synthetic generator to scatter initial arrivals across a warmup window. Spec update in `specs/simulator.md`.

## Sequencing

- **L0 (foundation):** A (concurrency sweep flag), E (priority_inversions fix). Independent and quick.
- **L1 (existing-corpus runs):** B (adversarial), C (saturated re-runs). Depend on E so metrics are trustworthy.
- **L2 (tuning):** D. Requires A, B, C done.

Plan 012 runs in parallel and feeds new scenarios into D when it lands; this plan does not block on it.

## Specs touched

- `specs/simulator.md` — `priority_inversions` definition (pillar E); new `--agents-sweep` semantics (pillar A).
- `specs/commands.md` — `kerfsim run` flag additions.

## Out of scope

- Real-workload corpus (Plan 012).
- Multi-agent concurrency primitives (`plans/_backlog/010_concurrency/` — dormant).
- Structural redesign of the scorer beyond weight values + optional `area_diversity`.

## Open decisions for user

1. **Weight-tuning decision rule** — the 60%-dominance / 5%-loss-cap rule above is a placeholder.
2. **Should pillar D include the real-workload scenarios from Plan 012 if they land before tuning starts?** Default: yes, opportunistically.
