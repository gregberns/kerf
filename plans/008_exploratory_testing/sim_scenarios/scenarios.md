# Exploratory Simulator Scenarios

Seven complex scenarios beyond the three canned ones, each run 3 times with seeds {42, 43, 44} via `kerfsim run --runs 3`. All YAMLs live under `/tmp/kerfsim-runs/scenarios/`; all results under `/tmp/kerfsim-runs/results/<name>/seed_{42,43,44}/{kerf,random,fifo-bead,fifo-work}/summary.json`.

Tables below report **median across the 3 seeds**. Policy in **bold** is the winner on that metric (lower-is-better for waits/idle/collisions/inversions/churn/ticks; higher-is-better for completion/goal).

## s1_rework_storm — `/tmp/kerfsim-runs/scenarios/s1_rework_storm.yaml`

**Theme:** Extreme rework arrival rate (0.06/tick, ~1200 rework arrivals over 20k ticks across 8 works). Stresses the rework weight.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | 466 | 468 | 468 | **471** |
| `wall_ticks` | 19997 | 19997 | 19997 | 19997 |
| `agent_idle_pct` | 0.799 | 0.799 | 0.799 | 0.799 |
| `agent_ticks_total` | 16000 | 16000 | 16000 | 16000 |
| `rework_p95_wait` | **0** | 0 | 0 | 0 |
| `top_of_queue_churn` | 0.491 | 0.516 | **0.487** | 0.492 |
| `goal_completion_3d` | 192 | 194 | 194 | **197** |
| `area_collisions` | 193 | **131** | 186 | 196 |

**Verdict:** kerf *loses* on throughput (work_completed, goal_completion_3d) to FIFO baselines by ~1%. Random has dramatically fewer area collisions (131 vs 193) — kerf's momentum weight is pulling agents to the same in-progress area. Rework waits are 0 everywhere (suspicious — see findings.md).

## s2_deep_chain — `/tmp/kerfsim-runs/scenarios/s2_deep_chain.yaml`

**Theme:** 20-work strict linear chain. No parallelism possible — tests serialization handling.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | 4 | 4 | 4 | 4 |
| `agent_idle_pct` | 0.998 | 0.998 | 0.998 | 0.998 |
| `agent_ticks_total` | 282 | 282 | 282 | 282 |
| all other metrics | identical across policies | | | |

**Verdict:** Tie. With one runnable work at a time, ordering policy is irrelevant. 3 of 4 agents are starved.

## s3_fanout_spike — `/tmp/kerfsim-runs/scenarios/s3_fanout_spike.yaml`

**Theme:** Single root with 15 dependents + 2 distractor roots. Tests whether kerf prioritizes the high-fan-out root.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | 18 | 18 | 18 | 18 |
| `agent_ticks_total` | 1101 | 1101 | 1101 | 1101 |
| `top_of_queue_churn` | **0.521** | 0.548 | 0.521 | 0.521 |
| all other metrics | identical | | | |

**Verdict:** Effectively a tie. All policies completed all 18 works; the durations and small bead-count budget meant no policy hit a meaningful divergence point.

## s4_area_collisions — `/tmp/kerfsim-runs/scenarios/s4_area_collisions.yaml`

**Theme:** 12 independent works, all touching overlapping areas {cli, queue, storage}. 4 agents.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | 40 | 40 | 40 | 40 |
| `top_of_queue_churn` | **0.574** | 0.726 | 0.574 | 0.574 |
| `area_collisions` | 118 | **98** | 118 | 118 |

**Verdict:** kerf ties FIFO on throughput, loses to random on area_collisions (118 vs 98). Momentum weight is causing agents to cluster on the same area.

## s5_asymmetric_sizes — `/tmp/kerfsim-runs/scenarios/s5_asymmetric_sizes.yaml`

**Theme:** Two 60-bead huge works + six 3-bead tiny works + dependent medium/tiny. Tests size disparity.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | **82** | 78 | 79 | 80 |
| `rework_p95_wait` | **1** | 14 | 103 | 9 |
| `top_of_queue_churn` | 0.556 | 0.786 | **0.548** | 0.552 |
| `goal_completion_3d` | **38** | 35 | 36 | 37 |
| `area_collisions` | 222 | **117** | 234 | 234 |

**Verdict:** **Best result for kerf.** kerf wins on throughput (82 vs 78–80), goal_completion_3d (38 vs 35–37), and crushes rework_p95_wait (1 vs fifo-bead's 103, random's 14). But still loses badly on area_collisions to random.

## s6_late_arrivals — `/tmp/kerfsim-runs/scenarios/s6_late_arrivals.yaml`

**Theme:** Explicit scripted rework bursts at tick 5000 and 9000. Tests reactive prioritization.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | 5 | 5 | 5 | 5 |
| `rework_p95_wait` | **18** | 34 | 18 | 18 |
| `top_of_queue_churn` | **0.443** | 0.671 | 0.443 | 0.443 |
| `area_collisions` | 42 | **26** | 42 | 42 |

**Verdict:** kerf ties FIFO baselines on rework wait and throughput; beats random on rework_p95_wait (18 vs 34) and churn (0.443 vs 0.671). Loses on area_collisions to random again.

## s7_diamond_layers — `/tmp/kerfsim-runs/scenarios/s7_diamond_layers.yaml`

**Theme:** Layered diamond DAG (root → 4 mid → 4 mid2 → 2 collectors → final). 5 agents.

| Metric | kerf | random | fifo-bead | fifo-work |
|---|---|---|---|---|
| `work_completed` | 21 | 21 | 21 | 21 |
| `top_of_queue_churn` | 0.299 | **0.293** | 0.299 | 0.299 |
| all other metrics | identical | | | |

**Verdict:** Tie. Layered DAG had enough parallelism width that all policies cleared the same number of beads in the time budget.
