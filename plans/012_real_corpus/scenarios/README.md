# Real-corpus scenarios (Plan 012 / D)

Scenario YAMLs imported from the harmonik pilot decompositions
(`/Users/gb/github/harmonik/docs/decompose-to-tasks/*-pilot-data.yaml`)
via `kerfsim import`, then re-wired to draw durations from the fitted
distributions in `../data/fitted_distributions.yaml` (`task_work`,
`spin_up`, `merge`, `conflict_resolution`). These feed the Plan 011
weight-tuning sweep. Smoke-run metrics from a single kerf-policy run
per scenario are in `../data/realcorpus_smoke.csv`.

## Scenarios

| File              | Works | Beads | Ticks | Notes                                |
|-------------------|-------|-------|-------|--------------------------------------|
| `cp.yaml`         | 1     | 85    | 20000 | single work, deps=[]                 |
| `hc.yaml`         | 1     | 80    | 20000 | single work, deps=[]                 |
| `on.yaml`         | 1     | 84    | 20000 | single work, deps=[]                 |
| `pl.yaml`         | 1     | 59    | 20000 | single work, deps=[]                 |
| `rc.yaml`         | 1     | 79    | 20000 | single work, deps=[]                 |
| `sh.yaml`         | 1     | 53    | 20000 | single work, deps=[]                 |
| `wm.yaml`         | 1     | 71    | 20000 | single work, deps=[]                 |
| `all_pilots.yaml` | 8     | 553   | 40000 | chain wm→rc→pl→hc→cp, on/sh fan-in   |
| `all_pilots_sat.yaml` | 8 | 553   | 60000 | saturated: agents=2, rework_rate=0.008, all works rework targets |

Bead counts come from the harmonik pilot YAMLs verbatim. Cross-pilot
dep edges are inferred by the importer and pruned to keep the
work-level DAG acyclic; per-pilot drop counts are recorded in each
scenario's header comment. All scenarios use `seed: 42`, `agents: 4`,
default kerf weights, and `rework_rate_per_tick: 0.001` on a small set
of target works.

Single-pilot scenarios stop `all-closed` (wall 6k–10k). `all_pilots`
stops `ticks-cap` at 40k, 6/8 works complete, idle_pct ~0.71 — the
unfinished works (`on`, `sh`) sit behind their dep chains in the last
quarter. Idle is below 0.8, so agents stay at 4.

`all_pilots_sat` stops `ticks-cap` at 60k with idle_pct ~0.17 (well
under the 0.3 saturation target). The high rework rate (0.008/tick on
all 8 works) keeps work generating across the full DAG, so
`work_completed` exceeds `work_total` — completions include rework
cycles. `rework_p95_wait` is in the ~19k–29k range, confirming the
rework path is meaningfully exercised; weight changes on momentum and
rework move the metric. This is the live signal-bearing scenario for
the Plan 011 / D follow-up sweep (see
`../../011_sim_validation/weight_tuning/sweep_v2.py`).

## Caveats

- The harmonik corpus references 11 pilot codenames
  (ar/bi/cp/em/ev/hc/on/pl/rc/sh/wm); only 7 have a `*-pilot-data.yaml`
  on disk. `all_pilots` also picks up `meta-pilot-data.yaml`, the
  workflow-task graph — so 8 works total, including a `meta` work with
  `areas: [workflow]`.
- The kerfsim binary must be built from the post-`5e617f1` tree
  (multi-pilot dep cycle fix).
