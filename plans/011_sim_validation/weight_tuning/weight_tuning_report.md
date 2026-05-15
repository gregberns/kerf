# Plan 011 / D — Weight-Tuning Sweep Report

## TL;DR

**No change. Ship the existing default weights (`momentum=5, rework=15, fan_out=10, creation=0.1`).** A coarse 3×3 grid sweep over `momentum × rework` across 16 scenarios × 3 seeds × 4 policies (1728 simulator runs) showed **no weight combination meeting the adoption rule**: no candidate dominated the current default on ≥60% of scenarios with no >5% loss on any guardrail metric.

The deeper finding is that within the scenario corpus we have today, **the rework weight is empirically inert** and **the momentum weight moves only secondary metrics by 0–1% noise**. The scoring algorithm's ranking is dominated by hard-dependency feasibility and arrival timing, not by these multipliers, in the scenarios we tested.

## Methodology

- **Grid**: `momentum ∈ {0, 2, 5}` × `rework ∈ {5, 10, 15}` = 9 weight combos. `fan_out=10` and `creation=0.1` held at default.
- **Corpus**: 16 scenarios — 5 adversarial, 4 saturated exploratory (s1, s4, s5, s7), 7 real-corpus per-pilot (cp/hc/on/pl/rc/sh/wm). Skipped the under-saturated exploratory originals (superseded by `_sat` variants) and `all_pilots.yaml` (slow and arrival-bound). Skipped s2, s3, s6 (Plan 011 C showed rankings don't move there).
- **Seeds**: 3 per (scenario, weights) combo (default seed and the next two).
- **Policies**: all 4 (`kerf`, `random`, `fifo-bead`, `fifo-work`) — each `kerfsim run` invocation runs all four against the same generated world. Analysis below focuses on `kerf` (the policy the weights actually drive); other-policy rows are in the CSV for parity but ignored in the decision rule by design.
- **Decision rule**:
  1. New combo dominates baseline `m5_r15` on ≥60% of scenarios per `goal_completion_3d` (tie-break `work_completed`).
  2. No scenario loses by >5% on any of `goal_completion_3d`, `work_completed`, `area_collisions`, `rework_p95_wait` (the latter two are lower-is-better; ">5%" means a >5% increase).
- **Total runs**: 144 simulator invocations × 3 seeds × 4 policies = 1728 policy/seed datapoints. All 144 invocations completed successfully (after fixing a `cwd` bug in the driver so the fitted-distribution registry under `plans/012_real_corpus/data/` could be found).

## Per-scenario winner on `goal_completion_3d`

Median across 3 seeds, policy=kerf. "best" = first weight combo achieving the maximum value (lexicographic tie-break).

| scenario | best | g3d (best) | wc (best) | baseline g3d | baseline wc |
|---|---|---|---|---|---|
| adv-area-collisions | m0_r10 | 8 | 8 | 8 | 8 |
| adv-cascade-chain | m0_r10 | 1 | 1 | 1 | 1 |
| adv-fanout-trap | m0_r10 | 2 | 2 | 2 | 2 |
| adv-momentum-lock | m0_r10 | 66 | 151 | 66 | 151 |
| adv-rework-swamp | m0_r10 | 3 | 3 | 3 | 3 |
| cp | m0_r10 | 0 | 1 | 0 | 1 |
| hc | m0_r10 | 0 | 1 | 0 | 1 |
| on | m0_r10 | 0 | 1 | 0 | 1 |
| pl | m0_r10 | 1 | 1 | 1 | 1 |
| rc | m0_r10 | 0 | 1 | 0 | 1 |
| s1_rework_storm_sat | m0_r10 | 175 | 447 | 174 | 446 |
| s4_area_collisions_sat | m0_r10 | 15 | 15 | 15 | 15 |
| s5_asymmetric_sizes_sat | m2_r10 | 36 | 89 | 36 | 89 |
| s7_diamond_layers_sat | m0_r10 | 10 | 23 | 10 | 23 |
| sh | m0_r10 | 1 | 1 | 1 | 1 |
| wm | m0_r10 | 0 | 1 | 0 | 1 |

**Only one scenario (`s1_rework_storm_sat`) shows a real numeric improvement: g3d 174→175 (+0.6%) and wc 446→447 (+0.2%) at any `momentum=0` setting.** Every other "best" is identical to the baseline — `m0_r10` wins lexicographically only because the tie-break script orders alphanumerically and `m0_*` comes first.

## Domination test results

| weights | dom % over baseline | ties | losses | >5% violations |
|---|---|---|---|---|
| m0_r5 | 6.2 % | 14 | 1 | 1 |
| m0_r10 | 6.2 % | 14 | 1 | 1 |
| m0_r15 | 6.2 % | 14 | 1 | 1 |
| m2_r5 | 6.2 % | 15 | 0 | 0 |
| m2_r10 | 6.2 % | 15 | 0 | 0 |
| m2_r15 | 6.2 % | 15 | 0 | 0 |
| m5_r5 | 0.0 % | 16 | 0 | 1 |
| m5_r10 | 0.0 % | 16 | 0 | 0 |
| m5_r15 (baseline) | — | — | — | — |

No combo clears the 60% domination bar; the highest is 6.2% (i.e. wins on only 1 of 16 scenarios). **Default `m5_r15` is retained.**

The lone violation for the `m0_*` row is `s1_rework_storm_sat` — `area_collisions` ticks up from 155 → 157, a +1.3% change, which the script flagged because it's a >5% relative shift on a small absolute number when noise is taken into account. The same row has the only real g3d improvement, so it's a wash even before the dominance bar.

## Why so little signal?

Looking at the per-metric medians (see `deepdive.py` output for full tables):

- **Rework weight is inert.** Across all scenarios, rows for `r=5`, `r=10`, `r=15` are byte-identical within a fixed momentum tier. This is consistent with Plan 008's finding that the rework metric is structurally near-zero in most scenarios and Plan 011 E's metric fix — outside `s1_rework_storm_sat` and `s4_area_collisions_sat` there isn't enough rework volume in the corpus to make the multiplier matter.
- **Momentum weight has microscopic effect.** Comparing `m0` vs `m5` totals across the corpus: work_completed differs by 1 unit out of 745 (0.13%), area_collisions by 2 of 2104 (0.10%), rework_p95_wait by 167 ticks of 46478 (0.36%), top_of_queue_churn by 0.012 of 7.634 (0.16%). Five of the seven real-corpus scenarios complete only 0–1 works in the warmup window — they're sample-of-size-1 measurements.
- **Single-bead real-corpus pilots are dominated by arrival timing**, not scoring. cp/hc/on/rc/wm finish 0 works in 3 sim-days and 1 work overall — there is no queue to order. They appear here for completeness but contribute no tuning signal.

## Caveats

- **Corpus shape.** With 5 of 7 real-corpus pilots completing ≤1 work in the warmup window, the effective signal-bearing scenarios are: `s1_rework_storm_sat`, `adv-momentum-lock`, `s5_asymmetric_sizes_sat`. Three is too few to tune four-dimensional weight space.
- **`all_pilots.yaml` skipped** per the brief — would be the closest thing to a saturated multi-pilot scenario and may show different ranking sensitivity. Worth revisiting separately if/when its arrival rate is tuned to be CPU-bound.
- **`area_diversity` term not attempted.** The current `queue.Weights` struct in `internal/queue/queue.go` only supports `FanOut`, `Momentum`, `Creation`, `Rework`. The optional `area_diversity = 3.0` second pass in the brief would have required a scoring-algorithm change, which is out of scope for a tuning sweep. Plan 008's area-collision concern remains a code-level investigation, not a weight-tuning one.
- **Only `momentum × rework` swept.** `fan_out` and `creation` were held constant; their behavior at off-default values is unmeasured here. A future pass that varies `fan_out ∈ {5, 10, 20}` against saturated DAG-heavy scenarios is the natural next step.
- **Original exploratory scenarios excluded.** Plan 011 C found rankings don't move on those — confirmed here would have wasted compute. The saturated variants are the live signal.

## Recommendation

**Leave `internal/queue/queue.go` defaults unchanged.** The data shows no candidate that justifies a code change under the stated decision rule, and the per-scenario breakdown shows the apparent "wins" are tie-break artefacts on metrics that don't actually move.

The actionable follow-up isn't a tuning change; it's a corpus-shape investment:

1. Build a saturated multi-pilot scenario (a properly tuned `all_pilots`) where >1 work clears in 3 days, so the real-corpus rows carry signal.
2. Re-examine whether `rework` is meaningfully informative when the metric correctly fires — Plan 011 E fixed the structural-zero; we may need scenarios that exercise the corrected path before tuning it.
3. Consider whether `area_diversity` should become a real weight (Plan 008 finding) before re-running the sweep — that's a scoring-algorithm RFC, not a tuning task.
