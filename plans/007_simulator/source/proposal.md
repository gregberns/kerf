# Tier 3 simulator — two design proposals (2026-05-14)

Two background agents produced complementary proposals for a kerf-queue simulator. Workflow/UX is one perspective; data model + loop + analysis is the other. They don't conflict — read them as two halves of one design.

---

## Proposal 1 — Workflow / UX

### Kicking off a run

Minimal invocation: `kerfsim run scenario-A.yaml`. One positional argument — a scenario file holding everything needed for reproducibility.

Optional flags:
- `--weights weights.yaml` — override scoring weights (the thing being tested).
- `--seed 42` — override the seed baked into the scenario.
- `--out runs/<name>/` — defaults to `runs/<timestamp>-<scenario>-<weights-hash>/`.
- `--runs 20` — repeat with seed, seed+1, … for noise smoothing.

No interactive prompts. No multi-file config dirs.

### While it runs

Default: single progress line updating in place:
```
scenario-A  seed=42  tick 1180/2000  done 34/61  idle-agents 1  rework-waiting 0
```
On finish, print path to the report. `--quiet` for exit code only. `--verbose` streams events. Full event log is always written.

### Outputs

Every run produces a directory with:
- `summary.json` / `summary.txt` — headline numbers
- `events.jsonl` — every dispatch, completion, rework arrival, queue snapshot
- `scenario.yaml` and `weights.yaml` — copies of exact inputs

Summary is a flat metric table with no composite score:
```
work_completed         58 / 61
wall_ticks             1,742
agent_idle_pct         8.3%
rework_avg_wait        14 ticks    (target: <30)
rework_p95_wait        41 ticks
priority_inversions    3
new-work-before-rework 1
```

Resist any single "score" — different weights trade these against each other; the whole point is seeing the trade.

### Comparing runs

```
kerfsim run scenario-A.yaml --weights fanout-10.yaml --seed 42
kerfsim run scenario-A.yaml --weights fanout-20.yaml --seed 42
kerfsim diff runs/<a>/ runs/<b>/
```

`diff` prints a side-by-side metric table with deltas. Same seed → identical work stream → any difference is the weights' fault. For noise: `--runs 20` produces 20 seeds; diff shows mean ± stdev.

### Scenario file (the unit of reproducibility)

Single YAML:
- `seed`, `ticks`, `agents` (count)
- `works` — codename, areas, fan-out (deps), bead count, dependencies on other works
- `bead_arrivals` — real `br list` export OR generator spec (`new_work_rate`, `rework_rate`, `rework_targets`)
- `agent_model` — bead durations (fixed, uniform range, or per-area distribution)

Two scenarios comparable iff scenario files are byte-identical.

Ship 3–4 canned scenarios: `small-linear.yaml`, `wide-fanout.yaml`, `rework-heavy.yaml`, `real-export.yaml`.

### Open questions
- Agent model fidelity — derive durations from real data, or hand-pick?
- "Rework waiting too long" threshold — suggest p95 rework > 2× p95 new-work = bad.
- Mid-run weight swap — out of scope v1.
- `kerfsim import <project>` to snapshot a live project into a scenario — probably yes, later.

---

## Proposal 2 — Inputs, loop mechanics, analysis

### Inputs / data model

**Synthetic project (scenario):**
- 10–80 works (default 30). Each has codename, areas (sampled from ~8 tags), creation timestamp, `depends_on` (must-complete-first).
- Dependency DAG built by clustering: 3–6 epics, intra-epic edges dense (60% chance to older sibling), inter-epic sparse (5%). Mimics real feature work, not uniform random.
- 5–40 beads per work (log-normal, mean ~12). Each bead has id, `new-work` or `rework` label, area tag (usually inherits from parent), and a duration.

**Bead durations.** Pre-rolled at scenario creation time, log-normal median ~30 min, long tail (rare 4-hour beads). Pre-rolling means swapping policy doesn't change the work itself — clean A/B. Runtime noise (flakiness/retries) layered on as a seeded per-bead multiplier.

**Mid-run events.** Two channels:
- **Scripted timeline** (deterministic, in scenario file): "at sim-minute 120, file 4 new rework beads against work X." For regression tests.
- **Probabilistic** (seeded): each completed bead has ~5% chance of spawning a rework on the same work; each work has ~1%/hr chance of an external urgent rework ("bug filed").

**Determinism.** Same seed → same scenario, durations, events, agent tie-breaking. Wall clock never part of state.

### Loop mechanics

**Tick = work-unit time, not wall time.** Clock jumps to next interesting event (bead completion, scripted event, agent freeing up). Fast and exact.

**Agents.** N parallel workers (default 3, range 1–10). Each idle worker calls `kerf next`, picks top entry. Default greedy. Optional `picky` mode lets a worker skip top if another worker is already in that area (simulates collision avoidance). No content-based rejection.

**State store.** In-memory bead store mimicking `br list --format json` shape. Wraps the same `internal/queue` package the real CLI uses. **No shim, no subprocess, no real `br`.** Single biggest speed + determinism win. State transitions are direct method calls; queue re-reads on every `next`.

**Stopping.** First of: all works terminal, sim-clock cap (default 30 sim days), or idle threshold (no state change for 4 sim hours).

### Output / analysis

**Per-run metrics.**
- Time to drain 50%, 90%, 100% of works.
- Agent idle fraction.
- Rework latency — distribution, not just mean.
- Top-of-queue churn — how often #1 changed between consecutive `next` calls. High = oscillating scoring.
- Goal completion at fixed deadlines (1d, 3d, 7d).
- Area collision count.

**Baselines (critical).** Every scenario also runs under (a) random order, (b) FIFO by bead creation, (c) FIFO by work creation, (d) "rework always first, else FIFO." Without these, absolute numbers are meaningless.

**Aggregation.** Each weight config runs against ~50 scenarios. Report median + 10th/90th percentile per metric — averages hide failure modes. Golden baseline file pins current numbers; CI flags regressions.

**One-shot questions the analysis should answer.**
- "Raising rework weight 15 → 25: latency change, throughput change, churn change — three numbers."
- "Does scoring beat FIFO on 90th-percentile completion time?"
- "Which scenarios does kerf lose on, and what do they have in common?"

### Open questions
- Agents have area affinity (specialty) or uniform identity?
- Bead-level deps inside a work — model or assume any bead in an open work is dispatchable? Real beads have deps; ignoring overstates parallelism.
- Model agent context bloat (degraded after K beads)? Inspiration project cared about this; for queue-ordering eval may be noise.
- Dropped bead — same bead reopened or new rework bead? Affects rework-latency measurement.

---

## How the two proposals fit

Proposal 1 = the *outside* (CLI shape, scenario as file, output shape).
Proposal 2 = the *inside* (data model, loop, metrics).

Where they agree: scenario is one YAML, seed is sacred, in-memory store wraps real `internal/queue`, baselines compared via `kerfsim diff`. No code yet — both stayed at design level.

Greg's review needed on:
- One-binary or separate `kerfsim` repo?
- Build phase 1 = just the metric loop on synthetic data (no `br` import), defer real-project import?
- The four canned scenarios — which mix matters most for tuning weights?
