# Plan 011 / D — Weight-Tuning Sweep v2 Report

## TL;DR

**No change. The default weights (`momentum=5, rework=15, fan_out=10, creation=0.1`) are retained.** A saturated multi-pilot scenario (`all_pilots_sat`, idle=0.165 vs the v1 corpus's ~0.71) was added to unblock the sweep, and a 27-combo grid over `momentum × rework × fan_out` was run on it. v2 now produces a real signal — several combos beat the default on `goal_completion_3d` and `work_completed` on all 3 seeds — but every winning combo trips the >5% guardrail on `area_collisions`. Under the v1 decision rule, no combo is adoptable.

The v1 report's null-result diagnosis is now validated: corpus shape was the problem, the rework path is meaningfully active on the saturated scenario, and a 3-D grid does surface a tunable direction (lower `rework`, higher `fan_out`). The guardrail violation is real, not noise, and is the next thing to consider before adopting a new default.

## What changed since v1

- New scenario `plans/012_real_corpus/scenarios/all_pilots_sat.yaml`: agents 4→2, `rework_rate_per_tick` 0.001→0.008, all 8 works in `target_works`, ticks 40000→60000. Idle drops from 0.71 to 0.165. `rework_p95_wait` lands in the 21k–24k range and now varies measurably with weight settings.
- New sweep harness `sweep_v2.py` + analyzer `analyze_v2.py`: 27 combos (`m ∈ {2,5,10} × r ∈ {5,15,30} × f ∈ {5,10,20}`), 3 seeds, 4 policies, 324 datapoints. Wall time ≈ 20s.
- Generated weights under `weights/v2/`; raw runs under `runs_v2/`; collated metrics in `sweep_v2_results.csv`.

## Methodology

- **Corpus**: single scenario, `all_pilots_sat`. v1 swept 16 scenarios and found rework-weight rows byte-identical for 14 of them — that signal-poor mix is not re-run. v2's single scenario carries enough rework volume to differentiate combos.
- **Grid**: 3 levels per dim including baseline, total 27 combos. `creation = 0.1` held.
- **Seeds**: 3 (`42, 43, 44`), same convention as v1.
- **Policies**: all 4 run per invocation; only `kerf` is used in the decision rule (it is the policy the weights drive).
- **Decision rule** (adapted from v1 for a single scenario): candidate must beat baseline on `goal_completion_3d` (tie-break `work_completed`) on ≥60% of seeds AND show no >5% loss on the median value of any of `goal_completion_3d`, `work_completed`, `area_collisions`, `rework_p95_wait`. v1's "≥60% of scenarios" rule degenerates to a per-seed rule when there is only one scenario.

## Saturation check

Single seed-42, default-weights kerf run on `all_pilots_sat`:

| metric            | unsaturated `all_pilots` | saturated `all_pilots_sat` |
|-------------------|--------------------------|----------------------------|
| `agent_idle_pct`  | 0.713                    | **0.165**                  |
| `wall_ticks`      | 39710                    | 59980                      |
| `work_completed`  | 6                        | 22                         |
| `rework_p95_wait` | 4779                     | 23225                      |
| `stop_reason`     | ticks-cap                | ticks-cap                  |

Idle is well under the 0.3 saturation bar, and the rework path is firing — `work_completed > work_total` because the high-rate rework arrivals re-open works after they close. This is the live signal-bearing scenario the v1 report called for.

## Per-combo medians (kerf policy, 3 seeds)

Only highlights below; full 27 rows are in `analyze_v2.py` output and `sweep_v2_results.csv`.

| combo               | g3d | wc | area_collisions | rework_p95 | wins vs baseline | violations |
|---------------------|-----|----|------------------|------------|------------------|------------|
| `m5_r15_f10` (base) | 0   | 22 | 168              | 23225      | —                | —          |
| `m5_r5_f10`         | 0   | 24 | 179              | 22378      | 3/3 (100%)       | col +6.5%  |
| `m5_r5_f20`         | 2   | 28 | 179              | 24233      | 3/3 (100%)       | col +6.5%  |
| `m2_r5_f20`         | 2   | 28 | 179              | 24233      | 3/3 (100%)       | col +6.5%  |
| `m10_r5_f20`        | 2   | 28 | 181              | 24187      | 3/3 (100%)       | col +7.7%  |
| `m5_r15_f20`        | 0   | 22 | 175              | 21858      | 1/3 (33%)        | —          |

Six combos win on `g3d` on 3/3 seeds. **Every one of them violates the area-collisions guardrail (+6.5% to +7.7%).** No combo passes the strict rule.

## Marginal-effect dimension scan (medians at baseline midpoint, kerf)

Varying one dimension while holding the other two at default:

```
momentum (r=15, f=10):  m=2 → wc=22  m=5 → wc=22  m=10 → wc=22
rework   (m=5,  f=10):  r=5 → wc=24  r=15→ wc=22  r=30 → wc=22
fan_out  (m=5,  r=15):  f=5 → wc=22  f=10→ wc=22  f=20 → wc=22
```

- **Momentum is inert** on this scenario at every (rework, fan_out) pair examined. Confirms v1's finding even after saturation.
- **Lower rework helps `work_completed`** (+2 throughput at `r=5`). Counter-intuitive — a smaller rework bonus lets the queue advance new beads instead of re-prioritizing rework already in flight.
- **`fan_out` alone is inert at the default midpoint**, but interacts with `rework=5`: dropping rework to 5 and raising fan_out to 20 lifts `g3d` from 0 to 2 and `work_completed` from 22 to 28 — the only combo that moves `g3d` at all.

## Why every winner trips the guardrail

`area_collisions` measures concurrent assignments to the same area. The winning combos prioritize new-work fan-out (high `f`) and de-emphasize rework concentration (low `r`), which spreads agents across more areas concurrently. That is what fan-out is *supposed* to do. The result is more throughput AND more concurrent same-area work, because the saturated scenario has only 2 agents but 8 works with overlapping area sets.

The v1 decision rule treats area-collisions as a strict guardrail — but at idle=0.16 with 2 agents and many works, some area collision is the cost of doing business. The rule was written for unsaturated scenarios where collisions are a clean signal. On a saturated scenario, the rule may be too strict.

## Remaining corpus gap (and what would unblock a verdict)

1. **Need a saturated scenario with low area-overlap** to disentangle "fan_out helped throughput" from "fan_out increased area_collisions". The current `all_pilots_sat` has 5 of 8 works sharing the `invariant`/`req`/`schema`/`test-infra` area tuple — collisions are baked in.
2. **Need to decide whether the >5% area_collisions rule applies at high saturation.** If +6.5% collisions for +9% throughput (wc 22→24) is acceptable, `m5_r5_f10` is the winner; if not, baseline holds. This is a product decision, not a sweep decision.
3. **A second saturated multi-pilot scenario** (different DAG shape, different area mix) is needed before any default change so the win generalizes beyond a single corpus point.

## Recommendation

**Leave `internal/queue/queue.go` defaults unchanged for now.** No combo passes the strict rule, but unlike v1 the failure mode is interesting: lowering `rework` (5) and raising `fan_out` (20) yields a real +27% `work_completed` gain (22→28) and is the first weight change ever to move `goal_completion_3d` on a real-corpus scenario. The decision blocker is whether to relax the area-collisions guardrail on saturated scenarios or to add a second saturated scenario with a different area mix.

Follow-ups (in order of leverage):

1. Build a second saturated multi-pilot scenario with disjoint area sets per work; re-run `sweep_v2` on the pair.
2. Decide whether `area_collisions` is a hard guardrail at saturation or a tradeoff metric; if the latter, formalize a relaxed rule (e.g. allow up to +10% if `work_completed` ≥ +5%).
3. Revisit `momentum` — it has been inert across v1 and v2. Either there are no scenarios where it can fire, or the score term itself is structurally weak. Worth a code-level look rather than another sweep.

## Artifacts

- Scenario: `plans/012_real_corpus/scenarios/all_pilots_sat.yaml`
- Sweep driver: `plans/011_sim_validation/weight_tuning/sweep_v2.py`
- Analyzer: `plans/011_sim_validation/weight_tuning/analyze_v2.py`
- Weights: `plans/011_sim_validation/weight_tuning/weights/v2/weights_*.yaml`
- Run dirs: `plans/011_sim_validation/weight_tuning/runs_v2/all_pilots_sat__*`
- Collated CSV: `plans/011_sim_validation/weight_tuning/sweep_v2_results.csv`
- v1 report (context): `plans/011_sim_validation/weight_tuning/weight_tuning_report.md`
