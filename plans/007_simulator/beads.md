# Plan 007 — Phase 1 Implementation Beads

## Overview

Phase 1 of `kerfsim`: deterministic queue simulator as a second binary, sharing `internal/queue` with kerf. This breakdown covers **14 beads across 5 layers** (L0 → L4), from leaf utilities through CLI wiring and a final determinism E2E test.

The previously-proposed BeadSource interface extraction (`internal/queue`) is **not** part of Phase 1. `queue.Compute` keeps its current signature `(works []*spec.SpecYAML, beadsByWork map[string]beads.EpicSummary, weights Weights) []Entry`. The simulator couples to it via adapter methods on `internal/sim/store.Store` that produce that exact tuple. If Phase 2's `import`/`sweep` work needs broader sharing, an interface can be extracted then — not now.

Phase 2 and Phase 3 fidelity layers (merge cost, startup latency, stochastic mode, `import`, `sweep`, rework-first baseline, adversarial scenarios, Claude-log import, calibration) are explicitly **not** in this plan.

## Dependency Graph

```
L0 (leaves, parallel)
  ┌───────────────────────┐  ┌──────────────────────────┐  ┌──────────────────────┐
  │ B1  sim/seed (subseed │  │ B2  sim/scenario (types  │  │ B3  sim/event (heap, │
  │     SHA256 derivation)│  │     + YAML loader +      │  │     event kinds,     │
  │                       │  │     validator)           │  │     ordering rule)   │
  └───────────────────────┘  └──────────────────────────┘  └──────────────────────┘
  ┌──────────────────────────────────────────────────────────────────────────────┐
  │ B12 canned scenarios (small-linear, wide-fanout, rework-heavy YAML files)    │
  │     — thin; depends on B2 schema only; ships under top-level /scenarios/     │
  └──────────────────────────────────────────────────────────────────────────────┘

L1 (depends on L0)
  ┌──────────────────────────────┐  ┌──────────────────────────────────────────────┐
  │ B5  sim/generator (clustered │  │ B6  sim/store (in-memory bead store,         │
  │     DAG + lognormal counts + │  │     mirrors br list shape; queue-adapter     │
  │     pre-rolled durations +   │  │     methods produce ([]*spec.SpecYAML,       │
  │     scripted/prob events)    │  │     map[string]beads.EpicSummary); Store.From│
  └──────────────────────────────┘  │     factory returns fresh store per policy)  │
       depends: B1, B2              └──────────────────────────────────────────────┘
                                         depends: B2

  ┌──────────────────────────────┐
  │ B8  sim/baselines (random,   │
  │     fifo-bead, fifo-work —   │
  │     each implements          │
  │     policy.Policy)           │
  └──────────────────────────────┘
       depends: B1, B6 (Store adapter methods)

  ┌──────────────────────────────┐
  │ B9a sim/metrics (collector + │
  │     warmup math + all Phase-1│
  │     metric formulas; pure,   │
  │     consumes event stream)   │
  └──────────────────────────────┘
       depends: B3

L2 (engines)
  ┌──────────────────────────────┐
  │ B7  sim/loop (event-driven   │
  │     tick loop, agent         │
  │     dispatch, stop cond.);   │
  │     defines policy.Policy    │
  │     interface AND loop Hooks │
  │     interface as deliverables│
  └──────────────────────────────┘
       depends: B1, B3, B6

  ┌──────────────────────────────┐
  │ B9b loop hook wiring         │
  │     (implements loop.Hooks   │
  │     against metrics.Collector│
  │     — thin)                  │
  └──────────────────────────────┘
       depends: B7, B9a

L3 (orchestration + outputs)
  ┌──────────────────────────────┐  ┌──────────────────────────────────────────────┐
  │ B10 sim/run (orchestrator:   │  │ B11 sim/output (summary.txt, summary.json,   │
  │     scenario → loop ×        │  │     events.jsonl, input copies, sha256s)    │
  │     {kerf, 3 baselines})     │  │                                              │
  │     uses Store.From to       │  │                                              │
  │     isolate per-policy state │  │                                              │
  └──────────────────────────────┘  └──────────────────────────────────────────────┘
       depends: B5, B6, B7, B8, B9a, B9b   depends: B9a

L4 (CLI + E2E)
  ┌──────────────────────────────┐  ┌──────────────────────────────────────────────┐
  │ B13 cmd/kerfsim run          │  │ B14 cmd/kerfsim diff                         │
  │     (--weights/--seed/--runs │  │     (median + p10/p90 across runs)           │
  │     /--out/--format=json);   │  │                                              │
  │     embeds /scenarios/*.yaml │  │                                              │
  └──────────────────────────────┘  └──────────────────────────────────────────────┘
       depends: B10, B11, B12             depends: B11

  ┌──────────────────────────────────────────────────────────────────────────────┐
  │ B15 E2E determinism test (canned scenario × fixed seed → byte-identical      │
  │     summary.json + events.jsonl across 3 invocations) — thin                 │
  └──────────────────────────────────────────────────────────────────────────────┘
       depends: B12, B13
```

