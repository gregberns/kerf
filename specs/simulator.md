# Simulator

> Deterministic queue simulator for kerf's scoring logic. Ships as a separate binary, `kerfsim`, in the same repo.

## Purpose

`kerfsim` exercises the same `internal/queue` package that backs `kerf next`. Given a scenario, a weights configuration, and a seed, it runs a synthetic workload through the queue and reports metrics. The same inputs always produce the same outputs.

The simulator answers questions that are otherwise opinion-driven:

- "If we raise the `rework` weight from 15 to 25, what changes?"
- "Does the current scoring beat FIFO at p95 rework wait?"
- "How much agent time does kerf's ordering cost compared to the baselines?"

It does not change queue behavior or replace any part of kerf. It observes the queue under controlled conditions.

## Relationship to kerf

`kerfsim` is a consumer of `internal/queue`, not a modifier of it.

- The `kerf` binary is unchanged. `kerf next` still reads project state and computes the queue.
- `kerfsim` lives at `cmd/kerfsim/`. A new `internal/sim` package carries the simulator's loop, scenario generator, baselines, and metrics.
- The in-memory bead store inside `internal/sim` mimics the JSON shape produced by `br list --format json` so the existing parser is reused.

The simulator's in-memory store implements the same `BeadSource` interface that the production `br list --format json` parser feeds. Both paths flow through this interface, so the simulator and production CLI cannot diverge on queue behavior. The interface is the single coupling point; if `internal/queue` adds a new query, both implementations gain it.

The simulator's N-agent greedy-pull model is a measurement choice for queue behavior; it does not constitute a kerf assumption about agent topology in production.

No spec elsewhere in kerf depends on the simulator. Removing `kerfsim` would not affect any other behavior.

## CLI

`kerfsim` is invoked separately from `kerf`. It has no shared state, no shared config, and no implicit project context.

```
kerfsim run <scenario.yaml> [--weights w.yaml] [--seed N] [--runs N] [--out dir/]
kerfsim diff <runA-dir/> <runB-dir/>
kerfsim sweep <scenario.yaml> --param weights.rework=5..30:5    # Phase 2
```

### `kerfsim run`

Executes one scenario and writes a run directory.

- `--weights` — path to a weights YAML file. If omitted, the defaults from [coordination.md](coordination.md) apply.
- `--seed` — overrides the seed in the scenario file. Useful for repeat runs without editing the scenario.
- `--runs N` — repeats the simulation with seeds `seed, seed+1, …, seed+N-1`, producing one run directory per seed plus an aggregate summary. The scenario generator re-runs per seed, so the dependency DAG, bead counts, and pre-rolled durations all vary across seeds. `kerfsim diff` of two `--runs` directories reports median plus p10/p90 across the N runs.
- `--out` — output directory. Defaults to a timestamped directory under the current working directory.

### `kerfsim diff`

Compares two run directories produced by `kerfsim run`. Reports each metric side by side with a delta column. Reads `summary.json` from each side; does not re-execute scenarios.

When the runs come from `--runs N` aggregates, `diff` reports median plus p10/p90 across the included runs.

### `kerfsim sweep`

Phase 2. Runs the same scenario across a parameter range (one weight at a time), producing a table of metric values vs. parameter value. Useful for sensitivity analysis.

## Scenario File

A scenario YAML captures everything needed to reproduce a run. Two scenario files are comparable if they are byte-identical (modulo a `--seed` override).

### Schema

```yaml
# scenario.yaml

# Top-level seed. Splits into named sub-seeds at runtime (see Determinism).
seed: 42

# Simulation clock cap, in ticks. A tick is one unit of work-unit time.
ticks: 10000

# Number of agents pulling from the queue. Each idle agent calls
# `kerf next` greedily. Default range: 1–10.
agents: 3

# Works present at the start of the scenario.
works:
  - codename: amber-fox
    areas: [cli, jig-system]
    deps: []
    bead_count: 8
  - codename: bright-mole
    areas: [bench-storage]
    deps: [amber-fox]
    bead_count: 5

# Bead arrival schedule (rework or late-arriving work). Two forms:
#   - generator spec: kerfsim creates arrivals from a distribution
#   - explicit list: each entry has a tick and a bead definition
bead_arrivals:
  generator:
    rework_rate_per_tick: 0.002
    target_works: [amber-fox, bright-mole]
  # OR
  # explicit:
  #   - tick: 1200
  #     work: amber-fox
  #     labels: [rework:true]

# Duration model. Phase 1 supports log-normal only.
# Durations are pre-rolled at scenario creation time and stored in the
# run directory; swapping weights does not change the work itself.
agent_model:
  duration:
    kind: lognormal
    median_ticks: 30
    sigma: 0.8
```

