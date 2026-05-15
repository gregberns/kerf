# Plan 007 — Queue Simulator (`kerfsim`)

> STATUS: DESIGN — phased build, no code yet

## Intent

Deterministic simulator for kerf's queue/scoring logic. Wraps `internal/queue` directly (no subprocess, no real `br`). Same scenario + weights + seed → byte-identical results. Answers questions like *"raise rework weight 15→25, what changes?"* and *"do we actually beat FIFO at p90?"* with measurable numbers instead of guesses.

Ships as a second binary (`kerfsim`) in the same repo, sharing `internal/queue` and other internals.

## Why

Today weights are tuned by eyeball. Smoke testing showed the signals fire correctly in isolation but offered no way to compare configurations or to know when a change makes things worse on a workload we don't usually look at. A simulator makes weight changes A/B-testable and gives kerf a regression harness.

## Shape

### CLI

```
kerfsim run scenario.yaml [--weights w.yaml] [--seed N] [--runs N] [--out dir/]
kerfsim diff runA/ runB/
kerfsim sweep scenario.yaml --param weights.rework=5..30:5    # Phase 2
```

### Inputs

A single YAML scenario file contains everything needed for reproducibility: seed, ticks, agent count, works (codename + areas + deps + bead count), bead arrivals (generator spec or imported `br list` export), duration model.

Two scenarios are comparable iff their scenario files are byte-identical (modulo `--seed` override).

### Outputs (per run)

A directory containing:
- `summary.txt` (compact human view) and `summary.json` (canonical).
- `events.jsonl` — every dispatch, completion, rework arrival, queue snapshot. Sufficient to replay.
- Copies of `scenario.yaml` and `weights.yaml` used.

### Metrics (flat table, no composite score)

| Metric | Notes |
|---|---|
| `work_completed` | n / total |
| `wall_ticks` | end of run |
| `agent_idle_pct` | one half of the cost picture |
| `agent_ticks_total` | other half — "agent-hours" spent. Lets you spot "kerf beat FIFO but cost 30% more agent time." |
| `rework_p50_wait`, `rework_p95_wait` | distribution, not mean |
| `top_of_queue_churn` | how often #1 changes between consecutive `next` calls; high = oscillating |
| `goal_completion_1d/3d/7d` | n done at fixed deadlines |
| `priority_inversions` | new-work dispatched while older rework waited |
| `area_collisions` | two agents working same area simultaneously |

Metrics are reported on the **post-warmup window** by default; first ~10% of ticks are noisy because the queue isn't full yet.

### Baselines (mandatory)

Every scenario also runs under random, FIFO-by-bead, FIFO-by-work. Phase 2 adds "rework-first else FIFO." Without baselines, absolute numbers don't mean anything.

### Loop mechanics (Phase 1)

- **Event-driven tick.** Min-heap of next events (bead-complete, scripted arrival, agent-free). Pop, advance clock, dispatch, repeat. Tick = work-unit time, not wall time.
- **N agents** (default 3, range 1–10). Each idle agent calls `kerf next` greedily.
- **In-memory bead store** mimicking `br list --format json` shape; queue re-reads on every `next`.
- **Stop on:** all works terminal | sim-clock cap | idle threshold reached.
- **Pre-rolled durations.** Log-normal, median ~30 min, long tail. Pre-rolled at scenario creation so swapping weights doesn't change the work itself — clean A/B.

### Determinism

A single top-level seed splits into sub-seeds for scenario generation, runtime noise, probabilistic events, and agent tie-breaking. Wall clock is never part of state.

### Confidence intervals

For non-trivial decisions, `--runs N` repeats with seeds `seed, seed+1, …, seed+N-1`. The generator re-runs per seed, so each run has a different DAG, bead count, and pre-rolled durations — variance bands are meaningful even in Phase 1. `kerfsim diff` reports median + p10/p90 across runs. Phase 2's stochastic mode adds per-run duration re-sampling on top, widening the bands further.

## Fidelity layers (the realism dial)

Each layer makes the simulator more honest. Phased so we don't build all of it before we know any of it works.

