# SIM-PROPOSALS-2026-05-14 — Reconstruction

> Reconstructed from kerf session transcripts `8215ce36-…` and `55603e31-…` by an investigation agent on 2026-05-15. The original `SIM-PROPOSALS-2026-05-14.md` was authored by two background agents during the May 14 session but never committed.

## Scenarios

Original proposal mentioned three to four canned scenarios.

From the proposal:
> "Ship 3–4 canned scenarios: `small-linear.yaml`, `wide-fanout.yaml`, `rework-heavy.yaml`, `real-export.yaml`."

From Plan 007 as shipped:
> Three canned scenarios: `small-linear`, `wide-fanout`, `rework-heavy`.

The fourth (`real-export.yaml`) was deferred. Plan 012 picks it back up.

Plan 008 later added seven exploratory scenarios (s1–s7).

## CLI & Workflow Design

**Minimal invocation:**
```
kerfsim run scenario-A.yaml
```

**Flags proposed:**
- `--weights weights.yaml` — override scoring weights
- `--seed 42` — override seed
- `--out runs/<name>/` — defaults to `runs/<timestamp>-<scenario>-<weights-hash>/`
- `--runs 20` — repeat with seed, seed+1, … for variance bands
- `--quiet` — exit code only
- `--verbose` — stream events

**Progress line:**
```
scenario-A  seed=42  tick 1180/2000  done 34/61  idle-agents 1  rework-waiting 0
```

**Comparison command:**
```
kerfsim diff runs/<a>/ runs/<b>/
```
Side-by-side metric table with deltas. Same seed → identical work stream → differences attributable to weights. For noise: `--runs 20` shows mean ± stdev.

**Phase 2 sweep:**
```
kerfsim sweep scenario.yaml --param weights.rework=5..30:5
```

## Loop & Metrics

**Tick model:** event-driven, not wall time. Clock jumps to next interesting event.

**Agents:** N parallel workers (default 3, range 1–10). Each calls `kerf next`, picks top greedily. Optional `picky` mode lets a worker skip top if another worker is already in that area.

**State store:** in-memory bead store mimicking `br list --format json`. No subprocess. Wraps the same `internal/queue` package the real CLI uses.

**Stopping conditions (first of):**
- All works terminal
- Sim-clock cap (default 30 sim days)
- Idle threshold (no state change for 4 sim hours)

**Metrics (flat table, no composite score):**
- `work_completed`, `wall_ticks`, `agent_idle_pct`, `agent_ticks_total`
- `rework_p50_wait`, `rework_p95_wait`
- `top_of_queue_churn`
- `goal_completion_1d/3d/7d`
- `priority_inversions`
- `area_collisions`

**Baselines (mandatory):**
- Random order
- FIFO by bead creation
- FIFO by work creation
- (Phase 2) "rework always first, else FIFO"

## Real-Data Ingestion (Phase 2/3 in original; Plan 012 here)

From the proposal:
> "`kerfsim import <project>` — snapshot a live project into a scenario."

> "Claude-log-derived duration distributions (extract bead-to-timestamp pairs from session logs, build empirical distributions, scale wall-clock to ticks)."

Phase 1 designed for **pre-rolled durations** (log-normal, median ~30 min) drawn at scenario-creation time, not real data.

## Open Decisions (deferred from May 14)

1. **Agent model fidelity** — derive durations from real data, or hand-pick?
2. **"Rework waiting too long" threshold** — suggest p95 rework > 2× p95 new-work = bad.
3. **Mid-run weight swap** — out of scope v1.
4. **`kerfsim import <project>`** — probably yes, later.
5. **One binary vs separate `kerfsim` repo.** (Decided: one binary.)
6. **Build phase 1 = just the metric loop on synthetic data?** (Decided: yes; real data deferred.)
7. **The four canned scenarios — which mix matters most for tuning weights?** (Decided implicitly: three shipped, fourth deferred.)

## Gap vs Shipped Plan 007

Shipped:
- CLI shape (`run`, `diff`)
- Scenario YAML format + three canned scenarios
- Event-driven tick loop, N greedy agents, in-memory store
- All Phase 1 metrics
- Three baselines (random, FIFO-bead, FIFO-work)
- Warmup window (first 10% of ticks, configurable)
- summary.txt + summary.json + events.jsonl
- Determinism (single seed → byte-identical results)
- `--runs` with confidence intervals (median + p10/p90)
- One-binary in `cmd/kerfsim/`

Deferred (Plan 012 picks these up):
- `real-export.yaml` scenario
- `kerfsim import <project>`
- Claude-log-derived duration distributions
- Merge/integration cost model
- Per-task startup latency model
- Stochastic-duration mode

Deferred (Plan 011 picks up the rest):
- Fourth baseline ("rework-first-else-FIFO")
- `kerfsim sweep` flag