### Fields

| Field | Meaning |
|---|---|
| `seed` | Top-level seed. Drives all randomness via sub-seeds. |
| `ticks` | Simulation clock cap. Run stops at or before this tick. |
| `agents` | Number of concurrent agents. |
| `works` | Initial work items, each with codename, areas, deps, and bead count. |
| `bead_arrivals` | How and when beads arrive after the start. Generator or explicit list. |
| `agent_model.duration` | Distribution used to pre-roll bead durations. |

Unknown keys are ignored — forward-compatible with later schema additions.

### Validation Rules

The scenario loader enforces:

- Required fields: `seed`, `ticks`, `agents`, `works`, `bead_arrivals`, `agent_model.duration`.
- `agents` is in `[1, 10]`. Values outside this range are a hard error.
- `bead_arrivals` carries exactly one of `generator` or `explicit`. Both, or neither, is a hard error.
- `agent_model.duration.kind` is recognized. An unknown `kind` is a hard error.
- Units: durations are expressed in ticks. `sigma` is the log-normal shape parameter (dimensionless). Exactly one of `mean_ticks` or `median_ticks` is provided; both, or neither, is a hard error. For a log-normal distribution with shape parameter `sigma`, the conversion between the two is: `median = exp(mu)` and `mean = exp(mu + sigma^2/2)`. The generator computes `mu` from whichever location parameter the scenario provides.

#### `bead_arrivals.explicit` Schema

When `bead_arrivals.explicit` is used, each entry has the field set:

| Field | Type | Required | Meaning |
|---|---|---|---|
| `tick` | int | yes | Tick at which the bead arrives. |
| `work` | string (codename) | yes | Codename of the work the bead belongs to. Must reference a known work; an unknown codename is a hard error during validation. |
| `labels` | `[string]` | no | Optional bead labels (e.g. `rework:true`). |
| `bead_id` | string | no | Optional explicit bead identifier. If absent, the generator assigns one. |

Unknown top-level keys are ignored (forward-compat). Unknown keys inside a validated subtree are also ignored.

### Synthetic Generator

`kerfsim` ships three canned scenarios in Phase 1:

- `small-linear` — a handful of works with sequential dependencies.
- `wide-fanout` — many works with shallow dependencies that share areas.
- `rework-heavy` — moderate work count with a high rework arrival rate.

The synthetic generator produces 10–80 works with a clustered dependency graph and log-normal bead counts. Defaults: 30 works, 3 agents.

#### Generator Parameters

Defaults (configurable in the scenario file):

- Epic count: 3–6.
- Intra-epic edge probability: 0.6.
- Inter-epic edge probability: 0.05.
- Per-work bead count: log-normal distribution with median 12.

### Real-Project Import (Phase 2)

`kerfsim import <project>` snapshots a live project — its works, areas, dependencies, and current bead counts — into a scenario file. Phase 1 has no real-project import.

## Weights File

The weights file uses the same schema as the `queue:` section of `project.yaml` (see [coordination.md](coordination.md)).

```yaml
# weights.yaml
fan_out: 10.0
momentum: 5.0
creation: 0.1
rework: 15.0
```

When `--weights` is omitted, the defaults from `coordination.md` apply. The full effective weights are written into the run directory so every run is self-contained.

## Run Output

Each `kerfsim run` produces a directory containing:

```
{out-dir}/
  summary.txt        # compact human-readable view
  summary.json       # canonical machine-readable summary
  events.jsonl       # one JSON event per line — sufficient to replay the run
  scenario.yaml      # copy of the scenario used
  weights.yaml       # copy of the effective weights
```

### `summary.json`

The canonical summary. Contains the metric table (see below), the warmup window, the agent count, the tick count, and the scenario and weights identifiers (file hashes). `kerfsim diff` reads this file and depends on a stable shape.