> Numbering note: B4 (BeadSource interface extraction) is **resolved with no work needed** and intentionally omitted. Subsequent beads keep their original numbers to preserve cross-references; B9 is split into B9a + B9b.

## Inter-Package Import Map

```
internal/queue
  └── Compute(works, beadsByWork, weights) — UNCHANGED in Phase 1.

internal/sim/seed        (B1)  — pure, no kerf deps
internal/sim/scenario    (B2)  — pure, uses gopkg.in/yaml.v3
internal/sim/event       (B3)  — pure
internal/sim/generator   (B5)  — imports seed, scenario
internal/sim/store       (B6)  — imports beads (EpicSummary), spec (SpecYAML);
                                  exposes Adapter methods that return
                                  ([]*spec.SpecYAML, map[string]beads.EpicSummary)
                                  for direct hand-off to queue.Compute.
internal/sim/policy      (B7)  — defines Policy interface; pure
internal/sim/loop        (B7)  — imports event, store, seed, policy;
                                  defines Hooks interface here
internal/sim/baselines   (B8)  — imports policy, store, seed
internal/sim/metrics     (B9a) — imports event; pure
                          (B9b) — additionally imports loop (to implement Hooks)
internal/sim/run         (B10) — imports everything above + output
internal/sim/output      (B11) — imports metrics
cmd/kerfsim/run.go       (B13) — imports sim/run, sim/output
cmd/kerfsim/diff.go      (B14) — imports sim/output (json shape only)
cmd/kerfsim/main.go      (B13) — cobra root; embed.FS over /scenarios/*.yaml
/scenarios/*.yaml        (B12) — top-level repo dir; embedded into kerfsim
```

`cmd/kerf/*` and `internal/queue` are **completely untouched** by this plan.

## Cross-Cutting Concerns

