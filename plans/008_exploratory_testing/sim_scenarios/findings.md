# Sim Findings — Complex Scenarios (Agent E)

Captured from the complex-scenarios agent because the agent itself was blocked from writing this file. Original analysis preserved verbatim where possible.

## Top finding — suspected bug / spec ambiguity

Three rework-sensitive metrics are **stuck at zero across every policy in every scenario**:

- `priority_inversions` = 0 in 7/7 scenarios
- `rework_p50_wait` = 0 in 7/7 scenarios
- `rework_p95_wait` = 0 in 5/7 scenarios (only `s5_asymmetric_sizes` and `s6_late_arrivals` show non-zero values, where kerf wins decisively)

Spec defines both metrics clearly (`specs/simulator.md` §Metric Definitions, lines ~277, 281). The universal-zero pattern suggests one of:

1. The simulator's generated rework beads aren't carrying a label the queue scoring recognizes.
2. The in-memory `BeadSource` shape diverges from the `br list --format json` shape that production uses for rework detection.
3. The metric collector's input path drops the `IsRework` signal somewhere between generator → store → loop → metrics.

**If rework labeling is broken in the simulator, kerf's most-justified weight (`rework=15.0`) is being tested against the wrong signal.** This is the most important diagnostic to chase before drawing any weight-tuning conclusions.

## Where kerf loses

- **`area_collisions`**: kerf consistently loses to `random` by 30–90%.
  - s1: 193 vs random 131
  - s4: 118 vs 98
  - s5: 222 vs 117
  - s6: 42 vs 26
  - Cause hypothesis: momentum weight pulls multiple agents to the same hot area.
- **Throughput on rework storms**: kerf loses ~1% on `work_completed` to FIFO-work in `s1_rework_storm`.
- **`top_of_queue_churn`** is slightly worse than FIFO in s1/s5 — kerf is oscillating its top pick.

## Where kerf wins

- **`s5_asymmetric_sizes`** (mixed 3-bead and 60-bead works): `work_completed` 82 vs 78–80, `rework_p95_wait` 1 vs fifo-bead 103. This is the clearest kerf win across all 7 scenarios.

## Weight-tuning hypotheses (from observed data)

1. **Cut `momentum` from 5.0 toward 2.0 or 0.0.** Strongest visible liability — directly causes the area-collision deficit. Likely no throughput cost.
2. **Cut `rework` from 15.0 toward 8.0.** Largest default weight but `rework_p95_wait` is 0 in 5/7 scenarios (and where it isn't, kerf already wins). Weight may be paying nothing in the common case while adding dispatch oscillation. **CAVEAT: validate the rework-labeling bug first** before acting on this.
3. **Leave `fan_out` (10.0) for now but build a scenario that actually pins it.** `s3_fanout_spike` failed to differentiate any policy — bead budget exhausted before ordering mattered.
4. **Consider a new negative `area_diversity` term** to directly attack the collision deficit.

## Methodology notes

- 5 of 7 scenarios produce identical `wall_ticks` and `agent_ticks_total` across all four policies because durations are pre-rolled and most runs end on `idle-threshold` with `agent_idle_pct ≥ 0.79`. This means policy ordering barely affects throughput when agents are heavily idle — most runs are bottlenecked on bead supply, not policy choice.
- Suggests future scenarios should size agents/ticks/bead-rate so agents are saturated, otherwise policies have nothing to differentiate.

## Spec ambiguity (minor)

`specs/simulator.md` line ~290: `top_of_queue_churn` semantics. In `s2_deep_chain` (fully linear), churn is 0.406 — but should "head unchanged because it is the only eligible bead" count as a non-change? Worth a one-line clarification.

## Artifacts

- Scenario YAMLs: `/tmp/kerfsim-runs/scenarios/s{1..7}_*.yaml`
- Raw results: `/tmp/kerfsim-runs/results/s{1..7}_*/seed_{42,43,44}/{kerf,random,fifo-bead,fifo-work}/summary.json`
- Aggregator script: `/tmp/kerfsim-runs/aggregate.py`
- Per-scenario tables: `scenarios.md` (this directory)