`summary.json` contains two top-level views of every metric: a `full` block (computed across all events of the run) and a `warmup` block (computed inside the warmup window only). When `warmup_skipped: true`, the `warmup` block carries the same metrics as `full` because no separation is possible. `kerfsim diff` compares the `full` block by default.

Minimal example:

```json
{
  "seed": 42,
  "agents": 3,
  "ticks": 10000,
  "wall_ticks": 7421,
  "warmup_ticks": 1000,
  "warmup_skipped": false,
  "scenario_sha256": "ab12…",
  "weights_sha256": "cd34…",
  "metrics": {
    "work_completed": 28,
    "work_total": 30,
    "agent_idle_pct": 0.18,
    "agent_ticks_total": 22263,
    "rework_p50_wait": 42,
    "rework_p95_wait": 310,
    "top_of_queue_churn": 0.22,
    "goal_completion_1d": 11,
    "goal_completion_3d": 24,
    "goal_completion_7d": 28,
    "priority_inversions": 6,
    "area_collisions": 3
  },
  "baselines": {
    "random":     { "...": "same shape as metrics" },
    "fifo-bead":  { "...": "same shape as metrics" },
    "fifo-work":  { "...": "same shape as metrics" }
  }
}
```

### `summary.txt`

A rendered view of `summary.json`. Compact, intended for terminal reading.

### `events.jsonl`

The audit trail. One JSON object per line. Event kinds in Phase 1:

- `dispatch` — an agent took a bead.
- `complete` — a bead finished.
- `arrival` — a bead entered the queue.
- `queue_snapshot` — periodic snapshot of the queue head. Emitted at a configurable cadence; the default cadence is once per mutating event (dispatch, arrival, completion). When the cadence is reduced, `top_of_queue_churn` is computed only over emitted snapshots.

One-line examples (field names are stable; ordering within a line is canonical):

```
{"tick": 0,    "kind": "arrival",        "bead_id": "amber-fox/b1", "work": "amber-fox", "rework": false}
{"tick": 12,   "kind": "dispatch",       "bead_id": "amber-fox/b1", "agent_id": 0, "score": 42.5}
{"tick": 47,   "kind": "complete",       "bead_id": "amber-fox/b1", "agent_id": 0, "duration": 35}
{"tick": 1000, "kind": "queue_snapshot", "head": ["amber-fox/b3", "bright-mole/b1"], "depth": 14}
```

Replaying `events.jsonl` reproduces the run state. The event log is what makes `events.jsonl` byte-identical across runs with the same inputs.

### `--format=json` Flag

`kerfsim run --format=json` writes the summary as JSON to stdout in addition to the run directory. Useful for piping into other tools.

## Metrics

Metrics are flat. No composite score. Each metric is reported on its own; readers compose meaning across them.

| Metric | Meaning |
|---|---|
| `work_completed` | Works terminal at run end, over total works. |
| `wall_ticks` | Tick at which the run stopped. |
| `agent_idle_pct` | Fraction of agent-ticks where no bead was assigned. |
| `agent_ticks_total` | Total agent-ticks consumed — the "agent-hours" cost. |
| `rework_p50_wait` | Median wait time, in ticks, for a rework-tagged bead between arrival and dispatch. |
| `rework_p95_wait` | 95th-percentile wait time for rework-tagged beads. |
| `top_of_queue_churn` | Fraction of consecutive `next` calls where the top-ranked bead changed. High values indicate oscillation. |
| `goal_completion_1d`, `goal_completion_3d`, `goal_completion_7d` | Count of works terminal at fixed deadlines, expressed in scenario ticks. |
| `priority_inversions` | Count of dispatches where a new-work bead was selected while an older rework bead remained available. |
| `area_collisions` | Count of intervals where two agents were assigned beads in the same area simultaneously. |

`agent_idle_pct` and `agent_ticks_total` are reported together because either alone misrepresents cost — a policy can beat another on completion while consuming far more agent time.

### Metric Definitions

A few metrics have precise definitions worth pinning, since the values are sensitive to them.