| Concern | Defining bead | Consuming beads |
|---|---|---|
| Determinism — same seed → byte-identical outputs | B1 (sub-seed derivation), B3 (event ordering rule) | B5, B7, B8, B9a, B11, B15 |
| Sub-seed names (`gen`, `dur`, `events`, `tiebreak`, `baseline_random`) | B1 | B5, B7, B8 |
| Event ordering: kind priority (complete < arrival < agent-free), then `(agent_id, bead_id)` ascending | B3 | B7 |
| `policy.Policy` interface (the loop's single contract for ordering) | B7 (defines), B8 (baselines), B10 (KerfPolicy wraps queue.Compute) | B7, B8, B10 |
| `loop.Hooks` interface (event/dispatch/snapshot callbacks) | B7 (defines) | B9b (implements) |
| Store adapter methods (`Works() []*spec.SpecYAML`, `SummaryByWork() map[string]beads.EpicSummary`) | B6 | B8, B10 (for KerfPolicy → queue.Compute) |
| `Store.From(*GeneratedWorld) *Store` — fresh isolated store per policy run | B6 | B10 |
| Warmup window `min(0.1 * ticks, 0.1 * wall_ticks)` with `warmup_skipped` fall-through | B9a | B11 |
| Canonical JSON output (stable key order, no wall-clock) | B11 | B15 |

## Per-Bead Specification

### Bead 1 — Sub-seed derivation utility
**Specs:** specs/simulator.md §Determinism, §Seed Splitting
**Package:** `internal/sim/seed`
**Deliverables:**
- `func Derive(topSeed uint64, name string) uint64` — `SHA256(topSeed_be8 || name_bytes)[:8]` interpreted as uint64.
- Named constants for the five Phase-1 sub-seed names: `Gen`, `Dur`, `Events`, `Tiebreak`, `BaselineRandom`.
- Helper `func NewRand(topSeed uint64, name string) *rand.Rand` returning a `math/rand.Rand` seeded from the derived value.
**Tests:**
- Golden values for `(topSeed=42, name="gen"|"dur"|"events"|"tiebreak"|"baseline_random")` — fixed expected uint64s. Determinism is load-bearing.
- Different names produce different sub-seeds; same name + same top-seed always produces the same sub-seed.

### Bead 2 — Scenario types, YAML loader, validator
**Specs:** specs/simulator.md §Scenario File (Schema, Fields, Validation Rules, `bead_arrivals.explicit` Schema)
**Package:** `internal/sim/scenario`
**Deliverables:**
- Go types mirroring the YAML schema: `Scenario`, `Work`, `BeadArrivals` (with `Generator` and `Explicit` mutually-exclusive sub-structs), `ExplicitArrival` (`tick int`, `work string`, `labels []string`, `bead_id string`), `AgentModel`, `Duration`.
- `func Load(path string) (*Scenario, error)` — yaml.v3 unmarshal + `Validate`.
- `func (s *Scenario) Validate() error` enforcing:
  - Required: `seed`, `ticks`, `agents`, `works`, `bead_arrivals`, `agent_model.duration`.
  - `agents` ∈ [1, 10].
  - Exactly one of `bead_arrivals.generator` or `bead_arrivals.explicit`.
  - `agent_model.duration.kind` is recognized (`lognormal` in Phase 1).
  - Exactly one of `mean_ticks` or `median_ticks` is set (hard error if both or neither).
  - Every `explicit[i].work` references a known work codename (hard error otherwise).
  - Every `explicit[i].tick >= 0`.
  - Unknown top-level keys are tolerated (forward-compat); unknown nested keys also tolerated.
- `func (s *Scenario) SHA256() string` — canonical hash of the raw YAML bytes (for `scenario_sha256`).
- Helper that resolves `mu` from whichever of `mean_ticks` / `median_ticks` was provided (per spec conversion formula).
**Tests:**
- Round-trip parse of each canned scenario shape.
- Each validation rule has at least one negative test case (agents=0, agents=11, both arrival forms set, neither set, unknown kind, both mean & median set, neither set, unknown explicit work codename).
- Unknown keys are accepted without error.

### Bead 3 — Event heap and ordering
**Specs:** specs/simulator.md §Loop Mechanics, §Event Ordering
**Package:** `internal/sim/event`
**Deliverables:**
- `type Kind int` constants in priority order: `KindComplete=0`, `KindArrival=1`, `KindAgentFree=2`.
- `type Event struct { Tick int; Kind Kind; AgentID int; BeadID string; Payload any }`.
- `type Heap` — min-heap implementing `container/heap.Interface`. Ordering: `(Tick, Kind, AgentID, BeadID)` ascending. Stable across runs.
- `Push(Event)`, `Pop() Event`, `Peek() Event`, `Len() int`.
**Tests:**
- Insert events with same tick across kinds, verify pop order is `complete → arrival → agent_free`.
- Same tick + same kind: pop order is `(agent_id, bead_id)` ascending.
- Random insertion order, fixed expected pop sequence (table-driven).

### Bead 4 — RESOLVED (no work)
The original plan called for extracting a `BeadSource` interface in `internal/queue`. **Decision:** Phase 1 does not extract a new interface. `queue.Compute` keeps its current signature. The simulator couples via Store adapter methods (B6) that produce the exact tuple `queue.Compute` accepts. If Phase 2 needs broader sharing for `import`/`sweep`, that's when an interface gets pulled out. No work for this bead; preserved in numbering to keep cross-references stable.

### Bead 5 — Synthetic scenario generator
**Specs:** specs/simulator.md §Synthetic Generator, §Generator Parameters
**Package:** `internal/sim/generator`
**Deliverables:**
- `func Generate(s *scenario.Scenario) (*GeneratedWorld, error)` returning works (with DAG edges), pre-rolled bead durations per bead, and scripted/probabilistic event schedule.
- Clustered DAG using spec defaults: 3–6 epics; intra-epic edge probability 0.6; inter-epic edge probability 0.05; cycles rejected.
- Bead counts per work drawn log-normal with median 12 (spec default; overridable in scenario).
- Durations pre-rolled per bead using `seed.Derive(topSeed, "dur")`.
- Scripted arrivals from `bead_arrivals.explicit`; probabilistic arrivals from `generator.rework_rate_per_tick` using `seed.Derive(topSeed, "events")`.
- Generator uses `seed.Derive(topSeed, "gen")` exclusively for structural randomness.
**Tests:**
- Same seed → byte-identical `GeneratedWorld` (compare a stable serialization).
- Generated DAG is acyclic.
- Epic count ∈ [3, 6]; intra/inter edge probabilities verified statistically across many seeds (loose bounds, e.g. ±5%).
- Bead count distribution shape check (mean within expected band).

### Bead 6 — In-memory bead store + queue adapter
**Specs:** specs/simulator.md §Relationship to kerf, §Loop Mechanics (Dispatch)
**Package:** `internal/sim/store`
**Deliverables:**
- `type Store struct { ... }` — in-memory bead store mirroring `internal/beads.Bead` shape (labels, deps, epic, status, arrival_tick, work_created_tick).
- Queue-adapter methods, callable by both KerfPolicy and tests:
  - `Works() []*spec.SpecYAML` — current works snapshot in canonical order.
  - `SummaryByWork() map[string]beads.EpicSummary` — current per-work bead summaries.
  These two together feed `queue.Compute` directly with no further transformation.
- Mutation API for the loop: `AddBead`, `MarkInProgress`, `MarkComplete`. Each mutation is observable for metrics.
- `func From(world *generator.GeneratedWorld) *Store` — factory returning a fresh, independent store seeded from the generated world. Used by B10 to give each of the four policy runs (kerf + 3 baselines) its own mutation-isolated state.
- Snapshot semantics: each adapter call reads current state (queue re-reads on every `next`, per spec).
**Tests:**
- Round-trip: load a generated world via `From`, query `Works()` + `SummaryByWork()`, feed to `queue.Compute`, assert non-empty ordering result.
- Mutation visibility: after `MarkComplete`, the bead no longer appears in `SummaryByWork().InProgress` and the work's completion summary reflects it.
- **Isolation:** `Store.From(w)` returns mutually independent stores — mutating one does not affect another constructed from the same world. Tested explicitly because B10 relies on this property.

### Bead 7 — Event-driven tick loop, Policy + Hooks interfaces
**Specs:** specs/simulator.md §Loop Mechanics, §Event Ordering, §Stop Conditions
**Packages:** `internal/sim/loop`, `internal/sim/policy`
**Deliverables:**
- **`internal/sim/policy` package (new, tiny):** defines the shared contract.
  ```go
  type Policy interface {
      Next(store *store.Store, agentID int) (beadID string, ok bool)
  }
  ```
  Lives in its own package so `loop`, `baselines`, and `run` (KerfPolicy) all import a neutral location.
- **`internal/sim/loop` package:** the tick loop.
  - `type Loop struct { ... }`, `func (l *Loop) Run(ctx, world, store, policy, hooks) (*Trace, error)`.
  - Drives `event.Heap`: pop earliest event, advance clock, mutate store, dispatch idle agents via `policy.Next`, push resulting events.
  - N agents (1–10), greedy: each agent that becomes idle at the same tick is called in `(tick, agent_id)` ascending order.
  - Score-tie resolution (for KerfPolicy) draws from `seed.Derive(topSeed, "tiebreak")`.
  - Stop conditions: all works terminal | sim-clock reaches `ticks` | heap empty AND all agents idle.
  - **`Hooks` interface defined here:**
    ```go
    type Hooks interface {
        OnEvent(e event.Event)
        OnDispatch(agentID int, beadID string, work string)
        OnSnapshot(top string)
    }
    ```
    The loop calls `hooks` methods at the canonical points. `Hooks` may be nil (no-op). This is the **only** seam between loop internals and metrics — B9b implements it.
- Emits a `Trace` (in-memory event log); does **not** write files.
**Tests:**
- Hand-crafted micro-scenario: 2 works, 4 beads, 1 agent, deterministic outcome — verify exact event sequence.
- Same inputs across two `Run` calls produce identical traces.
- Stop-condition coverage: each of the three triggers fires in a dedicated test.
- Event ordering invariant: instrument the heap pop sequence and assert canonical ordering on a same-tick fixture.
- `Hooks=nil` does not panic.

### Bead 8 — Baseline policies
**Specs:** specs/simulator.md §Baselines (Phase 1 section)
**Package:** `internal/sim/baselines`
**Deliverables:**
- Three implementations of `policy.Policy`:
  - `Random` — draws uniformly from eligible beads using `seed.Derive(topSeed, "baseline_random")`.
  - `FIFOBead` — `arrival_tick` asc, tiebreak `bead_id` asc.
  - `FIFOWork` — `(work_created_tick, arrival_tick)` asc, tiebreak `(work_codename, bead_id)` asc.
- Each policy reads from the `*store.Store` adapter methods and returns an eligible `beadID`.
**Tests:**
- Fixture: 5 beads with known arrival/work attributes; each policy produces its documented ordering exactly.
- `Random` is deterministic under fixed seed.

### Bead 9a — Metric collector (pure)
**Specs:** specs/simulator.md §Metrics, §Metric Definitions, §Warmup Window
**Package:** `internal/sim/metrics`
**Deliverables:**
- `type Collector struct { ... }` — pure consumer of an event stream. No dependency on `loop` internals (decoupling allows B9a to ship in parallel with B7).
- Surface (called from B9b's hook adapter):
  - `Observe(e event.Event, view StoreView)` — generic event ingest.
  - `RecordDispatch(...)`, `RecordComplete(...)`, `RecordArrival(...)`, `RecordSnapshot(top string)` — convenience entry points for B9b to forward into.
- Computes all Phase-1 metrics:
  - `work_completed`, `work_total`, `wall_ticks`.
  - `agent_idle_pct` per spec formula. Warmup-skipped denominator uses full-run `wall_ticks * num_agents` (per spec pin).
  - `agent_ticks_total`.
  - `rework_p50_wait`, `rework_p95_wait`.
  - `top_of_queue_churn` — first mutating event sets baseline top; numerator counts changes from the second mutating event onward (per spec pin).
  - `goal_completion_1d/3d/7d`.
  - `priority_inversions`.
  - `area_collisions` — each distinct concurrent-overlap interval counts once; re-overlaps after separation count as new events (per spec pin).
- Warmup window: `min(0.1 * ticks, 0.1 * wall_ticks)`. Fall-through with `warmup_skipped: true` when post-warmup window is empty.
- Emits a `Summary` struct carrying both `full` and `warmup` blocks per spec.
**Tests:**
- Each metric has at least one hand-built event sequence test asserting exact value.
- Warmup window: explicit test for the fall-through case (`wall_ticks` too short) and that `agent_idle_pct` uses full-run denominator in that case.
- `top_of_queue_churn` denominator counts only mutating events; first event is not counted as a change.
- Re-collision after separation increments `area_collisions`.

### Bead 9b — Loop hook wiring (thin)
**Specs:** specs/simulator.md §Loop Mechanics, §Metrics
**Package:** `internal/sim/metrics` (additional file: `hooks.go`)
**Deliverables:**
- `type LoopHooks struct { C *Collector }` implementing `loop.Hooks`.
- `OnEvent`/`OnDispatch`/`OnSnapshot` forward into the matching `Collector` method.
- This is the only file that imports both `loop` and `metrics`; isolates the dependency.
**Tests:**
- Smoke: a fake `loop.Hooks` consumer driven from a synthetic event sequence calls the right Collector methods.
**Note:** Orchestrator should dispatch as a thin bead — small surface, no logic of its own.

### Bead 10 — Run orchestrator
**Specs:** specs/simulator.md §`kerfsim run`, §Outputs (run dir contents)
**Package:** `internal/sim/run`
**Deliverables:**
- `func Run(s *scenario.Scenario, w queue.Weights, opts Options) (*Result, error)`.
- Steps: `generator.Generate` once → for kerf policy and each of the 3 baselines: build a **fresh store** via `store.Store.From(world)` (mutation isolation), build a `metrics.Collector` + `metrics.LoopHooks`, run the loop, harvest the summary.
- `KerfPolicy` lives here: implements `policy.Policy` by calling `store.Works()` + `store.SummaryByWork()` and feeding them to `queue.Compute`, then selecting the top entry (with `tiebreak` sub-seed when multiple top scores tie).
- Assembles a `Result` containing four metric blocks (kerf + 3 baselines), seed, weights, scenario, warmup info, sha256s.
- `--runs N` invoked once per seed in `[seed, seed+N-1]`; returns a `MultiRunResult`.
**Tests:**
- Smoke: small-linear scenario, fixed seed → exact metric values on kerf + 3 baselines (golden values pinned once).
- Identity: kerf and `FIFOBead` policies see identical generated worlds (same durations, same DAG) — verified via store inspection at run start.
- **Isolation:** explicitly assert that each of the four per-policy stores has independent state at end-of-run (e.g. dispatch/complete bead sets differ as expected, no shared-mutation aliasing).
- `--runs 3` produces 3 distinct seed-tagged sub-results.

### Bead 11 — Output writers
**Specs:** specs/simulator.md §Run Output, `summary.json` schema, `events.jsonl` event schemas
**Package:** `internal/sim/output`
**Deliverables:**
- `func WriteRun(dir string, result *Result, inputs Inputs) error`. Writes:
  - `summary.json` — canonical, stable key order; top-level `full` and `warmup` blocks per spec.
  - `summary.txt` — rendered from `summary.json`.
  - `events.jsonl` — one event per line; kinds: `dispatch`, `complete`, `arrival`, `queue_snapshot`. `queue_snapshot` cadence default = every mutating event.
  - Copies of `scenario.yaml` and `weights.yaml` (effective weights, defaults filled in).
- `scenario_sha256` and `weights_sha256` computed from input bytes.
- No wall-clock data in any output content.
- `--format=json` writes `summary.json` to stdout in addition to disk.
**Tests:**
- Golden file: fixed `Result` produces byte-identical `summary.json` and `events.jsonl`.
- All four event kinds round-trip through their canonical line format.
- Effective weights (with defaults filled) are written even when input weights file omits fields.
- `full` and `warmup` blocks both present; `warmup` mirrors `full` when `warmup_skipped: true`.

### Bead 12 — Canned scenario YAML files (thin)
**Specs:** specs/simulator.md §Synthetic Generator (three canned scenarios)
**Location:** **Top-level `/scenarios/`** (not `testdata/`). Embedded into `cmd/kerfsim` via `embed.FS` so `kerfsim run small-linear` works without a file path. Decision: discoverable for users + self-contained binary.
**Deliverables:**
- `/scenarios/small-linear.yaml` — handful of works (5–10) with sequential dependencies, low rework rate.
- `/scenarios/wide-fanout.yaml` — 30 works, shallow deps, shared areas.
- `/scenarios/rework-heavy.yaml` — 20 works, elevated `rework_rate_per_tick`.
- Each is a complete, validated scenario file consumable by `kerfsim run`.
**Tests:**
- Each loads + validates without error via `scenario.Load`.
- (Smoke run under all four policies happens implicitly in B15.)
**Note:** Thin bead — YAML authoring + path placement, no logic.

### Bead 13 — `cmd/kerfsim/run` command
**Specs:** specs/simulator.md §CLI, §`kerfsim run`
**Package:** `cmd/kerfsim`
**Deliverables:**
- `cmd/kerfsim/main.go` — cobra root, version flag, `embed.FS` over `/scenarios/*.yaml`.
- `cmd/kerfsim/run.go` — `kerfsim run <scenario>` (path OR a built-in canned name like `small-linear`) with flags:
  - `--weights <path>` (optional; defaults from `coordination.md`)
  - `--seed N` (optional; overrides scenario seed)
  - `--runs N` (default 1)
  - `--out <dir>` (default: timestamped under cwd)
  - `--format=json` (also stream summary.json to stdout)
- Wires scenario load → `run.Run` → `output.WriteRun`.
**Tests:**
- CLI smoke against each canned scenario.
- Flag overrides verified (seed override changes the run; `--runs 2` produces 2 sub-directories + an aggregate).

### Bead 14 — `cmd/kerfsim/diff` command
**Specs:** specs/simulator.md §`kerfsim diff`
**Package:** `cmd/kerfsim`
**Deliverables:**
- `kerfsim diff <runA-dir> <runB-dir>` — reads `summary.json` from each side; emits side-by-side metric table with a delta column. Compares the `full` block by default (per spec).
- When either side is a `--runs N` aggregate dir, reports median + p10/p90.
- Does **not** re-execute scenarios.
**Tests:**
- Diff of a run against itself shows zero delta everywhere.
- Diff of two known runs produces expected per-metric deltas.
- Aggregate diff: synthetic 3-run input, expected median/p10/p90 across known values.

### Bead 15 — E2E determinism test (thin)
**Specs:** specs/simulator.md §Determinism, §Testing
**Package:** `cmd/kerfsim` (E2E) or top-level `e2e/`
**Deliverables:**
- Run `kerfsim run small-linear --seed 42 --out /tmp/runX` three times.
- Assert `summary.json` and `events.jsonl` byte-identical across all three invocations.
- Repeat for `wide-fanout` and `rework-heavy`.
- Repeat under `--runs 3` (aggregate must also be byte-identical).
**Tests:** the bead IS the test.
**Note:** Thin bead — pure assertion harness over already-built CLI.

## Parallelization Plan

| Phase | Beads | Parallelizable? | Notes |
|---|---|---|---|
| 1 | B1, B2, B3, B12 | yes — all leaves | B12 needs only B2's schema; can start as soon as B2 lands |
| 2 | B5, B6, B9a | yes — B5 needs B1+B2; B6 needs B2; B9a needs B3 | B9a runs parallel with B5/B6 because it doesn't depend on the loop |
| 3 | B7, B8 | yes — B7 needs B1+B3+B6; B8 needs B1+B6 | |
| 4 | B9b | thin, sequential after B7 + B9a | trivial hook adapter |
| 5 | B10, B11 | partial — B10 needs B5–B9b; B11 needs B9a only | |
| 6 | B13, B14 | yes — independent | B13 needs B10+B11+B12; B14 needs B11 only |
| 7 | B15 | sequential, final | gates the plan |

Critical path: **B1 → B2 → B5 → B10 → B13 → B15** (6 sequential beads). Everything else fits in parallel slack.

## Judgment Calls (resolved)

1. **BeadSource interface placement.** **Resolved:** no extraction in Phase 1. Adapter methods on `store.Store` produce the exact tuple `queue.Compute` accepts. Revisit in Phase 2 if `import`/`sweep` motivates it.
2. **`Policy` interface location.** **Resolved:** new tiny package `internal/sim/policy`. Neutral home so `loop`, `baselines`, and the KerfPolicy in `run` all import a common type without circular deps.
3. **Canned scenarios location.** **Resolved:** top-level `/scenarios/`, embedded via `embed.FS` in `cmd/kerfsim`. Discoverable for users; binary stays self-contained.
4. **`Result` aggregate shape.** Recommendation stands: richer in-memory `Result`, projected to canonical `summary.json` shape at serialization time in B11.

## Spec Gaps — status

All previously-flagged gaps are now pinned in `specs/simulator.md`:

- `mean_ticks` vs `median_ticks` — pinned (Validation Rules: exactly-one + conversion formula).
- `bead_arrivals.explicit` schema — pinned (`tick`, `work`, `labels?`, `bead_id?`; unknown codename = hard error).
- `summary.json` dual `full`/`warmup` view — pinned.
- `queue_snapshot` cadence — pinned (default = every mutating event; configurable).
- `agent_idle_pct` under `warmup_skipped` — pinned (full-run denominator).
- `top_of_queue_churn` first event — pinned (first event sets baseline; not counted as change).
- `area_collisions` re-collision — pinned (re-overlap after separation counts as a new event).
- Generator parameters — pinned (3–6 epics; 0.6 intra; 0.05 inter; log-normal median 12).