| Layer | Phase | What |
|---|---|---|
| Pre-rolled task durations | 1 | Log-normal, fixed at scenario creation |
| Merge / integration cost | 2 | Per-bead completion has a tail: baseline merge time + conflict factor when two beads touched overlapping areas in overlapping windows. Merges are hard-serialized (can't land two at once) — two parallel completions queue. **This is the load-bearing fidelity layer:** it's what makes "at every tick something might or might not happen, and multiple things landing delays subsequent actions" emerge naturally. |
| Per-task startup latency | 2 | Real agents take 60–90s to roll a task; not just durations. Likely doesn't change rankings (everyone pays it) but worth measuring. |
| Stochastic durations | 2 | Re-sample from distribution per run for variance bands. |
| Calibration against a real project | 2 | Snapshot a real harmonik-shape run, sim it, compare. If sim diverges badly, the model is wrong. Phase 2 uses hand-tuned distributions; **Phase 3** explores scraping Claude session logs for real bead→timestamp data to build empirical duration distributions. |
| Area affinity, intra-work bead deps | 3 | Specialist agents, real bead graphs inside a work. |
| Adversarial scenarios | 3 | Hand-crafted to break specific signals (rework storm during wide fan-out, etc.). |

## Phases

### Phase 1 — working tool, synthetic only

Goal: produce a useful number with the simplest viable loop.

- Scenario YAML + synthetic generator (10–80 works, clustered DAG, log-normal bead counts).
- Pre-rolled durations.
- Event-driven tick loop, N greedy agents, in-memory store wrapping `internal/queue`.
- Three baselines: random, FIFO-bead, FIFO-work.
- Three canned scenarios: `small-linear`, `wide-fanout`, `rework-heavy`.
- Compact text output + `--format=json` + `events.jsonl`.
- Warmup window in metrics.
- `agent_idle_pct` + `agent_ticks_total` from day one.
- `kerfsim diff`.

Phase 1 has **no merge model, no startup latency, no stochastic mode, no real-project import.** Just the simplest thing that compares two weight configs honestly on synthetic data.

### Phase 2 — fidelity & comparability

Goal: trust the numbers, and start using real shapes.

- Merge/integration cost model (baseline + conflict factor + hard serialization).
- Per-task startup latency.
- Stochastic-duration mode.
- `--runs N` confidence intervals (median + p10/p90 reporting in `diff`).
- `kerfsim sweep` for sensitivity analysis (one weight at a time across a range).
- `kerfsim import <project>` — snapshot a live project into a scenario.
- Fourth baseline (rework-first-else-FIFO).
- Aggregation across ~50 scenarios; golden file + CI regression hook.
- Calibration pass against a real snapshotted project.

### Phase 3 — depth

- Adversarial scenarios.
- Claude-log-derived duration distributions (extract bead-to-timestamp pairs from session logs, build empirical distributions, scale wall-clock to ticks). Trickier scraping; defer until Phase 2's calibration shows we need empirical data.
- Area affinity (specialist agents).
- Intra-work bead dependencies.
- Additional canned scenarios.

## Specs Affected

| Spec | Change |
|---|---|
| `specs/_index.md` | Add `simulator.md` to the map |
| `specs/simulator.md` | **New.** CLI, scenario schema, weights schema, output format, baselines, determinism rules, fidelity-layer phasing |
| `specs/coordination.md` | No change — queue logic unmodified |
| `specs/cli.md` | No change — `kerfsim` is a separate binary |

## Open Questions (Phase 1 only)

The Phase 2/3 layers each have their own sub-questions; not enumerating those until we get there. For Phase 1:

1. ~~**Scenario generator knobs.** Default work count? Default agent count?~~ Resolved in `specs/simulator.md`: 30 works, 3 agents.
2. **Warmup window definition.** First 10% of ticks? First N completions? (Proposed: first 10% of ticks, configurable.)
3. **`kerfsim diff` output shape.** Side-by-side table or stacked? (Proposed: side-by-side with delta column.)

## Implementation Notes

1. **`kerfsim` binary in `cmd/kerfsim/`**, same repo. Imports `internal/queue` and a new `internal/sim` package.
2. **`internal/sim` carries the loop, generator, baselines, metrics.** In-memory bead store mimics `br list` JSON shape so the existing parser is reused.
3. **Event-driven tick loop is the core.** Min-heap of `(when, kind, payload)` events. Phase 2's merge model layers on by inserting "merge-complete" events between "bead-complete" and "work-state-updated" — keeps the tick loop generic.
4. **Seed splitting.** One top-level seed → named sub-seeds (`gen`, `dur`, `events`, `tiebreak`). Documented so the same seed always means the same thing across versions.
5. **Outputs are durable and diff-friendly.** `summary.json` canonical, `summary.txt` rendered, `events.jsonl` is the audit trail. Scenarios and weights copied into the run dir.
6. **Tests.** Determinism is central: same seed → byte-identical `events.jsonl`. Plus unit tests for generator distributions, tick loop event ordering, each baseline policy.

## Implementation Beads

See [beads.md](beads.md) for the Phase 1 implementation task breakdown — 14 beads across 5 layers (B4 resolved without work). Critical path is 6 hops. Beads are tracked in `bd`; see [/plans/bead-id-map.md](../bead-id-map.md) for the bd ID mapping.

## Source

Two-perspective design (workflow/UX + data/loop/analysis) in `source/proposal.md`.