- **`top_of_queue_churn`** — after every event that mutates the bead store (dispatch, arrival, completion), the top-ranked bead is computed. Churn is the count of such events where the top changed divided by the total number of mutating events, evaluated inside the post-warmup window. The first mutating event establishes the initial top; the numerator counts changes starting from the second mutating event onward.
- **`agent_idle_pct`** — agent idle time accumulates as `(event_tick - prev_event_tick) * idle_agent_count` at each event boundary. The reported value is `total_idle_agent_ticks / (wall_ticks * num_agents)`, evaluated inside the post-warmup window. When `warmup_skipped: true`, the denominator uses full-run `wall_ticks * num_agents`.
- **`priority_inversions`** — count of dispatch events where the selected bead is new-work and at least one rework bead was queue-eligible (dependencies met, not in-progress) at the time of dispatch with a lower arrival tick. Ties on arrival tick are broken by `bead_id` ascending.
- **`area_collisions`** — count of distinct concurrent-overlap intervals between agent pairs on the same area. Incremented once when two agents become concurrently active in the same area; not proportional to overlap duration. If a pair separates and later re-overlaps on the same area, that is a new collision event.

### Warmup Window

Metrics are reported on the **post-warmup window**. The warmup window is `min(0.1 * ticks, 0.1 * wall_ticks)` and is configurable per scenario.

If `wall_ticks < ticks * 0.2`, the post-warmup window may be empty. In that case metrics are reported on the full window and `summary.json` records `warmup_skipped: true`.

The pre-warmup window is excluded because the queue is still filling and metrics are noisy. The full and warmup-only views are both written to `summary.json` so a reader can inspect either.

## Baselines

Every `kerfsim run` also runs the scenario under fixed baseline policies. Without baselines, absolute metric values carry no interpretation.

### Phase 1 (mandatory)

- `random` — uniformly random selection from available beads.
  - Seed source: a dedicated `baseline_random` sub-seed, derived per the Phase 1 sub-seed rule.
  - Ordering key: a uniform draw over the eligible bead set.
  - Tiebreakers: none needed — selection is a single draw.
- `fifo-bead` — oldest available bead first.
  - Seed source: deterministic; no randomness.
  - Ordering key: `arrival_tick` ascending.
  - Tiebreakers: `bead_id` ascending.
- `fifo-work` — oldest work first, then any available bead from it.
  - Seed source: deterministic; no randomness.
  - Ordering: works are ordered by `work_created_tick` ascending, with `work_codename` ascending as tiebreaker. Within the selected work, beads are ordered by `arrival_tick` ascending, with `bead_id` ascending as tiebreaker.

### Phase 2

- `rework-first-else-fifo` — rework-tagged beads first, then FIFO.

Each baseline is reported alongside the kerf-weighted ordering in `summary.json`. `kerfsim diff` shows both the kerf-vs-kerf comparison and the kerf-vs-baseline comparison.

## Loop Mechanics

The simulator is **event-driven**.

- A min-heap holds upcoming events: `bead-complete`, `arrival`, `agent-free`.
- Each iteration pops the earliest event, advances the simulation clock, dispatches any newly-idle agents, and pushes any resulting events.
- A "tick" is one unit of work-unit time. The simulation clock has no relationship to wall-clock time.

### Dispatch

Each idle agent calls into `internal/queue` greedily. The queue re-reads the in-memory bead store on every call; this mirrors how `kerf next` works in production.

### Event Ordering

When multiple events share the same tick, they are processed in this canonical order:

1. `bead-complete`
2. `bead-arrival`
3. `agent-free`

Within the same kind at the same tick, events are ordered by `(agent_id, bead_id)` ascending. This ordering is part of determinism — `events.jsonl` is byte-identical across implementations that respect it.

When multiple agents become idle at the same tick, they are processed in `(tick, agent_id)` ascending order, each calling `kerf next` in turn. When the queue returns multiple candidates with identical computed scores, the selection draws from the `tiebreak` sub-seed rather than relying on map-iteration order.

Agent idle time accumulates as `(event_tick - prev_event_tick) * idle_agent_count` at each event boundary; `agent_idle_pct = total_idle_agent_ticks / (wall_ticks * num_agents)`.

### Stop Conditions

The run stops when any one of these is true:

- All works are terminal.
- The simulation clock reaches `ticks`.
- All agents are idle AND no future events remain in the heap — no scripted arrivals remaining, no in-flight beads to complete.

### Pre-rolled Durations

Bead durations are pre-rolled at scenario creation and stored as part of the scenario state. Two runs of the same scenario with different weights see identical durations — only the dispatch ordering differs. This keeps weight comparisons clean.

Stochastic mode (Phase 2) re-samples durations per run so that `--runs N` produces variance bands.

## Determinism

Same scenario + same weights + same seed → byte-identical `summary.json` and `events.jsonl`.

### Seed Splitting

The top-level seed splits into named sub-seeds, each used by exactly one subsystem:

| Sub-seed | Used by |
|---|---|
| `gen` | Synthetic scenario generator. |
| `dur` | Duration pre-rolling. |
| `events` | Probabilistic events (arrivals, etc.). |
| `tiebreak` | Score-tie resolution when the queue returns multiple candidates with identical computed scores. |
| `baseline_random` | The `random` baseline's selection draws. |

Sub-seeds derive from the top-level seed as:

```
sub_seed[name] = SHA256(top_seed_bytes || name_bytes)[:8]   # interpreted as uint64
```

where `top_seed_bytes` is the top-level seed encoded as 8 big-endian bytes and `name_bytes` is the ASCII name of the sub-seed. The named sub-seeds in Phase 1 are `gen`, `dur`, `events`, `tiebreak`, and `baseline_random`. This derivation is fixed and part of the determinism guarantee — the same top-level seed means the same sub-seeds across simulator versions.

### Wall Clock Is Not State

The simulator does not read the wall clock during the run. Output filenames may include timestamps for human convenience, but no simulation state — and nothing in `summary.json` or `events.jsonl` — depends on wall-clock time.

## Confidence Intervals

In Phase 1, `--runs N` produces N deterministic runs with seeds `seed, seed+1, …, seed+N-1`. The scenario generator re-runs per seed, so the dependency DAG, bead counts, and pre-rolled durations all vary across seeds — variance bands are real even in Phase 1. Within a single seed there is no variance. `kerfsim diff` reports median plus p10/p90 across the included runs.

In Phase 2, **stochastic-duration mode** additionally re-samples bead durations from the distribution per run, widening the variance bands without changing the DAG.

## Fidelity Layers

The simulator gains realism in layers. Phasing is explicit so each layer can be evaluated before the next is built.

| Layer | Phase | Description |
|---|---|---|
| Pre-rolled task durations | 1 | Log-normal, fixed at scenario creation. |
| Merge / integration cost | 2 | Per-bead completion has a tail: a baseline merge time plus a conflict factor when two beads touched overlapping areas in overlapping windows. Merges are hard-serialized — two parallel completions queue. This is the load-bearing fidelity layer; it is what makes "something might or might not happen at each tick, and parallel completions delay subsequent actions" emerge naturally. |
| Per-task startup latency | 2 | Real agents take 60–90s to roll a task. All policies pay it, so it likely does not change rankings; worth measuring. |
| Stochastic durations | 2 | Re-sample durations from the distribution per run for variance bands. |
| Calibration against a real project | 2 | Snapshot a live project, simulate it, compare. If the simulator diverges badly, the model is wrong. Phase 2 uses hand-tuned distributions. |
| Claude-log-derived durations | 3 | Empirical distributions built from session-log bead→timestamp pairs. Deferred until Phase 2 calibration shows empirical data is needed. |
| Area affinity, intra-work bead deps | 3 | Specialist agents; real bead graphs inside a work. |
| Adversarial scenarios | 3 | Hand-crafted to break specific signals (rework storm during wide fan-out, etc.). |

### What Phase 1 Does Not Include

Phase 1 is the simplest viable comparator on synthetic data. It does **not** include:

- A merge or integration cost model.
- Per-task startup latency.
- Stochastic-duration mode (`--runs N` is deterministic in Phase 1).
- `kerfsim import` for real projects.
- `kerfsim sweep`.
- The rework-first baseline.
- Adversarial scenarios.
- Claude-log-derived duration data.
- Intra-work bead dependencies or area-affine agents.

Layering these onto the event-driven tick loop is the work of Phase 2 and Phase 3.

## Testing

Determinism is the central property. Tests cover:

- Same inputs → byte-identical `events.jsonl` and `summary.json`.
- Generator distributions match expected shape under fixed seed.
- Event ordering inside the tick loop is stable.
- Each baseline policy produces the expected ordering on a small fixture.

See [testing.md](testing.md) for the overall test strategy.
